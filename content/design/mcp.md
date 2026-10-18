---
draft: true
title: "MCP in Go: a client and a server, both sides"
short: MCP in Go
description: The Model Context Protocol lets any agent use tools a separate program offers. What the protocol is on the wire, how a Go client matches responses to calls and stops a server in the order the protocol asks, what the extra process costs, how much to trust another program's tools, and when to use the official Go SDK.
chapters: 10
stage: 3
date: 2026-10-18
weight: 70
---

An agent's own tools are code in its own program.
The services it might use, an issue tracker, a code host, a database, are mostly someone else's, and their owners know best what tools they should offer.
The [Model Context Protocol](/glossary/#mcp) is a standard way for a program to offer tools, and data, to an agent that runs elsewhere, and for the agent to call them without code written for that service.

Chapter 10 of the book writes both sides by hand, a server for its issue tracker and a client in the agent, before showing the official Go SDK.
Writing it by hand has a second purpose: an MCP server's tool descriptions go straight into your model's context, and deciding what to check requires knowing which messages carry that text.

## The protocol in brief

An MCP server offers tools, which the model calls; resources, data the client can read; and prompts, templates a person picks from.
An agent needs the tools.

The messages are [JSON-RPC](/glossary/#json-rpc) 2.0: requests with an ID, responses carrying the ID of the request they answer, and notifications that expect no answer.
For a local server, the transport is the server's standard input and output, one message per line.
The server's standard error is its own, for logs.

The book's code needs one Go type for every kind of message, since they differ only by which fields are set:

```go include=ch10/mcp/jsonrpc.go tag=message
```

In the current revision, 2026-07-28, every request carries what the server needs to answer it, the protocol version, the client's capabilities and name, in a `_meta` field, so the server keeps nothing between requests and any copy of it can answer any request.
Earlier revisions opened each connection with a handshake instead, and most servers you can download still speak one of them; the book's client tries the current discovery first and falls back to the handshake.

A session goes: `server/discover` to learn the versions and capabilities, `tools/list` a page at a time, then `tools/call` with a name and arguments.

## Two kinds of failure

A tool that ran and failed returns a result with `isError` set and text meant for the model; the agent sends it back as a tool error, and the model can correct its call.
A call to a tool the server doesn't have is a protocol error, a JSON-RPC `error` with code -32602, which concerns the request itself.
And a call that gets no answer in time is cancelled: the client sends `notifications/cancelled` naming the request, and the server stops the work.

## The client: one reader, many callers

Several goroutines may call through one connection at once.
Each call registers a buffered channel under its request ID before it sends, and one goroutine reads everything the server writes and hands each response to its call:

```go include=ch10/mcp/client.go tag=route
```

The channel has room for one message, so the reader never waits on a call, and a response to a call that already gave up is dropped.
When the server's output ends, every call still waiting returns `ErrServerGone`, with the last lines the server wrote to standard error, since the reason a server died is usually there.

Every call has a timeout, a minute by default, set with `context.WithTimeoutCause` so the error says which limit fired; in the chapter's test, a 100 ms timeout cancelled the tool's own context 100 ms after the call began, and the next call on the connection worked.

## Starting and stopping a server

Two details matter more than they look.

**Build the server's environment from nothing.** `exec.Cmd` inherits the parent's environment when `Env` is nil, and the agent's environment holds its API key; a server you downloaded has no business with it.
A test sets the key and asks the server whether it can see it; it can't.
This isn't a sandbox: the server still runs as you.

**Stop it in the protocol's order.** Close its input, which a server takes as the signal to exit; wait; and only then kill it.
`exec.CommandContext` would kill it at once and skip the first two steps, so the client uses `exec.Command` and `context.AfterFunc` to start the shutdown when the run's context ends.
Production clients go further, starting the server in a process group of its own and sending SIGTERM before SIGKILL, since real servers are often started through a launcher whose child survives the launcher being killed.

## What the extra process costs

The chapter benchmarked one `tools/call` three ways on one machine:

| How the tool is reached | Time per call |
| --- | --- |
| A server in its own process, over standard input and output | 141 µs |
| A server in the same process, over pipes | 18 µs |
| The tool's function, called directly | 0.16 µs |

A call to a server process took about 900 times as long as a direct call, and under a ten-thousandth of a model call.
Running the triage task three times through the tracker's MCP server and three times with hand-written tools, every run passed every check, and the model worked with the server's tools as readily as with its own.
The protocol cost the agent nothing it could measure.

## Server tools as agent tools

An adapter makes each server tool an ordinary `agent.Tool`, with three rules:

- **Prefix names with the server's name**, the one you gave it in your configuration, such as `tracker__issue_search`, since two servers can both have a `search`. Names are rewritten to the characters model APIs accept, and one registry refuses any two that come out alike.
- **Take the tool list once per run**, so tools can't change partway through.
- **Treat what a server says as untrusted.** Descriptions and results are capped and have invisible characters removed, annotations such as "read-only" are hints, ignored unless the server is marked trusted, and a server that dies becomes `ErrToolFatal`, which stops the run instead of failing call after call.

The trust question deserves its own page: [one sentence in a tool description decided 79 of 80 first moves](/findings/tool-description-poisoning/), across four models.
The defense that works is upstream of the model: a person reads a server's tools once, and the client pins a hash of each definition, so a changed description stops the run until someone looks.

## When to use the official SDK

The official MCP Go SDK implements the whole protocol: the HTTP transport with its authorization rules, resources and prompts, progress notifications, requests that let a server ask the person a question, and both protocol eras on both sides.
`mcp.AddTool` generates a tool's schema from its argument type and checks each call against it before the handler runs.
The book's client and the SDK's server talk to each other, and so do the SDK's client and the book's server; the module's tests check both directions.

The chapter's advice: for a server anyone else will run, use the SDK almost always, since it speaks both eras and changes when the protocol does.
For a client inside an agent, a hand-built one, about 490 lines without comments plus 190 shared with the server, does what an agent needs and keeps the dependency out until you choose to take it.

## When to use it

```fit
reach: Tools live in another program or team. A call to a server process took 141 microseconds, under a ten-thousandth of a model call, and three triage runs through the tracker's server passed every check, as did three with the agent's own tools (chapter 10).
leave: Servers whose tool definitions nobody has read. One sentence in a description steered 79 of 80 first moves, and a label cut that only to 57 (chapter 10).
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), with no key set:

```console
$ go run ./ch10
$ go test ./ch10/mcp/ -run 'Cancel|Close|StartEnv' -v
```

The first replays a triage run that files its issue through the tracker's MCP server, started as a process; the second runs the cancellation, shutdown, and environment tests.
The SDK version of the server is a module of its own, in [`ch10/sdk`](code:ch10/sdk).
