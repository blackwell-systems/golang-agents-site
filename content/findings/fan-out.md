---
draft: true
title: Four agents at once finished no faster than one and cost 25 to 60 percent more
short: Fan-out, measured
description: Chapter 9 triaged a batch of 99 crash reports two ways, with one agent per cause running four at a time and with a single agent over the whole batch. The fan-out took about as long and cost more, though its cost was steadier and its requests stayed small.
chapters: 9
stage: all
date: 2026-10-18
weight: 40
art: ch09
---

```result
figure: 25 to 60%
unit: more cost, no faster
task: triage a batch of 99 crash reports from 23 causes, as one agent and as one agent per cause
models: `anthropic/claude-haiku-5.5`
runs: two or three per design, on a shared machine
date: October 2026
source: Chapter 9

Twenty-three agents, four at a time, took about as long as one agent working through the same 99 crash reports, and cost $0.124 per run against the single agent’s $0.077 to $0.100.
```

Splitting a batch across several agents that run at the same time, called [fan-out](/glossary/#fan-out), sounds like the obvious way to get through it sooner.
In Go it is also little code: start a goroutine for each agent, give them one budget to share, and collect their events on a channel.
Chapter 9 of the book built exactly that and raced it against a single agent.

## What was tested

The batch is 99 crash reports from `remind`, the small Go web service the book's agent works on.
Grouped by fingerprint, the reports come from 23 causes.

**Fan-out:** the harness, not the model, decides to start one agent per group.
Every agent patches its own copy of the code, so they never edit the same files, and all of them spend from one shared pool of money.
When they finish, the harness merges their fixes into a single tree, with a three-way merge wherever two groups touched the same file.

**Single agent:** one agent takes the whole batch, using chapter 8's context policy (prompt caching, plus clearing or summarizing old turns).

Both ran on the default model, `anthropic/claude-haiku-5.5`, in October 2026.
The machine was shared with other work, and two runs of the same setup could differ by as much as four minutes.

## The result

| Runs | Wall time | Model calls | Largest request (tokens) | Cost | Checks passed (of 94) |
| --- | --- | --- | --- | --- | --- |
| One agent, two runs | 8m45s; 6m20s | 38; 52 | 103,581; 98,891 | $0.100; $0.077 | 86; 86 |
| One agent, chapter 8's program | 7m02s | 52 | 96,275 | $0.087 | 84 |
| One agent per group, four at a time | 9m20s; 7m39s | 263; 251 | 28,275; 27,199 | $0.124; $0.124 | 82; 81 |
| One agent per group, one at a time | 22m10s; 18m25s | 255; 265 | 24,992; 25,835 | $0.124; $0.126 | 85; 81 |

Running the group agents four at a time instead of one at a time made the batch about 2.4 times quicker and cost the same, which makes sense: the same 23 investigations happen either way.
Against a single agent, though, four at a time finished in about the same time and spent roughly 25 to 60 percent more.

## Why

Count the model calls first.
One agent got through the batch in 38 to 52 calls.
The 23 group agents needed about 250 to 265 in total.
A group agent knows nothing when it starts, so each one repeats the same opening: list its reports, read the code on the crash's path, write a test, run the tests.
The single agent did that reading once, and what it had read was enough to fix several groups.

The tools did more work too.
Across a batch, the group agents ran the test suite 46 to 58 times, where a single agent ran it 2 to 4 times.
Every group agent drew from the same two test slots, so four agents running together often queued for them; that queueing is one reason the speedup over one at a time stopped well short of four.

The extra spending went to two things a single agent does less of.
Each of the 23 agents wrote its own cached prefix, about 400,000 tokens of cache writes in all, against 170,000 to 260,000 for one agent.
The group agents also produced more output: roughly 95,000 tokens, where one agent produced 55,000 to 62,000.
Total input tokens were close, 2.5 to 3.2 million in both designs, since the cache served most of each request.

## What fan-out did buy

**A steady cost.** Four fan-out runs cost between $0.124 and $0.126.
With the same model and policy, single-agent runs across chapters 8 and 9 cost anywhere from $0.06 to $0.42.

**Small requests.** No group agent sent a request larger than 28,275 tokens.
Each single-agent run came within 4 percent of the model's 100,000-token price tier at least once, and one run crossed it; past that tier, the whole request is billed at a higher rate.

**Isolated failures.** If one group's agent fails or runs out of turns, only that group is affected, and every group's fix is checked by itself before the merge.

## What it can't show

Each row is two or three runs on one model, on a shared machine.
The times overlap, so "about as long" is the fair reading; calling fan-out slower would go beyond the data.
Checks passed shift by a few points between runs of one setup, so that column doesn't rank the designs.
It is also a single batch, one that a lone agent finished in 30 to 62 turns in the book's runs.

## When fan-out pays

Chapter 9 names three cases where fan-out comes out ahead:

- **Large groups**, where every group takes many turns by itself and the reading the agents have in common is a small part of their work.
- **Requests one agent couldn't keep small**: a job where a single agent's requests would grow past what chapter 8's context policy can hold down.
- **Far more than four agents running at once.**

For a batch like this one, a single agent with a good context policy matched fan-out's time and cost less per run, while fan-out gave the more predictable bill.

## The shared budget

One part of the fan-out design is useful by itself: a pool of money every agent spends from, in [`ch09/agent/spend.go`](code:ch09/agent/spend.go).

```go include=ch09/agent/spend.go tag=spend
```

Handing each agent an equal slice up front wastes money.
An easy group, say one whose crash is already filed, wraps up in three turns with most of its slice unused, and meanwhile the hardest group runs dry.
With a shared pool, what the quick groups don't spend is there for the slow ones.
The pool checks the balance and records the spending as separate steps, so it can go over by at most one call for each agent running at that moment, and a test holds it to that limit.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), with no key set:

```console
$ go run ./ch09 -task g3-mixed-batch -groups -parallel 4
```

This replays a smaller recorded batch with five group agents.
Their calls appear interleaved, each line tagged with its group, followed by the merge, the filed issues, and the crash catalog's grade.
