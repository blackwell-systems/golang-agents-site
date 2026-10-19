---
title: "Getting set up to build AI agents in Go"
short: Getting set up
description: What you need before the first example runs. Go 1.27, the companion code and how it is laid out, the switch between offline replays and a live model, and the newer Go features the code relies on.
card: Go 1.27, the companion code, offline and live runs, and the newer Go features the code uses.
chapters: 1, 2
date: 2026-10-19
weight: 5
---

Every example on this site is a Go program in one public repository, and every one of them runs on your machine without an API key.
This page gets you to the point where they do, and lists the Go you'll want to be comfortable with before reading the code.

## Go 1.27

The code needs Go 1.27 or later.
Check what you have:

```console
$ go version
```

If you already have Go 1.21 or later, you don't have to install anything by hand: the module's `go.mod` asks for Go 1.27, and the `go` command downloads that toolchain the first time it builds the module.
With an older Go, or none, install the current release from [go.dev/dl](https://go.dev/dl/).

## The companion code

Clone the repository and move into its `code` directory, where every command on this site runs:

```console
$ git clone https://github.com/blackwell-systems/go-agents-book-code
$ cd go-agents-book-code/code
$ go run ./ch01
```

If the last command prints an agent fixing a failing test, turn by turn, everything works.

The repository holds one directory per chapter of the book, `ch01` to `ch17`.
Each is a complete snapshot of the agent at that point, so you can run any of them without the others, and compare two to see what changed.
Inside each, the agent itself is the package in `chNN/agent`, and `main.go` is a thin program that sets it up.
A few directories sit beside the chapters: `remind`, a small web service with bugs planted on purpose for the agent to find; `tracker`, a small issue tracker the agent files issues in; and `catalog`, the set of tasks used to grade the agent.
[Meet them on the examples page](/run/) when the guides start using them.

Every directory has tests, and none of them calls a model:

```console
$ go test ./ch01/...
```

## Offline or live

A program decides how to run from one environment variable.

- **Without `OPENROUTER_API_KEY`,** it replays model replies recorded from a real run. The tools still run for real on your machine, `go test` included, so the agent's actions are genuine; only the model's side is a recording. This costs nothing and gives the same result every time.
- **With `OPENROUTER_API_KEY` set,** the same program calls a live model and costs money, usually fractions of a cent per run for the early chapters. [Your first model call](/basics/first-model-call/) shows how to get a key.

`OPENROUTER_MODEL` picks a different model for a live run; leave it unset to use the one the code recommends.

## The Go you'll need

You should be comfortable writing Go: structs and interfaces, errors, packages, and starting a goroutine.
The code also uses some newer parts of the language and standard library, which these guides use without stopping to explain:

| Feature | Since | Where it shows up |
| --- | --- | --- |
| Generics, as in `decodeArgs[T any]` | Go 1.18 | Decoding a tool's arguments into the struct that tool expects |
| `context.WithTimeoutCause` and `context.Cause` | Go 1.21 | A deadline that records why a call was cancelled |
| `for range n` over an integer | Go 1.22 | The loop's turn limit |
| Iterators, `iter.Seq` | Go 1.23 | Reading a streamed reply event by event |
| `sync.WaitGroup.Go` and `testing/synctest` | Go 1.25 | Running tool calls side by side, and testing timeouts without waiting |
| `errors.AsType` | Go 1.26 | Checking an error's type in one line |
| `encoding/json/v2` | Recent | All JSON on the wire |

One habit from `encoding/json/v2` matters right away: it matches JSON field names to struct fields exactly, case included, so every field that comes from an API carries an explicit tag such as `` `json:"finish_reason"` ``.
[Common first errors](/basics/common-errors/) shows what happens when one is missing.
