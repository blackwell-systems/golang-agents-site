---
title: The agent loop in Go
short: The agent loop
description: An agent is a model inside a loop, and the loop is plain Go. This page reads the whole loop, about 30 lines, stage by stage, along with the rules that keep it correct.
chapters: 1, 4, 5
stage: all
date: 2026-10-18
weight: 20
art: ch01
---

An AI agent lets a model choose the next tool to run, turn after turn, using whatever the earlier tools reported, and it keeps going until the work is finished or a limit cuts it off.
The model takes part in that only by talking.
It sits behind an API, takes text in, and gives text back.
Keeping the conversation, executing the tools it requests, and recognizing the end of the run are all jobs for code on your side.
That code is the [harness](/glossary/#harness), and the loop at the middle of it fits on one screen.

Below is that loop as chapter 1 of the book builds it, taken from the [companion repository](code:ch01/agent/loop.go).
No model is needed to run it.
That is deliberate: the loop is ordinary Go, so you can write a test for each of its rules long before you pay for an API call.

## Four stages, every turn

A turn is a single exchange.
The harness sends the conversation so far, gets one reply, and does what the reply calls for.
Every turn passes through the same four stages:

<ol class="loop-stages"><li><strong>Build the request</strong>Choose what goes to the model this time: instructions, the messages so far, and the tool definitions.</li><li><strong>Call the model</strong>Send that request and wait until the reply comes back.</li><li><strong>Run the tools</strong>Carry out each call in the reply and append its result to the conversation.</li><li><strong>Decide</strong>Finish if the reply is an answer; if not, and no limit has been hit, start another turn.</li></ol>

The stages double as a debugging map.
If a run never ends, look at Decide.
If the agent acts as though it never saw a file it read long ago, look at Build the request.
Knowing which stage misbehaved narrows the search to a few lines.

## The model is one method

From the loop's side, a model is just a thing that returns the next reply:

```go include=ch01/agent/types.go tag=model
```

A `Request` carries the messages and the definitions of the tools on offer.
What comes back is one assistant message, and it may contain text, tool calls, or a mix.

With only one method to satisfy, a canned stand-in and a real HTTP client look identical to the loop.
Chapter 1 plugs in `Fake`, which hands back replies from a prepared list.
Chapter 4 plugs in a client that talks to a real model, and the loop itself is untouched.

## The loop resends the whole conversation each turn

Here is `Run`:

```go include=ch01/agent/loop.go tag=run
```

Read it stage by stage.

**Build the request.** Every call sends `msgs` in full.
A model carries no state from one call to the next, so the only thing it knows on a given call is what that request contains.
That means the requests keep getting bigger as the run goes on; [tool calling on the wire](/loop/tool-calling/) shows how many tokens the first request already takes before the task is read.
`Run` calls `slices.Clone` before it appends anything, so the caller's slice and its backing array stay as they were.

**Call the model.** `m.Generate(ctx, req)` receives the run's `ctx`, which lets a real client drop the HTTP request if the run is cancelled.

**Run the tools.** The assistant's reply goes into `msgs` *first*, and only then do its tools run.
That way every tool result refers to a call already present in the conversation, and each call gets one tool message, in the order the model listed them.
This order is required, since the APIs refuse a request where some tool call lacks its result.

**Decide.** A reply without tool calls is the answer, and the loop returns.
If it does, the loop takes another turn, and when `for range lim.MaxTurns` is exhausted, `Run` returns `ErrMaxTurns`.

Two details are easy to skim past.
`Run` hands back a `Result` with the messages even on failure, because the turns leading up to a failure are the evidence for why it happened.
And each turn begins by checking `ctx.Err()`, so a deadline or Ctrl-C ends the run without paying for one more model call.

## Calls run in order

In the bug fix that chapter 1 replays, one reply asks for two things: change `duration.go`, then run the tests.
If both ran at once, the test run might see the file before the change and report a failure that the edit had already fixed.
So this loop runs the calls in a reply sequentially, in the model's order, which can never cause that problem.
Parallel tool calls are faster, but they force the harness to work out which calls are safe to overlap, and that is chapter 9's subject.

## Every problem becomes a message

Nothing in `Run` ends the run because a tool went wrong.
`runTool` converts every kind of failure into a tool message for the model to read:

```go include=ch01/agent/loop.go tag=run-tool
```

If the model names a tool that doesn't exist, the reply lists the tools that do.
If a tool returns an error, the error text becomes the result, with `IsError` set.
On the next turn the model sees what went wrong and can send a corrected call.

Consider the other options.
Ending the run discards all the work done before the mistake.
Picking the nearest tool name and running it executes a call the model never made.
Reporting the error costs a single turn.
Chapter 4 replays a hand-written run in the API's format where the model makes three mistakes in a row: it calls a tool that isn't defined, passes a path as an array, and asks to replace text the file doesn't contain.
Every error message explains the problem, and every following call is correct.
The [ch04 example](/run/ch04-first-agent/) replays that run.

How quickly the model recovers depends on how the error is worded.
A message like `old text appears 0 times in duration.go, want exactly once` tells it what to change, while a bare "please try again" tells it nothing.

## The turn limit comes first

Without a turn limit, nothing guarantees the loop ends, and a real model bills you for every turn it takes.
So `MaxTurns` is the loop's first setting, and leaving it at zero permits no turns at all, so every caller has to set it.

There's no correct number you can know ahead of time.
In chapter 1, all 40 bug-fix runs (ten on each of four models, October 2026) finished in four to six turns, so the program sets the limit to ten.
Agent frameworks ship defaults that range from 10 turns to thousands of steps to no limit at all; chapter 1 lists them.
A default can't know what your task needs, so choose the limit yourself and report hitting it to the caller as a failure.

## A real model fits behind the same interface

Chapter 4 runs this same `Run` against a real model.
On the way in, the program wraps the client in another `Model` that totals the cost of the run:

```go include=ch04/main.go tag=meter
```

Because `meter` embeds a `Model` and defines its own `Generate`, it satisfies the interface itself, and the loop has no way to know there is a client inside.
Later chapters use the same trick, wrapping the model to add retries, timeouts, and tracing while the loop stays as it is.

Chapter 5 deals with what this short loop leaves out: a provider that rejects the request, a reply the loop can't use (a tool-call ID that appears twice, an answer truncated by the token limit), a run that overspends, and Ctrl-C arriving while a reply's tools are running.
Each one ends the run with a named [stop reason](/glossary/#stop-reason) that callers test with `errors.Is`, for example `ErrMaxTurns` or `ErrBudgetExceeded`.

## Run it

From the `code` directory of a clone of the repository:

```console
$ go run ./ch01
$ go test ./ch01/...
```

The first command plays the bug fix through the loop and prints every turn.
Here are the first two:

```text include=ch01/testdata/transcript.txt lines=1..13 title="go run ./ch01, turns 1 and 2"
```

Each `model asks` line is one tool call in the model's reply, and each `tool result` line is a tool message the loop appended, one per call and in the same order.
In turn 1 the model runs the tests and lists the files; in turn 2 it reads both files, so its third reply can name the exact line to change.
Nothing in `Run` knows it is fixing a test: it builds the request, calls the model, runs what was asked, and goes around again.
The [ch01 example](/run/ch01-scripted-loop/) shows all four turns, including the edit and the final answer.
The tests cover each rule separately: an answer ends the run, the turn limit ends the run, each tool call receives its own tool message, failing tools come back flagged as errors, and a made-up tool name is answered with the real list.
None of them uses the network or an API key.

## Where this goes next

- [Tool calling in Go, on the wire](/loop/tool-calling/): what the request and reply look like as JSON once a real model is attached.
- [A real agent that fixes a failing test](/run/ch04-first-agent/): the same loop with a real model's recorded replies.
- [Glossary](/glossary/): agent, harness, turn, tool call, and the rest of the vocabulary.
