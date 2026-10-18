---
title: AI Agents in Go
description: Learn how AI agents work at the level of every request, reply, and tool call, by building one step by step with Go's standard library.
layout: home
---

## What this site is

A guide to building AI agents in Go, one question per page: how the loop works, what a tool call looks like on the wire, how to keep a long run's context in check, how to test an agent whose answers vary, and how to run one in production.

Every page is short and points at runnable code.
The code comes from a [public repository](https://github.com/blackwell-systems/go-agents-book-code) under Apache 2.0, where each excerpt is part of a tested program, linted and run with the race detector.
The examples run offline: with no API key, they replay model replies recorded from real runs (a few, written by hand, show failures that are hard to provoke), and the tools themselves, `go test` among them, really execute on your machine.

## Learn it by building it

The way to understand an agent is to build one, one piece at a time, and see what each piece is for.
This guide and the book behind it start from a loop with no model in it at all, a scripted stand-in, so the control flow is plain before any AI arrives.
Then the pieces go in one by one: a real model call, tools, streaming, context management, concurrency, evals, safety, and recovery from a crash.
Each piece is a small, tested Go change that you can run and break on purpose.

The code uses Go's standard library: `net/http` for the API, `encoding/json/v2` for the wire format, `context` for deadlines and cancellation.
No framework sits between you and the model, so every request, reply, and tool call is something you wrote and can read.
Along the way you also see what happens inside the model, from tokens and sampling to why answers vary, because that explains much of how an agent behaves.

By the end you can read any agent framework and know what it does underneath, and decide what to adopt and what to write yourself.

## The loop, in four stages

An agent is a model inside a [harness](/glossary/#harness), the program around it.
On every turn the harness does the same four things:

<ol class="loop-stages"><li><strong>Build the request</strong>Decide what the model sees: instructions, the conversation so far, tool results, and the tools it may call.</li><li><strong>Call the model</strong>Send the request and read the reply, with a deadline and a count of what it cost.</li><li><strong>Run the tools</strong>Carry out the calls the model asked for, and add each result to the conversation.</li><li><strong>Decide</strong>Stop with an answer, stop at a limit, or go around again.</li></ol>

<p class="loop-back">Then back to stage 1, with everything that just happened in the conversation.</p>

Each page on this site works on one of these stages or on the whole loop, and says which.
When an agent misbehaves, the stage tells you where to look: one that never stops has a Decide problem, and one that forgets what it read has a Build-the-request problem.

## The whole loop in 30 lines

Here is the loop, as the first chapter of the companion code writes it:

```go include=ch01/agent/loop.go tag=run
```

It sends the whole conversation on every call, because the model keeps nothing between calls.
It appends the reply before running its tools, so each tool result answers a call the conversation already holds.
It stops on an answer, at the turn limit, or when `ctx` is cancelled.
[The agent loop in Go](/foundations/agent-loop/) explains each line.

Run it with no model at all, from a scripted list of replies:

```console
$ git clone https://github.com/blackwell-systems/go-agents-book-code
$ cd go-agents-book-code/code
$ go run ./ch01
```

The output replays a real model's fix of a failing test, turn by turn, through this loop.
Then `go run ./ch04` runs the same loop against recorded replies from a real model, with a real `go test` behind its `run_tests` tool.
Both need Go 1.27 or later.

## How this guide works

- **One agent, grown step by step.** The examples follow one agent from a 30-line loop that fixes a failing test to a crash-triage agent for a small Go web service, so each page adds to code you have already seen.
- **Measured, with the method.** Claims about what works come from recorded runs, with the models, the number of runs, and the date. For example: [one sentence in a tool description decided 79 of 80 first moves](/findings/tool-description-poisoning/).
- **Offline by default.** Every example runs without a key and costs nothing. With `OPENROUTER_API_KEY` set, the same programs call a live model.
- **The standard library first.** The code reaches the model over plain `net/http` and `encoding/json/v2`, so you can see every moving part before deciding what to adopt.
