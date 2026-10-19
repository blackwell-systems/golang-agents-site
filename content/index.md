---
title: AI Agents in Go
description: Learn how AI agents work at the level of every request, reply, and tool call, by building one step by step with Go's standard library.
layout: home
---

## Common questions

Each links to the guide that answers it, with Go code you can run.

<ul class="questions"><li><a href="/basics/getting-set-up/">What do I need to get started?</a><span>Go 1.27, the companion code, and examples that run with no API key.</span></li><li><a href="/basics/what-is-an-agent/">What is an AI agent, and how is it different from a chatbot?</a><span>A program where the model picks each next step and your Go code carries it out.</span></li><li><a href="/basics/first-model-call/">How do I make my first LLM API call?</a><span>An API key, one request with curl, and every part of the reply, including its cost.</span></li><li><a href="/basics/messages-and-roles/">What are messages, roles, and the system prompt?</a><span>The four roles in a conversation, and how to write instructions for an agent.</span></li><li><a href="/basics/calling-a-model/">How do I call an LLM API from Go?</a><span>A chat completions client in standard-library Go, with deadlines and error checks.</span></li><li><a href="/basics/tool-calling/">How does tool calling work?</a><span>The JSON a tool call is made of, and how a Go client sends and reads it.</span></li><li><a href="/build/streaming/">How do I stream a model's reply?</a><span>Server-sent events read with bufio, and tool calls rebuilt from their fragments.</span></li><li><a href="/basics/choosing-a-model/">Which model should I use for an agent?</a><span>Tool support, cost per finished task, reasoning models, and local models.</span></li><li><a href="/build/workflows-vs-agents/">Should I build a workflow or an agent?</a><span>Five shapes of program that call a model, and how to pick the simplest that works.</span></li><li><a href="/build/tool-design/">How do I design tools a model uses well?</a><span>Names, descriptions, schemas from Go structs, short results, and useful errors.</span></li><li><a href="/build/context-management/">How do I keep a long run affordable?</a><span>Prompt caching, clearing old tool results, compaction, and notes.</span></li><li><a href="/build/mcp/">How do I connect an agent to MCP servers?</a><span>The Model Context Protocol on the wire, and a Go client for it.</span></li><li><a href="/production/testing-offline/">How do I test an agent without calling a model?</a><span>Scripted models, replayed HTTP, and checks on outcomes in place of transcripts.</span></li><li><a href="/production/prompt-injection/">How do I defend against prompt injection?</a><span>A threat model, sandboxes for the code an agent runs, and layers of defense.</span></li><li><a href="/ecosystem/go-agent-frameworks/">Should I use a Go agent framework?</a><span>ADK, Eino, Genkit, LangChainGo, and trpc-agent-go, set against the parts of a harness.</span></li><li><a href="/production/launch-checklist/">What does it take to run an agent in production?</a><span>Twenty-three checks for an agent that runs unattended and spends money.</span></li></ul>

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
The repository holds {{code files}} Go files and {{code tests}} test functions, and none of them calls a model; [Examples](/run/) lists every chapter's program.

## How this guide works

- **Drawn from a book, with tested code.** Each guide draws on the book *Building AI Agents in Go*, and its code comes from the book's [public repository](https://github.com/blackwell-systems/go-agents-book-code) (Apache 2.0), where every excerpt belongs to a program with tests.
- **One agent, grown step by step.** The examples follow one agent from a 30-line loop that fixes a failing test to a crash-triage agent for a small Go web service, so each page adds to code you have already seen.
- **Measured, with the method.** Claims about what works come from recorded runs, with the task, the models, the number of runs, and the date. The failures are measured too: [twelve measured mistakes](/findings/measured-mistakes/), each with its method.
- **Offline by default.** Every example runs without a key and costs nothing. With `OPENROUTER_API_KEY` set, the same programs call a live model.
- **The standard library first.** The code reaches the model over plain `net/http` and `encoding/json/v2`, so you can see every moving part before deciding what to adopt.
- **Clear about its limits.** Retrieval, memory across unrelated runs, and a model that hands work to other agents at run time are left out on purpose; [what this covers](/scope/) says why, and what was measured instead. [Why Go](/ecosystem/why-go/) makes the case for the language from the book's measurements.
