# fs-inspect

[中文](#中文) · [English](#english)

---

## 中文

面向**多实例 FreeSWITCH 集群**的现代化运维 CLI。

`fs-inspect` 不是官方 `fs_cli` 的替代品。`fs_cli` 一次连一台 FS 执行命令；`fs-inspect` 站在一整个 FS 集群的上层，并行通过 ESL 查询每个实例并聚合结果，回答这一类问题：*"分机 8001 现在注册在哪台 FS 上？"*、*"把所有节点上的活跃通话都列出来"*、*"哪一台在漏 channel？"*。

### Demo

真机运行输出（单节点实验集群，FreeSWITCH 1.10.12）：

```
$ fs-inspect channels
NODE         STATE      CALLER           CALLEE          DUR       UUID
──────────────────────────────────────────────────────────────────────────────

0 active channel(s) across 1 node(s)

$ fs-inspect reg 1010
✓ fs-test      127.0.0.1:8021        user=1010  contact=192.168.65.1:32249  (1ms)

$ fs-inspect reg 1012
✓ fs-test      127.0.0.1:8021        user=1012  contact=192.168.65.1:51826  (2ms)

$ fs-inspect reg 9999
extension 9999 not registered on any known node
```

`tail` 子命令跨节点合并实时 ESL 事件流（下面是真机抓到的一次 loopback 测试呼叫，颜色在真终端里是绿/黄/红，重定向时自动降级为纯文本）：

```
$ fs-inspect tail
› tailing 1 node(s) for events: CHANNEL_CREATE CHANNEL_ANSWER CHANNEL_HANGUP_COMPLETE
› press Ctrl+C to exit

17:22:42 fs-test      CHANNEL_CREATE            0000000000 → 9999              9a9f9508
17:22:42 fs-test      CHANNEL_CREATE            0000000000 → 9999              ea2fbafc
17:22:53 fs-test      CHANNEL_HANGUP_COMPLETE   0000000000 → 9999              ea2fbafc
17:22:53 fs-test      CHANNEL_HANGUP_COMPLETE   0000000000 → 9999              9a9f9508
```

多节点环境下每一行前面的 `NODE` 列会混合多台不同的 FS 名称——这正是 `fs-inspect` 存在的意义。

### 项目状态

早期开发阶段。进展记录见 [Build Log 博客](https://shaguaguagua.github.io/fs-inspect/)。

### 为什么做

单机 FreeSWITCH 的运维问题早就被解决了。但**横向扩展**为带自动网关发现的集群（`fs_instance` 表 + ESL + Lua 路由）之后，配套的运维工具链就停在了 `fs_cli`——节点一多，日常排障问题就变得非常难回答。

`fs-inspect` 就是我自己搭这种集群时，想要却没有的工具。

### 命令

```
fs-inspect reg 8001              # 这个分机现在注册在哪台？           ✓ 已实现
fs-inspect channels              # 跨节点列出所有活跃通道              ✓ 已实现
fs-inspect tail                  # 跨节点实时合并 ESL 事件流            ✓ 已实现
fs-inspect shell                 # bubbletea 交互式多节点 shell        ✓ 已实现
fs-inspect probe                 # 单节点 ESL 调试（JSON 高亮）        ✓ 已实现
fs-inspect node ls               # 列出已知 FS 实例 + 健康状态          Roadmap
```

所有命令的输出都走 ANSI 彩色，支持 `NO_COLOR` 环境变量，管道/重定向时自动降级到纯文本。

### 技术栈

- Go
- [fiorix/go-eventsocket](https://github.com/fiorix/go-eventsocket) —— ESL 客户端
- [gopkg.in/yaml.v3](https://gopkg.in/yaml.v3) —— 配置文件解析
- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) + [bubbles](https://github.com/charmbracelet/bubbles) + [lipgloss](https://github.com/charmbracelet/lipgloss) —— 交互式 shell
- [alecthomas/chroma/v2](https://github.com/alecthomas/chroma) —— JSON 语法高亮
- [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) —— TTY 检测

### 协议

MIT

---

## English

A modern CLI for inspecting and operating **multi-instance FreeSWITCH clusters**.

`fs-inspect` is not a replacement for the official `fs_cli`. `fs_cli` connects to a single FS and runs commands against it. `fs-inspect` sits above a fleet: it answers questions like *"which FS is extension 8001 registered on right now?"*, *"show me every active call across the cluster"*, and *"which node is leaking channels?"* — by querying ESL across every instance in parallel and aggregating the result.

### Demo

Real output from a live run (single-node lab cluster, FreeSWITCH 1.10.12):

```
$ fs-inspect channels
NODE         STATE      CALLER           CALLEE          DUR       UUID
──────────────────────────────────────────────────────────────────────────────

0 active channel(s) across 1 node(s)

$ fs-inspect reg 1010
✓ fs-test      127.0.0.1:8021        user=1010  contact=192.168.65.1:32249  (1ms)

$ fs-inspect reg 1012
✓ fs-test      127.0.0.1:8021        user=1012  contact=192.168.65.1:51826  (2ms)

$ fs-inspect reg 9999
extension 9999 not registered on any known node
```

The `tail` subcommand merges live ESL event streams across every node (real-run output from a loopback test call below; colors in a real terminal are green/yellow/red, auto-stripped when redirected):

```
$ fs-inspect tail
› tailing 1 node(s) for events: CHANNEL_CREATE CHANNEL_ANSWER CHANNEL_HANGUP_COMPLETE
› press Ctrl+C to exit

17:22:42 fs-test      CHANNEL_CREATE            0000000000 → 9999              9a9f9508
17:22:42 fs-test      CHANNEL_CREATE            0000000000 → 9999              ea2fbafc
17:22:53 fs-test      CHANNEL_HANGUP_COMPLETE   0000000000 → 9999              ea2fbafc
17:22:53 fs-test      CHANNEL_HANGUP_COMPLETE   0000000000 → 9999              9a9f9508
```

On a multi-node deployment the `NODE` column fills with different FS names — which is the whole point of this tool existing.

### Status

Early development. See the [build log](https://shaguaguagua.github.io/fs-inspect/) for progress notes.

### Why

Running FreeSWITCH in a single-node topology is solved. Running it as a horizontally-scaled cluster with auto gateway discovery (`fs_instance` + ESL + Lua routing) is not — the operational tooling stops at `fs_cli`, and once you have more than two nodes the day-to-day ops questions become annoying to answer.

`fs-inspect` is the tool I wanted while building exactly that kind of cluster.

### Commands

```
fs-inspect reg 8001              # where is this extension registered?     ✓ shipped
fs-inspect channels              # active channels across all nodes        ✓ shipped
fs-inspect tail                  # merged live ESL event stream            ✓ shipped
fs-inspect shell                 # interactive multi-node bubbletea shell  ✓ shipped
fs-inspect probe                 # single-node ESL debug (JSON highlight)  ✓ shipped
fs-inspect node ls               # list known FS instances + health        roadmap
```

All output is ANSI-colorized, respects `NO_COLOR`, and auto-degrades to plain text when piped or redirected.

### Stack

- Go
- [fiorix/go-eventsocket](https://github.com/fiorix/go-eventsocket) — ESL client
- [gopkg.in/yaml.v3](https://gopkg.in/yaml.v3) — config file parsing
- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) + [bubbles](https://github.com/charmbracelet/bubbles) + [lipgloss](https://github.com/charmbracelet/lipgloss) — interactive shell
- [alecthomas/chroma/v2](https://github.com/alecthomas/chroma) — JSON syntax highlighting
- [golang.org/x/term](https://pkg.go.dev/golang.org/x/term) — TTY detection

### License

MIT
