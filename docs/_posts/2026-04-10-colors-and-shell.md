---
layout: post
title: "颜值补课：颜色、JSON 高亮、bubbletea 交互 shell，外加一场 Docker ACL 翻车"
date: 2026-04-10 21:00:00 +0800
---

[中文](#zh) · [English](#en)

<a id="zh"></a>

第一篇立项的时候我写过这么一句——*我想做个和 fs_cli 相反的现代终端体验*。但前两篇博客和前几个 commit 全部在讲"能不能用"的骨架：配置、fan-out、子命令派发、CI、真机 demo。**"颜值"那部分一个字没做。** 直到用户（也就是我自己）回头来问我："你一开始说的高亮 JSON 格式化之类的彩色啥的 都实现了吗？"

诚实答：**没有。** 这篇补课的帖子就是这个问题的答案。

## 这一轮 ship 的四件事

1. **彩色输出 + TTY 检测**——`internal/display` 包封装了一组 ANSI 颜色 helper（Red / Green / Cyan / Yellow / Gray / Bold / Dim），以及"走 TTY 才着色"的开关。尊重 [NO_COLOR](https://no-color.org) 环境变量。管道输出、CI 日志、重定向到文件，都会自动降级成纯文本——这不是 bonus feature，是 CLI 工具的最低礼仪。
2. **JSON 格式化 + 语法高亮**——`probe` 现在会先用 `encoding/json` 的 `Indent` 把 FS 返回的一行 JSON 展开成嵌套好的格式，再走 [alecthomas/chroma](https://github.com/alecthomas/chroma) 的 `json` lexer + `terminal256` formatter 做 token-level 高亮。碰上纯文本响应（比如 `status`、`uptime`）就原样输出，不会崩。
3. **reg / channels 上色**——`✓` 绿、`✗` 红、节点名青色、延迟灰色、状态按 FS 的 channel state 分别绿/黄/红。
4. **`fs-inspect shell`** ——**第一版交互式多节点 shell**。基于 [bubbletea](https://github.com/charmbracelet/bubbletea) + [bubbles](https://github.com/charmbracelet/bubbles) + [lipgloss](https://github.com/charmbracelet/lipgloss)。支持命令历史（↑/↓）、标题栏、viewport 滚屏、内建命令 `reg` / `channels` / `probe <node> <cmd>` / `nodes` / `help` / `quit`。

## 关键取舍

**一条：render 函数只写一份。** 一开始我差点给 shell 写一套专门的输出逻辑，后来意识到那是慢性分叉——CLI 模式和 shell 模式长期会不一样。现在 `RenderReg` / `RenderChannels` 是 `cmd/fs-inspect/main.go` 里的纯函数，返回 `string`；shell 直接调同一份。改一处，两边都受益。

**两条：shell 里强制开颜色。** 按 TTY 检测逻辑，shell 的输出是写进 bubbletea 的 viewport，不是 process stdout，`term.IsTerminal(os.Stdout.Fd())` 会返回 false，结果就是 shell 里的 reg 结果会是灰扑扑的一堆字。解法是 shell 启动时调 `display.ForceColor(true)` 把判断覆盖掉。不优雅，但对。

**三条：chroma 的 fallback 要兜到两层。** `lexers.Get("json")` 可能返回 nil、`styles.Get("monokai")` 可能返回 nil、`formatter.Format` 可能返回错误——任何一层失败都不能让 probe 命令崩。三个 nil-check 都指向"原样返回未高亮字符串"。碰到问题最多是少看到颜色，绝不会少看到数据。

## 那场 Docker ACL 翻车

这一轮还顺手踩了一个和代码没关系但值得记下来的坑：从 Mac 宿主机直连 fs-test 容器的 ESL。

FS 默认的 `event_socket.conf.xml` 里 `apply-inbound-acl="loopback.auto"`，这在裸机 FS 上是对的：只信任 127.0.0.0/8。但在 Docker 里，任何从容器外进来的连接从容器视角看都不是 loopback——是 Docker bridge 的 gateway IP。于是我试了几条路：

1. **改成 `rfc1918.auto`**。断了容器内部 fs_cli 自己连自己（因为它走 loopback，loopback 不属于 RFC1918）。❌
2. **改成精确的 `192.168.65.0/24`**（Docker Desktop for Mac 的转发段）。这个在我第一次测试的时候是对的。但紧接着 FS 又开始拒我，日志显示源 IP 变成了 `147.182.188.245` ——一个公网 IP。查下来才发现：**我 Mac 上开了 VPN 隧道**（utun5）。Docker Desktop for Mac 的网络栈在某些条件下会把 host→container 的转发路由到 VPN 出口，然后从 VPN 出口的公网 IP "绕一圈" 回到 container。这意味着 ACL 的"源 IP 白名单"方案在 Mac + VPN 环境下**根本不稳定**——今天这个 IP，明天换个机场节点就是另一个。❌
3. **`default="allow"` + 继续靠 ESL 密码**。承认"在 lab 环境里 IP 层过滤不是有意义的安全边界"，把责任全部交给密码。✅

有一个额外的坑：**改 ACL 的 reload 顺序不能反**。

```bash
# 正确
fs_cli -x "reloadacl"
fs_cli -x "reload mod_event_socket"

# 错误：mod_event_socket 加载时找不到 ACL 名 → fallback 到 deny-all
fs_cli -x "reload mod_event_socket"
fs_cli -x "reloadacl"
```

我反过来跑了一次，结果连 fs_cli 自己都连不上自己（因为 deny-all 把 loopback 也挡了），只能重启容器救回来。

## 下一步

`fs-inspect shell` 的第一版只是"基础可用"：固定高度的 viewport、简单的命令派发、没有自动补全、没有 tab 键帮助、没有把多行命令支持好。这些都是常规打磨工作，写进 issues 等以后迭代。

更想做的两件：

- **`fs-inspect tail`** —— 跨节点实时合并 FS 事件流，用 bubbletea 做真正的 live UI（不停更新的事件列表 + 过滤栏）。这是这个工具最像"现代 fs_cli"的那张脸。
- **asciinema 录屏**——把 `shell` 真实运行的录像嵌到 README 里。比静态代码块震撼一百倍。

下一篇博客大概率写其中之一。

---

<a id="en"></a>

Way back in post one I wrote a line — *I want the opposite of the fs_cli terminal experience*. But the first two posts and every commit since have been about whether the thing *functions*: config, fan-out, subcommand dispatch, CI, a real-machine demo. **None of the "feel modern" work had happened.** Until the user — me — came back and asked: *"All that highlighting and JSON formatting and color stuff you promised at the start... did you actually build any of it?"*

Honest answer: **no.** This post is the remedial work.

## Four things that shipped this round

1. **Colorized output + TTY detection.** `internal/display` is a small package of ANSI color helpers (Red / Green / Cyan / Yellow / Gray / Bold / Dim) gated by an "are we a TTY" check. [NO_COLOR](https://no-color.org) is respected. Piping, CI logs, redirecting to a file — everything degrades to plain text automatically. This isn't a bonus feature, it's table stakes for a CLI.
2. **JSON pretty-print + syntax highlighting.** `probe` now runs FS's one-line JSON response through `encoding/json.Indent` first, then through [alecthomas/chroma](https://github.com/alecthomas/chroma)'s `json` lexer + `terminal256` formatter for token-level highlighting. Plain-text responses (`status`, `uptime`) pass through unchanged.
3. **Colorized reg / channels.** Green `✓`, red `✗`, cyan node names, gray latencies, channel states colored by FS state (green/yellow/red).
4. **`fs-inspect shell`** — the first cut of the **interactive multi-node shell**. Built on [bubbletea](https://github.com/charmbracelet/bubbletea) + [bubbles](https://github.com/charmbracelet/bubbles) + [lipgloss](https://github.com/charmbracelet/lipgloss). Command history (↑/↓), a header line, a scrolling viewport, and built-ins `reg` / `channels` / `probe <node> <cmd>` / `nodes` / `help` / `quit`.

## Trade-offs worth recording

**One: write the render functions exactly once.** My first instinct was to give the shell its own output code. That's slow-motion divergence — two weeks from now the CLI mode and the shell mode would drift. `RenderReg` and `RenderChannels` are now pure functions in `cmd/fs-inspect/main.go` that return a `string`; the shell calls the same ones. Fix in one place, fix everywhere.

**Two: the shell forces colors on.** The TTY-detection logic says "is `os.Stdout` a terminal?" — in the shell, output goes into a bubbletea viewport, not to process stdout, so the check returns false and you get grey mush. The fix is a `display.ForceColor(true)` at shell boot. Not elegant, but correct.

**Three: chroma's fallback chain is belt-and-braces.** `lexers.Get("json")` can return nil, `styles.Get("monokai")` can return nil, `formatter.Format` can return an error. Any one of them failing must not crash probe. Three nil-checks, all pointing at "return the unhighlighted string". Worst case you lose colors, never data.

## That Docker ACL saga

This round also bought me an unrelated-to-code war story that's worth writing down: making the Mac host reach the fs-test container's ESL.

FS ships with `apply-inbound-acl="loopback.auto"` in `event_socket.conf.xml`. That's correct for bare-metal FS — only trust 127.0.0.0/8. In Docker, connections from outside the container don't *look* like loopback from the container's perspective, they look like Docker's bridge gateway. So I tried three things:

1. **Switch to `rfc1918.auto`.** Broke in-container `fs_cli` connecting to itself via loopback, because loopback isn't RFC1918. ❌
2. **Narrow to `192.168.65.0/24`** — Docker Desktop for Mac's forwarding subnet. Worked on the first test. Then FS started rejecting me again, log showed source IP `147.182.188.245` — a public IP. Investigation: **my Mac had an active VPN tunnel (utun5).** Docker Desktop for Mac's network stack sometimes routes host→container forwards through the default route, which is the VPN, and the container sees the VPN exit's public IP. Meaning: **any CIDR whitelist on a Mac with a VPN is fundamentally unstable.** Today it's one IP; tomorrow you switch VPN regions and it's another. ❌
3. **`default="allow"` + still require the ESL password.** Acknowledge that IP-level filtering isn't a meaningful security boundary in a lab, and let the password do its job. ✅

And a bonus gotcha: **reload order matters**.

```bash
# Correct
fs_cli -x "reloadacl"
fs_cli -x "reload mod_event_socket"

# Wrong: mod_event_socket reloads with an ACL name the ACL subsystem
# doesn't know about yet, falls back to deny-all, locks you out.
fs_cli -x "reload mod_event_socket"
fs_cli -x "reloadacl"
```

I ran them in the wrong order once and ended up with a FreeSWITCH where `fs_cli` couldn't even connect to itself — the deny-all fallback blocked loopback too. Recovery was a `docker restart`.

## Next up

This first cut of `fs-inspect shell` is "basically usable" territory: fixed-height viewport, simple dispatch, no tab completion, no multi-line commands. All normal-polish work, filed for later.

The two I actually want next:

- **`fs-inspect tail`** — a merged event stream across every node, presented as a real live bubbletea UI (continuously updating list + filter bar). This is the thing that will most make fs-inspect feel like a modern fs_cli.
- **asciinema recording** embedded in the README. A hundred times more convincing than a static code block.

Next post will probably be one of those two.
