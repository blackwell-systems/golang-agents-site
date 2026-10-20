---
title: Streaming an agent's progress to a browser
short: Streaming to a browser
description: A server written with net/http alone sends an agent's events, as they happen, to a page that reads them with EventSource. The headers, the flush, a deadline per write, a heartbeat, a client that leaves, and a shutdown that ends open streams, from a tested example.
card: An agent's progress streamed to a web page with net/http and server-sent events.
kind: Example
chapters: 6
date: 2026-10-18
weight: 30
art: ch06
---

Reading server-sent events from the model's API turns them into the agent's own events: text as it arrives, each tool call, each result, the end of a turn, and the end of a run.
This example sends those events onward, from a Go server to a web page, so a person can watch a run as it happens and stop it.
It is the companion code's [`ssebrowser` example](code:ssebrowser), built on `net/http` alone, with tests that run under the race detector.

The model is scripted, a replay of a run on the `duration` project slowed down so you can watch it, so everything runs offline.
The tools, `go test` among them, run for real.

## Run it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go run ./ssebrowser
```

Open the address it prints and press Run.
The page shows the model's text as it streams, each tool call and its result, and the end of each turn.
Press Stop partway through and the run on the server stops too.
To see the raw stream without a browser:

```console
$ curl -N 'http://localhost:8080/events?task=fix'
```

The page sends its task in the query string, but the scripted model gives the same replies whatever the task says.

## One request, one stream

The event stream is a long-lived `GET`, since a browser's `EventSource` can't send anything else.
Here is the handler:

```go include=ssebrowser/server.go tag=events
```

Three headers make it a stream: `text/event-stream` tells the browser what it is reading, `Cache-Control: no-cache` keeps caches out of the way, and `X-Accel-Buffering: no` asks a proxy such as nginx not to hold events back in a buffer.

The run gets a context derived from the request's.
When the browser closes the connection, because the person pressed Stop or closed the tab, the request's context ends, and the run's model call and tools stop with it.
The deferred function cancels the run and then reads the channel until it closes, so the goroutine that drives the agent has always finished before the handler returns.
That goroutine has one owner, the handler, and one way to stop, the context.

The agent runs in its own goroutine for a reason: while the model is thinking, nothing arrives for a while, and the handler has to keep sending something.
`pump`, in the same [file](code:ssebrowser/server.go), waits on two things at once, the next event and a ticker, and sends a comment line, `: heartbeat`, when the ticker fires first.
Proxies and load balancers may close a connection that stays silent too long; the heartbeat keeps it open, and `EventSource` ignores comment lines.
Only the handler writes to the client, because a `ResponseWriter` isn't safe to use from two goroutines.

## Every write is flushed, with a deadline

```go include=ssebrowser/server.go tag=send
```

`http.ResponseController` gives a handler two things a plain `ResponseWriter` hides.
`Flush` pushes each event to the client at once instead of letting it wait in a buffer, which is the whole point of a stream.
`SetWriteDeadline` sets a limit on each write: a client that stops reading, with the connection still open, would otherwise hold its goroutine and its run forever.
Past the deadline the write fails, `pump` returns, and the deferred cancel stops the run.
The program allows ten seconds per write and sends a heartbeat every fifteen.

## The wire format

Each event is a line naming its kind, a line of JSON, and a blank line:

```go include=ssebrowser/sse.go tag=encode
```

A newline inside the data would break its single `data:` line, and a blank line would end the event; JSON escapes newlines, so a tool result of any length fits on its single `data:` line.
Naming each event lets the page attach one listener per kind instead of parsing a type field out of every message.

## The page

The page reads the stream with `EventSource`, the browser's built-in client for server-sent events:

```js include=ssebrowser/index.html tag=client
```

One detail matters more than it looks.
When the stream ends, `EventSource` reconnects by default, and here a reconnection would start a whole new run.
So the page closes the `EventSource` itself when it sees the `stopped` event or an error, and the Stop button does the same.

## Shutting down

A stream never goes idle by itself, so `http.Server.Shutdown`, which waits for active connections to go idle, would wait out its whole timeout.
The program gives the server a `BaseContext` that returns the program's own context, which Ctrl-C cancels:

```go include=ssebrowser/main.go tag=serve
```

Every request's context descends from it, so cancelling it ends every open stream and its run, and `Shutdown` then finds nothing left to wait for.
`ReadHeaderTimeout` bounds how long a client may take to send its headers, a limit worth setting on any server that faces the internet.

## What the tests check

```console
$ go test -race ./ssebrowser
```

The tests read the stream from an `httptest` server and check the headers and every event of a full run, that a quiet stream sends heartbeats, that a client leaving cancels the run, that a failed write stops the handler, that bad requests are refused, and that shutdown ends open streams.

## What a real service adds

The example is small on purpose, and its README lists what a public service would change:

- **Error text.** The page shows a run's error as it is; a public service would send a short stop reason instead of internal detail.
- **Limits.** Each request starts a run. A real service caps how many run at once, and per user, since each one spends money.
- **Reconnecting.** A dropped connection loses the run. To let a client pick up where it left off, give each event an `id:` line, keep the run going apart from the request, and resume from the `Last-Event-ID` header the browser sends when it reconnects.

## Key files

- [`ssebrowser/server.go`](code:ssebrowser/server.go): the handler, the goroutine that runs the agent, and the writes.
- [`ssebrowser/sse.go`](code:ssebrowser/sse.go): each agent event in the event stream format.
- [`ssebrowser/index.html`](code:ssebrowser/index.html): the page.
- [`ssebrowser/main.go`](code:ssebrowser/main.go): the program, the scripted run, and the server's start and shutdown.
