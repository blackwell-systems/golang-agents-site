---
title: "Messages, roles, and the system prompt"
short: Messages and roles
description: A conversation with a model is an ordered list of messages, each with a role. What system, user, assistant, and tool messages are for and who writes each, the Go type that holds them, and how to write a system prompt for an agent, with a real one taken apart.
card: The four roles in a conversation, the Go type that holds them, and how to write a system prompt.
group: Talk to a model
chapters: 1, 2, 7
stage: 1
date: 2026-10-19
weight: 27
---

Everything you send a model is one list of messages, and everything an agent does adds to that list.
Each message has a role, which tells the model who is speaking.
There are four, and knowing who writes each one is most of what you need to read an agent's code.

## The four roles

| Role | Who writes it | What it holds |
| --- | --- | --- |
| `system` | You, the developer, once, at the start | Instructions: the job, the rules, how to finish |
| `user` | The person using the program, or your program on their behalf | The request or the task |
| `assistant` | The model | Its reply: text, a request to run tools, or both |
| `tool` | Your program, after running a tool | What the tool returned, tagged with the call it answers |

A plain chat uses the first three.
An agent adds the fourth: when the model's reply asks for a tool, your program runs it and appends a `tool` message with the result, and the next request carries that result back to the model.
[Tool calling on the wire](/basics/tool-calling/) shows those messages as JSON.

The list is the model's whole view of the conversation.
The model keeps nothing between requests, so your program sends the full list every time, oldest message first.
On the way in, the provider joins the messages into a single text, with markers for where each role starts and ends; [How models pick the next token](/basics/how-models-work/) shows that text.

## The messages in Go

The companion code holds a conversation as a slice of one small struct, and a request as that slice plus the tools the model may call:

```go include=ch02/agent/types.go tag=messages
```

`Content` holds the text, `ToolCalls` holds the tool requests in an assistant message, and `ToolCallID` ties a tool message to the request it answers.
The client turns these into the JSON the API expects and back again, so the rest of the program never handles JSON directly.

## The system prompt

The system message, usually called the **system prompt**, is where you tell the model what job it's doing.
In a chat program it can be one sentence; the [first model call](/basics/first-model-call/) used a single line asking for short answers aimed at Go programmers.

An agent's first task can get by without one.
The bug-fixing agent in the early chapters sends a single `user` message, two short sentences saying the project's tests fail and asking the model to find the cause and fix it, and that's enough, because the task says everything the model needs and the tools do the rest.

A system prompt earns its place once the agent has a standing job that every task shares.
This is the one the crash-triage agent sends before every batch of crash reports:

```text include=ch07/triage/prompt.md title="The crash-triage agent's system prompt"
```

It's short, and each part does a specific job:

- **What and where.** The first paragraph names the job, the service, and where its code is, so the model doesn't have to guess at paths.
- **The steps.** A numbered list of what "done" means for each problem, in order, including the test that proves the cause.
- **How to use the tools.** Which tool to start with and which tools fit which step. The tools' own descriptions say what each one does; the prompt says when to reach for it.
- **One rule.** "Don't change or delete existing tests", since an agent that deletes a failing test has made the tests pass without fixing anything.
- **How to finish, and what to do when stuck.** Without a defined ending, a model may stop with a summary in prose that your program can't read.

## What a system prompt can't do

The model reads the system prompt as instructions, and follows them most of the time.
It also reads everything else in the conversation, including text that came from tools, files, and web pages, and some of that text may give it instructions of its own.
So a rule that matters for safety, such as "never edit this file" or "never spend more than a dollar", belongs in your Go code, where the model can't argue with it.
[Prompt injection and sandboxing](/production/prompt-injection/) and [human approval](/production/approval/) cover how.

## Rules for the list

In the chat completions format, the API checks the list's shape and refuses a request that breaks it:

- The `system` message comes first.
- Every tool call in an assistant message is answered by exactly one `tool` message, with the call's ID, before the next assistant message.

The API can't check one more rule, so the program has to: messages stay in the order they happened. Reordering or dropping one, to save space, can leave a tool result with no call or a call with no result, and the API refuses that.

[The agent loop](/basics/agent-loop/) keeps all three by appending the model's reply before running its tools, and [context management](/build/context-management/) covers how to shorten a long list without breaking them.
