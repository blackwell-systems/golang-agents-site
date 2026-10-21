---
title: "Goroutines with an owner: concurrency patterns for Go agents"
short: Goroutines with an owner
description: Concurrency in a Go agent, with every goroutine owned and stoppable. sync.WaitGroup.Go, merging several agent runs into one stream without leaking a goroutine, requests in flight with their cancel functions behind a mutex, shutting down in order, limiting how much runs at once, and finding leaks with the race detector and the goroutine leak profile.
card: Start goroutines with an owner and a way to stop, merge their results, and prove none is left behind.
group: Go in depth
chapters: 6, 9, 10, 15
stage: all
date: 2026-10-19
weight: 88
art: go-goroutines
---

Go makes starting concurrent work cheap: `go f()` is all it takes.
An agent uses that everywhere, for tool calls in parallel, one run per batch of work, a server answering several requests at once, a timer that watches a stream.
The hard part is never starting a goroutine; it is knowing who stops it, and making sure it can stop.

The rule this page follows comes from the companion code's style guide: every goroutine has an owner and a way to stop.
The owner is the code that waits for it to finish, and the way to stop is almost always a context.
The examples are from the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), where they are tested with the race detector.

## Start and wait: sync.WaitGroup.Go

`sync.WaitGroup` is how an owner waits for the goroutines it started.
Since Go 1.25 it has a `Go` method, which starts a function in a goroutine and counts it in one call, `wg.Go(func() { ... })`.
It replaces the older three-step form, `wg.Add(1)`, `go func() { defer wg.Done(); ... }()`, and the mistakes that came with it: an `Add` placed inside the goroutine, where `Wait` can run before it, or a `Done` forgotten on one return path.

A closure started in a loop can use the loop's variables directly.
Since Go 1.22, each iteration of a `for` loop has its own copy of its variables, so the old habit of copying them first, `name := name`, is no longer needed.

## Merging several runs into one stream

Running several agents at once, one per group of crash reports, is a [fan-out](/findings/fan-out/); reading all their events as one stream is a fan-in.
Here is the fan-in, which starts one goroutine per run and sends every event through one channel:

```go include=ch15/agent/fanin.go tag=merge-start
```

Each goroutine forwards its run's events, tagged with the run's name.
A separate goroutine waits for all of them and then closes the channel, so the reader knows when every run has finished; closing a channel is the sender's job, and here the senders' owner does it once, after the last of them is done.

The reading side is where the leaks are avoided:

```go include=ch15/agent/fanin.go tag=merge-read
```

When the caller stops reading, by breaking out of its `range` loop, three things happen in order.
`quit` is closed, so a goroutine waiting to send gives up and returns, which also ends its run.
The context is cancelled, which stops the runs that were busy rather than sending.
And the reader drains the channel until it closes, which happens only once every goroutine has returned, so when `Merge` returns, nothing it started is still running.

Look at what the send doesn't wait on: the context.
After Ctrl-C, each run still has a last event to report, the one that says how it stopped, and the caller is still reading.
A send that also gave up on `ctx.Done()` would sometimes drop it, because when several cases of a `select` are ready, Go picks one at random; `TestMergeCancel` checks that every run's last event arrives.

Without both protections, the `quit` case and the drain, a goroutine blocked on a send that nobody will ever receive stays blocked for the life of the process: a goroutine leak.
In a service that runs agents all day, each abandoned run leaves one behind.

## Proving there's no leak

A leak doesn't fail anything; it just accumulates.
The companion code tests for one with the goroutine leak profile in `runtime/pprof`, which reports goroutines blocked forever on a channel or lock that no running code can reach anymore:

```go include=ch15/agent/shared_test.go tag=test-leak
```

The test runs a deliberately broken version, `leakyMerge`, beside the real one, so it checks both that the profile finds a leak when there is one and that `Merge` leaves none.

The other tool is the race detector, `go test -race`, which reports two goroutines touching the same memory without synchronization while the tests run.
The companion code's checks run every test with it.

## Requests in flight: a map behind a mutex

A server that answers several requests at once needs to know which are running, to cancel one when the client asks.
The [MCP](/build/mcp/) server keeps each request's cancel function in a map, and starts each request in its own goroutine:

```go include=ch15/mcp/server.go tag=inflight
```

The map is shared by every request goroutine and the reading loop, so it sits behind a `sync.Mutex`, and every access locks it:

```go include=ch15/mcp/server.go lines=155..172
```

`add` refuses a request ID already in use and cancels the context it was given, so nothing leaks from a request that never starts.
`remove` runs when the request's goroutine ends, and cancels its context too, which releases what `context.WithCancelCause` holds.

## Shutting down in order

The server's main loop owns every request goroutine, and its deferred calls put the shutdown in order:

```go include=ch15/mcp/server.go tag=serve highlight=8-9
```

Deferred calls run in reverse order, so `stop` runs before `calls.wg.Wait()`: first every running request is cancelled, then the loop waits for each goroutine to return.
The other order would wait for requests that nobody has told to stop.
When the client closes its end, `finish` first gives the requests a short grace period to answer:

```go include=ch15/mcp/server.go lines=177..188
```

The goroutine inside `finish` can outlive the call, when the grace period runs out first; it ends when the last request does.
The deferred calls cancel the requests' contexts right after, so that happens soon.

## Limiting how much runs at once

Starting a goroutine per task is fine for a handful; for hundreds of tool calls or crash reports, something has to cap how many run at once, both to stay within a provider's rate limit and to bound memory.
The companion code uses a buffered channel as a counting semaphore, with an `Acquire` that gives up when the context ends; the [context page](/basics/context/#waiting-in-your-own-code) shows it, beside a rate limiter that spaces requests out in time.
The [parallel tool calls](/build/parallel-tool-calls/) guide shows the runner that decides which of a reply's tool calls may run together, and what running them in parallel saved.

## Mistakes to avoid

- **A goroutine with no way to stop.** Every blocking operation inside it needs a `ctx.Done()` case, or a channel its owner closes.
- **A send nobody will receive.** If the reader can leave early, the sender needs another way out, like `Merge`'s `quit`, or the reader has to drain what is left.
- **Closing a channel from the receiving side, or more than once.** Close it from the sender's side, once, after the last send; with several senders, their owner closes it after they finish.
- **`wg.Add` inside the goroutine.** Use `wg.Go`, which counts before it starts.
- **Waiting before cancelling.** At shutdown, cancel first, then wait; deferred calls run last-in, first-out, so defer the wait before the cancel.
- **Shared maps without a lock.** A map read and written from several goroutines needs a mutex; the race detector finds the ones the tests reach.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test -race ./ch15/agent -run 'TestMerge' -v
```

1. In `TestMergeLeak`, read the leak profile the test logs, and find `leakyMerge`'s blocked senders in it.
2. In `Merge`, remove both the `case <-quit` branch and the drain loop, run `TestMergeLeak` again, and see the profile name `Merge`'s blocked sends.
3. Put the `quit` case back and add a `case <-ctx.Done(): return` beside it, then run `TestMergeCancel`: some runs' last events go missing.
