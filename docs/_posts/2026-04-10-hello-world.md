---
layout: post
title: "Hello, fs-inspect"
date: 2026-04-10
---

开坑。目标很小也很具体：做一个 CLI，专门回答"当 FreeSWITCH 跑在不止一台机器上时"你每天都会遇到的那些问题。

## 为什么是现在

过去几个月我在搭一套带自动网关发现的 FreeSWITCH 集群——共享数据库里存 `fs_instance` 表，ESL 驱动路由，一堆 Lua 粘合。它能跑。但运维体验很糟。每次我要回答 *"分机 8001 现在注册在哪台节点上，那台节点还健康吗？"*，我都要手写同一套临时 SQL + `fs_cli` 组合拳。痒点就在这里。

## 它是什么，它不是什么

`fs-inspect` **不是** `fs_cli` 的替代品。`fs_cli` 在它擅长的事情上做得很好：给你一台 FreeSWITCH 进程的命令行。缺口在它上面一层——一旦你有一队 FS，你就需要一个能同时跟所有节点说话、把答案聚合起来的工具。

所以 Roadmap 上前几个命令长这样：

- `fs-inspect reg <ext>` — 这个分机在整个集群里注册在哪？
- `fs-inspect channels` — 跨节点列出所有活跃通道
- `fs-inspect node ls` — 列出已知的 FS 实例清单和健康状态
- `fs-inspect tail` — 把所有节点的事件流合并成一路实时输出

## 今天发布了什么

还没什么实质性功能。今天的 commit 是骨架：一个 Go module，一层围绕 [fiorix/go-eventsocket](https://github.com/fiorix/go-eventsocket) 的 ESL 薄封装，以及一个连到单台 FS 跑 `show channels as json` 的最小 demo。我想让它是"证明从 CLI 到 FS 的管道打通了"的最小闭环。

## 技术选型，简短说

- **Go**，因为一个 CLI 工具需要是一个启动快、能通过 `brew` 或 GitHub Release 分发的单二进制。我的主业是 Java，边学边做 Go 也是这次的小乐趣之一。
- **[bubbletea](https://github.com/charmbracelet/bubbletea)** 处理之后的交互部分。官方 `fs_cli` 是非常 2005 风格的终端体验，我想做个相反的。
- **MIT 协议**，因为这东西的全部意义就是让其他跑 FS 集群的人能直接用。

下一篇大概率会写配置文件格式——`fs-inspect` 怎么知道集群里有哪些节点——因为那是第一个真正的设计决策，而我现在还没想清楚。

---

Starting a thing. The plan is small and specific: a CLI for the day-to-day questions you hit the moment you run FreeSWITCH on more than one box.

## Why now

I've spent the last few months building a FreeSWITCH cluster with auto gateway discovery — `fs_instance` rows in a shared database, ESL-driven routing, a pile of Lua. It works. The operational experience does not. Every time I need to answer *"which node is extension 8001 registered on, and is that node healthy?"* I end up writing the same ad-hoc SQL + `fs_cli` dance by hand. That's the itch.

## What it is, what it isn't

`fs-inspect` is **not** a replacement for `fs_cli`. `fs_cli` is great at what it does: open a shell into one FreeSWITCH process. The gap is above that — once you have a fleet, you need a tool that talks to all of them at once and aggregates the answers.

So the first commands on the roadmap are:

- `fs-inspect reg <ext>` — where is this extension registered, across every node?
- `fs-inspect channels` — every active channel, cluster-wide
- `fs-inspect node ls` — inventory + health of every known FS instance
- `fs-inspect tail` — live event stream merged from all nodes

## What's shipped today

Nothing interesting yet. Today's commit is the scaffold: a Go module, a thin ESL wrapper around [fiorix/go-eventsocket](https://github.com/fiorix/go-eventsocket), and a one-command demo that connects to a single FS and runs `show channels as json`. That's the smallest thing I could build that proves the plumbing works end-to-end.

## Stack choices, briefly

- **Go**, because a CLI needs to be a single binary that starts fast and ships via `brew` or a GitHub release. My day job is Java; learning Go as I go is part of the fun.
- **[bubbletea](https://github.com/charmbracelet/bubbletea)** for the interactive bits, when they arrive. The official `fs_cli` is very much a 2005 terminal experience, and I'd like the opposite.
- **MIT license**, because the whole point is for other people running FS clusters to pick it up.

Next post will probably be about the config file format — how `fs-inspect` learns what nodes exist — because that's the first real design decision, and I haven't made it yet.
