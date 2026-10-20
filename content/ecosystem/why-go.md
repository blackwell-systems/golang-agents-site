---
title: Why Go for agents, from the book's evidence
short: Why Go
description: An agent spends most of its time waiting on a network, running work side by side, and deciding when to stop. Where Go fits that job, from its standard library and build speed to the infrastructure written in it, with numbers from recorded runs, and what Go costs you here.
card: Where Go's standard tools did an agent's work in the book's code, with numbers, and what Go costs.
group: Why Go
chapters: 5, 9, 11, 12, 15, 16
date: 2026-10-18
weight: 5
---

Very little of an agent is machine learning.
The model lives behind an HTTP API; the program you write sends requests, waits, runs tools, keeps a budget, and decides when to stop.
That is the kind of program Go was built for, and the companion code, with its measurements, makes the case better than a list of language features would.

Below are the places where Go fits that job, most with numbers from recorded runs, and then the costs of choosing Go, since a fair comparison names both.

## One value carries the deadline and the reason

Every call in the companion agent takes a `ctx context.Context` first.
It carries cancellation and deadlines through the HTTP request, the stream reader, and the `go test` command a tool starts, so a Ctrl-C reaches all of them without any of them knowing about the others.

A context can also carry why it was cancelled.
That is used in three places, each of which needed the reason and not only the fact:

- **Stop reasons**. A cancelled run returns an error that holds both `context.Canceled` and its cause, so a caller learns why the run ended as well as that it did.
- **Retries**. A call that ran out of its own time limit deserves another attempt; a job whose caller's deadline passed does not. Both fail with `context.DeadlineExceeded`; the call's limit is set with `context.WithTimeoutCause`, so the classifier checks the caller's context first and retries only a timeout of the call's own.
- **Shutdown**. A platform stopping the service sends SIGTERM and kills the process after a grace period. The service cancels the run with `ErrShutdown` as the cause, and the loop saves its state on the way out.

Here is the shutdown, from the companion code:

```go include=ch15/durable.go tag=shutdown
```

Cancelling rather than waiting was a measured choice.
Kubernetes waits 30 seconds by default and `docker stop` waits 10.
Across recorded live runs, the median model call took 2.35 seconds, but 9 of 209 calls ran past 10 seconds and the slowest took 27.5, so waiting for the call in progress could have run out the grace period.

## Goroutines, with tools that check them

Running a reply's tool calls side by side is a few lines with `sync.WaitGroup.Go`: one goroutine per call, each writing only its own slot of the results slice.
The part worth having is the checking.
`go test -race` runs on every test in the companion code, so a shared write between those goroutines would fail a test run instead of waiting to be noticed in review.
Go 1.27's `goroutineleak` profile finds goroutines blocked forever on a channel nothing can reach; the companion code writes a fan-in that leaks on purpose and a test that catches both stuck senders.

The measurements also say where concurrency didn't help.
Replaying one recorded run's 87 tool calls, the 60 read-only calls added up to about a seventh of a second, so running them in parallel saved milliseconds; the time went to model calls and to the two tools that compile code.
And running 23 agents four at a time finished no sooner than one agent on the same batch (see [measured mistakes](/findings/measured-mistakes/#fan-out)).
Goroutines make parallel work cheap to write and safe to check; whether it pays is a separate measurement.

## The standard library covers the wire

Until it adopts an official SDK, the companion code's main module has no dependencies at all: `net/http` for the API, `encoding/json/v2` for the format, `bufio` for the stream, `testing` and `httptest` for offline tests.
Every byte the agent sends is something you wrote and can print.

A recorded check put that to the test: the same request, the last one of a recorded run, encoded with the hand-written client and with OpenRouter's Go SDK.
The two came to 55,162 and 55,227 bytes, the same content spelled almost the same way.
What writing the client yourself buys is sight of every byte it sends, so when a provider's behavior changes you can tell whether your request did.

## One static binary

The finished agent builds to one binary with nothing to install beside it, which suits both a command people download and a container that a platform starts and stops.
The same task built three ways, for linux/amd64: 10.6 MB on the hand-written parts, and 25.4 MB and 29.3 MB on two agent frameworks.
The comparison is careful about what that means: for a long-running server the difference rarely counts, and for a downloaded tool or a function started cold on each request it can.

Runtime speed rarely matters for an agent.
The median model call in the recorded runs took 2.35 seconds while tools took milliseconds, so the case for Go rests on clarity, safety, and tooling, and raw speed hardly enters into it.

## Fast builds, on every turn

A coding agent builds and tests code on most turns, so the toolchain's speed is part of every run's wall time.
In one recorded run's trace, a single `run_tests` call, which builds the service and runs its tests with `go test`, took 4.8 seconds, while the read-only tools took milliseconds.
Replaying one recorded run's 87 tool calls, the 60 read-only ones added up to about a seventh of a second in all, as [parallel tool calls](/build/parallel-tool-calls/) describes.

Go compiles quickly and caches build results, so the second run of `go test` over unchanged packages is much faster than the first.

## The infrastructure is already Go

Agents that operate infrastructure work in Go's home territory.
[Kubernetes](https://github.com/kubernetes/kubernetes), [Docker's engine](https://github.com/moby/moby), [Terraform](https://github.com/hashicorp/terraform), and [Prometheus](https://github.com/prometheus/prometheus) are written in Go, and AWS, Google Cloud, and Azure publish official Go SDKs.
A tool that talks to those systems can call their own Go libraries instead of shelling out to a command-line tool and parsing its text.

## A small language keeps the harness reviewable

The harness is the code that decides what the model may do: which tools exist, what they may touch, when a person must approve.
Go has 25 keywords ([the Go specification lists them](https://go.dev/ref/spec#Keywords)), one formatting style enforced by `gofmt`, and few ways to write the same thing.
That makes it practical for the people responsible for an agent's safety to read all of its harness code.

## What Go costs here

These belong in any argument for Go:

- **A smaller agent ecosystem.** Python has more agent libraries, and research code and provider examples often appear there first.
- **You may build parts that exist elsewhere.** This agent writes its own loop, client, and safety code; under the comparison task sit 5,351 lines of loop, client, and safety code that a framework would partly replace (about 1,500 of them are safety code neither framework tested provides), and the comparison maps five Go frameworks against the parts you built.
- **Some libraries trail a Go release.** On Go 1.27, one framework's JSON library warned at startup that it supported Go only up to 1.26 and fell back to `encoding/json`.
- **Rust is a genuine alternative.** Rust frees memory deterministically with no garbage collector and its compiler rejects data races before the program runs, where Go finds races at run time with the race detector; for an agent, which spends most of its time waiting on a model, those strengths matter less than they do for a latency-critical service, but a team already fluent in Rust loses little by staying there.

None of that argues against other languages.
It says what Go makes cheap for an agent: carrying the reason a call stopped, running work side by side under a race checker, keeping every byte visible, and shipping one binary that stops on time.
Whether those matter more than a larger ecosystem depends on your agent, and [the ecosystem comparison](/ecosystem/libraries/) is a place to start weighing it.
