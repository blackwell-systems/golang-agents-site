---
title: A real agent that fixes a failing test
short: "ch04: A real agent fixes a failing test"
description: The same 30-line loop, now driven by a real model's recorded replies, with tools that run go test for real on your machine. Replay two models' different paths to the same fix, and a run that recovers from three bad tool calls.
card: The same loop on a real model's recorded replies, with go test running for real.
chapters: 4
stage: 3
date: 2026-10-18
weight: 20
art: ch04
---

This program is the first real agent in the companion code.
Nothing scripts the model's moves this time: it looks around a small Go project, runs the tests, works out where the bug is, and repairs it on its own.
Without an API key, the program replays the model's replies from a run recorded in October 2026, but only the replies are saved: every tool, `go test` included, runs for real each time.

## What it shows

- The loop from the [ch01 example](/run/ch01-scripted-loop/), unchanged, driving a client for a real model through the same `Model` interface.
- Tool calls on the wire: the client sends tool definitions and reads calls back, as [tool calling in Go, on the wire](/basics/tool-calling/) explains.
- A `run_tests` tool that copies the project to a fresh temporary directory and runs `go test` there, with a deadline and a scrubbed environment.
- Two models taking different paths to the same fix, and a run that recovers from three bad tool calls.

## Run it

From the `code` directory of a clone of [go-agents-book-code](https://github.com/blackwell-systems/go-agents-book-code), with no `OPENROUTER_API_KEY` set:

```console
$ go run ./ch04
```

The run takes four turns.
In the first two, the model runs the tests and lists the files, then reads both files.
Here are the last two turns and the summary:

```text include=ch04/testdata/out/fix.txt lines=18..40 title="go run ./ch04, turns 3 and 4"
```

The `PASS` in turn 3 is a real `go test` run on your machine, against the file as the model edited it.
The last lines total the run: 4 model calls, 5,427 tokens in, 569 out, and $0.0008, as recorded from `anthropic/claude-haiku-5.5` in October 2026, one run of the task.
The input grows with every call, because each request resends the whole conversation, including both files once the model has read them.

## The same task, another model

```console
$ go run ./ch04 -replay fix-again
```

This replays the same task on `openai/gpt-5.6-luna`.
This model starts by listing the files. Next it reads both files and runs the tests together in one turn, makes its edit, spends a separate turn rerunning the tests, and then answers. That is five model calls where `anthropic/claude-haiku-5.5` needed four, and the fix is the same.
Chapter 1 measured this model ten times, and every one of those runs followed this path.

Since each model finds its own route, this program's tests look at results and ignore the transcript. They check three things: the fix is in `duration.go`, a test run came after the final edit, and the run finished with an answer.
A live run that wanders down some other route still passes as long as those three hold.

## A run that recovers

```console
$ go run ./ch04 -replay recover
```

This saved run is written by hand, in the API's format, to show three mistakes in a row: a tool name that doesn't exist, a path sent as an array, and an edit whose old text is missing a space.
Each mistake comes back as a tool error that says what was wrong and what to send, such as `bad arguments: "path": got an array, want a string; send JSON like {"path": "a.go"}`, and each next call is right.
The detour costs one turn per mistake and stops nothing.

## What `run_tests` runs with

The code under test may be code the model wrote, and it runs with your permissions.
So the tool doesn't pass on your environment.
It builds one from scratch:

```go include=ch04/agent/gotest.go tag=test-env
```

The test process never sees your `OPENROUTER_API_KEY`, or any other secret exported in your shell.
The settings keep `go test` offline, out of any workspace above the temporary directory, away from your own Go settings, and on the toolchain you have.
One of the package's tests puts an API key in its own environment, then confirms that a test running inside the project finds no trace of it.

A scrubbed environment is not a [sandbox](/glossary/#sandbox): the tests still run as you, and can read your home directory or open network connections.
Real limits on what tool code can touch come later in the companion code, in `ch14/safety`.

## Save a run

```console
$ go run ./ch04 -save run.json
```

The file holds every message of the run, in order: each tool call with its arguments, and each tool result.
If a run misbehaves, this file is where you look: you can follow what the model opened, what `go test` reported, and how the model responded.

## Run it live

With a key from [OpenRouter](https://openrouter.ai) and a little prepaid credit, the same program calls a live model:

```console
$ export OPENROUTER_API_KEY=...
$ go run ./ch04
```

The default model is `anthropic/claude-haiku-5.5`; set `OPENROUTER_MODEL` to try another tool-capable model.
Expect to spend well under a cent. Expect different wording too, and possibly a different route to the fix.
Unset the key to go back to the recorded replies.

## Run the tests

```console
$ go test ./ch04/...
```

The tests replay every saved run and check its outcome, compare the client's requests with the saved ones, and check the tools' rules, including the scrubbed environment.
They run `go test` for real many times, so they take longer than chapter 1's.

## Key files

| File | What it holds |
| --- | --- |
| [`ch04/main.go`](code:ch04/main.go) | The program: the client, a `meter` that adds up cost, flags, and saving |
| [`ch04/openrouter/wire.go`](code:ch04/openrouter/wire.go) | The API's JSON as Go types, with tools |
| [`ch04/openrouter/client.go`](code:ch04/openrouter/client.go) | `Generate`: converting requests, reading tool calls back |
| [`ch04/agent/gotest.go`](code:ch04/agent/gotest.go) | The real `run_tests`: copy out, run `go test -json`, report |
| [`ch04/agent/tools.go`](code:ch04/agent/tools.go) | The file tools and the strict argument decoder |
| [`ch04/replay/replay.go`](code:ch04/replay/replay.go) | Replaying saved runs, and recording new ones with `-record` |
| [`ch04/testdata/replies`](code:ch04/testdata/replies) | The saved model replies, one file per call |
