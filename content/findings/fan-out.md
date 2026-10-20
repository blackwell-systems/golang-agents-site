---
title: "Fan-out on one shared codebase: four agents finished no faster than one and cost 25 to 60 percent more"
short: Fan-out, measured
description: Chapter 9 triaged a batch of 99 crash reports two ways, with one agent per cause running four at a time and with a single agent over the whole batch. The fan-out took about as long and cost more, though its cost was steadier and its requests stayed small.
card: One agent per cause, four at a time, against one agent for 99 reports: no faster, and it cost more.
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
In Go it also takes little code: start a goroutine for each agent, give them one budget to share, and collect their events on a channel.
The measurements below race that design against a single agent.

## What was tested

The batch is 99 crash reports from `remind`, the small Go web service in the companion code.
Grouped by [fingerprint](/glossary/#fingerprint), a short identifier meant to be shared by crashes with one cause, the reports come from 23 causes.

**Fan-out:** the harness, not the model, decides to start one agent per group.
Every agent patches its own copy of the code, so no two agents write to the same working tree, and all of them spend from one shared pool of money.
When they finish, the harness merges their fixes into a single tree, with a three-way merge wherever two groups touched the same file.

**Single agent:** one agent takes the whole batch, using a [context policy](/build/context-management/) of prompt caching, plus clearing or summarizing old turns.

Both ran on the default model, `anthropic/claude-haiku-5.5`, in October 2026.
The machine was shared with other work, and two runs of the same setup could differ by as much as four minutes.

```diagram
name: ch09-fanout
alt: The run's context at the top; below it one agent per fingerprint group, each with its own context and workspace, sharing the model's pace, the spending pool, and the test slots; inside each agent the stream's idle timer and run_tests' time limit; at the bottom the harness merges the workspaces, joins diagnoses with one root cause, files the issues, and grades.

One agent per crash group, each with its own context and workspace, merged by the harness.
```

## The result

The last column counts the crash catalog's checks on this batch, 94 in all: for each of the 23 causes, whether it was found and its status and root cause were right, plus checks on the run as a whole, such as one issue per new cause.

| Runs | Wall time | Model calls | Largest request (tokens) | Cost | Checks passed (of 94) |
| --- | --- | --- | --- | --- | --- |
| One agent, two runs | 8m45s; 6m20s | 38; 52 | 103,581; 98,891 | $0.100; $0.077 | 86; 86 |
| One agent, the context-policy program | 7m02s | 52 | 96,275 | $0.087 | 84 |
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
Every group agent drew from the same two test slots, a limit of two `go test` runs at a time across all agents, so four agents running together often queued for them; that queueing is one reason the speedup over one at a time stopped well short of four.

The extra spending went to two things a single agent does less of.
Each of the 23 agents wrote its own cached prefix, about 400,000 tokens of cache writes in all, against 170,000 to 260,000 for one agent.
The group agents also produced more output: roughly 95,000 tokens, where one agent produced 55,000 to 62,000.
Total input tokens, counting those the cache served, were close, 2.5 to 3.2 million in both designs: the group agents sent five times as many requests, but each was a fraction of the size.

## What fan-out did buy

**A steady cost.** Four fan-out runs cost between $0.124 and $0.126.
With the same model and policy, single-agent runs recorded across these guides cost anywhere from $0.06 to $0.42.

**Small requests.** No group agent sent a request larger than 28,275 tokens.
Each single-agent run came within 4 percent of the model's 100,000-token price tier at least once, and one run crossed it; past that tier, the whole request is billed at a higher rate ([tokens and cost](/basics/tokens-and-cost/) covers the tiers).

**Isolated failures.** If one group's agent fails or runs out of turns, only that group is affected, and every group's fix is checked by itself before the merge.

## What it can't show

Each row is two or three runs on one model, on a shared machine.
The times overlap, so "about as long" is the fair reading; calling fan-out slower would go beyond the data.
Checks passed shift by a few points between runs of one setup, so that column doesn't rank the designs.
It is also a single batch, one that a lone agent finished in 30 to 62 turns across all its recorded runs, including those of the [context management](/build/context-management/) guide.

## When fan-out pays

Fan-out comes out ahead when the work splits cleanly:

- **Independent pieces with no shared bottleneck**, such as building separate files of a codebase in parallel, where each agent owns its own files and its own checks, needs little of what the others read, and doesn't queue for a shared resource. That is close to the opposite of this batch, where the agents read the same code and waited on the same two test slots.
- **Large groups**, where every group takes many turns by itself and the reading the agents have in common is a small part of their work.
- **Requests one agent couldn't keep small**: a job where a single agent's requests would grow past what a context policy can hold down.
- **Far more than four agents running at once.**

For a batch like this one, crashes in one shared codebase, a single agent with a good context policy matched fan-out's time and cost less per run, while fan-out gave the more predictable bill.

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
Their calls appear interleaved, each line tagged with its group, followed by the merge, the filed issues, and the [crash catalog's](/glossary/#crash-catalog) grade.
