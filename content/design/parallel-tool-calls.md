---
draft: true
title: Running an agent's tool calls in parallel with goroutines
short: Parallel tool calls
description: A single model reply often requests several tools. This page covers which of those calls can safely overlap, a runner built on sync.WaitGroup.Go and a semaphore that returns answers in the model's order, and why the time it saves on a coding agent is measured in milliseconds.
chapters: 9
stage: 3
date: 2026-10-18
weight: 50
---

When a reply contains several tool calls, the model has only listed them; executing them is up to the harness.
Starting them all at the same moment seems like a free speedup, and in Go it takes a few lines.
The harder questions are which calls are allowed to overlap, how to keep the conversation valid and the run reproducible, and whether the speedup is real.
This page works through those questions with chapter 9 of the book and its runner, [`ch09/agent/runner.go`](code:ch09/agent/runner.go).

## How often models ask for several calls

Models do this on their own, and frequently.
In the runs the book recorded, between a quarter of replies and every reply requested more than one tool, depending on which model and which task:

| Task | Model | Replies with more than one call | Mean calls per reply | Most in one reply |
| --- | --- | --- | --- | --- |
| Bug fix, 4 tools | `anthropic/claude-haiku-5.5` | 100% | 2.00 | 2 |
| Bug fix, 4 tools | `openai/gpt-5.6-luna` | 25% | 1.50 | 3 |
| Crash triage, 14 tools | `anthropic/claude-haiku-5.5` | 64% | 2.59 | 6 |
| Crash triage, 14 tools | `openai/gpt-5.6-luna` | 50% | 3.14 | 12 |

The book never sets `parallel_tool_calls` on a request.
On models that honor it, setting it to false limits each reply to one call.
There is no value that makes a model request more than one, and in spite of the name, the flag has no bearing on how your harness executes the calls it gets.
Anthropic's documentation is explicit that execution order is the caller's business: run the calls however you like, provided each one gets exactly one result.

## Which calls may run together

Some pairs must not overlap.
In each of the default model's ten bug-fix runs, a single reply contained an edit to the file followed by a test run.
If both start together, the tests can execute before the edit lands, check the old code, and report a failure that the edit had already fixed.

Chapter 9 decides with the read-only flag that every tool has carried since chapter 7:

- A run of adjacent calls to [read-only tools](/glossary/#read-only-tool) executes concurrently.
- Every other call executes by itself, waiting for all earlier calls and holding back all later ones; this is a [barrier](/glossary/#barrier).

`run_tests` doesn't count as read-only, since it executes code, including tests the agent wrote, so it acts as a barrier.
A call to a tool the harness doesn't recognize also gets its own stage, because nothing is known about what it does.

```go include=ch09/agent/runner.go tag=stages
```

A finer scheme is possible: have each call declare the resources it reads and writes, build a dependency graph, and let two patches to separate files run at once.
That scheme depends on every tool describing its effects precisely, and ties would still fall back to the order of the reply.
A single bit per tool handles the cases that matter.

## Running a stage

The runner launches each call in a stage with `sync.WaitGroup.Go`, added in Go 1.25, which does the counting that used to need a manual `Add(1)` and `defer Done()`, a pair that was easy to misplace.
To bound concurrency, a buffered channel acts as a semaphore with eight slots per reply.
Only three replies in the recorded runs asked for more than eight calls, and all three came from one model on the triage task.
If the run is cancelled while a call is waiting for a slot, the call never starts and its answer is "cancelled".

Three decisions make the runner correct:

- **Each goroutine writes only its own slot.** Answers go into a slice at the position the call had in the reply. Since every goroutine owns a distinct index, there is no shared write and no mutex, and the results end up in the model's order regardless of which call finished first.
- **Answers are reported only after `Wait`.** The loop's goroutine reports them, never a tool's goroutine, because the loop's event iterator has to be fed by the goroutine that is ranging over it.
- **A panic in one tool is recovered at that tool's boundary.** The panic turns into that call's answer; the other calls in the stage carry on, and one tool's failure never cancels its neighbors.

The API accepts results in any order, so why preserve the model's?
Mostly for reproducibility: a run replays identically, and the book's recorded runs and tests keep passing.
It also keeps the next request byte-for-byte equal to what sequential execution would have produced, so the prompt cache still hits.
Streaming results back as each call completes wouldn't gain anything, since the next model call can't start until every result is in.

The package's tests check this.
They replay the edit-then-test reply using a deliberately slow fake edit and a fake test tool that reports the version of the code it ran against.
With every call started at once, the fake tests see the old code; with the staged runner, they see the edited code.

## What it saves on a coding agent

Almost nothing, and chapter 9 has the numbers.
It took the 87 tool calls from one recorded triage run and replayed them one by one, timing each:

| Tool | Calls | Median (ms) | Longest (ms) |
| --- | --- | --- | --- |
| `code_read` | 21 | 1 | 2 |
| `code_search` | 14 | 10 | 22 |
| Reading reports, issues, file lists | 25 | 0 | 1 |
| `notes_write` | 3 | 0 | 0 |
| `code_patch` | 13 | 517 | 2,652 |
| `run_tests` | 11 | 772 | 4,937 |

All 60 read-only calls together took about a seventh of a second.
Nearly all the time went to two tools: `run_tests`, and `code_patch`, which runs the formatter and a compile on every edit.
Together they took 33 seconds, and both are barriers, so neither could overlap with anything.
For comparison, a short answer from the default model took a median of 3.1 seconds per call.
Overlapping the reads on this agent saves a few milliseconds per turn.

The runner stays in the code anyway.
It is correct, it costs little, and it pays off for tools that spend their time waiting on remote services, a search API for instance, or a database on another machine.

## A tool that can't run at all

Some failures leave the model nothing to try.
When the workspace has disappeared, every `code_read` fails, and a model that only sees "no such file" keeps guessing new paths until it hits the turn limit.
For this case a tool returns an error wrapping `ErrToolFatal`.
The runner halts at the current stage, gives every call that didn't run the answer "not run" so the API still accepts the conversation, and the loop returns the error to its caller.

## Others choose differently

The OpenAI Agents SDK, by default, launches all of a turn's function tool calls together, with an optional limit on how many run at once.
Ordering is left to the tools, except that its shell and patch tools each run one call at a time within their own kind.
For independent lookups that works well.
For a reply that edits a file and then runs tests, the ordering would have to come from the tools themselves.

## When to use it

```fit
reach: Your model asks for several calls at once. In the book's saved runs, a quarter to all of the replies with tool calls asked for more than one, depending on the model and the task (chapter 9).
reach: Tools wait on something slow, such as a network service; then the time saved is all but the slowest call's. On the book's local tools, the time went elsewhere (chapter 9).
leave: Expecting it to speed up a coding agent. Replaying 87 recorded triage calls, the 60 read-only ones took a seventh of a second in all, so running them side by side saved milliseconds per turn (chapter 9).
leave: Calls that change files or run tests. An edit and a test run in the same reply must run in order, alone, or the test can read the file from before the edit (chapters 1 and 9).
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch09/agent/ -run 'Barrier|Stages' -race -v
```

These tests run the stages on real goroutines with the race detector on.
The semaphore is in [`ch09/agent/slots.go`](code:ch09/agent/slots.go).
