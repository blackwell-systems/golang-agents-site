---
title: A scripted agent loop, no model needed
short: "ch01: A scripted agent loop"
description: Run a complete agent loop that fixes a failing Go test, driven by a script of replies recorded from a real model. No API key and no network, with tests that run in milliseconds.
card: A complete agent loop that fixes a failing test from a script. No key, no network.
chapters: 1
stage: all
date: 2026-10-18
weight: 10
art: ch01
---

This program runs the whole agent loop, with four tools and a small Go project whose tests fail, against a model that is a script.
The script is a copy of what a real model, `anthropic/claude-haiku-5.5`, replied when it fixed the bug in October 2026, so the run has the shape of a real one while nothing leaves your machine.
It is the smallest complete agent in the companion code, and every later example grows from it.

## What it shows

- The loop's four stages in about 30 lines of Go: assemble a request, send it to the model, execute any tool calls, and choose whether to go around again. [The agent loop in Go](/loop/agent-loop/) walks through them.
- A `Model` interface with one method, which a script satisfies as well as a real client does.
- Tools as a definition the model sees plus a Go function it never sees.
- Errors from tools going back to the model as messages, so they never stop the run.

## Run it

From the `code` directory of a clone of [go-agents-book-code](https://github.com/blackwell-systems/go-agents-book-code), with Go 1.27 or later:

```console
$ go run ./ch01
```

The output, turn by turn:

```text include=ch01/testdata/transcript.txt title="go run ./ch01"
```

Read the middle of each turn as requests: every `model asks` line is a [tool call](/glossary/#tool-and-tool-call), and every `tool result` line is your program doing the work.
In turns 1 to 3 the model asks for two things at once, and the loop runs them in the order asked, which matters in turn 3, where the edit has to land before the tests run.
Turn 4 has no tool calls, so the loop stops.

Nothing touched your disk.
The project lives in an in-memory file system, a map from names to contents, and `run_tests` is a stand-in that checks for the fixed line; the [ch04 example](/run/ch04-first-agent/) swaps in a real `go test`.

## The script

The replies are plain Go values, built with three small helpers:

```go include=ch01/script.go tag=script
```

`Calls` builds a reply that requests tools, `Call` builds a single tool call whose arguments are JSON, and `Says` builds a reply that only contains text; a reply without tool calls counts as the answer.
To decide which reply comes next, the scripted model counts how many assistant turns the conversation already holds. A loop that lost a message or put one in the wrong place would knock the script out of step, and the tests would fail.

## Run the tests

```console
$ go test ./ch01/...
```

Each test in `ch01/agent` pins down a single rule of the loop.
An answer ends the run, and so does reaching the turn limit.
Each tool call is answered by exactly one tool message, even when one reply holds several calls.
A tool that fails sends its error back flagged as an error, and a call to a tool that doesn't exist is told which tools do.
The whole suite finishes in milliseconds without touching the network, and that is the reason the book writes the loop before it adds a model.

## Things to try

Each of these is a one-line change; run `go run ./ch01` after it.

**Misspell a tool.** In `script.go`, change the name in the `call_3` call from `read_file` to `read_files`.
Turn 2 now prints a `tool error` whose text starts with `no tool named "read_files"` (the printer cuts long results short, so the list of real tools is cut off on screen), and the run keeps going, because the loop hands the error to the model and carries on.
With a real model, the next reply would usually correct the name.

**Lower the turn limit.** In `main.go`, set `MaxTurns` to 3.
The run prints three turns, then ends with `run agent: agent: max turns reached`.
`Run` still returns the conversation so far, which is how the program prints those three turns.

**Cancel before it starts.** Wrap the context in `main.go` with a deadline that has already passed, `context.WithTimeout(ctx, 0)`.
`Run` checks `ctx` at the top of every turn, so it stops before the first model call, and the error matches `context.DeadlineExceeded` under `errors.Is`.

## Key files

| File | What it holds |
| --- | --- |
| [`ch01/agent/types.go`](code:ch01/agent/types.go) | `Model`, `Request`, `Message`, `ToolDef`, `ToolCall`, `Tool`, `Limits`, `Result` |
| [`ch01/agent/loop.go`](code:ch01/agent/loop.go) | `Run`, the loop, and `runTool` |
| [`ch01/agent/fake.go`](code:ch01/agent/fake.go) | The scripted model and its helpers |
| [`ch01/agent/tools.go`](code:ch01/agent/tools.go) | `read_file`, `list_files`, `edit_file`, `run_tests`, and the argument decoder |
| [`ch01/agent/memfs.go`](code:ch01/agent/memfs.go) | The in-memory project |
| [`ch01/main.go`](code:ch01/main.go), [`ch01/script.go`](code:ch01/script.go) | The program: Ctrl-C handling, the task, and the script |
| [`ch01/testdata/duration`](code:ch01/testdata/duration) | The `duration` package with its bug |

## Next

[A real agent that fixes a failing test](/run/ch04-first-agent/) runs the same `Run` against a real model's recorded replies, with a real `go test` behind `run_tests`.
