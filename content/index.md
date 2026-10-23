---
title: AI Agents in Go
description: Learn how AI agents work at the level of every request, reply, and tool call, by building one step by step with Go's standard library.
layout: home
---

## The loop, in four stages

An agent is a [model](/glossary/#model) (a large language model, reached through an API) inside a [harness](/glossary/#harness), the program around it.
On every turn, one pass through these four stages, the harness does the same four things:

{{loop}}

Each page on this site works on one of these stages or on the whole loop, and says which. You can [browse every guide by stage](/guides/), for example [the Decide guides](/guides/?stage=4).
When an agent misbehaves, the stage tells you where to look: one that never stops has a Decide problem, and one that forgets what it read has a Build-the-request problem.

## The whole loop, in code

You don't need to follow this code yet; the [Start path](/start/) walks through it step by step.
The loop talks to the model through one small interface:

```go include=ch01/agent/types.go tag=model
```

Anything with a `Generate` method is a `Model`: a scripted list of replies for tests, or a client for a real model's API.
The loop calls `m.Generate` once per turn and never needs to know which one it has.
Here is the loop as the [companion code](https://github.com/blackwell-systems/go-agents-book-code) writes it, each line marked with its stage's number and color from the ring above:

```go include=ch01/agent/loop.go tag=run stages=7-8:1,10-12:4,13:1,14-19:2,20-22:4,23-25:3,27:4
```

Its first line calls `index`, which splits the tools in two: `defs`, the definitions that go to the model in every request, and `byName`, which the loop uses to find the Go code for the tool a call names:

```go include=ch01/agent/loop.go lines=49..63
```

The other names are plain types and helpers: a `Message` is one entry in the conversation, a `Request` is the messages and tool definitions for one call, `Limits` holds settings such as `MaxTurns`, a `Result` holds the conversation a run ends with, `runTool` runs one call and turns its output (or its error) into a message for the model, and `ErrMaxTurns` is the error a run returns when it hits the turn limit.

The loop sends the whole conversation on every call, because the model keeps nothing between calls.
It appends the model's reply to the conversation before running its tools, so each tool result is paired with a tool call the conversation already holds.
It stops on an answer, at the turn limit, or when `ctx` is cancelled.
[The agent loop in Go](/basics/agent-loop/) explains each line.

Run it with no model at all, from a scripted list of replies:

```console
$ git clone https://github.com/blackwell-systems/go-agents-book-code
$ cd go-agents-book-code/code
$ go run ./ch01
```

The output replays a real model's fix of a failing test, turn by turn.
You will see counts of tokens in it; a [token](/glossary/#token) is the unit of text a model reads and writes, roughly a short word or a piece of a longer one.
Then [`go run ./ch04`](/run/ch04-first-agent/) runs the same loop against a real model's [recorded replies](/production/testing-offline/), with a real `go test` behind its `run_tests` tool.
Both need Go {{code go-version}} or later and no API key.
New to Go? Finish the [Tour of Go](https://go.dev/tour/) first; [Getting set up](/basics/getting-set-up/#new-to-go) lists what to brush up on.
The repository holds {{code files}} Go files and {{code tests}} test functions, and none of them calls a model; [Examples](/run/) lists every program in the repository.

<!-- paths -->

## Common questions

Each links to the guide that answers it, with Go code you can run.

<ul class="questions"><li><a href="/basics/getting-set-up/">What do I need to get started?</a><span>Go 1.27, the companion code, and examples that run with no API key.</span></li><li><a href="/basics/what-is-an-agent/">What is an AI agent, and how is it different from a chatbot?</a><span>A program where the model picks each next step and your Go code carries it out.</span></li><li><a href="/basics/first-model-call/">How do I make my first LLM API call?</a><span>An API key, one request with curl, and every part of the reply, including its cost.</span></li><li><a href="/basics/messages-and-roles/">What are messages, roles, and the system prompt?</a><span>The four roles in a conversation, and how to write instructions for an agent.</span></li><li><a href="/basics/tool-calling/">How does tool calling work?</a><span>The JSON a tool call is made of, and how a Go client sends and reads it.</span></li><li><a href="/build/first-agent/">How do I build my first agent in Go?</a><span>A complete agent in a few short files of standard-library Go, start to finish.</span></li><li><a href="/basics/choosing-a-model/">Which model should I use for an agent?</a><span>Tool support, cost per finished task, reasoning models, and local models.</span></li><li><a href="/ecosystem/go-agent-frameworks/">Should I use a Go agent framework?</a><span>ADK, Eino, Genkit, LangChainGo, and trpc-agent-go, set against the parts of a harness.</span></li></ul>
