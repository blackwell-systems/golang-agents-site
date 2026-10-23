---
title: "context.Context in a Go agent: cancellation, deadlines, and causes"
short: context.Context in an agent
description: How context.Context works and how an AI agent in Go uses it. The interface and the tree of contexts, stopping a run on Ctrl-C, waiting on ctx.Done in your own code, a deadline for each model call, saying why a run stopped with WithTimeoutCause and context.Cause, cancelling the commands tools start, an idle timer on a stream, work that must finish with WithoutCancel, testing timeouts with synctest, and the common mistakes.
card: Stop a run, give each call a deadline, say why it stopped, and let the work that must finish still finish.
group: Control and failure
chapters: 1, 2, 5, 6, 12, 15
stage: all
level: intermediate
date: 2026-10-19
weight: 10
art: go-context
---

An agent run makes many slow calls: a model call that takes half a minute, a tool that runs a test suite, another model call, and so on for dozens of turns.
At any point, the person who started it may want it stopped, a call may hang, or the process may be shutting down.
In Go, all three travel the same way, through a `context.Context` passed into every function that waits on something.

```definition
term: context.Context

A **`context.Context`** carries a cancellation signal, an optional deadline, and a reason for stopping down a chain of calls; when it is done, every function that was handed it should stop waiting and return.
```

This page goes through the ways an agent uses one, each with code from the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), where it is tested.
Several of the tools are recent additions to Go, so here is what each needs:

| Feature | Since |
| --- | --- |
| `signal.NotifyContext` | Go 1.16 |
| `context.Cause`, `context.WithCancelCause`, `exec.Cmd.WaitDelay` | Go 1.20 |
| `context.WithTimeoutCause`, `context.WithoutCancel`, `context.AfterFunc` | Go 1.21 |
| `testing.T.Context` | Go 1.24 |
| `testing/synctest` | Go 1.25 |
| `errors.AsType` | Go 1.26 |

The companion code needs Go {{code go-version}}; with an older toolchain, `go` downloads the right one when it reads the module's `go.mod`.

## Without a context

Take the context away and two things go wrong.

The first is Ctrl-C.
By default, a Go program that gets an interrupt exits on the spot, and its deferred functions never run.
For an agent, that means a model call dropped partway, which the provider may bill anyway; a tool stopped in the middle of an action, such as writing a file or filing an issue; no checkpoint saved, so the run can't resume; and no report of what it spent or why it ended.
Stopped with a plain `kill`, the agent's process gets SIGTERM alone, so a `go test` or a tool server it started can keep running on its own.

The second is that every kind of waiting needs its own way to stop.
You can catch the signal yourself, but then something has to reach everything that is blocked: an HTTP request, a child process to kill, a timer to stop, each of your own goroutines waiting on a channel.
Each has a different mechanism, and before Go 1.7 brought `context` into the standard library, each library had its own; `net/http` still carries one of them, the `Cancel` field on a request, now deprecated in favor of the request's context.
`context` replaces all of those with one stop signal that reaches every layer and says why it fired.
Every blocking call agrees on it, from `net/http` and `os/exec` to the model providers' SDKs and your own code, and it carries the reason with it.

The rest of this page is that alternative, piece by piece.

## How a context works

`context.Context` is an interface with four methods:

- `Done()` returns a channel that is closed when the context ends, for use in a `select` ([Goroutines with an owner](/go/goroutines/) explains channels and `select`).
- `Err()` is `nil` while the context is live, then `context.Canceled` or `context.DeadlineExceeded`.
- `Deadline()` reports when the context will end on its own, if it has a deadline.
- `Value(key)` returns a value stored in the context, or `nil`.

A context is never changed.
Functions such as `context.WithCancel` and `context.WithTimeout` take a parent and return a new child, with a function that cancels it, so the contexts in a program form a tree with `context.Background()` at the root.
Cancellation travels down the tree and never up:

```diagram
name: ctx-cancel
alt: A tree of contexts. main, made with signal.NotifyContext, derives the run; the run derives a model call, a tool call, and, through a dashed link labeled values only, no cancellation, a checkpoint save made with WithoutCancel. The model call derives its HTTP request. The model call and its request are highlighted: the call's deadline passed, and the request ended with it, while the run above and the tool call beside them go on.

When a model call's deadline passes, the call and the HTTP request under it end; the run above it and the tool call beside it go on. Cancelling the run would end every context under it except the save, which kept the run's values and dropped its cancellation.
```

Two more rules complete the picture.
The first cancellation wins: once a context is done, later calls to its cancel function change nothing, including its cause.
And `context.Cause` (explained under "Say why it stopped" below) always has an answer: the cause given when the context was cancelled, the parent's cause when cancellation came from above, and `Err()` itself when nobody gave one.

`context.Background()` is the root for `main`, for `init`, and for anything that starts a whole operation.
`context.TODO()` is the same empty context under another name, a marker for code that should be passed a context and isn't yet; linters and readers can find it.

## One context, from main down

The context starts in `main`, and the first thing it is good for is Ctrl-C:

