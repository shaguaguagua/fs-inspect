# fs-inspect

A modern CLI for inspecting and operating **multi-instance FreeSWITCH clusters**.

`fs-inspect` is not a replacement for the official `fs_cli`. `fs_cli` connects to a single FS and runs commands against it. `fs-inspect` sits above a fleet: it answers questions like *"which FS is extension 8001 registered on right now?"*, *"show me every active call across the cluster"*, and *"which node is leaking channels?"* — by querying ESL across every instance in parallel and aggregating the result.

## Status

Early development. See the [build-in-public blog](#) for progress notes.

## Why

Running FreeSWITCH in a single-node topology is solved. Running it as a horizontally-scaled cluster with auto gateway discovery (`fs_instance` + ESL + Lua routing) is not — the operational tooling stops at `fs_cli`, and once you have more than two nodes the day-to-day ops questions become annoying to answer.

`fs-inspect` is the tool I wanted while building exactly that kind of cluster.

## Planned commands

```
fs-inspect reg 8001              # where is this extension registered?
fs-inspect channels              # active channels across all nodes
fs-inspect node ls               # list known FS instances + health
fs-inspect tail                  # live-tail events cluster-wide
```

## Stack

- Go
- [fiorix/go-eventsocket](https://github.com/fiorix/go-eventsocket) for ESL
- [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) for TUI

## License

MIT
