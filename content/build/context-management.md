---
title: Keeping a long agent run's context in check
short: Context management
description: Each turn, an agent resends everything so far, so long runs get expensive and then hard for the model to follow. This page shows five context policies in Go (prompt caching, clearing old tool results, trimming turns, compaction, and notes) and what each did on a 99-report batch.
card: Five ways to keep a long run affordable and on task, and what each did on a 99-report batch.
chapters: 8
stage: 1
date: 2026-10-18
weight: 50
art: ch08
---

A model has no memory between requests, so the harness sends the entire history each turn: the task, every crash report opened so far, every file read, every test output.
Over a long run that history gets big.
Money is the first thing it hurts; the model's ability to pay attention to all of it comes later.
Deciding what goes into each request is stage 1 of the loop, and the design below turns that decision into Go code you can swap and test.

## The problem, measured

One task in the crash catalog throws everything at the agent together: 99 reports from 23 different causes, the kind of pile a crash feed produces after a release goes badly.
The default model ran it with no context policy at all.
It needed 35 turns and sent 3.1 million input tokens across them, roughly 89,000 per request on average, for a total of $1.33.

```result
figure: $0.23
unit: down from $1.33, for the same work
task: the crash catalog's batch task, 99 crash reports from 23 causes, triaged by one agent with chapter 8's prompt
models: anthropic/claude-haiku-5.5, the book's default model, through OpenRouter
runs: one run with no context policy and one with cache markers; the provider's billed cost
date: October 2026
source: Chapter 8

Two cache markers per request, and no other change, moved 95 percent of the input to the cache price. Part of the drop comes from the price tier described below, which both runs crossed; at the base price alone, the chapter puts the saving at about four and a half times.
```

Pricing explains part of that bill.
For requests above 100,000 tokens, the default model's price jumps to five times its normal rate, and close to half the requests in this run crossed that threshold.
The window was never the constraint: at a million tokens, it held far more than any one request in the run.
So for an agent the order of trouble is money first, and only on longer runs the window size and how well the model attends to a very long request.

## Where the tokens go

A request is made of five parts, and each grows at its own pace.
The system prompt and the tool definitions are fixed.
The model's own messages and tool calls add a few hundred tokens per turn.
Tool results arrive in big steps whenever the agent reads something.
And reasoning models return encrypted reasoning with each tool call, which has to be sent back and grows as well.

By turn 10 of the batch run, tool results made up 71 percent of the request.
By turn 35 the reasoning had caught up: it took about as many bytes as every tool result in the request combined.
Since it is encrypted, the harness has no way to condense it, so all the policies below treat each turn's reasoning as part of that turn and keep or drop them together.

## Policies, not rewrites

How to build a request is a value you pass to the loop:

```go include=ch08/agent/policy.go tag=policy
```

