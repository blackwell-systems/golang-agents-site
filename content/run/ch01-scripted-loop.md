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

This program runs the whole agent loop, with four tools and a small Go project with a bug a test would catch, against a model that is a script.
The script is a copy of what a real model, `anthropic/claude-haiku-5.5`, replied when it fixed the bug in October 2026, so the run has the shape of a real one while nothing leaves your machine.
It is the smallest complete agent in the companion code, and every later example grows from it.

## What it shows

- The loop's four stages in about 30 lines of Go: assemble a request, send it to the model, execute any tool calls, and choose whether to go around again. [The agent loop in Go](/basics/agent-loop/) walks through them.
- A `Model` interface with one method, which a script satisfies as well as a real client does.
- Tools as a [definition the model sees](/basics/tool-calling/) plus a Go function it never sees.
- [Errors from tools going back to the model](/basics/errors/#errors-the-model-reads) as messages, so they never stop the run.

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

`Calls` builds a reply that requests tools, `Call` builds one tool call whose arguments are JSON (the backtick strings, such as `{}` for "no arguments", are Go raw strings holding that JSON), for `Calls` to wrap, and `Says` builds a reply that only contains text; a reply without tool calls counts as the answer.
To decide which reply comes next, the scripted model counts how many of the model's own replies (the messages with the `assistant` role, the ones the model wrote; [Messages and roles](/basics/messages-and-roles/) explains the roles) the conversation already holds. A loop that lost a message or put one in the wrong place would knock the script out of step, and the tests would fail.

## Run the tests

```console
$ go test ./ch01/...
```

Each test in `ch01/agent` pins down a single rule of the loop.
An answer ends the run, and so does reaching the turn limit.
Each tool call is answered by exactly one tool message, even when one reply holds several calls.
A tool that fails sends its error back flagged as an error, and a call to a tool that doesn't exist gets an error naming the tools that do exist.
The whole suite finishes in milliseconds without touching the network, and that is why the loop is written before it is connected to a real model.

## Things to try

Each of these is a change of a line or two; run `go run ./ch01` after it.

**Misspell a tool.** In `script.go`, change the tool name in `call_3`, the `read_file` call in turn 2, to `read_files`.
Turn 2 now prints a `tool error` whose text starts with `no tool named "read_files"` (the printer cuts long results short, so the list of real tools is cut off on screen), and the run keeps going, because the loop hands the error back to the model.
With a real model, the next reply would usually correct the name.

**Lower the turn limit.** In `main.go`, set `MaxTurns` to 3.
The run prints three turns, then ends with `run agent: agent: max turns reached`.
`Run` still returns the conversation so far, which is how the program prints those three turns.

**Cancel before it starts.** In `main.go`, the `run` function starts with `func run(ctx context.Context, w io.Writer) error {`.
Add two lines directly after that one (and add `"time"` to the imports, or `go run` will report an undefined `time`):

```go
ctx, cancel := context.WithTimeout(ctx, 0) // a deadline of zero seconds
defer cancel()
```

`WithTimeout` gives back a copy of `ctx` that is cancelled after the given time, and zero means the deadline has already passed.
`Run` checks `ctx` at the top of every turn, so it stops before the first model call.
The program ends with an error whose text mentions `context deadline exceeded`.
In code, `errors.Is(err, context.DeadlineExceeded)` is how you ask whether that is the reason the error occurred, even when other errors have wrapped it.

When you are done experimenting, undo your edits from the `code` directory with `git restore .` (or `git checkout -- .`).

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

[A real agent that fixes a failing test](/run/ch04-first-agent/) runs the same `Run` against a real model's [recorded replies](/production/testing-offline/), with a real `go test` behind `run_tests`.
