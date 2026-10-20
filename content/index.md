---
title: AI Agents in Go
description: Learn how AI agents work at the level of every request, reply, and tool call, by building one step by step with Go's standard library.
layout: home
---

## The loop, in four stages

An agent is a model inside a [harness](/glossary/#harness), the program around it.
On every turn the harness does the same four things:

<ol class="loop-stages"><li><strong>Build the request</strong>Decide what the model sees: instructions, the conversation so far, tool results, and the tools it may call.</li><li><strong>Call the model</strong>Send the request and read the reply, with a deadline and a count of what it cost.</li><li><strong>Run the tools</strong>Carry out the calls the model asked for, and add each result to the conversation.</li><li><strong>Decide</strong>Stop with an answer, stop at a limit, or go around again.</li></ol>

<p class="loop-back">Then back to stage 1, with everything that just happened in the conversation.</p>

Each page on this site works on one of these stages or on the whole loop, and says which.
When an agent misbehaves, the stage tells you where to look: one that never stops has a Decide problem, and one that forgets what it read has a Build-the-request problem.

## The whole loop, in code

The loop talks to the model through one small interface:

```go include=ch01/agent/types.go tag=model
```

Anything with a `Generate` method is a `Model`: a scripted list of replies for tests, or a client for a real model's API.
The loop calls `m.Generate` once per turn and never needs to know which one it has.
Here is the loop as the companion code writes it:

```go include=ch01/agent/loop.go tag=run
```

Its first line uses a small helper, `index`, to split the tools in two: `defs`, the name, description, and parameters of each tool, which go to the model in every request, and `byName`, a map from each tool's name to the Go code that runs it, which the loop uses to find the tool a call names.
The other names are plain types and helpers: a `Message` is one entry in the conversation, a `Request` is the messages and tool definitions for one call, `Limits` holds settings such as `MaxTurns`, a `Result` holds the conversation a run ends with, `runTool` runs one call and turns its output (or its error) into a message for the model, and `ErrMaxTurns` is the error a run returns when it hits the turn limit.

It sends the whole conversation on every call, because the model keeps nothing between calls.
It appends the reply before running its tools, so each tool result answers a call the conversation already holds.
It stops on an answer, at the turn limit, or when `ctx` is cancelled.
[The agent loop in Go](/basics/agent-loop/) explains each line.

Run it with no model at all, from a scripted list of replies:

```console
$ git clone https://github.com/blackwell-systems/go-agents-book-code
$ cd go-agents-book-code/code
$ go run ./ch01
```

The output replays a real model's fix of a failing test, turn by turn.
Then `go run ./ch04` runs the same loop against a real model's recorded replies, with a real `go test` behind its `run_tests` tool.
Both need Go {{code go-version}} or later and no API key.
The repository holds {{code files}} Go files and {{code tests}} test functions, and none of them calls a model; [Examples](/run/) lists every program in the repository.

<!-- paths -->

## Common questions

Each links to the guide that answers it, with Go code you can run.

<ul class="questions"><li><a href="/basics/getting-set-up/">What do I need to get started?</a><span>Go 1.27, the companion code, and examples that run with no API key.</span></li><li><a href="/basics/what-is-an-agent/">What is an AI agent, and how is it different from a chatbot?</a><span>A program where the model picks each next step and your Go code carries it out.</span></li><li><a href="/basics/first-model-call/">How do I make my first LLM API call?</a><span>An API key, one request with curl, and every part of the reply, including its cost.</span></li><li><a href="/basics/messages-and-roles/">What are messages, roles, and the system prompt?</a><span>The four roles in a conversation, and how to write instructions for an agent.</span></li><li><a href="/basics/tool-calling/">How does tool calling work?</a><span>The JSON a tool call is made of, and how a Go client sends and reads it.</span></li><li><a href="/build/first-agent/">How do I build my first agent in Go?</a><span>A complete agent in a few short files of standard-library Go, start to finish.</span></li><li><a href="/basics/choosing-a-model/">Which model should I use for an agent?</a><span>Tool support, cost per finished task, reasoning models, and local models.</span></li><li><a href="/ecosystem/go-agent-frameworks/">Should I use a Go agent framework?</a><span>ADK, Eino, Genkit, LangChainGo, and trpc-agent-go, set against the parts of a harness.</span></li></ul>