A [context policy](/glossary/#context-policy) takes the full conversation and returns the messages for one request.
It leaves the conversation alone; `Result.Messages` still holds every message.
You want that complete history when you debug a bad run, when an eval grades one, and when a stopped run resumes.
Policies are also stateless: whatever a policy has decided earlier is handed back to it in a `ContextState`.
Given the same conversation and state, a policy always produces the same request, so you can unit-test it and resume a run without surprises.

Whatever a policy removes, it removes in whole turns.
The API requires every tool call in an assistant message to be followed by its result, and every result to follow its call; break that pairing and the request is refused.

## Five tools, cheapest first

**Prompt caching.**
Consecutive requests share almost everything up front, and providers charge roughly a tenth of the normal input price for a prefix they have already seen.
Some models cache on their own; the default model wants explicit markers.
`CacheMarks` sets two of them, one after the system prompt and task and one at the end of the request.
That single change took the batch from $1.33 to $0.23, and 95 percent of the input tokens were read from the cache.
The cache only works if the beginning of the request stays identical, so anything that varies there breaks it: a clock reading in the system prompt, say, or tool definitions emitted in map iteration order.

**Clearing old results.**
In an old turn, the tool result is usually the bulk of the tokens, and if the model needs it again it can repeat the call.
`ClearResults` swaps each large, old result for a short note and keeps all the messages in place.
An early version started clearing on turn one.
The agent then kept re-reading what had vanished: it repeated a read it had already made, with the same arguments, 47 times across 16 files and 9 crash reports, and the run went 132 turns against roughly 35 for the other policies.
The version that ships waits until the request crosses a size limit, and then advances its boundary several turns at a time, so consecutive requests keep the same prefix and the cache keeps working.

**Trimming.**
Once a request is too long, `Trim` drops the oldest complete turns.
It always keeps the system prompt and the task, and it adds a note saying turns were removed, so the model knows to re-read a file instead of concluding it never looked.
The batch cost $0.09 this way.
Trimming suits this agent because it handles crash groups one after another; turns from a group it finished long ago are almost never needed again.

**Compaction.**
Sometimes the useful findings are scattered over dozens of turns.
Then the harness can ask a model to summarize the older turns and send the summary in their place; this is [compaction](/glossary/#compaction).
The summary is written from the complete conversation, before any results were cleared, and it goes into the request marked as the harness's own record.
Summaries tend to drop exact values such as crash IDs, file paths, and issue numbers, and losing an issue number can lead the agent to file the same crash twice.
So code compares the summary against every identifier the agent has used and appends any that are missing.
The batch runs produced one summary; it kept 48 of 54 identifiers, and the check added back the remaining six.

**Notes the agent keeps.**
Two tools let the agent save and fetch notes for each crash group.
Since they are stored apart from the conversation, policies never touch them.
The idea is that the model, better than any harness rule, can tell which findings are worth keeping.
On this batch it wrote as many as 11 notes in a run and read none of them, since each group was finished before its turns were trimmed.

## The default chain

The triage agent's default policy chains these together, starting with the one that loses least:

```go include=ch08/task.go tag=policy
```

Once a request passes 60,000 tokens, estimated from its byte count, it clears old results; it summarizes only if clearing falls short; and last it places cache markers on whatever remains.
The estimate assumes four bytes per token.
On the recorded run it averaged 0.57 of the token count the provider reported, and never went above 0.70.
Code and crash reports take fewer bytes per token than English prose, so four bytes per token undercounts them, and the estimate also leaves out the tool definitions, about 4,000 tokens on every request.
That puts 60,000 estimated tokens near 100,000 real ones.
If the limit exists to keep requests under the price tier, set it lower, or calibrate it from the token usage the last reply reported.

## What the runs showed

| Policy | Turns | Cost | From the cache | Checks passed (of 94) |
| --- | --- | --- | --- | --- |
| None | 35 | $1.33 | 0% | 86 |
| Cache markers | 33 | $0.23 | 95% | 86 |
| Trim, then cache markers | 35 | $0.09 | 88% | 85 |
| Clear from the start, then cache markers | 132 | $0.68 | 76% | 81 |
| Clear past 60,000 tokens, then cache markers (two runs) | 62 and 30 | $0.42 and $0.06 | 82% and 90% | 87 and 82 |

Every row is one batch run on the default model, except the last, which is two, and the checks are the catalog's 94 for this task.
Read that last row before the others.
Nothing changed between its two runs except the model's choices, and one run took 62 turns for $0.42 while the other took 30 turns for $0.06, a sevenfold difference in cost.
So a gap between two single-run rows means something only when it is large and has a cause you can point to.
Two of the gaps can be read that way: caching, where 95 percent of the input came from the cache against none, and the 132 turns from clearing too early, which line up with the 47 repeated reads.
The checks-passed column, which ranges from 81 to 87, is within the spread you could get by running any row again.

## What to take from it

- **Mark the cache first**, and keep the beginning of every request identical from one turn to the next.
- **Keep the full conversation**, and derive each request from it through a policy you can unit-test.
- **Clear before you summarize.** A cleared result costs a line and can be fetched again; a summary is another model's lossy rewrite.
- **Act only past a limit, and in chunks**, so the request's prefix stays stable and keeps hitting the cache.
- **Check summaries in code** for the identifiers the agent relies on.
- **Measure on your own task**, and run each policy more than once.

## When to use it

```fit
reach: Requests grow toward a price tier. Past 100,000 tokens, the default model bills every token of the request at five times the base rate, and every single-agent batch run came within 4 percent of that line, one crossing it.
reach: The provider caches only what you mark. Cache markers cut one batch run on the default model from $1.33 to $0.23 for the same work.
leave: Clearing old results early in a run. Cleared from the start, the agent reread the same things 47 times and took 132 turns where the others took about 35.
leave: Runs that stay short. On the longest single task recorded, no request passed 38,000 tokens, well under where the default policy starts clearing.
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), with no key set:

```console
$ go run ./ch08 -task batch
```

This replays the recorded batch run under the default policy (30 turns), executes every tool against fresh copies of the service, and grades the outcome against the catalog.
Because it runs the service's tests many times, it takes several minutes.
The policies are in [`ch08/agent/trim.go`](code:ch08/agent/trim.go) and [`ch08/agent/compact.go`](code:ch08/agent/compact.go).
