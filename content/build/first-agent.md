---
title: "Build your first AI agent in Go, start to finish"
short: Build your first agent
description: A complete agent in standard-library Go, in a handful of short files. The message types, a client for a chat completions API, the loop, two tools confined to one directory, and a system prompt; run it offline from a recorded run, then live on your own files.
card: A complete agent in a few short files of standard-library Go, run offline and then on your own files.
group: Start building
chapters: 1, 2, 4
stage: all
date: 2026-10-19
weight: 5
work: true
---

This page builds a whole agent and runs it.
It answers questions about the files in one directory: you ask "which release added rate limiting?", and it lists the files, reads the ones that look relevant, and answers from what they say.
Every piece is here, from the types that hold the conversation to the program's `main`, and nothing comes from a framework or an SDK.

The task is deliberately safe.
The agent's two tools only read, and only inside the directory you give it, so you can point it at your own notes without worrying about what it might change.

It assumes you've read [what an agent is](/basics/what-is-an-agent/) and made [a first model call](/basics/first-model-call/), but nothing more.

## Run it first

The program is `firstagent` in the [companion code](https://github.com/blackwell-systems/go-agents-book-code).
From the repository's `code` directory, with no API key:

```console
$ go run ./firstagent
```

Without a key, it replays a run recorded from a live model, `xiaomi/mimo-v2.6-pro`, on October 19, 2026, while the tools read the files of a small sample project for real.
This is what it prints:

```text include=firstagent/testdata/out.txt title="go run ./firstagent"
```

Three calls to the model.
On the first, it asked to see the files.
On the second, it asked to read two of them at once.
On the third, it had what it needed and answered, naming the file each fact came from.
The whole run cost about five hundredths of a cent.

The rest of this page builds that program, one file at a time.
The files import only the standard library, so you can copy the `firstagent` directory into a module of your own and change it from there.

## The types

Four types carry everything: a message, a tool call inside a message, a tool, and, in the next section, the client.

```go include=firstagent/agent.go tag=types
```

`Message` has the same fields, with the same JSON names, as a message in the chat completions API, so the client can send the conversation as it is and decode the model's reply straight into one.
That keeps this program short.
The companion code keeps its own types apart from the wire format instead, which lets one agent talk to providers whose formats differ; [calling a model from Go](/basics/calling-a-model/) shows that design.

Two fields need a word.
A tool call's `Arguments` is a string that holds JSON, not a JSON object, so each tool decodes its own.
`Reasoning` holds the reasoning some models return with a reply; a model that returns it expects it back, unchanged, with the next request, and keeping it on the message does that without any extra code.

A `Tool` has two halves: the name, description, and parameter schema that the model reads, and a Go function, `Run`, that the model never sees.

## The client

The client turns the conversation and the tools into one HTTP request, and the response into a reply:

```go include=firstagent/client.go tag=chat
```

Each call gets a deadline of two minutes, so a request that never comes back can't hang the agent, and Ctrl-C reaches the HTTP request through `ctx`.
The tool definitions go out with every request; the model has no memory of them between calls.

Reading the response is where most of the care goes, because the API can report failure in several ways:

```go include=firstagent/client.go tag=read-reply
```

A non-200 status, an error object inside a 200, and a reply with no choices all become errors.
[Common first errors](/basics/common-errors/) shows what each looks like on the wire.

## The loop

This is the agent:

```go include=firstagent/agent.go tag=run
```

Each pass is one turn: send the conversation, append the reply, and either stop or run the tools.
A reply with no tool calls is the answer.
A reply with tool calls gets one `tool` message for each call, tagged with the call's ID, and the loop goes around again with everything so far.

The order of the appends matters.
The model's reply goes into the conversation before its tools run, so every tool result answers a call the conversation already holds; the API refuses a request where it doesn't.

The loop stops in three ways: an answer, a reply cut off by `finish_reason: length`, which is never treated as an answer, or the turn limit.
The deferred line reports what the run cost however it ended.
[The agent loop](/basics/agent-loop/) goes through each of these rules in more depth.

Running one call is a small function of its own:

```go include=firstagent/agent.go tag=run-tool
```

A tool that fails doesn't stop the run.
Its error goes back to the model as the tool's result, worded so the model can do something about it, and an unknown tool name gets the list of real ones.
Models misspell tool names and send arguments in the wrong shape more often than you'd expect, and most of them correct the call when the error says how.

## The tools

```go include=firstagent/tools.go tag=tools
```

Both tools read through an `fs.FS`, and `main` builds that from an `os.Root`, which refuses any path that leads out of the directory, whether through `..` or a symbolic link.
That makes the confinement a property of the code: the model can ask for `../../.ssh/id_rsa`, and the read simply fails.
The read is capped at 20,000 bytes, so one large file can't fill the conversation.

The descriptions are written for the model, which reads nothing else about a tool.
`read_file`'s says that paths come from `list_files`, which steers the model to list before it reads.
[Designing tools](/build/tool-design/) covers writing tools a model uses well.

## The prompt and main

```go include=firstagent/main.go tag=prompt
```

The system prompt names the job, the tools and when to use them, and how to answer, including what to do when the files don't have the answer.
That last line matters more than it looks: without it, a model asked something the files can't answer is inclined to answer anyway, from its general knowledge.
[Messages, roles, and the system prompt](/basics/messages-and-roles/) has more on writing one.

```go include=firstagent/main.go tag=main
```

`run` builds the client, picks how it reaches the network, opens the directory, and starts the conversation with the system prompt and the question.
With no key, `setTransport` gives the client a transport that answers from the recording; with `-record`, one that saves the run.
Because the switch happens at the HTTP layer, the agent's code is identical online and offline, and the tests use the same idea with a fake server.

## Run it on your own files

Set a key, as [your first model call](/basics/first-model-call/) describes, and point the agent at a directory:

```console
$ export OPENROUTER_API_KEY=sk-or-...
$ go run ./firstagent -dir ~/notes "What did I decide about the cache?"
```

`OPENROUTER_MODEL` picks another model, and [choosing a model](/basics/choosing-a-model/) covers which.
`-record run.json` saves the run's API calls, so you can read exactly what was sent and what came back.

## What it leaves out

This agent is complete, but it is not ready to run unattended.
Each of these is a page of its own:

- **Retries.** A 429 or a 503 ends the run here. [Retries and observability](/production/retries-and-observability/)
- **A budget.** It stops at ten turns, but nothing caps what it spends. [Why the agent stopped](/production/stop-reasons/)
- **Tests of the agent's behavior.** The program's own tests use a fake server; testing what an agent decides is a different job. [Testing an agent offline](/production/testing-offline/)
- **Long conversations.** Every file it reads stays in the conversation, and a run over a big directory grows expensive. [Context management](/build/context-management/)
- **Tools that change things.** Writing files, running commands, or calling APIs needs a sandbox and, for some actions, a person's approval. [Prompt injection and sandboxing](/production/prompt-injection/) and [human approval](/production/approval/)
- **Streaming.** It prints nothing until each reply is complete. [Streaming replies](/build/streaming/)
