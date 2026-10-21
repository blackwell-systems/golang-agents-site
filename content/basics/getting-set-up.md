---
title: "Getting set up to build AI agents in Go"
short: Getting set up
description: What you need before the first example runs. Go 1.27, the companion code and how it is laid out, the switch between offline replays and a live model, and the newer Go features the code relies on.
card: Go 1.27, the companion code, offline and live runs, and the newer Go features the code uses.
group: Get oriented
chapters: 1, 2
date: 2026-10-19
weight: 5
art: getting-set-up
---

Every example on this site is a Go program in one public repository, and every one of them runs on your machine without an API key.
This page gets you to the point where they do, and lists the Go you'll want to be comfortable with before reading the code.

## Go 1.27

The code needs Go 1.27 or later.
Check what you have:

```console
$ go version
```

If you already have Go 1.21 or later, you don't have to install anything by hand: the module's `go.mod` (the file that names the module and the Go version it needs) asks for Go 1.27, and the `go` command downloads that toolchain (the compiler and tools for that Go version) the first time it builds the module.
With an older Go, or none, install the current release from [go.dev/dl](https://go.dev/dl/).

## The companion code

Clone the repository and move into its `code` directory, where every command on this site runs:

```console
$ git clone https://github.com/blackwell-systems/go-agents-book-code
$ cd go-agents-book-code/code
$ go run ./ch01
```

If the last command prints an agent fixing a failing test, turn by turn, everything works.
The output begins like this:

```text
task: The tests in this project fail. Find out why and fix it.

turn 1
  model asks:  run_tests {}
  model asks:  list_files {}
  tool result: --- FAIL: TestParse (0.00s) (5 lines)
  tool result: duration.go, duration_test.go

turn 2
  model asks:  read_file {"path": "duration.go"}
```

and so on, until the agent has fixed the bug and the tests pass.

The repository holds one numbered directory per step of the build, `ch01` to `ch17`.
Each is a complete snapshot of the agent at that point, so you can run any of them without the others, and compare two to see what changed.
Inside each, the agent itself is the package in `chNN/agent`, and `main.go` is a thin program that sets it up.
A few directories sit beside those: `remind`, a small web service with bugs planted on purpose for the agent to find; `tracker`, a small issue tracker where the agent files issues; and `catalog`, the set of tasks used to grade the agent.
[Meet them on the examples page](/run/) when the guides start using them.

Every directory has tests, and none of them calls a model:

```console
$ go test ./ch01/...
```

## Offline or live

A program decides whether to run offline or live from one environment variable: whether `OPENROUTER_API_KEY` is set.
That key belongs to [OpenRouter](/glossary/#openrouter), a service that gives you one API for many models; it is your [API key](/glossary/#api-key).

- **Without `OPENROUTER_API_KEY`,** it replays model replies recorded from a real run. The tools still run for real on your machine, `go test` included, so the agent's actions are genuine; only the model's side is a recording. This costs nothing and gives the same result every time.
- **With `OPENROUTER_API_KEY` set,** the same program calls a live model and costs money, usually fractions of a cent per run for the early examples. [Your first model call](/basics/first-model-call/) shows how to get a key.

`OPENROUTER_MODEL` [picks a different model](/basics/choosing-a-model/) for a live run; leave it unset to use the one the code recommends.

## The Go you'll need

You should be comfortable writing Go: structs and interfaces, errors, packages, and starting a goroutine.
If some of that is shaky, the "New to Go?" box below explains the handful of ideas the examples lean on most, with a link for each.
The code also uses some newer parts of the language and standard library, listed in the table; the guides name them where they appear, and the [Go in depth](/basics/#go-in-depth) pages cover the ones an agent leans on most.
You can come back to the table later:

| Feature | Since | Where it shows up |
| --- | --- | --- |
| Generics, as in `decodeArgs[T any]` | Go 1.18 | Decoding a tool's arguments into the struct that tool expects |
| [`context.WithTimeoutCause` and `context.Cause`](/basics/context/) | Go 1.21 | A deadline that records why a call was cancelled |
| `for range n` over an integer | Go 1.22 | The loop's turn limit |
| [Iterators, `iter.Seq`](/basics/iterators/) | Go 1.23 | Reading a streamed reply event by event |
| [`sync.WaitGroup.Go`](/basics/goroutines/) and `testing/synctest` | Go 1.25 | [Running tool calls side by side](/build/parallel-tool-calls/), and testing timeouts without waiting |
| [`errors.AsType`](/basics/errors/) | Go 1.26 | Checking an error's type in one line |
| [`encoding/json/v2`](/basics/json-v2/) | Go 1.27 (Go 1.26 with `GOEXPERIMENT=jsonv2`) | All JSON on the wire |

<details class="go-primer" id="new-to-go">
<summary>New to Go? The Go you'll meet on this site</summary>

If you have finished the [Tour of Go](https://go.dev/tour/), you have seen most of these once.
Each is a sentence or two, with a link for more.

- **`context.Context`** is a value passed into every call that can be slow. It carries a deadline and a cancel signal, so Ctrl-C stops the run wherever it is. By convention it is the first parameter, named `ctx`. See [context in Go by Example](https://gobyexample.com/context) and [context in an agent](/basics/context/).
- **Interfaces are satisfied implicitly.** A type has no `implements` clause; any type with the interface's methods is one. That is why a scripted list of replies and a real client can both be a `Model`. See [interfaces in the Tour](https://go.dev/tour/methods/9).
- **`%w` and `errors.Is`.** `fmt.Errorf("read file: %w", err)` adds context to an error and keeps the original inside it. `errors.Is(err, target)` then finds the original through any number of wrappings. See [errors in Go by Example](https://gobyexample.com/errors).
- **Struct tags** are the backtick text after a field, as in `` Path string `json:"path"` ``. They tell the JSON decoder which JSON name fills that field. See [JSON and struct tags in Go by Example](https://gobyexample.com/json) and the [glossary](/glossary/#struct-tag).
- **`defer`** runs a call when the surrounding function returns, whichever way it returns. It is how the code closes a response body or releases a resource. See [defer in the Tour](https://go.dev/tour/flowcontrol/12).
- **Method receivers** are the parameter before a method's name, as in `func (c *Client) Generate(...)`. A pointer receiver (`*Client`) lets the method change the value it is called on. See [methods in the Tour](https://go.dev/tour/methods/1).
- **Raw strings** in backticks, such as `` `{"path": "main.go"}` ``, can hold quotes and newlines with no escaping, which suits JSON written inside Go code. See [string literals in the Go spec](https://go.dev/ref/spec#String_literals).

</details>

One habit from `encoding/json/v2` matters right away: it matches JSON field names to struct fields exactly, case included, so every field that comes from an API carries an explicit tag (a [struct tag](/glossary/#struct-tag), the backtick text after the field) such as `` `json:"finish_reason"` ``.
[Common first errors](/basics/common-errors/) shows what happens when one is missing.
