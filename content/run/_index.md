---
title: Run it
description: Every chapter's program from the companion repository, with what it shows, its size and test count computed from the code, and the command that runs it offline from recorded model replies.
---

Every example runs without an API key.
With no `OPENROUTER_API_KEY` set, a program replays model replies recorded from a real run, while its tools, including `go test`, run for real on your machine.
You need Go {{code go-version}} or later; older `go` commands from 1.21 on download it when they read the module's `go.mod`.

```console
$ git clone https://github.com/blackwell-systems/go-agents-book-code
$ cd go-agents-book-code/code
```

Or download the repository as a [zip of the main branch](https://github.com/blackwell-systems/go-agents-book-code/archive/refs/heads/main.zip) and run the same commands from its `code` directory.

## Every chapter's program

Each chapter's directory is a full snapshot of the agent at that point, so any row runs without the ones above it.
The counts are computed from the code each time this site is built: Go files, lines in those files, and top-level test functions, leaving out the fixtures under `testdata`.
In all, the repository holds {{code files}} Go files and {{code tests}} test functions in {{code packages}} packages.

```catalog
ch01 | The loop with a scripted model, fixing a failing test | `go run ./ch01` | /run/ch01-scripted-loop/
ch02 | A chat program on the chat completions API, over `net/http` | `go run ./ch02` |
ch03 | A toy language model, a tokenizer, embeddings, and attention | `go run ./ch03 toy` |
ch04 | A real model with four tools fixes the test | `go run ./ch04` | /run/ch04-first-agent/
ch05 | Stop reasons, budgets, and a reply the loop refuses | `go run ./ch05 -replay dup-id` |
ch06 | Streaming, and a stream that fails partway | `go run ./ch06 -replay stream-error` |
ch07 | The crash-triage agent on one catalog task | `go run ./ch07` |
ch08 | Context policies on a batch of 99 crash reports | `go run ./ch08 -task batch` |
ch09 | Parallel tool calls, sub-agents, and one agent per crash group | `go run ./ch09 -task g3-mixed-batch -groups -parallel 4` |
ch10 | An MCP client and server, with pinned tool definitions | `go run ./ch10` |
ch11 | Other providers' APIs and official SDKs | `go run ./ch11` |
ch12 | Retries, traces, and cost, with injected 503s | `go run ./ch12 -trace -faults 503` |
ch13 | Evals graded by code, a gate, and a reviewer | `go test ./ch13/evals/` |
ch14 | Sandboxed tools, injection defenses, and approval | `go run ./ch14` |
ch15 | Checkpoints, idempotency keys, and the crash sweep | `go test ./ch15/durable/ -run TestCrashSweep -v` |
ch16 | One task on the book's harness, Eino, and Genkit (its own module) | `cd ch16 && go run ./cmd/harness` |
ch17 | The finished crash-triage agent | `go run ./ch17` |
catalog | The crash catalog's tasks and its grader | `go test ./catalog/` |
tracker | The small issue tracker the agent files issues in | `go test ./tracker/...` |
remind | The practice service, with bugs seeded on purpose (its own module) | `cd remind && go test ./...` |
```

A few rows end on purpose with an error or a pause:

- `ch05` and `ch06` replay failures, so they exit with the error they demonstrate.
- `ch14` and `ch17` stop when the agent asks to file an issue, because filing waits for a person's approval; the program prints the call it is waiting on.
- `ch08 -task batch` replays a whole batch and grades it, which takes a few minutes; the other rows finish within two.

With `OPENROUTER_API_KEY` set, the same commands call a live model and cost money; each chapter of the book says what its runs cost when they were recorded.
