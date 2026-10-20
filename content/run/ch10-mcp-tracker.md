---
title: An agent filing issues through an MCP server
short: "ch10: Filing issues over MCP"
description: The crash-triage agent reaching its issue tracker only through the Model Context Protocol, with the server started as a separate process. Replay the run offline, serve the tracker to any MCP client, and pin the server's tool definitions so a changed description stops the run.
card: The agent filing issues through an MCP server, with the server's tool definitions pinned.
chapters: 10
stage: 3
date: 2026-10-18
weight: 40
art: ch10
---

The tracker is the one part of the triage job that lives in another program, with its own data.
In this example the agent reaches it only through [MCP](/glossary/#mcp): the program starts the tracker's MCP server as a child process, lists its tools, and offers them to the model under prefixed names such as `tracker__issue_search`.
The model's replies are recorded, but the server, the protocol, and every tool call run for real.

## What it shows

- A hand-written MCP client and server in Go, newline-delimited JSON-RPC over standard input and output. [MCP in Go](/build/mcp/) explains both sides.
- Server tools adapted into agent tools, with names prefixed by the server's name, descriptions labeled as the server's, and the server's hints about its tools, such as which ones only read, ignored unless the server is trusted.
- A server whose tool definitions can be pinned by hash, so a change stops the run before it starts.

## Run it

From the `code` directory of a clone of [go-agents-book-code](https://github.com/blackwell-systems/go-agents-book-code), with no `OPENROUTER_API_KEY` set:

```console
$ go run ./ch10
```

It replays a 13-turn run of the same task as the [ch07 example](/run/ch07-crash-triage/), a batch of four crash reports, two of which already have issues filed.
Notice the tool names in the second turn:

```text include=ch10/testdata/out/m1-mcp.txt lines=8..18 title="go run ./ch10, turn 2"
```

`tracker__issue_search` is the server's `issue_search`, reached over the protocol.
On this run the agent marked both already-filed crashes as duplicates of their existing issues, filed one new issue through the server, and scored 16 out of 16 on the [crash catalog's](/glossary/#crash-catalog) checks, for $0.0055:

```text include=ch10/testdata/out/m1-mcp.txt lines=139..149 title="The end of the run"
```

The agent caught the duplicates this time because the prompt had changed since that earlier run; the protocol didn't change what the agent could do.
In three runs each way on the default model, `anthropic/claude-haiku-5.5`, the agent worked with the server's tools as readily as with its own hand-written ones, and every run passed every check.

## Serve the tracker to anything

The server is an ordinary command, and any MCP client can start it: the triage agent, or an assistant app you already use.

```go include=ch10/cmd/tracker-mcp/main.go tag=main
```

Its log goes to standard error, since standard output carries only protocol messages: a single leftover `fmt.Println` would reach the client as a line it can't parse as a message.
To connect it to an assistant app, add a server entry to that app's MCP configuration naming the command to run, `go run ./ch10/cmd/tracker-mcp -data /path/to/issues.json`, from the `code` directory; each app documents its own format.

## Pin the server's tools

A server's tool descriptions go straight into the model's context.
[One sentence in a description decided 79 of 80 first moves](/findings/tool-description-poisoning/) in one recorded measurement, so the client can pin each tool's definition by hash after a person reads it:

```console
$ go run ./ch10 -print-digests > pins.json
$ go run ./ch10 -tracker-pins pins.json
```

The first prints the four tools' digests; the second runs the replay with them checked.
Next, edit a single word in the description of `issue_search` in `ch10/triage/issuetools.go`, which defines the tools the server serves, then repeat the second command.
The run stops before it starts, with an error naming the tool, its new digest, and the pinned one.

## Run the tests

```console
$ go test ./ch10/mcp/ ./ch10/mcptools/
```

They cover the protocol (discovery, pagination, cancellation at a timeout, falling back to the older `initialize` handshake for servers that need it), launching and shutting down a server process in the sequence the protocol requires, keeping the agent's API key out of the server's environment, and the trust rules: hidden characters removed, descriptions labeled, and pins that fail on any change to a description or a schema.

## Key files

| File | What it holds |
| --- | --- |
| [`ch10/mcp/jsonrpc.go`](code:ch10/mcp/jsonrpc.go) | JSON-RPC messages, errors, and line framing |
| [`ch10/mcp/client.go`](code:ch10/mcp/client.go) | The client: one reader, calls matched by ID, timeouts, starting and stopping a server |
| [`ch10/mcp/server.go`](code:ch10/mcp/server.go) | The server: one goroutine per request, cancellation, pagination |
| [`ch10/mcptools/adapter.go`](code:ch10/mcptools/adapter.go) | Server tools as agent tools |
| [`ch10/mcptools/trust.go`](code:ch10/mcptools/trust.go) | Labels, hidden characters, schema limits, digests, and pins |
| [`ch10/trackermcp/trackermcp.go`](code:ch10/trackermcp/trackermcp.go) | The tracker's four tools, served |
| [`ch10/sdk`](code:ch10/sdk) | The same server on the official MCP Go SDK, in a module of its own |
