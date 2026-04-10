---
layout: post
title: "配置文件选型和 fan-out 里那个我没用的 errgroup"
date: 2026-04-10 18:00:00 +0800
---

[中文](#zh) · [English](#en)

<a id="zh"></a>

第一篇里我留了一句话——*下一篇大概率会写配置文件格式*——于是这篇就来了。顺便附赠一个并发小话题：为什么 fan-out 那段代码我故意没用 `errgroup`。

## 配置文件：YAML

选型只有四个候选：

1. **YAML** ——人类友好，注释支持好，Go 生态里 `gopkg.in/yaml.v3` 是事实标准
2. **TOML** ——比 YAML 简单，但在运维工具这个品类里，大家的肌肉记忆还是 YAML
3. **JSON** ——不支持注释，写给人看的配置文件不应该用它
4. **从数据库读** ——我的生产环境里 `fs_instance` 表就是节点清单的事实源头

前三个里 YAML 赢得毫无悬念。真正让我犹豫的是第 4 个：既然我手里已经有一张 `fs_instance` 表了，为什么还要再写一个 YAML？

最后没选数据库，理由有两个：

**第一，这是一个开源 CLI**。别人用 `fs-inspect` 的时候，他们的节点未必存在数据库里，就算存了也未必叫 `fs_instance`。一个要求所有人"先按我的表结构建表"的工具，活不过第一个 issue。

**第二，YAML 和数据库不冲突**。将来我完全可以加一个 `fs-inspect nodes sync --from-db` 子命令，从用户自己的数据源拉节点、写进 `fs-inspect.yaml`。这样 YAML 是唯一的"运行时事实源头"，数据库只是众多"生成来源"之一。抽象层次对了。

结构故意做得很扁：

```yaml
nodes:
  - name: fs-01
    addr: 10.0.0.1:8021
    password: ClueCon
```

`name` 纯粹是输出时的标签，没有任何"唯一性"约束——重名也能跑，只是看起来怪。`password` 缺省回落到 `ClueCon`。没有 `enabled: false`，没有 `tags: [prod, edge]`，没有 `weight: 100`，没有 `region: us-west`。当我真需要其中某个字段的时候再加，但绝不提前写。

## 并发 fan-out：为什么没用 errgroup

`cluster.Query` 这个函数要做的事很简单：对清单里每一台 FS 并发跑一个 ESL 命令，拿回结果。Go 里做并发最常见的姿势是 `golang.org/x/sync/errgroup`，写起来像这样：

```go
g, ctx := errgroup.WithContext(ctx)
for _, node := range cfg.Nodes {
    node := node
    g.Go(func() error {
        return query(ctx, node)
    })
}
if err := g.Wait(); err != nil {
    return err
}
```

干净、地道，几乎是 Go 社区里 fan-out 的标准答案。但它有一个我不想要的语义：**任何一个 goroutine 返回 error，它就会把 ctx cancel 掉，其余的兄弟任务连带被取消。**

这对"全或无"的场景是正确的——比如你在做一个需要 N 个下游服务都成功才能继续的请求。但对一个运维工具来说，这恰恰是错的。想象下面这种真实情况：

> 你有 5 台 FS。其中 `fs-03` 正在重启或者压根挂了。你想用 `fs-inspect reg 8001` 查分机在哪——**剩下 4 台上的答案对你有用**。用 `errgroup` 的话，`fs-03` 的 `Dial` 一失败，ctx 被取消，另外 4 台正在路上的查询全部半途毙命，最后你只拿到一个错误。你没信息排障，只知道"集群里有东西坏了"，这谁都知道。

所以 `cluster.Query` 用了最原始的 `sync.WaitGroup`：每个 goroutine 独立跑完，成功或失败都把结果写进它自己那格的 slice，主协程 `Wait` 完拿到一个 `[]Result`。调用方挨个看每一格，决定某台节点的错误算"部分降级"还是"整体失败"。

```go
func Query(cfg *config.Config, apiCmd string) []Result {
    results := make([]Result, len(cfg.Nodes))
    var wg sync.WaitGroup
    for i, node := range cfg.Nodes {
        wg.Add(1)
        go func(i int, node config.Node) {
            defer wg.Done()
            // ... dial, query, write results[i]
        }(i, node)
    }
    wg.Wait()
    return results
}
```

不优雅，但**语义是对的**。这是写运维工具和写请求处理链路的一个本质差别——运维工具里"部分可用"是一等公民，不是需要特判的边缘情况。

等以后加 `-timeout` 的时候我会引 `context.WithTimeout`，但**取消的粒度是"每个节点自己那份任务的 ctx"**，不是"所有节点共享一个 ctx"。这一点我会在以后那篇 timeout 的帖子里再写。

## 今天 ship 的

`feat: add reg subcommand for cluster-wide extension lookup` —— 这篇博客说的所有东西都在那个 commit 里。

下一步：实现 `channels` 命令（复用同一个 fan-out 框架），加一条 GitHub Actions CI，把真机跑一次的截图贴进 README。

---

<a id="en"></a>

In the last post I left a breadcrumb — *next post will probably be about the config file format* — so here we are. Plus a bonus concurrency side note: why the fan-out code is deliberately not using `errgroup`.

## Config file: YAML

There were really only four candidates:

1. **YAML** — human-friendly, great comment support, `gopkg.in/yaml.v3` is the de-facto standard in Go
2. **TOML** — simpler than YAML, but in the ops-tooling space everyone's muscle memory is still YAML
3. **JSON** — no comments; you do not ship human-edited config as JSON in 2026
4. **Read from a database** — my own production `fs_instance` table *is* the source of truth for the node list

YAML wins the first three trivially. What actually made me hesitate was option 4: I already have that table. Why am I writing a second source of truth on top of it?

I didn't pick the database for two reasons:

**First, this is an open-source CLI.** Other people using `fs-inspect` won't necessarily have their nodes in a database, and if they do it won't be called `fs_instance` with my schema. A tool that demands *"first create this table"* dies at issue #1.

**Second, YAML and a database aren't mutually exclusive.** I can absolutely add `fs-inspect nodes sync --from-db` later that pulls from whatever the user's data source is and writes `fs-inspect.yaml`. That makes YAML the single runtime source of truth and the database one of many possible *generators*. Abstraction layered at the right seam.

The schema is aggressively flat:

```yaml
nodes:
  - name: fs-01
    addr: 10.0.0.1:8021
    password: ClueCon
```

`name` is just an output label — no uniqueness constraint, dupes work fine, they just look weird. `password` defaults to `ClueCon` if omitted. No `enabled: false`, no `tags: [prod, edge]`, no `weight: 100`, no `region: us-west`. I'll add any of those the moment I need them. Not before.

## Concurrent fan-out: why no errgroup

`cluster.Query` has a simple job: run one ESL command against every FS in the inventory in parallel, return the results. The idiomatic Go answer is `golang.org/x/sync/errgroup`:

```go
g, ctx := errgroup.WithContext(ctx)
for _, node := range cfg.Nodes {
    node := node
    g.Go(func() error {
        return query(ctx, node)
    })
}
if err := g.Wait(); err != nil {
    return err
}
```

Clean, idiomatic, basically the community reference answer for fan-out. But it has one semantic I don't want: **the moment any goroutine returns an error, errgroup cancels its ctx and all the sibling tasks get cancelled along with it.**

That's correct for all-or-nothing fan-outs — say you're running N downstream calls and the request can only succeed if all of them do. For an ops tool it's exactly wrong. Picture the real scenario:

> You have 5 FS boxes. `fs-03` is currently rebooting or just plain down. You run `fs-inspect reg 8001` to find where an extension is registered — and **you want the answers from the other 4 nodes**. With errgroup, `fs-03`'s `Dial` failure cancels ctx, the four in-flight queries die mid-call, and you end up with one error and zero information. You don't get to debug anything; you just learn "something in the cluster is broken", which you already knew.

So `cluster.Query` uses plain `sync.WaitGroup`: each goroutine runs to its own conclusion — success or failure — and writes its result into its own slot of the result slice. The main goroutine `Wait`s and hands back a `[]Result`. Callers walk the slice and decide, per node, whether a failure is a "partial degradation" or a "full outage".

```go
func Query(cfg *config.Config, apiCmd string) []Result {
    results := make([]Result, len(cfg.Nodes))
    var wg sync.WaitGroup
    for i, node := range cfg.Nodes {
        wg.Add(1)
        go func(i int, node config.Node) {
            defer wg.Done()
            // ... dial, query, write results[i]
        }(i, node)
    }
    wg.Wait()
    return results
}
```

Not elegant. But the **semantics are right**. That's a real difference between ops tooling and request-path code: in an ops tool, "partially available" is a first-class state, not an edge case you special-case around.

When I add `-timeout` later I'll use `context.WithTimeout`, but **the cancellation scope will be per-node, not shared across all nodes**. I'll write that one up when the flag ships.

## What shipped today

`feat: add reg subcommand for cluster-wide extension lookup` — everything in this post lives in that single commit.

Next up: the `channels` command (same fan-out framework, second user of it), a GitHub Actions CI, and a real-run screenshot in the README.