```go include=ch01/main.go tag=signal
```

`signal.NotifyContext` returns a context that is cancelled when the process gets an interrupt.
From here the context is passed down, as the first parameter of every function that does I/O or can block: the loop, the model client, each tool.
Nothing below `main` makes a fresh `context.Background()`, because a fresh context cuts the chain: whatever ran under it would ignore Ctrl-C.

Passing it as a parameter, never storing it in a struct, keeps each call's context its own.
A client that stored one context would cancel every later request when the first request was cancelled.

A server starts its chains elsewhere.
In an HTTP handler, `r.Context()` is the request's context, and it ends when the client disconnects or the handler returns.
An agent run started from a handler derives its context from it, so a person who closes the browser tab stops the run; the [browser streaming example](/run/stream-to-browser/) is built that way.

## A deadline for each model call

A model call that hangs should fail on its own, without waiting for a person to notice.
The client gives every request its own deadline, derived from the run's context:

```go include=ch06/openrouter/client.go lines=72..84 highlight=4-6
```

The derived context ends at whichever comes first: the caller's cancellation, or the call's own timeout.
`defer cancel()` releases the timer when the call returns early, which is most of the time; without it, the timer lives until it fires.

The run has limits of its own above this, a turn limit and a cost budget, so a deadline for the whole run is rarely needed.
When it is, wrap the run's context the same way, and each call's shorter deadline still applies inside it.
`context.WithDeadline` does the same with a time instead of a duration.

Code can also read the time left: `deadline, ok := ctx.Deadline()` reports the deadline, if there is one, and `time.Until(deadline)` how long remains.
An agent can use it to skip starting work that can't finish in time, such as a model call that usually takes 40 seconds when 10 remain, and stop with a clear reason instead of a timeout partway through.

## Say why it stopped

A cancelled context reports one of two errors, `context.Canceled` or `context.DeadlineExceeded`, and neither says which deadline passed.
"Deadline exceeded" from a run with a call timeout, a tool timeout, and a test timeout is a puzzle.

`context.WithTimeoutCause` and `context.WithCancelCause` attach a cause, an error of your own, and `context.Cause` reads it back.
Above, the call's timeout carries `ErrCallTimeout`.
Go's HTTP transports don't all report a cancelled request the same way, so the client puts both the cause and the kind of cancellation into the error it returns, and callers can check either one with `errors.Is`.

<div class="hood">

The detail: when a request is cancelled, Go's HTTP/1 transport returns the cause, and its HTTP/2 transport returns only the kind of cancellation.
The client joins both into the error:

```go include=ch06/openrouter/client.go tag=call-error
```

</div>

With both in the chain, the program can say in plain words why a run ended, checking the specific causes before the general kinds:

```go include=ch06/main.go tag=why-stopped
```

