---
title: Running an agent's tool calls in parallel with goroutines
short: Parallel tool calls
description: A single model reply often requests several tools. This page covers which of those calls can safely overlap, a runner built on sync.WaitGroup.Go and a semaphore that bounds concurrency, and per-call slots that return answers in the model's order, and why the time it saves on a coding agent is measured in milliseconds.
card: Which tool calls can safely overlap, a runner that keeps the model's order, and what it saves.
group: Scale up
chapters: 9
stage: 3
date: 2026-10-18
weight: 60
art: ch09
---

When a reply contains several tool calls, the model has only listed them; executing them is up to the [harness](/glossary/#harness), the program that drives the [agent loop](/basics/agent-loop/).
Starting them all at the same moment seems like a free speedup, and in Go it takes a few lines.
The harder questions are which calls are allowed to overlap, how to keep the conversation valid and the run reproducible, and whether the speedup is real.
This page works through those questions with the runner in [`ch09/agent/runner.go`](code:ch09/agent/runner.go).

## How often models ask for several calls

Models do this on their own, and frequently.
In the recorded runs, between a quarter of replies and every reply requested more than one tool call, depending on which model and which task.
All four models were counted on both tasks; here are two of them on each.
"Crash triage" is the companion code's [crash-triage agent](/glossary/#crash-triage-agent), which reads crash reports from the `remind` practice service and files issues in a tracker; "bug fix" is its smaller first task:

| Task | Model | Replies with more than one call | Mean calls per reply | Most in one reply |
| --- | --- | --- | --- | --- |
| Bug fix, 4 tools | `anthropic/claude-haiku-5.5` | 100% | 2.00 | 2 |
| Bug fix, 4 tools | `openai/gpt-5.6-luna` | 25% | 1.50 | 3 |
| Crash triage, 14 tools | `anthropic/claude-haiku-5.5` | 64% | 2.59 | 6 |
| Crash triage, 14 tools | `openai/gpt-5.6-luna` | 50% | 3.14 | 12 |

Models send several calls on their own, and the companion code never asks for or against it.

<div class="hood">

The companion code never sets `parallel_tool_calls` on a request.
On models that honor it, setting it to false limits each reply to one call; of the four models tested only `z-ai/glm-5.3-flash` accepts it, and Anthropic's own API has a separate switch for the same purpose, `disable_parallel_tool_use` (the [ecosystem page on API differences](/ecosystem/differences/) covers more of these).
There is no value that makes a model request more than one, and in spite of the name, the flag has no bearing on how your harness executes the calls it gets.
Anthropic's documentation is explicit that execution order is the caller's business: run the calls however you like, provided each one gets exactly one result.

</div>

## Which calls may run together

Some pairs must not overlap.
In each of the default model's ten bug-fix runs, a single reply contained an edit to the file followed by a test run.
If both start together, the tests can execute before the edit lands, check the old code, and report a failure that the edit had already fixed.

The runner decides this with the read-only flag that every tool carries:

- A run of adjacent calls to [read-only tools](/glossary/#read-only-tool) executes concurrently.
- Every other call executes by itself, waiting for all earlier calls and holding back all later ones; this is a [barrier](/glossary/#barrier).

`run_tests` doesn't count as read-only, since it executes code, including tests the agent wrote, so it acts as a barrier.
A call to a tool the harness doesn't recognize also gets its own stage, because nothing is known about what it does, and so does the tool that ends the task (`submit_diagnosis` in the triage agent): once it accepts an answer, the calls after it are answered "not run".

```go include=ch09/agent/runner.go tag=stages
```

A finer scheme is possible: have each call declare the resources it reads and writes, build a dependency graph, and let two patches to separate files run at once.
That scheme depends on every tool describing its effects precisely, and ties would still fall back to the order of the reply.
A single bit per tool handles the cases that matter.

## Running a stage

Read-only calls start together, at most eight at once, and their answers go back in the model's order.
Starting them together uses goroutines: a [goroutine](/glossary/#goroutine) is a function running at the same time as the rest of the program, and a [channel](/glossary/#channel) passes values between goroutines (see [Goroutines with an owner](/go/goroutines/)).
The details are below, and you can skip them on a first read.

<div class="hood">

The runner launches each call in a stage with [`sync.WaitGroup.Go`](/go/goroutines/), added in Go 1.25, which does the counting that used to need a manual `Add(1)` and `defer Done()`, a pair that was easy to misplace.
To bound concurrency, a buffered channel acts as a semaphore with eight slots per reply.
Only three replies in the recorded runs asked for more than eight calls, and all three came from one model on the triage task.
If the run is cancelled while a call is waiting for a slot, the call never starts and its answer is "cancelled".

Three decisions make the runner correct:

- **Each goroutine writes only its own slot.** Answers go into a slice at the position the call had in the reply. Since every goroutine owns a distinct index, there is no shared write and no mutex, and the results end up in the model's order regardless of which call finished first.
- **Answers are reported only after `Wait`.** The loop's goroutine reports them, never a tool's goroutine, because the loop's [event iterator](/go/iterators/) has to be fed by the goroutine that is ranging over it.
- **A panic in one tool is recovered at that tool's boundary.** The panic turns into that call's answer; the other calls in the stage carry on, and one tool's failure never cancels its neighbors.

</div>

The API accepts results in any order, so why preserve the model's?
Mostly for reproducibility: a run replays identically, and recorded runs and tests keep passing.
It also keeps the next request byte-for-byte equal to what sequential execution would have produced, so the [prompt cache](/glossary/#prompt-caching), which reuses the already-processed start of the prompt, still hits.
Streaming results back as each call completes wouldn't gain anything, since the next model call can't start until every result is in.

The package's tests check this.
They replay the edit-then-test reply using a deliberately slow fake edit and a fake test tool that reports the version of the code it ran against.
With every call started at once, the fake tests see the old code; with the staged runner, they see the edited code.

```diagram
name: ch09-stages
alt: Five calls in the model's order: code_read, code_read, crash_logs, code_patch, run_tests. Stage 1 runs the first three at once; stage 2 runs code_patch alone; stage 3 runs run_tests alone. The answers go back as five tool messages, in order.

One reply's calls in stages: read-only calls run side by side, every other call runs alone, and the answers go back in the model's order.
```

## What it saves on a coding agent

Almost nothing, and the numbers show it.
One measurement replayed the 87 tool calls from one recorded triage run one by one, timing each:

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
For comparison, a short answer from the default model took a median of 3.1 seconds per model API call.
Overlapping the reads on this agent saves a few milliseconds per turn.

The runner stays in the code anyway.
It is correct, it costs little, and it pays off for tools that spend their time waiting on remote services, a search API for instance, or a database on another machine.

## What was measured, and what wasn't

Three things were measured here: how often each of four models asks for several calls in one reply, across the saved runs; how long each triage tool takes, by replaying one recorded run's 87 calls one at a time; and, in the tests, that the staged runner keeps an edit ahead of the test run that follows it.
The runner was not measured on tools that wait on a network, where the saving should be largest, since the triage agent has no such tool.
The claim that overlapping those calls saves all but the slowest call's time is arithmetic, not a measurement, and worth checking on your own tools before you count on it.
The finer dependency-graph scheme described above was not built or measured either.

## A tool that can't run at all

Some failures leave the model nothing to try.
When the workspace has disappeared, every `code_read` fails, and a model that only sees "no such file" keeps guessing new paths until it hits the turn limit.
For this case a tool returns an error wrapping the sentinel `ErrToolFatal`, which the runner detects with `errors.Is` (see [Errors in a Go agent](/go/errors/)).
The runner halts at the current stage, gives every call that didn't run the answer "not run" so the API still accepts the conversation, and the loop returns the error to its caller.

## Others choose differently

The OpenAI Agents SDK, as its source stood in October 2026, launches all of a turn's function tool calls together by default, with an optional limit on how many run at once.
Ordering is left to the tools, except that its shell and patch tools each run one call at a time within their own kind.
For independent lookups that works well.
For a reply that edits a file and then runs tests, the ordering would have to come from the tools themselves.

## When to use it

```fit
reach: Your model asks for several calls at once. In the saved runs, a quarter to all of the replies with tool calls asked for more than one, depending on the model and the task.
reach: Tools wait on something slow, such as a network service; then the time saved is all but the slowest call's. On the local tools in these guides, the time went elsewhere.
leave: Expecting it to speed up a coding agent. Replaying 87 recorded triage calls, the 60 read-only ones took a seventh of a second in all, so running them side by side saved milliseconds per turn.
leave: Calls that change files or run tests. An edit and a test run in the same reply must run in order, alone, or the test can read the file from before the edit.
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch09/agent/ -run 'Barrier|Stages' -race -v
```

The `-run` pattern picks the two tests by name, `-race` turns on Go's data-race detector, and `-v` prints each test's name and result.
It needs no key.
The output ends like this:

```text
=== RUN   TestStages
--- PASS: TestStages (0.00s)
=== RUN   TestBarrier
--- PASS: TestBarrier (0.00s)
PASS
```

`TestStages` checks how replies split into stages, and `TestBarrier` runs the edit-then-test reply both ways, every call at once and through the runner, inside a `testing/synctest` bubble (a test helper in which time is virtual, so the slow fake edit doesn't make the test wait), with the race detector on.
The semaphore is in [`ch09/agent/slots.go`](code:ch09/agent/slots.go).
