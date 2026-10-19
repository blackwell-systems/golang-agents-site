---
title: "MCP in Go: a client and a server, both sides"
short: MCP in Go
description: The Model Context Protocol lets any agent use tools a separate program offers. What the protocol is on the wire, how a Go client matches responses to calls and stops a server in the order the protocol asks, what the extra process costs, how much to trust another program's tools, and when to use the official Go SDK.
card: The Model Context Protocol on the wire, a Go client for it, and how far to trust another program's tools.
chapters: 10
stage: 3
date: 2026-10-18
weight: 80
art: ch10
---

An agent's own tools are code in its own program.
The services it might use, an issue tracker, a code host, a database, are mostly someone else's, and their owners know best what tools they should offer.
The [Model Context Protocol](/glossary/#mcp) lets a program publish tools and data so that an agent in another process can use them, with no code written for that particular service.

This page writes both sides by hand, a server for an issue tracker and a client in the agent, before turning to the official Go SDK.
Writing it by hand has a second purpose: an MCP server's tool descriptions go straight into your model's context, and deciding what to check requires knowing which messages carry that text.

## The protocol in brief

An MCP server offers tools, which the model calls; resources, data the client can read; and prompts, templates a person picks from.
An agent needs the tools.

The messages are [JSON-RPC](/glossary/#json-rpc) 2.0, which has three kinds: requests, each with an ID; responses, each naming the ID it replies to; and notifications, which get no reply.
For a local server, the transport is the server's standard input and output, one message per line.
The server's standard error is its own, for logs.

One Go type covers every kind of message, since they differ only by which fields are set:

```go include=ch10/mcp/jsonrpc.go tag=message
```

In the revision dated 2026-07-28, the current one, each request is self-contained: a `_meta` field carries the protocol version along with the client's name and capabilities.
A server therefore needs no memory of earlier requests, and any instance of it can serve any request.
Before that revision a connection began with a handshake, and many published servers still work that way, so a client sends the newer discovery request first and drops back to the handshake when the server doesn't understand it.

A session goes: `server/discover` to learn the versions and capabilities, `tools/list` a page at a time, then `tools/call` with a name and arguments.

## Two kinds of failure

A tool that ran and failed returns a result with `isError` set and text meant for the model; the agent sends it back as a tool error, and the model can correct its call.
Asking for a tool the server never listed produces something else, a protocol error: a JSON-RPC `error` with code -32602, about the request rather than the work.
And a call that gets no answer in time is cancelled: the client sends `notifications/cancelled` naming the request, and the server stops the work.

## The client: one reader, many callers

Several goroutines may call through one connection at once.
Each call registers a buffered channel under its request ID before it sends, and one goroutine reads everything the server writes and hands each response to its call:

```go include=ch10/mcp/client.go tag=route
```

Because each channel is buffered for exactly one message, the reading goroutine never blocks on a slow caller, and a response for a caller that has already given up goes nowhere.
If the server's output stops, each caller still waiting gets `ErrServerGone`, along with the tail of the server's standard error, which is usually where a crashed server explains itself.

Every call has a timeout, a minute by default, set with `context.WithTimeoutCause` so the error says which limit fired.
In the `TestCancel` test, a tool that waits on its context saw that context end 100 ms in, under a 100 ms timeout, and the connection kept working for the following call.

## Starting and stopping a server

Two details matter more than they look.

**Build the server's environment from nothing.** `exec.Cmd` inherits the parent's environment when `Env` is nil, and the agent's environment holds its API key; a server you downloaded has no business with it.
A test sets the key and asks the server whether it can see it; it can't.
This isn't a sandbox: the server still runs as you.

**Stop it in the protocol's order.** First close the server's standard input, its cue to exit; then wait for it; kill it only if it is still running after a grace period.
`exec.CommandContext` would kill it at once and skip the first two steps, so the client uses `exec.Command` and `context.AfterFunc` to start the shutdown when the run's context ends.
Production clients go further, starting the server in a process group of its own and sending SIGTERM before SIGKILL, since real servers are often started through a launcher whose child survives the launcher being killed.

## What the extra process costs

`BenchmarkCall` timed one `tools/call` three ways, on an Apple M1 Pro that was busy with other jobs, taking the median of five runs of 2,000 calls each (October 2026):

| How the tool is reached | Time per call |
| --- | --- |
| A server in its own process, over standard input and output | 141 µs |
| A server in the same process, over pipes | 18 µs |
| The tool's function, called directly | 0.16 µs |

The process boundary made a call roughly 900 times slower than calling the function directly, and it still cost less than a ten-thousandth of one model call, which takes seconds.
The m1 triage task was also run on the default model three times through the tracker's MCP server and three times with the agent's own tracker tools; all six runs passed all 16 checks, and the model used the server's tools as readily as its own.
The protocol cost the agent nothing it could measure.

## Server tools as agent tools

An adapter makes each server tool an ordinary `agent.Tool`, with three rules:

- **Prefix names with the server's name**, the one you gave it in your configuration, such as `tracker__issue_search`, since two servers can both have a `search`. Names are rewritten to the characters model APIs accept, and one registry refuses any two that come out alike.
- **Take the tool list once per run**, so tools can't change partway through.
- **Treat what a server says as untrusted.** Descriptions and results are capped and have invisible characters removed, annotations such as "read-only" are hints, ignored unless the server is marked trusted, and a server that dies becomes `ErrToolFatal`, which stops the run instead of failing call after call.

The trust question deserves its own page: [one sentence in a tool description decided 79 of 80 first moves](/findings/tool-description-poisoning/), across four models.
The defense that works is upstream of the model: a person reads a server's tools once, and the client pins a hash of each definition, so a changed description stops the run until someone looks.

## When to use the official SDK

The official MCP Go SDK covers what a hand-written client skips: servers on the network over HTTP and the authorization that goes with them, resources and prompts, progress reports, servers that pause a call to ask the user something, and the old and new protocol revisions, as client and as server.
`mcp.AddTool` generates a tool's schema from its argument type and checks each call against it before the handler runs.
The hand-written client and the SDK's server talk to each other, and so do the SDK's client and the hand-written server; the module's tests check both directions.

Build a server that other people will run on the SDK, nearly always, since it follows protocol revisions for you.
Inside your own agent the client is small: the one here is about 490 lines, not counting comments, plus 190 it shares with its server, and it covers what an agent uses without adding a module to your build until you decide you want one.

## When to use it

```fit
reach: Tools live in another program or team. A call to a server process took 141 microseconds, under a ten-thousandth of a model call, and three triage runs through the tracker's server passed every check, as did three with the agent's own tools.
leave: Servers whose tool definitions nobody has read. One sentence in a description steered 79 of 80 first moves, and a label cut that only to 57.
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), with no key set:

```console
$ go run ./ch10
$ go test ./ch10/mcp/ -run 'Cancel|Close|StartEnv' -v
```

The first replays a triage run that files its issue through the tracker's MCP server, started as a process; the second runs the cancellation, shutdown, and environment tests.
The SDK version of the server is a module of its own, in [`ch10/sdk`](code:ch10/sdk).