The last check, `errors.AsType[*openrouter.APIError](err)`, has a type in square brackets: it says which kind of error to look for. `AsType` returns it already converted to that type, with `ok` false if the chain has none. The [errors page](/go/errors/) covers it, and the [glossary](/glossary/#type-parameter) explains the brackets.

"The model call timed out, worth retrying" and "cancelled" call for different responses, and only the cause tells them apart.

## Between turns, and the calls that didn't run

Every model call notices cancellation on its own, because the context reaches the HTTP request.
The loop also checks between turns, so it doesn't start a call it would cancel at once:

```go include=ch06/agent/loop.go tag=before-turn stages=all:4
```

When cancellation arrives partway through a reply that asked for several tools, some calls have run and some haven't.
Every tool call in a conversation [needs a matching result](/basics/tool-calling/), or the next request to the model is rejected, so the calls that didn't run get a result that says why:

```go include=ch06/agent/loop.go tag=run-tools stages=all:3
```

In `runTools`, `yield` is a function the caller passes in; the loop calls it to hand over each message as it is made, and it returns false when the caller wants no more. [Iterators in an agent](/go/iterators/) explains the pattern.

## Waiting in your own code

The standard library's blocking calls take a context and stop on their own.
Code that waits on its own channels and timers has to watch the context itself, with a `select` on `ctx.Done()` beside whatever it is waiting for.
The agent limits how many tool calls run at once with a buffered channel of slots, and waiting for a slot gives up when the context ends:

```go include=ch15/agent/slots.go tag=slots
```

A rate limiter that waits on a timer does the same, and stops the timer when it leaves early:

<div class="hood">

```go include=ch09/agent/ratelimit.go tag=wait
```

</div>

Both return the cause, so a caller sees why the wait ended.
Without the `ctx.Done()` case, a goroutine blocked on a full channel waits forever after its run is gone: a goroutine leak, which in a long-running service adds up one stopped run at a time.
[Every goroutine needs an owner and a way to stop](/go/goroutines/), and for most of them the way to stop is a context.

## Tools that start processes

A tool that runs `go test` starts a process, and a cancelled context has to reach it too.
`exec.CommandContext` kills the process when the context is done:

```go include=ch06/agent/gotest.go tag=go-test-run
```

Three details here are easy to miss:

- `go test` gets its own `-timeout`, three quarters of the tool's, so the tests usually stop themselves first and report which test hung.
- `cmd.WaitDelay` bounds how long `Run` waits for the output pipes after the process is killed; a test that started a child process can otherwise keep them open.
- The tool checks `ctx.Err()` before looking at the command's error, because a killed process reports only "signal: killed", and the cause says why it was killed.

A tool that is a Go function, rather than a separate process, must check its context itself, by passing it to every call that can block.
A tool that ignores its context can't be stopped: a timeout wrapper around it still has to wait until the tool returns, however late.

Some calls can't take a context at all, such as a read from a connection or a pipe.
`context.AfterFunc` runs a function when the context is done, and closing the connection there unblocks the read.
The [MCP client](/build/mcp/) starts a server process and talks to it over pipes, and arranges exactly that while it connects:

```go include=ch15/mcp/client.go lines=533..539 title="ch15/mcp/client.go, starting an MCP server"
```

The `stop` function it returns cancels the arrangement, for when the work finishes first.

## A timer as a cause: the idle stream

A [streamed reply](/build/streaming/) can stop arriving without ending: the connection stays open and nothing comes.
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

The listing's comment mentions the resolution ladder: the three steps a resumed run uses to settle a tool call whose result was never recorded. It runs the call again if a repeat is harmless, asks the service whether the call happened, and only then asks a person. [Durable runs](/production/durable-runs/#the-resolution-ladder) covers it.

Use it narrowly, for short writes that must complete, and give the operation its own timeout if it can block, since nothing else will stop it.

A graceful shutdown goes one step further: in-flight work gets a grace period before it is cancelled.

<div class="hood">

This function builds that from the same parts: a context detached with `WithoutCancel`, and a goroutine that cancels it with the original cause, after the grace period when the cause is a shutdown and at once for anything else:

```go include=ch15/agent/shutdown.go tag=graced
```

The goroutine has an owner: the returned `stop` function ends it, so a run that finishes early doesn't leave it waiting.

</div>

## Testing timeouts without waiting

A test for a two-minute timeout shouldn't take two minutes.
`testing/synctest` gives the test its own fake clock, which moves forward only when every goroutine in the test is blocked, and then straight to the next timer:

```go include=ch06/openrouter/client_test.go tag=synctest
```

The test checks both the cause and the kind of error, and that the call gave up after exactly two minutes of fake time.
It finishes instantly.
`t.Context()` is a context that is cancelled when the test ends, so anything the test starts with it stops with the test.

## Values in a context

A context can also carry values, and the rule is narrow: values that describe the request, which pass through layers that don't know about them, never optional parameters.
The agent uses one, the ID of the tool call that a tool is running for, so that logging and tracing wrappers can name the call without every tool passing it along:

```go include=ch12/agent/traced.go lines=12..20 title="ch12/agent/traced.go"
```

The key is an unexported type, `callKey`; since only this package can name that type, only this package can set or read the value, and `CallFrom` hands it out with its real type.

## Mistakes to avoid

- **`context.Background()` deep inside the code.** It cuts the chain, and nothing under it can be cancelled. Take a `ctx` parameter instead.
- **A context stored in a struct.** Each call should get its own; a stored one outlives the request it belonged to.
- **A forgotten `cancel`.** `WithTimeout`, `WithCancel`, and their `Cause` forms each return one; `defer` it, or the timer and goroutine it holds linger. `go vet` reports a `cancel` that some path never calls.
- **A timeout with no cause.** "Context deadline exceeded" from somewhere in a run is hard to act on; a cause names the deadline.
- **A tool that ignores its context.** It can't be stopped, and a run can't finish shutting down while it runs.
- **Parameters passed as context values.** A value the function needs belongs in its signature; a context value is invisible to the caller and unchecked by the compiler.
- **Comparing error strings.** Check `errors.Is(err, context.Canceled)` and your own causes with `errors.Is`; the text of a cancellation error differs between transports.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch06/openrouter -run 'TestTimeoutSynctest|TestStreamIdleSynctest' -v
$ go test ./ch06/agent -run TestCancelPartwayThroughAReply -v
```

1. In `TestTimeoutSynctest`, change the client's timeout and watch the test check the new duration, still instantly.
2. In `ch06/openrouter/client.go`, change `WithTimeoutCause` to `WithTimeout` and drop the cause: which tests fail, and what does `whyStopped` report for a call timeout now?
3. Run a tool that sleeps without checking its context. `TestTimeouts` in `ch12/agent/wrappers_test.go` already has one, named `deaf`, under a one-second timeout. Change its `time.Sleep(3 * time.Second)` to 5 seconds and run `go test ./ch12/agent -run TestTimeouts -v`. The test fails with `ignores its context: "done", <nil> after 5s`: the timeout fired, but the tool never looked, so the call finished only when the tool chose to (the test uses a fake clock, so it still runs instantly).

Undo your edits with `git restore .` in the `code` directory.
