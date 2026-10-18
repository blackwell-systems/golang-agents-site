---
title: Why Go for agents, from the book's evidence
short: Why Go
description: An agent spends most of its time waiting on a network, running work side by side, and deciding when to stop. Four places where Go's standard tools did that job in the book's code, with the numbers, and what Go costs you here.
chapters: 5, 9, 11, 12, 15, 16
date: 2026-10-18
weight: 90
---

Very little of an agent is machine learning.
The model lives behind an HTTP API; the program you write sends requests, waits, runs tools, keeps a budget, and decides when to stop.
That is the kind of program Go was built for, and the case for Go here is best made from what the book's code did, with its measurements, rather than from a list of language features.

Below are four places where Go's own tools carried the weight, and then the costs of choosing Go, since a fair comparison names both.

## One value carries the deadline and the reason

Every call in the book's agent takes a `ctx context.Context` first.
It carries cancellation and deadlines through the HTTP request, the stream reader, and the `go test` command a tool starts, so a Ctrl-C reaches all of them without any of them knowing about the others.

A context can also carry why it was cancelled.
The book uses that in three places, each of which needed the reason and not only the fact:

- **Stop reasons** (chapter 5). A cancelled run returns an error that holds both `context.Canceled` and its cause, so a caller learns why the run ended as well as that it did.
- **Retries** (chapter 12). A call that ran out of its own time limit deserves another attempt; a job whose caller's deadline passed does not. Both fail with `context.DeadlineExceeded`; the call's limit is set with `context.WithTimeoutCause`, so the classifier checks the caller's context first and retries only a timeout of the call's own.
- **Shutdown** (chapter 15). A platform stopping the service sends SIGTERM and kills the process after a grace period. The service cancels the run with `ErrShutdown` as the cause, and the loop saves its state on the way out.

Here is the shutdown, from the companion code:

```go include=ch15/durable.go tag=shutdown
```

Cancelling rather than waiting was a measured choice.
Kubernetes waits 30 seconds by default and `docker stop` waits 10.
Across chapter 12's live runs, the median model call took 2.35 seconds, but 9 of 209 calls ran past 10 seconds and the slowest took 27.5, so waiting for the call in progress could have run out the grace period.

## Goroutines, with tools that check them

Running a reply's tool calls side by side is a few lines with `sync.WaitGroup.Go`: one goroutine per call, each writing only its own slot of the results slice (chapter 9).
The part worth having is the checking.
`go test -race` runs on every test in the book, so a shared write between those goroutines would fail a test run instead of waiting to be noticed in review.
Go 1.27's `goroutineleak` profile finds goroutines blocked forever on a channel nothing can reach; chapter 9 writes a fan-in that leaks on purpose and a test that catches both stuck senders.

The measurements also say where concurrency didn't help.
Replaying one recorded run's 87 tool calls, the 60 read-only calls added up to about a seventh of a second, so running them in parallel saved milliseconds; the time went to model calls and to the two tools that compile code.
And running 23 agents four at a time finished no sooner than one agent on the same batch (see [measured mistakes](/findings/measured-mistakes/#fan-out)).
Goroutines make parallel work cheap to write and safe to check; whether it pays is a separate measurement.

## The standard library covers the wire

Through chapter 10, the book's main module has no dependencies at all: `net/http` for the API, `encoding/json/v2` for the format, `bufio` for the stream, `testing` and `httptest` for offline tests.
Every byte the agent sends is something you wrote and can print.

Chapter 11 put that to a check by encoding the same request, the last one of a run recorded in chapter 10, with the hand-written client and with OpenRouter's Go SDK.
The two came to 55,162 and 55,227 bytes, the same content spelled almost the same way.
The hand-written client isn't smaller; the point is that you can see what it sends, so when a provider's behavior changes you can tell whether your request did.

## One static binary

The finished agent builds to one binary with nothing to install beside it, which suits both a command people download and a container that a platform starts and stops.
Chapter 16 measured the same task built three ways, for linux/amd64: 10.6 MB on the book's own parts, and 25.4 MB and 29.3 MB on two agent frameworks.
The chapter is careful about what that means: for a long-running server the difference rarely counts, and for a downloaded tool or a function started cold on each request it can.

## What Go costs here

The book states these plainly, and so should any page arguing for Go:

- **A smaller agent ecosystem.** Python has more agent libraries, and research code and provider examples often appear there first.
- **You may build parts that exist elsewhere.** The book writes its own loop, client, and safety code; under chapter 16's task sit 5,351 lines of loop, client, and safety code that a framework would partly replace (about 1,500 of them are safety code neither framework tested provides), and the chapter maps five Go frameworks against the parts you built.
- **Some libraries trail a Go release.** On Go 1.27, one framework's JSON library warned at startup that it supported Go only up to 1.26 and fell back to `encoding/json` (chapter 16).

None of that argues against other languages.
It says what Go makes cheap for an agent: carrying the reason a call stopped, running work side by side under a race checker, keeping every byte visible, and shipping one binary that stops on time.
Whether those matter more than a larger ecosystem depends on your agent, and [the chapter 16 comparison](/providers/libraries/) is a place to start weighing it.
