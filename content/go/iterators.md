---
title: "Iterators in Go agents: iter.Seq, range-over-func, and stopping early"
short: Iterators in an agent
description: How range-over-func iterators fit an AI agent in Go. What iter.Seq and iter.Seq2 are, how yield and break work, writing an iterator that adapts a whole reply into a stream, consuming a stream of values and errors, wrapping one as middleware with cleanup in a defer, stopping a run by breaking out of the loop, and the mistakes, from the runtime panic to an iterator ranged twice.
card: Streams and runs as iterators: yield, break, cleanup in a defer, and the mistakes that panic.
group: Concurrency
chapters: 6, 9, 11, 12
stage: all
level: intermediate
date: 2026-10-19
weight: 70
art: go-iterators
---

An agent produces things over time: the pieces of a streamed reply, then a tool call, then its result, then the next turn.
Since Go 1.23, a function can hand such a sequence to an ordinary `for ... range` loop, and the caller reads it as it is made.
The companion code uses that shape throughout: a model's stream is an `iter.Seq2[Delta, error]`, one piece of a reply per step, and a whole run is an `iter.Seq[Event]`, one thing that happened per step.
This page covers how these iterators work and the patterns an agent needs, with code from the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), where it is tested; the [streaming guide](/build/streaming/) shows the server-sent events reader built the same way, and [Streams in Go](/go/streams/) explains the `io.Reader` and `bufio.Scanner` underneath it.

## An iterator is a function

The `iter` package defines two types (an [iterator](/glossary/#iterator) is a function that hands values to a `range` loop one at a time).
The square brackets hold [type parameters](/glossary/#type-parameter): `iter.Seq[Event]` is a sequence whose values are `Event`s, and `V` below stands for whatever type you pick.

- `iter.Seq[V]` is `func(yield func(V) bool)`: a function that calls `yield` once for each value.
- `iter.Seq2[K, V]` is `func(yield func(K, V) bool)`: the same with two values per step, such as a value and an error.

`for v := range seq { ... }` calls `seq` with a `yield` function made from the loop's body.
Each call to `yield` runs the body once.
When the body finishes normally, `yield` returns true and the iterator goes on; when the body leaves the loop, with `break`, `return`, or a panic, `yield` returns false, and the iterator must stop.
Calling `yield` again after it returned false is a bug the runtime catches: the program panics with "range function continued iteration after function for loop body returned false".

The iterator runs in the caller's goroutine, and nothing happens until the caller ranges over it.
There is no channel to close and no goroutine to stop, unless the iterator starts one itself.

## Writing one: a whole reply as a stream

The agent's loop reads model replies as streams.
A model that returns a whole reply at once, such as a scripted model in a test, is adapted with a few lines:

```go include=ch06/agent/stream.go lines=52..85
```

Two habits are worth copying.
An error is a step of its own, `yield(Delta{...}, err)`, followed by `return`; the delta beside it carries the usage, since a failed call can still have cost something.
And every later `yield` checks its result, returning as soon as the caller has stopped reading.

## Reading one: values and errors

A caller of an `iter.Seq2[Delta, error]` checks the error on every step, the way it would check the error from any call:

```go include=ch15/agent/stream.go tag=collect
```

`Collect` assembles a whole reply from a stream, for code that wants the reply and not its pieces.
It stops at the first error, keeping the usage the failed step reported, and it handles one more kind of step: a `Delta` with its `Reset` flag set, which a retry sends when a stream starts over, meaning the pieces before it belonged to a failed attempt and are discarded. See [retries and observability](/production/retries-and-observability/).

## Wrapping one: middleware with cleanup

Because a stream is just a function, wrapping one is a function too.
The cost meter wraps a model's stream, passes each step through, and records what the call used:

```go include=ch15/agent/meter.go tag=metered highlight=8
```

The recording is in a `defer`, so it runs whether the stream ended, failed, or the caller stopped reading partway.
That is the iterator's version of a `Close` method: cleanup in a deferred call inside the iterator runs when the caller's loop ends, for whatever reason, without the caller doing anything.

## Stopping early

A whole run is an iterator of events, so a caller that only wants the result reads until the last event:

```go include=ch06/agent/loop.go tag=run
```

A `return` inside the loop ends the range; the run's iterator sees its `yield` return false and stops.
The run's iterator cancels its own [context](/go/context/) when that happens, through a deferred `cancel`, so breaking out of the loop stops the model call in progress.
Stopping a run is as simple as leaving the loop that reads it, and `TestBreakOutOfEvents` checks that breaking out stops the run.

## Iterators with goroutines inside

An iterator can start goroutines, as long as it stops them before it returns.
The fan-in that merges several runs' events into one iterator does exactly that, and the [goroutines page](/go/goroutines/#merging-several-runs-into-one-stream) walks through how it releases every goroutine when the caller stops reading.
That is the trade-off between the two ways of producing values over time: an iterator with no goroutine inside can't leak one, while a channel always needs a goroutine on the other end, and someone responsible for stopping it.

## The standard library's iterators

Go 1.23 added iterators across the standard library, and they combine with the ones you write:

- `maps.Keys(m)` and `maps.Values(m)` iterate over a map.
- `slices.Collect(seq)` gathers an iterator into a slice, and `slices.Sorted(seq)` gathers and sorts it.
- `slices.All`, `slices.Values`, and `slices.Backward` iterate over a slice.
- `iter.Pull` turns an iterator into a `next` function and a `stop` function, for code that has to step through two sequences at once; call `stop` when you finish early, or the iterator is never released.

The agent's tool lookup uses two of them in one line, `slices.Sorted(maps.Keys(byName))`, to list the real tools in a stable order for the error the model reads; the [errors page](/go/errors/#errors-the-model-reads) shows it.

## Mistakes to avoid

- **Calling `yield` after it returned false.** The runtime panics. Check its result on every call, and return when it is false.
- **Ignoring the error in an `iter.Seq2[T, error]`.** It is a step like any other; check it before using the value.
- **Cleanup after the loop instead of in a `defer`.** Code after the last `yield` doesn't run when the caller breaks out; a deferred call does.
- **Ranging over the same iterator twice.** An iterator like `Events` runs its work each time it is ranged over: ranging twice runs the agent twice. Keep the results, not the iterator.
- **A goroutine inside an iterator, left running.** If an iterator starts goroutines, it stops them and waits for them before it returns.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch06/agent -run 'TestBreakOutOfEvents|TestEventsOrder|TestStreamErrorDropsPartialReply' -v
```

1. In `ch06/agent/stream_test.go`, change `watchedStream` to call `yield` three times without checking its result, run `TestBreakOutOfEvents`, and read the panic.
2. Write a filter, a function that takes an `iter.Seq[Event]` and returns one holding only the tool-call events, and range over it in place of `Events`.
3. Range over `Events` twice with the same arguments and a scripted model, and count the model calls.
