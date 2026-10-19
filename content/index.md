---
title: AI Agents in Go
description: Learn how AI agents work at the level of every request, reply, and tool call, by building one step by step with Go's standard library.
layout: home
---

## About this site

Guides to building AI agents in Go, one question per page: what a tool call looks like as JSON, when a run should stop, how to test an agent without calling a model.
They draw on the book *Building AI Agents in Go*, and [Start here](/start/) puts them in order for three kinds of reader.

The code on every page comes from the book's [public repository](https://github.com/blackwell-systems/go-agents-book-code) (Apache 2.0), where each excerpt belongs to a program with tests.
The programs run offline: with no API key, they replay saved model replies, most of them recorded from real runs, and the tools, `go test` included, run on your machine.

## Learn it by building it

The way to understand an agent is to build one, one piece at a time, and see what each piece is for.
The book behind this guide starts from a loop with no model in it at all, a scripted stand-in, so the control flow is plain before any AI arrives.
Then it adds the pieces one at a time: a real model call, tools, streaming, context management, concurrency, evals, safety, and recovery from a crash.
Each piece is a small, tested Go change that you can run and break on purpose, and every chapter's program is in the public repository.

The code uses Go's standard library: `net/http` for the API, `encoding/json/v2` for the wire format, `context` for deadlines and cancellation.
No framework sits between you and the model, so every request, reply, and tool call is something you wrote and can read.
Along the way you also see what happens inside the model, from tokens and sampling to why answers vary, because that explains much of how an agent behaves.

By the end of the book you can read any agent framework and know what it does underneath, and decide what to adopt and what to write yourself.

## The loop, in four stages

An agent is a model inside a [harness](/glossary/#harness), the program around it.
On every turn the harness does the same four things:

<ol class="loop-stages"><li><strong>Build the request</strong>Decide what the model sees: instructions, the conversation so far, tool results, and the tools it may call.</li><li><strong>Call the model</strong>Send the request and read the reply, with a deadline and a count of what it cost.</li><li><strong>Run the tools</strong>Carry out the calls the model asked for, and add each result to the conversation.</li><li><strong>Decide</strong>Stop with an answer, stop at a limit, or go around again.</li></ol>

<p class="loop-back">Then back to stage 1, with everything that just happened in the conversation.</p>

Each page on this site works on one of these stages or on the whole loop, and says which.
When an agent misbehaves, the stage tells you where to look: one that never stops has a Decide problem, and one that forgets what it read has a Build-the-request problem.

## The whole loop, as chapter 1 writes it

Here is the loop from the first chapter of the companion code:

```go include=ch01/agent/loop.go tag=run
```

It sends the whole conversation on every call, because the model keeps nothing between calls.
It appends the reply before running its tools, so each tool result answers a call the conversation already holds.
It stops on an answer, at the turn limit, or when `ctx` is cancelled.
[The agent loop in Go](/loop/agent-loop/) explains each line.

Run it with no model at all, from a scripted list of replies:

```console
$ git clone https://github.com/blackwell-systems/go-agents-book-code
$ cd go-agents-book-code/code
$ go run ./ch01
```

The output replays a real model's fix of a failing test, turn by turn, through this loop.
Then `go run ./ch04` runs the same loop against recorded replies from a real model, with a real `go test` behind its `run_tests` tool.
Both need Go {{code go-version}} or later and no API key.
The repository holds {{code files}} Go files, {{code lines}} lines in all, and {{code tests}} test functions, none of which calls a model or needs an API key; [Run it](/run/) lists each chapter's program with its own counts.

## Three measured results

Each number below comes from runs recorded for the book, and each says how it was measured.

<div class="result-cards">

```result
figure: 79 of 80
unit: first replies
task: the triage agent's first request, with one sentence in an MCP tool's description asking for a different call
models: four inexpensive models: `claude-haiku-5.5`, `gpt-5.6-luna`, `glm-5.3-flash`, `deepseek-v4.1-flash`
runs: ten per model and condition
date: October 2026
source: Chapter 10
link: /findings/tool-description-poisoning/
more: The finding

One sentence in a tool the model hadn't called yet decided its first move. Labeling the text as the server's cut that to 57 of 80.
```

```result
figure: 910
unit: crashes, 0 duplicates
task: the agent's own loop, crashed at each of its 38 crash points and resumed, then crashed a second time at every point each restart reaches
models: a scripted model, against a tracker that honors idempotency keys
runs: 910 crash-and-resume scenarios
date: deterministic; `go test ./ch15/durable/` repeats it
source: Chapter 15
link: /findings/measured-mistakes/#save-per-turn
more: What saving once per turn did

With a key on every call that changes something, no scenario filed anything twice, lost an issue or comment, or had to ask a person.
```

```result
figure: 39 of 40
unit: runs passed
task: fixing one failing Go test as a three-step workflow, with a single model call
models: the same four models
runs: ten per model
date: October 2026
source: Chapter 1, and chapter 5’s table of shapes
link: /findings/measured-mistakes/#workflow-tab
more: The one failure

The workflow cost between a sixth and a quarter of what the agent cost on each model. Its one failed run shows what a fixed sequence gives up.
```

</div>

## How this guide works

- **One agent, grown step by step.** The examples follow one agent from a 30-line loop that fixes a failing test to a crash-triage agent for a small Go web service, so each page adds to code you have already seen.
- **Measured, with the method.** Claims about what works come from recorded runs, with the task, the models, the number of runs, and the date, as in the three results above. The failures are measured too: [twelve measured mistakes](/findings/measured-mistakes/), each with its method.
- **Offline by default.** Every example runs without a key and costs nothing. With `OPENROUTER_API_KEY` set, the same programs call a live model.
- **The standard library first.** The code reaches the model over plain `net/http` and `encoding/json/v2`, so you can see every moving part before deciding what to adopt.
- **Clear about its limits.** Retrieval, memory across unrelated runs, and a model that hands work to other agents at run time are left out on purpose; [what this covers](/scope/) says why, and what was measured instead. [Why Go](/loop/why-go/) makes the case for the language from the book's measurements.
