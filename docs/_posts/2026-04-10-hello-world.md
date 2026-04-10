---
layout: post
title: "Hello, fs-inspect"
date: 2026-04-10
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
