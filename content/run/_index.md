---
title: Examples
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

New here? Run `ch01` first: it is the whole loop with a scripted model, and it finishes in a few seconds.
Then run `ch04`, the same task with a real model's recorded replies and a real `go test`.
Their pages, [ch01](/run/ch01-scripted-loop/) and [ch04](/run/ch04-first-agent/), explain each line of the output.

## Every chapter's program

Each chapter's directory is a full snapshot of the agent at that point, so any row runs without the ones above it.
The last three rows are examples beyond the chapter snapshots: the workflow shapes `ch05` names and an agent's progress streamed to a browser, both built on `ch06`'s agent, and a standalone first agent that a tutorial builds from scratch.
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
shapes | The workflow shapes: a chain, a router, a vote, and check and revise | `go run ./shapes` | /build/workflows-vs-agents/
ssebrowser | An agent's events streamed to a browser page | `go run ./ssebrowser` | /run/stream-to-browser/
firstagent | A standalone first agent: questions about the files in a directory | `go run ./firstagent` | /build/first-agent/
```

## What each one prints

The end of each program's output, as it printed in a replay on a laptop in October 2026.
Programs that replay a run start with `No OPENROUTER_API_KEY: replaying a saved run. The tools run for real.`
Durations and some counts vary from machine to machine; the answers and the grades don't, because the model's side is recorded.

**`ch01`**, the loop with a scripted model, ends on the model's answer:

```text
turn 4
  model says:  The tests now pass. The cause was a typo in the unit table in
               duration.go: the 'm' (minute) entry mapped to time.Second. So
               "45m" parsed as 45 seconds, and "1h30m" parsed as 1h + 30
               seconds. I changed it to time.Minute.
```

**`ch02`** replays a three-question chat and prints the tokens and cost after each reply:

```text
> What is a goroutine?
A goroutine is a lightweight thread of execution managed by the Go
runtime rather than the operating system. ...
[43 tokens in, 122 out, $0.000065; total $0.000065]
```

**`ch03 toy`** shows how temperature reshapes the toy model's choices, then writes text greedily and by sampling:

```text
After "the model", the next word can be:
  temperature 1   sees 0.13 asks 0.11 as 0.05 can 0.05
  temperature 0.5 sees 0.33 asks 0.21 as 0.05 can 0.05
  temperature 2   sees 0.07 asks 0.06 as 0.04 can 0.04
```

**`ch04`** runs the same fix as `ch01` with a real model's replies and a real `go test`, and ends with the bill:

```text
4 model calls, 5427 tokens in, 569 out, $0.0008
duration.go was changed.
```

**`ch05`** and **`ch06`** replay failures on purpose and exit with the error they demonstrate, a reply the loop refuses and a stream cut off partway:

```text
stopped: a reply the loop can't use
error: run agent: agent: bad reply: bad tool call: the ID "call_dupid_1" is already in use
```

```text
turn 1
  model says:  The tests fail because the units table maps 'm'
  (cut off: not in the conversation)

stopped: a reply the loop can't use
error: run agent: generate: agent: bad reply: provider failed partway: server_error Provider disconnected
```

**`ch07`** is the first crash-triage agent.
It finds all three causes, but it doesn't yet recognize that two were already filed, so the grade marks two duplicates as missed:

```text
graded against the catalog: 12 of 16 checks passed
  failed: status N1A: answered "fixed", want "duplicate"
  failed: duplicate of N1A: answered issue 0, want 101
  failed: status N2A: answered "fixed", want "duplicate"
  failed: duplicate of N2A: answered issue 0, want 102
```

**`ch08 -task batch`** replays a recorded run over the 99-report batch and grades it, which takes a few minutes. The recorded run missed some checks, and the grade shows which:

```text
graded against the catalog: 82 of 94 checks passed
```

**`ch09`** runs one agent per crash group, four at a time:

```text
4 diagnoses; 0 groups unresolved; 0 groups' changes left out
filed #1 Parse in duration/duration.go: out of range
filed #2 (*Store).List in store/store.go: out of range
filed #3 (*Store).Get in store/store.go: nil value
filed #4 (*API).Routes in api/api.go: nil value
graded against the catalog: 22 of 22 checks passed
spent $0.0235 in all
```

**`ch10`** and **`ch11`** replay the same triage run, through an MCP tracker and then through `ch11`'s provider layer, and it now catches both duplicates:

```text
diagnosis:
  fp_4828a6 cr_1e2ea0,cr_ee551c: fixed, root cause store/store.go (*Store).Get, issue #103
  fp_6b14b3 cr_f9ee9e: duplicate, root cause auth/auth.go parseBearer, duplicate of #101
  fp_2bcd17 cr_fd4982: duplicate, root cause notify/notify.go (*Dispatcher).Stop, duplicate of #102
filed #103: store.Get returns (nil, nil) for a deleted reminder, crashing api.owned

graded against the catalog: 16 of 16 checks passed
```

**`ch12 -trace -faults 503`** injects a 503 into the first call, retries it, and prints the run's trace after the same result:

```text
trace 23539b6b0b71eaee97e07f9cf0978e97
invoke_agent triage 8.298s gen_ai.operation.name=invoke_agent ...
  chat anthropic/claude-haiku-5.5 195ms ...
    attempt 336µs attempt=1 error.type=503 error="openrouter: 503 Service Unavailable: no details"
    attempt 529µs attempt=2
  execute_tool crash_list 310µs ...
```

**`ch13`**'s evals run as tests, and a pass looks like any other:

```text
ok  	github.com/blackwell-systems/go-agents-book/code/ch13/evals	0.410s
```

**`ch14`** and **`ch17`** pause when the agent asks to file an issue, because filing needs a person's approval.
The program shows the call, its arguments, and the crash report behind it, then asks:

```text
type "yes 93761f" to approve:
```

Type that line and the run finishes and is graded, 16 of 16; press Ctrl-D instead and the run stops with the call left waiting.

**`ch15`**'s crash sweep crashes the agent at every save point and counts the runs left with a duplicate, for each of five designs:

```text
crash point            v1     v2     v3     v4     v5
reply received          0      0      0      0      0
marker saved            0      0      0      0      0
tracker accepted        3      3      0      0      0
tool returned           3      3      0      0      0
...
--- PASS: TestCrashSweep (14.63s)
```

**`ch16`** runs the task on the harness in the companion code and ends with what it did:

```text
issues filed 1, diagnosis submitted true
```

**`catalog`**, **`tracker`**, and **`remind`** are test suites, so they print one `ok` line per package; `remind` passes its own tests with its bugs still in place.

**`shapes`** runs each workflow shape against a scripted model, including a vote that reaches its quorum and one that doesn't:

```text
Parallel calls with a vote: 3 of 5 must agree
  votes: ["yes" "no" "yes" "Yes" " yes\n"]
  answer: yes
  votes: ["yes" "no" "unsure" "no" "yes"]
  error: shapes: no answer reached the quorum: 5 answers, 0 failed calls
```

**`ssebrowser`** is a server: it prints `open http://127.0.0.1:8080` and runs until you press Ctrl-C.

With `OPENROUTER_API_KEY` set, the same commands call a live model and cost money; the runs recorded for these guides list what they cost at the time.
