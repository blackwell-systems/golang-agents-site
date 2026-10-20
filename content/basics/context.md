---
title: "context.Context in a Go agent: cancellation, deadlines, and causes"
short: context.Context in an agent
description: How an AI agent in Go uses context.Context. Stopping a run on Ctrl-C, a deadline for each model call, saying why a run stopped with WithTimeoutCause and context.Cause, cancelling the commands tools start, an idle timer on a stream, work that must finish with WithoutCancel, testing timeouts with synctest, and the common mistakes.
card: Stop a run, give each call a deadline, say why it stopped, and let the work that must finish still finish.
group: Go foundations
chapters: 1, 2, 5, 6, 12, 15
stage: all
date: 2026-10-19
weight: 80
---

An agent run makes many slow calls: a model call that takes half a minute, a tool that runs a test suite, another model call, and so on for dozens of turns.
At any point, the person who started it may want it stopped, a call may hang, or the process may be shutting down.
In Go, all three travel the same way, through a `context.Context` passed into every function that waits on something.

```definition
term: context.Context

A **`context.Context`** carries a cancellation signal, an optional deadline, and a reason for stopping down a chain of calls; when it is done, every function that was handed it should stop waiting and return.
```

This page goes through the ways an agent uses one, each with code from the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), where it is tested.

## One context, from main down

The context starts in `main`, and the first thing it is good for is Ctrl-C:

```go include=ch01/main.go tag=signal
```

`signal.NotifyContext` returns a context that is cancelled when the process gets an interrupt.
From here the context is passed down, as the first parameter of every function that does I/O or can block: the loop, the model client, each tool.
Nothing below `main` makes a fresh `context.Background()`, because a fresh context cuts the chain: whatever ran under it would ignore Ctrl-C.

Passing it as a parameter, never storing it in a struct, keeps each call's context its own.
A client that stored a context would cancel every later request with the first one's.

## A deadline for each model call

A model call that hangs should fail on its own, without waiting for a person to notice.
The client gives every request its own deadline, derived from the run's context:

```go include=ch06/openrouter/client.go lines=72..84
```

The derived context ends at whichever comes first: the caller's cancellation, or the call's own timeout.
`defer cancel()` releases the timer when the call returns early, which is most of the time; without it, the timer lives until it fires.

The run has limits of its own above this, a turn limit and a cost budget, so a deadline for the whole run is rarely needed.
When it is, wrap the run's context the same way, and each call's shorter deadline still applies inside it.

## Say why it stopped

A cancelled context reports one of two errors, `context.Canceled` or `context.DeadlineExceeded`, and neither says which deadline passed.
"Deadline exceeded" from a run with a call timeout, a tool timeout, and a test timeout is a puzzle.

`context.WithTimeoutCause` and `context.WithCancelCause` attach a cause, an error of your own, and `context.Cause` reads it back.
Above, the call's timeout carries `ErrCallTimeout`.
One detail makes this harder than it looks: when a request is cancelled, Go's HTTP/1 transport returns the cause, and its HTTP/2 transport returns only the kind of cancellation.
The client joins both into the error, so callers can check either one:

```go include=ch06/openrouter/client.go tag=call-error
```

With both in the chain, the program can say in plain words why a run ended, checking the specific causes before the general kinds:

```go include=ch06/main.go tag=why-stopped
```

"The model call timed out, worth retrying" and "cancelled" call for different responses, and only the cause tells them apart.

## Between turns, and the calls that didn't run

Every model call notices cancellation on its own, because the context reaches the HTTP request.
The loop also checks between turns, so it doesn't start a call it would cancel at once:

```go include=ch06/agent/loop.go tag=before-turn
```

When cancellation arrives partway through a reply that asked for several tools, some calls have run and some haven't.
Every tool call in a conversation needs a matching result, or the next request to the model is rejected, so the calls that didn't run get a result that says why:

```go include=ch06/agent/loop.go tag=run-tools
```

## Tools that start processes

A tool that runs `go test` starts a process, and a cancelled context has to reach it too.
`exec.CommandContext` kills the process when the context is done:

