---
draft: true
title: "Durable agent runs: checkpoints, resuming, and doing things exactly once"
short: Durable runs
description: An agent that survives a crash without losing work or repeating it. What a run's state is, the five points where the loop saves it, how to write a checkpoint that a crash can't tear, what a resumed run does in each phase, and keys, markers, and a three-rung ladder for calls whose outcome is unknown.
chapters: 15
stage: all
date: 2026-10-18
work: true
weight: 70
art: ch15
---

The triage agent is halfway through a batch: it has filed an issue for one crash group and is writing a test for the next, when the machine restarts for a deploy.
Two things can go wrong.
The obvious one is lost work: the run starts over and pays again for every model call.
The worse one is repeated work: if the process died a moment after the tracker accepted the issue, the restarted run has no record of it and files it again.
For an agent with other tools, that is two refunds, two deploys, or two emails to a customer.

Chapter 15 of the book fixes both, in [`ch15/durable`](code:ch15/durable) and the loop in [`ch15/agent`](code:ch15/agent).

## What a run's state is

[Run state](/glossary/#run-state) is everything a run needs to continue after its process is gone: the conversation, what it has spent, the turn count, the context policy's summary, the calls waiting for a person, and a phase saying what the loop was about to do:

- `model`: the next step is a model call.
- `tools`: a reply's calls are fixed, and some may not have run yet.
- `paused`: a call waits for a person.
- `done`: the run has its answer.

Spending and turns are saved so a restart can't reset the run's limits: a run that crashes every third turn and starts its count over would never reach its turn limit.
The workspace is state too: a resumed run that finds a different file from the one its conversation says it patched would reason from a file that no longer exists, so the state can record a hash of each file the run wrote and refuse to continue if one changed.

## Save at five points, not once per turn

Saving once per turn, after the tools have run, is the obvious choice and the wrong one.
A crash between the model's reply and the end of the turn loses the reply, so the restarted run asks the model again, and a real model can answer with different tool calls, which run on top of the first reply's.
The loop saves at five points instead:

| # | When | Without it |
| --- | --- | --- |
| 1 | Before the first model call | A run accepted and then lost, with nothing on disk to finish it |
| 2 | After the reply is appended, before any of its tools run | The restart asks again and can run a different reply's calls on top of the first |
| 3 | Just before each call that may change something outside the run | Every unanswered side-effecting call looks as if it may have acted |
| 4 | After each tool result | A finished call runs again |
| 5 | When the run pauses for a person, and when it finishes | A finished run's restart pays for an answer it already had |

From save point 2 on, a restarted run runs the saved reply's calls, never a fresh reply's.

## A checkpoint a crash can't tear

A [checkpoint](/glossary/#checkpoint) has to leave a complete copy on disk whenever the crash lands, the one before or the one after.
Writing in place can leave half a file; the standard answer is write, sync, rename, then sync the directory:

```go include=ch15/durable/checkpoint.go tag=write
```

Each step guards against a different loss.
Without the file's sync, the rename can reach the disk before the data, and a power loss leaves the new name on an empty file.
Without the directory's sync, the rename itself can be lost.
On macOS, Go's `File.Sync` asks the drive to flush its own cache too.
On the book's test machine a synced save took 8 to 12 ms at the median, against model calls whose median is over 2 seconds.
The temporary file is created readable only by its owner, which matters: a checkpoint holds every tool result, and tool results hold whatever the agent read.

The save ignores the run's cancellation, deliberately: when the process is asked to stop, the save that records where it stopped is exactly the one that must not be skipped, so the loop calls it with `context.WithoutCancel(ctx)`.

Two details from `encoding/json/v2` matter for saved state.
Zero fields are left out, so a nil field loads as nil rather than as `null` or `[]`, which a client might otherwise send back to the provider.
And every raw JSON value is written as a string, because a model's tool-call arguments can be invalid JSON, and a checkpoint that embedded them as-is would fail to save exactly the run that hit the problem.

## Resuming

`Resume` continues a saved state with the same tools and limits, by phase:

- **`done`**: the saved answer comes back, with no model call. Restarting a finished run costs nothing.
- **`model`**: the loop calls the model, as it would have.
- **`tools` and `paused`**: the last reply's unanswered calls run, in order, except those with an attempt marker, which go to the resolution ladder.

The turn count and usage come from the state, so limits apply to the run as a whole, across every process it ran in.

## Doing something exactly once

Checkpoints solve lost work, not repeated work: there is always a moment between a tool acting and the run recording it.
From inside one process, only [at most once](/glossary/#at-most-once) or [at least once](/glossary/#at-least-once) is on offer.
What looks like exactly-once is at-least-once delivery plus a receiver that recognizes a repeat.

**Idempotency keys** need the receiver's help.
The key for a call is a hash of the run's ID and the call's ID, which was saved with the reply before the call ran, so every retry of that call, in this process or after a restart, sends the same key, and no other call does.
A tracker that remembers keys answers a repeat with the first result.

**Attempt markers** need nothing from the receiver.
Saved just before a side-effecting call, a marker with no result means the call may have acted.
Without markers, a resumed run would have to treat every unanswered side-effecting call as unknown; with them, a call with no marker certainly never started, and simply runs.

## The resolution ladder

A call with a marker and no result climbs three rungs, each tried only when the one before can't decide:

1. **Run it again**, when repeating is harmless: the tool only reads, is idempotent, or sent a key the other side still holds.
2. **Ask the other side** whether it happened, with the tool's lookup, such as a search by key or title. A "no such record" counts only if the store is certain to show a create on the next read.
3. **Ask a person**, through the same approval machinery as any other decision. "Rerun" means it didn't happen; "done", with what they found, becomes the call's answer.

Why ask a person instead of retrying?
For a tool without a key, a retry is a guess that the first call failed, and a wrong guess costs the duplicate the whole design exists to prevent.
With keys the tracker honors, an unknown outcome never gets past the first rung.

## Does it work? Crash it everywhere

The chapter tests all of this with a crash sweep: it crashes a run at every point a crash could land, restarts it, runs it to the end, and counts duplicates.
Saving once per turn duplicated an issue or comment at 8 of its 18 crash points.
On the agent's own loop with keys and markers, 910 scenarios made no duplicates, lost nothing, and asked nobody.
[910 crash scenarios, no duplicate issues](/findings/crash-sweep/) has the method and its limits.

## When to use it

```fit
reach: Tools change the world outside the run. With one save per turn, a crash led to a duplicate issue or comment at 8 of 18 crash points; keys and markers left no duplicate in any scenario (chapter 15).
reach: The platform stops processes on its own schedule. Kubernetes allows 30 seconds and `docker stop` 10, and one model call in chapter 12's runs took 27.5 seconds (chapters 12 and 15).
leave: Markers without keys, where a person must answer. Against a tracker that ignored keys, markers turned every crash inside a call into a question: 624 over 892 scenarios of the agent's loop (chapter 15).
leave: Short runs with no side effects. No design in the sweep lost an issue or comment; what a crash costs such a run is the model calls since the last save (chapter 15).
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), with no key set (it replays a recorded run):

```console
$ go run ./ch15 -task m1-done-after-delete -state /tmp/triage-state -die-after code_patch
$ go run ./ch15 -task m1-done-after-delete -state /tmp/triage-state -approve off
```

The first kills the process with SIGKILL right after the first `code_patch` returns, before its answer is saved.
Open the `.json` checkpoint in `/tmp/triage-state` and find the phase, the turn count, and the marker for the patch.
The second resumes the run: the ladder’s second rung settles the patch from the workspace, and the run finishes and is graded.
