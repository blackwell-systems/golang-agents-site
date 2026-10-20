---
title: "Durable agent runs: checkpoints, resuming, and doing things exactly once"
short: Durable runs
description: How to make an agent survive a crash without losing its work or doing it twice. What to save, the five moments to save it, a checkpoint write that a crash can't tear, a lock and a restart limit, resuming by phase, and idempotency keys, attempt markers, and a three-step ladder for calls whose outcome nobody recorded.
card: Surviving a crash without losing work or doing it twice: checkpoints, locks, and idempotency keys.
group: Running it
chapters: 12, 15
stage: all
date: 2026-10-18
work: true
weight: 70
art: ch15
---
Picture the triage agent partway through a long batch.
It has filed one issue and is writing a test for the next crash group when a deploy restarts the machine.
If the run simply starts over, it pays a second time for every model call it had made.
Worse, if the process died just after the tracker created that issue and before the agent wrote the answer down, the new process has no idea the issue exists and files another.
With other tools the same gap means a customer refunded twice or a deploy run twice.

Both gaps close with two pieces: [`ch15/durable`](code:ch15/durable), which stores checkpoints, and [`ch15/agent`](code:ch15/agent), whose loop saves and resumes them.

## What to save

[Run state](/glossary/#run-state) is whatever a later process needs in order to carry on: the messages so far, tokens and dollars spent, the turn count, the context policy's summary, any calls waiting on a person, and a phase recording the loop's next move.
This loop uses four phases: `model` when a model call comes next, `tools` once a reply's calls are fixed but not all answered, `paused` while a person decides, and `done` once the answer exists.

Spending and turn counts are in the state for a reason that is easy to miss.
If a restart forgot them, a run that crashed every few turns would start each life with a fresh budget and never reach its limits.
Restarts get a limit of their own as well, ten by default, after which the run stops with `ErrMaxResumes`; the count is saved before anything else happens, so a run that dies during every restart still reaches it.

Files the agent changed are part of the state too.
The state can keep a SHA-256 for every file the run has written; on resume, a file that no longer matches stops the run with `ErrWorkspaceChanged` rather than letting the model reason about code its conversation doesn't describe.
The sample triage command doesn't wire that check yet, since only the run's own process touches its workspace; the finished crash-triage agent records each write and checks the workspace on every resume.

## Save at five moments

The natural place to save is the end of each turn, after its tools have run, and it is the wrong place.
Tools run in the middle of the turn, and some of them change things outside the process.
Lose the turn and the new process asks the model again, and a real model may answer with different calls, which then run on top of the first reply's calls.

So the loop saves more often:

| # | When | What a crash right after it would otherwise lose |
| --- | --- | --- |
| 1 | Before the first model call | The run itself: it was accepted, and nothing on disk says so |
| 2 | Once the reply is in the conversation, before any of its tools start | The reply, so a restart asks again and may run a second set of calls |
| 3 | Immediately before each call that can change the outside world | The knowledge that the call may have acted |
| 4 | After every tool result | The result, so a finished call runs again |
| 5 | When the run pauses for a person, and when it ends | That it is waiting, or that it already has its answer |

After save point 2, a restarted run only ever runs the calls of the saved reply.
A reply that was saved is never paid for twice; a reply that arrived and was lost before its save is requested again.

## Write a checkpoint a crash can't tear

A [checkpoint](/glossary/#checkpoint) is only useful if, whenever the crash comes, the disk holds either the old copy or the new one, never half of one.
Writing over the file in place breaks that.
The usual fix has four steps, written out in the companion code:

```go include=ch15/durable/checkpoint.go tag=write
```

The temporary file goes in the same directory because a rename can't cross file systems, and the rename is what swaps old for new in one step.
Syncing the file first keeps the new name from pointing at data that never reached the disk; syncing the directory afterwards keeps the rename itself from vanishing in a power cut.
On macOS, Go's `File.Sync` also asks the drive to empty its own cache.
On an M1 Pro, a recorded batch run made 142 saves, with a median of 8 to 12 ms across seven measurements, against model calls whose median was over two seconds.
The temporary file is created with permissions 0600, which matters because a checkpoint contains every tool result, and tool results contain whatever the agent read.

Three smaller choices are worth copying:

- The loop saves with `context.WithoutCancel(ctx)`. When a shutdown cancels the run, the save that records where it stopped is the one that must happen.
- The state is written with [`encoding/json/v2`](/basics/json-v2/) with zero fields omitted, so an unset field loads as unset instead of as an empty value that the client would send back to the provider.
- Raw JSON values, such as a model's tool-call arguments, are stored as strings. A model can write invalid arguments, and a checkpoint that embedded them directly would refuse to save the very run that needs saving.

## One process per run

Two processes resuming one run would each run its unanswered calls, which brings back the duplicates through another door.
`Open` takes an `flock` lock on the run's lock file before reading anything, and a second process gets `ErrLocked` at once.
The kernel drops the lock when the process exits by any means, so a crash never leaves a run locked by a process that no longer exists.
That holds on one machine's local disk; on a network file system `flock` means different things on different clients, and a service spread over several machines needs a lease in a database instead.

## Resume by phase

`Resume` takes a saved state and the same tools and limits, and continues from the phase:

- `done` returns the saved answer without calling the model, so restarting a finished run is free.
- `model` makes the model call it was about to make.
- `tools` and `paused` run the last reply's unanswered calls in order, except calls with an attempt marker (below), which go to the resolution ladder.

Because turns and spending come from the state, a run's limits cover its whole life across every process that ran it.

## Doing something exactly once

A checkpoint stops lost work.
It can't stop repeated work, because some moment always separates a tool acting from the run recording that it acted.
A single process can promise [at most once](/glossary/#at-most-once) (never retry, and risk losing the action) or [at least once](/glossary/#at-least-once) (retry until answered, and risk doing it twice).
What people call exactly once is the second promise plus a receiver that recognizes a repeat.

**An [idempotency key](/glossary/#idempotency-key)** is the receiver's half.
The key must stay the same whenever one call is sent again, even from a new process, and must differ from every other call's key.
A hash of the run ID and the tool-call ID has both properties, since save point 2 stored the call ID before the call ran:

```go include=ch15/agent/once.go tag=key
```

The run ID carries the uniqueness, so it must never be reused; the triage command builds one from the task, the time, and eight random characters.
The key travels in the harness's channel, an `Idempotency-Key` header over HTTP or `_meta` over MCP, never in arguments the model writes.
Keys expire on real services (Stripe's v1 API may drop one after 24 hours), so a keyed tool declares how long its service keeps them, and an expired key is treated as no key.

**An attempt marker** needs nothing from the receiver.
The loop saves it just before a call that can change something, at save point 3.
After a crash, an unanswered call with no marker certainly never started and simply runs; one with a marker may have acted.
For a call that needs approval, the marker is written after the yes, so a crash during the decision leaves nothing to resolve.

The triage agent also keeps a refile guard in `issue_file`: a second filing for a cause the run already filed is refused with the number of the existing issue.
It covers a case keys can't: the model filing one crash a second time through a fresh call, which carries a fresh key.

```diagram
name: ch15-timeline
alt: A sequence of three participants: the agent, its checkpoint file, and the tracker. Before the crash, the agent saves the reply with call c2 to issue_file, then an attempt marker for c2 with key k, then asks the tracker to create the issue with Idempotency-Key k. The tracker records issue 1 and answers 201, but the answer is never read, because the process is killed. After the restart, the agent loads the checkpoint and finds c2 with a marker and no result. Without a key, creating the issue again makes issue 2, a duplicate. With key k, the tracker answers 200 with issue 1, the same issue, and the agent saves c2 as answered and drops the marker.

The crash a checkpoint can't cover: the side effect happened, but its result was never saved.
```

## The resolution ladder

When a marked call has no saved result, it goes up three steps, each tried only if the one below it can't decide:

1. **Run it again** when a repeat is harmless: the tool only reads, is idempotent, or sent a key the service still remembers.
2. **Ask the service** through the tool's lookup, such as a search by key. A lookup may answer "no such record" only when it is certain; a search index that lags behind writes has to return an error and pass the question up.
3. **Ask a person**, through the same approver used for tool approvals, with its own answers: "rerun" if it didn't happen, or "done" with what they found, which becomes the call's result.

The third step exists because, for a tool with no key, retrying is a bet that the first call failed, and losing that bet produces the duplicate all of this was built to prevent.
With keys that the service honors, a call never gets past the first step.

```diagram
name: ch15-ladder
alt: A flowchart starting from a call with a marker and no result. First question: is it safe to run again, because it is read-only or idempotent? If yes, rung 1: run it again with the same key. If no, second question: can the other side be asked by key? If the record is found, rung 2 answers the call with what the record says; if the other side is sure there is no record, rung 2 runs the call. If there is no lookup, or it can't tell, as with a search that may lag, rung 3 asks a person through Limits.Approver. Approved means run it again; denied means keep it as done, with the reason as the call's answer; no decision yet means ApprovalPending and the run waits.

What to do with a call whose outcome nobody recorded.
```

## Does it work? Crash it everywhere

The crash sweep runs a small scripted triage job once to find every point where a crash could land, then reruns it from scratch for each point: crash there, restart, finish, count.
It does it again with a second crash during the restart.

```result
figure: 8 of 18
unit: crash points with duplicates, saving once per turn
task: a scripted triage job that files two issues, comments, and searches, crashed at every point a crash could land and then resumed
models: a scripted model, so a repeated call gets the same reply
runs: one scenario per crash point, plus a second crash at every point each restart reaches
date: deterministic; `go test ./ch15/durable/` repeats it
source: Chapter 15
link: /findings/measured-mistakes/#save-per-turn
more: The finding

Saving once per turn left a duplicate issue or comment at 8 of its 18 crash points. On the agent's own loop with keys and markers, 910 scenarios ended with no duplicate, nothing missing, and no question for a person.
```

Without keys, the same loop made no duplicates either, but only by asking a person 624 times over 892 scenarios.
The sweep has these limits: a panic is not a power loss, the syncs are off so it runs in seconds, the tracker never crashes, the scripted person always answers correctly, and a scripted model repeats itself where a real one wouldn't.

## Steps for your own agent

1. Define a state type that holds the conversation, usage, turns, phase, pending calls, and markers, and test that no field of your result type is left out of it.
2. Save at the five points, with a write, sync, rename, and directory sync, and save with a context that ignores cancellation.
3. Lock each run, count restarts before doing anything, and never reuse a run ID.
4. Send an idempotency key made from the run and call IDs with every call that changes something, and ask the services you call to honor it.
5. Save an attempt marker before each such call, and give each tool a lookup if its service can answer "did this happen?"
6. On resume, run unmarked calls, send marked ones up the ladder, and route the last step to a person.
7. Crash your own loop at every save point in a test, and count duplicates and questions.

## When to use it

```fit
reach: Tools change the world outside the run. With one save per turn, a crash left a duplicate issue or comment at 8 of 18 crash points; keys and markers left none in any scenario.
reach: The platform stops processes on its own schedule. Kubernetes allows 30 seconds and `docker stop` 10, and one model call in the recorded runs took 27.5 seconds.
leave: Markers without keys when nobody is on call. Against a tracker that ignored keys, markers turned crashes inside calls into 624 questions for a person over 892 scenarios of the agent's loop.
leave: Short runs with no side effects. No variant in the sweep lost an issue or comment; what a crash costs such a run is the model calls since its last save.
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), with no key set, so the command replays a recorded run:

```console
$ go run ./ch15 -task m1-done-after-delete -state /tmp/triage-state -die-after code_patch
$ go run ./ch15 -task m1-done-after-delete -state /tmp/triage-state -approve off
$ go test ./ch15/durable/
```

The first command sends itself SIGKILL as soon as the first `code_patch` returns, before its result is saved.
Open the `.json` file in `/tmp/triage-state`: its `Phase` is `tools`, `Turns` is 8, and `Attempts` holds a marker for the patch.
The second command resumes that run; the ladder's second step finds the patch already in the workspace, the replay continues, and the run finishes with all 16 of the catalog's checks passed.
`-approve off` is there because the restart would otherwise stop and ask you to approve the issue it files.
The third runs the crash sweep, which takes about a minute, and compares its table with the saved copy in `ch15/durable/testdata/sweep.txt`.
