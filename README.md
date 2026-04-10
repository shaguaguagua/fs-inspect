# fs-inspect

[中文](#中文) · [English](#english)

---

## 中文

面向**多实例 FreeSWITCH 集群**的现代化运维 CLI。

`fs-inspect` 不是官方 `fs_cli` 的替代品。`fs_cli` 一次连一台 FS 执行命令；`fs-inspect` 站在一整个 FS 集群的上层，并行通过 ESL 查询每个实例并聚合结果，回答这一类问题：*"分机 8001 现在注册在哪台 FS 上？"*、*"把所有节点上的活跃通话都列出来"*、*"哪一台在漏 channel？"*。

### 项目状态

早期开发阶段。进展记录见 [Build Log 博客](https://shaguaguagua.github.io/fs-inspect/)。

### 为什么做

单机 FreeSWITCH 的运维问题早就被解决了。但**横向扩展**为带自动网关发现的集群（`fs_instance` 表 + ESL + Lua 路由）之后，配套的运维工具链就停在了 `fs_cli`——节点一多，日常排障问题就变得非常难回答。

`fs-inspect` 就是我自己搭这种集群时，想要却没有的工具。

### 规划中的命令

```
fs-inspect reg 8001              # 这个分机现在注册在哪台？
fs-inspect channels              # 跨节点列出所有活跃通道
fs-inspect node ls               # 列出已知 FS 实例 + 健康状态
fs-inspect tail                  # 跨节点实时合并事件流
```

### 技术栈

- Go
- [fiorix/go-eventsocket](https://github.com/fiorix/go-eventsocket) 处理 ESL
- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) 处理 TUI

### 协议

MIT

---

## English

A modern CLI for inspecting and operating **multi-instance FreeSWITCH clusters**.

`fs-inspect` is not a replacement for the official `fs_cli`. `fs_cli` connects to a single FS and runs commands against it. `fs-inspect` sits above a fleet: it answers questions like *"which FS is extension 8001 registered on right now?"*, *"show me every active call across the cluster"*, and *"which node is leaking channels?"* — by querying ESL across every instance in parallel and aggregating the result.

### Status

Early development. See the [build log](https://shaguaguagua.github.io/fs-inspect/) for progress notes.

### Why

Running FreeSWITCH in a single-node topology is solved. Running it as a horizontally-scaled cluster with auto gateway discovery (`fs_instance` + ESL + Lua routing) is not — the operational tooling stops at `fs_cli`, and once you have more than two nodes the day-to-day ops questions become annoying to answer.

`fs-inspect` is the tool I wanted while building exactly that kind of cluster.

### Planned commands

```
fs-inspect reg 8001              # where is this extension registered?
fs-inspect channels              # active channels across all nodes
fs-inspect node ls               # list known FS instances + health
fs-inspect tail                  # live-tail events cluster-wide
```

### Stack

- Go
- [fiorix/go-eventsocket](https://github.com/fiorix/go-eventsocket) for ESL
- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) for TUI

### License

MIT