```go include=ch06/agent/gotest.go tag=go-test-run
```

Three details here are easy to miss:

- `go test` gets its own `-timeout`, three quarters of the tool's, so the tests usually stop themselves first and report which test hung.
- `cmd.WaitDelay` bounds how long `Run` waits for the output pipes after the process is killed; a test that started a child process can otherwise keep them open.
- The tool checks `ctx.Err()` before looking at the command's error, because a killed process reports only "signal: killed", and the cause says why it was killed.

A tool written in Go, rather than a process, must check its context itself, by passing it to every call that can block.
A tool that ignores its context can't be stopped: a timeout wrapper around it still has to wait until the tool returns, however late.

## A timer as a cause: the idle stream

A streamed reply can stop arriving without ending: the connection stays open and nothing comes.
A deadline for the whole call doesn't fit, because a long reply legitimately takes minutes.
What fits is a timer that is reset every time a piece arrives, and cancels with its own cause when it fires:

```go include=ch06/openrouter/stream.go tag=stream
```

`time.AfterFunc` runs the cancellation on the timer's own goroutine.
The timer is armed only while the program is waiting for bytes, by a wrapper around the response body:

```go include=ch06/openrouter/sse.go tag=idle
```

Each `Read` resets the timer before it waits and stops it once data arrives, so the timeout measures how long the connection has been silent, and time the program spends handling an event never counts against it.
The cause, `ErrStreamIdle`, is what lets the program report "the stream went quiet, worth retrying" instead of "cancelled".

## Work that must finish anyway

Some work should finish even when the run is being cancelled: saving the checkpoint that preserves what the run has done, or writing the record of a tool call that already changed something.
`context.WithoutCancel` returns a context that keeps the parent's values but never ends with it:

```go include=ch15/agent/journal.go tag=save
```

Use it narrowly, for short writes that must complete, and give the operation its own timeout if it can block, since nothing else will stop it.

A graceful shutdown goes one step further: in-flight work gets a grace period before it is cancelled.
This function builds that from the same parts: a context detached with `WithoutCancel`, and a goroutine that cancels it with the original cause, after the grace period when the cause is a shutdown and at once for anything else:

```go include=ch15/agent/shutdown.go tag=graced
```

The goroutine has an owner: the returned `stop` function ends it, so a run that finishes early doesn't leave it waiting.

## Testing timeouts without waiting

A test for a two-minute timeout shouldn't take two minutes.
`testing/synctest` gives the test its own fake clock, which moves forward only when every goroutine in the test is blocked, and then straight to the next timer:

```go include=ch06/openrouter/client_test.go tag=synctest
```

The test checks both the cause and the kind of error, and that the call gave up after exactly two minutes of fake time.
It finishes instantly.

## Mistakes to avoid

- **`context.Background()` deep inside the code.** It cuts the chain, and nothing under it can be cancelled. Take a `ctx` parameter instead.
- **A context stored in a struct.** Each call should get its own; a stored one outlives the request it belonged to.
- **A forgotten `cancel`.** Every `WithTimeout`, `WithCancel`, and their `Cause` forms returns one; `defer` it, or the timer and goroutine it holds linger.
- **A timeout with no cause.** "Context deadline exceeded" from somewhere in a run is hard to act on; a cause names the deadline.
- **A tool that ignores its context.** It can't be stopped, and a run can't finish shutting down while it runs.
- **Comparing error strings.** Check `errors.Is(err, context.Canceled)` and your own causes with `errors.Is`; the text of a cancellation error differs between transports.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch06/openrouter -run 'TestTimeoutSynctest|TestStreamIdleSynctest' -v
$ go test ./ch06/agent -run TestCancelPartwayThroughAReply -v
```

1. In `TestTimeoutSynctest`, change the client's timeout and watch the test check the new duration, still instantly.
2. In `ch06/openrouter/client.go`, change `WithTimeoutCause` to `WithTimeout` and drop the cause: which tests fail, and what does `whyStopped` report for a call timeout now?
3. Write a tool that sleeps without checking its context, wrap it with a short timeout, and see when the run can finish.
