---
title: "Streaming in Go: io.Reader, bufio.Scanner, server-sent events, and http.Flusher"
short: Streams in Go
description: How to read and write a stream in Go, for an agent that streams a model's reply. What io.Reader promises, reading lines with bufio.Scanner and its size limit, the server-sent events format and the [DONE] marker, a reader that turns a stream into an iterator, stopping a stream with the context and an idle timeout, writing a stream from a server with http.Flusher and http.ResponseController, and the mistakes.
card: Reading and writing a stream in Go: io.Reader, bufio.Scanner, server-sent events, and flushing.
group: Calling APIs
chapters: 6
stage: 2
level: intermediate
date: 2026-10-23
weight: 50
art: go-streams
---

A language model writes its reply one token at a time (a token is a short piece of text, often a word or part of one).
A program can wait until the whole reply exists and receive it in one piece, or it can ask the API to send each piece as it is written.
The second way is called streaming.
This page covers the Go that streaming needs, from the byte-level contract of `io.Reader` up to a server that sends its own stream, so that you can read one and write one.
The code is from the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), where it is tested.
It assumes you know the [HTTP client](/go/http-client/) basics and have read the page on [iterators](/go/iterators/).

## Why an agent streams

Without streaming, the program sends a request and then has nothing to show until the model has written the whole reply, which can take many seconds.
With streaming, the first words arrive almost at once and the rest follow as they are written, so a person watching sees text appear, and the program learns early when a connection has died.
[Streaming model replies](/build/streaming/#what-streaming-buys-measured) gives the measured numbers: the first text shows up well before the unstreamed reply does, while the total time barely changes.
The model takes about as long either way; streaming changes when you see the reply.

## The idea underneath: io.Reader

A response body in Go is an `io.Reader`, an interface with one method:

```go
type Reader interface {
	Read(p []byte) (n int, err error)
}
```

You pass `Read` a slice, and it copies some bytes into it and tells you how many.
The documentation (`go doc io.Reader`) sets out the contract, and each part matters for streams:

- `Read` reads **up to** `len(p)` bytes.
  By convention, when some data is available but not `len(p)` bytes, it returns what is available instead of waiting for more.
  On a stream, then, one `Read` call may return a single token's worth of bytes.
  Never assume a `Read` fills the slice.
- `Read` may wait.
  On an `http.Response.Body`, a call blocks until bytes arrive from the network, the stream ends, or something fails.
  That waiting is how a stream's bytes "arrive over time" in a Go program: you call `Read`, and it returns when there is something to return.
- `Read` can return `n > 0` and an error together.
  The caller must **process the `n` bytes first** and only then look at `err`.
- The end of the data is the error `io.EOF`.
  It means "no more input", the normal way for data to end.
  A reader may return the final bytes with `err == io.EOF` in the same call, or return them with a nil error and report `io.EOF` with `n == 0` on the next call.
- A `Read` that returns `0, nil` means nothing happened, and does not mean the end.

You rarely write a `Read` loop yourself.
A few helpers in the standard library do it for you: `io.ReadAll` reads until `io.EOF` and returns everything, `io.Copy` moves everything from a reader to a writer, and `bufio.Scanner` hands you one line at a time.
Of the three, only the last is any use for a reply that should show up as it arrives, because the other two do not return until the stream ends.

## Reading line by line: bufio.Scanner

The format a model API streams in is text with one item per line, so the natural tool is `bufio.Scanner`, which wraps a reader and splits what it reads into lines:

- `sc.Scan()` reads the next line and reports whether there was one.
  It returns `false` at the end of the input or on an error.
  By default the line ends at `\n`, and a trailing `\r` before it is dropped (`bufio.ScanLines`).
- `sc.Bytes()` returns the line as a `[]byte`, and `sc.Text()` returns a `string` copy of it.
- `sc.Err()` returns the first error that was not `io.EOF`.
  A normal end of input leaves it nil, so after a `Scan` loop ends, `Err() == nil` means the reader reached `io.EOF`.

`Bytes` is cheaper, since it makes no copy, but the slice is only valid until the next call to `Scan`: the documentation warns that "the underlying array may point to data that will be overwritten by a subsequent call to Scan".
If you keep a line past the next `Scan`, store `sc.Text()` or a copy of the bytes, never `sc.Bytes()` itself.
The reader in the next sections uses `Bytes` safely, because it finishes with each line before it scans the next one.

### The 64 KiB line limit

A scanner holds a whole line in memory, so it caps the line length.
The default cap is `bufio.MaxScanTokenSize`, 64 KiB (65,536 bytes).
A line longer than that stops the scan: `Scan` returns `false`, and `Err()` returns `bufio.ErrTooLong`.
The same stop happens in the middle of a stream, so the lines after the long one are never read.

A model's stream can reach that length, because a tool call's arguments are sent as one string and a long argument can come through on a single `data:` line.
`Scanner.Buffer` raises the limit.
It takes the starting buffer and the largest size the buffer may grow to, and it must be called before the first `Scan` (it panics otherwise).
The companion reader does this on its second line:

```go include=ch06/openrouter/sse.go lines=21..22
```

The buffer starts at 64 KiB and may grow to `maxBody`, the client's 16 MiB response cap, so the reader copes with an enormous line without allowing an unbounded one.
The test `TestStreamLongLine` feeds the reader a 100 KiB line to check it.

## What the stream looks like: server-sent events

[Server-sent events](/glossary/#server-sent-events) (SSE) is a plain-text format for a server to push messages to a client over one open HTTP response.
It is defined in the WHATWG HTML Standard, and its content type is `text/event-stream`.
A model API streams its reply in this format, and the example server later on this page writes it too.

The format is made of lines.
An **event** is a group of lines ended by a blank line.
Each line is one of these:

- A line starting with a colon (`:`) is a **comment**, and a reader ignores it.
  Servers send comments as a keep-alive, so that a connection with nothing to say does not look dead.
  The standard suggests one about every 15 seconds, since some proxies drop a connection that stays silent.
- Any other line is a **field**: a name, a colon, and a value, with one space after the colon removed if there is one.
  The field `data` holds the event's payload.
  If an event has several `data` lines, the reader joins them with newlines into a single payload.
- The field `event` names the event's type, so that a client can handle different kinds differently.
- The field `id` sets an ID for the event.
  When a browser's `EventSource` reconnects after a dropped connection, it sends the last ID it saw in a `Last-Event-ID` header.
- The field `retry` sets how many milliseconds the client waits before it reconnects.
- A field with any other name is ignored.
- A blank line ends the event, and that is when the client delivers it.

A made-up stream that uses each part:

```text
: keep-alive

event: greeting
id: 7
data: hello

data: first line
data: second line

```

The first event is a comment alone, which delivers nothing.
The second has type `greeting`, ID `7`, and the data `hello`.
The third has the data `first line` and `second line` joined by a newline.
The standard also says that if the stream ends in the middle of an event, before its blank line, the incomplete event is thrown away.

### What a model API sends

A model API's stream uses only some of this.
Each event is a single `data:` line holding a JSON chunk, followed by a blank line, and the last event is the text `data: [DONE]`:

```text
: OPENROUTER PROCESSING

data: {"choices":[{"delta":{"content":"Hel"}}]}

data: {"choices":[{"delta":{"content":"lo"},"finish_reason":"stop"}]}

data: [DONE]

```

Those chunks are shortened by hand; a real one carries more fields, as the recorded stream on [Streaming model replies](/build/streaming/#server-sent-events) shows.
The `[DONE]` marker comes from OpenAI-compatible APIs, which OpenRouter follows; **the SSE standard doesn't define it**.
A general SSE client treats it as ordinary data, so a Go reader for these APIs has to look for it.
Each JSON chunk holds a [delta](/glossary/#delta), the piece of the reply added since the previous chunk.

## Reading a stream in Go

Here is the companion reader, from the client that talks to OpenRouter:

```go include=ch06/openrouter/sse.go tag=sse highlight=10,12,17,24-27
```

Reading it from the top:

1. `sseData` takes an `io.Reader`, which in practice is the response body, and returns an `iter.Seq2[[]byte, error]`.
   That is an [iterator](/glossary/#iterator) of pairs: you range over it with `for data, err := range sseData(body)`, and each step gives you one payload and an error that is nil unless something went wrong.
   Because the function takes a plain `io.Reader`, a test can hand it a `strings.Reader` and no network is involved.
2. It builds the scanner and raises its line limit, as above.
3. The loop calls `sc.Scan()`, which waits for the next line, and takes the prefix `data:` off the line with `bytes.CutPrefix`.
   A line without that prefix (a comment, a blank line, or an `event:` or `id:` line) is skipped with `continue`.
4. It removes one space after the colon, as the standard says to, and checks for the `[DONE]` marker.
   On `[DONE]` it `return`s, which ends the iteration normally.
5. Otherwise it calls `yield(data, nil)`, which hands the payload to the loop body.
   If `yield` returns false, the caller has left its loop with `break` or `return`, so the function stops too, without reading any more.
6. When `Scan` returns false, the stream is over, and the function has to decide why.
   If `sc.Err()` is not nil, something failed: the connection broke, the context was cancelled, or a line was too long, and that error is yielded.
   If it is nil, the reader reached `io.EOF` **without ever seeing `[DONE]`**, so the function yields `io.ErrUnexpectedEOF`.

The last case is deliberate.
For the client, `io.EOF` alone does not mean the reply is complete, because a connection that drops in the middle of a reply can also end with `io.EOF`.
Only `[DONE]` says the server finished, so a stream that ends without it is an error.
The code that reads the chunks decides what to do next; [Streaming model replies](/build/streaming/#reading-it-with-an-iterator) shows it accepting a reply that already carried its finish reason and treating anything else as incomplete.

### What this reader leaves out

The reader follows the parts of the standard that these APIs use and simplifies the rest:

| Standard | This reader |
| --- | --- |
| A `data` field's lines are joined, and the event is delivered at the blank line | Each `data:` line is yielded at once as its own event, which is how OpenAI-compatible servers send them |
| A line `data` with no colon is a data field with an empty value | Not recognized, so the line is skipped |
| `event`, `id`, and `retry` fields have effects | Skipped like any other line |
| A line can end in `\n`, `\r\n`, or a lone `\r` | `bufio.ScanLines`, which accepts `\n` and `\r\n` |
| A leading byte order mark is removed | Not handled |
| An event cut off before its blank line is discarded | A `data:` line is yielded as soon as it is read |

None of these differences matters for the APIs the book uses.
They would matter for a reader of arbitrary SSE servers, which is what a browser's `EventSource` is, so use a library for that case.

## Stopping a stream

A stream can run for minutes, so a program needs ways to end one early, and a way to notice when one has stopped by itself.

**Cancel the context.**
The request was built with a [context](/go/context/), and cancelling that context ends the request.
The connection closes, and a `Read` that was waiting on the body returns an error.
The scanning loop ends, and `sc.Err()` reports the cancellation (`context canceled`).
The same chain makes a person's Ctrl-C stop a run, since the context that Ctrl-C cancels is the parent of every request.

**Close the body.**
Every response from `http.Client.Do` has a body that the caller must close, even when it reads nothing from it.
For a stream the rule matters more, since a stream is long-lived and the body holds the connection open.
The usual form is one line, right after the error check:

```go include=ch06/openrouter/stream.go tag=stream-request highlight=10
```

The `defer` runs however the function ends, whether the loop finished, the consumer broke out, or an error stopped it.
The `net/http` documentation says the default transport may not reuse a connection when the body is not read to completion and closed, so a skipped `Close` also costs the reuse that [the HTTP client page](/go/http-client/#keep-connections-reusable) describes.

**Notice a stream that goes quiet.**
A connection can stay open while the server sends nothing at all, and a `Read` on it can then wait with no end in sight.
A deadline on the whole request would also cut off healthy streams that are simply long, so the client uses an [idle timeout](/glossary/#idle-timeout): a limit on the gap between arrivals instead of the total time.
[Streaming model replies](/build/streaming/#stopping-a-stream-that-goes-quiet) covers the design and the numbers behind the 30-second default.
The mechanism is a small reader that wraps the body.

<div class="hood">

The reader that restarts the timer:

```go include=ch06/openrouter/sse.go tag=idle
```

`ir.r` is the real body.
Before every `Read`, the timer is reset to the timeout, and after the `Read` returns it is stopped.
The timer therefore runs only while the program is waiting on the network.
Time the program spends handling a chunk it already has does not count against the stream.
If the timer fires, a function started by `time.AfterFunc` cancels the stream's context with the cause `ErrStreamIdle`, which makes the blocked `Read` fail, and the caller can tell an idle stream from a Ctrl-C by that cause.
A comment line is bytes like any other, so the `: OPENROUTER PROCESSING` keep-alives restart the timer, which `TestStreamCommentsKeepItAlive` checks.

This works because `idleReader` is itself an `io.Reader`: it satisfies the same interface as the body it wraps, so the scanner needs no change.
Wrapping a reader in another reader is the usual way to add behavior to a stream in Go, and `io.LimitReader` and `io.TeeReader` work the same way.

</div>

## Writing a stream from a Go server

The other end of a stream is a server that sends one, as the book's agent does when it shows a run in a browser.
A handler writes its response in pieces, with one write per event.
The handler's `http.ResponseWriter` does not send each write at once, though.
It collects small writes in a buffer and sends them when the buffer fills or the handler returns, which is efficient for an ordinary page and wrong for a stream.
So a handler that writes one event and then waits a second, without flushing, sends nothing during that second: the client gets the event only when the handler finishes.

To send what has been written so far, the handler flushes.
The older way is the `http.Flusher` interface, which has one method, `Flush()`, and which a handler gets by checking the writer with a type assertion (`w.(http.Flusher)`).
The `net/http` documentation says that a handler should always test for it at runtime, because a wrapper around a `ResponseWriter` may not provide it.
The current way is `http.ResponseController`, added in Go 1.20:

```go
rc := http.NewResponseController(w)
err := rc.Flush()
```

Its `Flush` method does the same job and returns an error.
It finds the flush method through wrappers that expose an `Unwrap` method, and when the writer cannot flush at all it returns an error you can check, where a `Flusher` assertion leaves you to handle the missing method yourself.
(A `ResponseWriter` with no flush support gives the error `feature not supported`.)
The controller also has `SetWriteDeadline`, which limits how long one write may take, so that a client that has stopped reading cannot hold the handler forever.
The documentation notes that a write after the deadline "will not block, but may succeed if the data has been buffered", so the deadline is paired with a flush: the flush is what forces the write onto the network, where the deadline can apply.

The companion's example server sends each event with one small function:

```go include=ssebrowser/server.go tag=send
```

It sets a deadline, writes the event text, and flushes.
Calling it once per event is what makes the stream appear to a client as the agent runs.

A handler also sets the response headers, before the first write, since the headers go out with the first flush.
The example's handler sets three:

```go include=ssebrowser/server.go lines=59..62
```

`Content-Type: text/event-stream` tells the client what it is receiving, and a browser's `EventSource` fails the connection when the response has any other type.
`Cache-Control: no-cache` asks caches not to reuse the response without checking with the server first.
`X-Accel-Buffering: no` is for nginx, which by default buffers a proxied response and would hold the events back; the header asks it to pass them straight through.

The server code is a larger topic than this page: it also starts the agent in its own goroutine, sends heartbeat comments, and stops the run when the client leaves.
[Streaming an agent's progress to a browser](/run/stream-to-browser/) walks through all of it.

## Mistakes to avoid

- **Forgetting to flush.** Without `Flush`, the events wait in the server's buffer and a client sees them in a burst at the end, or when the buffer fills, which makes the stream pointless.
- **Holding `sc.Bytes()` past the next `Scan`.** The slice can be overwritten.
  Copy the bytes, or use `sc.Text()`.
- **A line longer than the scanner's buffer.** The default 64 KiB limit ends the scan with `bufio.ErrTooLong`, and tests with short lines never find it.
  Call `Buffer` before the first `Scan`.
- **Not closing the body.** The connection is held open and cannot be reused.
  Put `defer resp.Body.Close()` right after the error check.
- **Treating `io.EOF` as a failure.** It is how every `Read` loop normally ends.
  `bufio.Scanner` already hides it, so after the loop, `Err() == nil` means the input ended normally.
- **Reading a stream with `io.ReadAll`.** It returns only when the stream ends, so the program gets nothing until the end, which gives up the whole reason to stream.
  On a stream that never ends it also never returns.
- **Treating a missing `[DONE]` as success.** A stream that ends without the marker may be a reply cut off by a dropped connection.
  Report it as an error, as the companion reader does.
- **Forgetting that the browser reconnects.** When an `EventSource` stream ends, the browser reconnects on its own, and for a stream that starts work, a reconnect starts the work again.
  The example page closes the `EventSource` itself when it sees the end.
- **A proxy that buffers the response.** `http.Flusher`'s documentation warns that behind a proxy, buffered data may not reach the client until the response completes.
  Check the proxy's settings, and send a header like `X-Accel-Buffering: no` where one exists.

## Try it

Everything here runs offline, with no API key.
From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

1. Run two tests of the reader.
   The first sends a recorded stream in pieces of 1, 7, and 4,096 bytes, so that the reader proves it does not depend on how `Read` splits the bytes.
   The second sends a 100 KiB line.

   ```console
   $ go test ./ch06/openrouter/ -run 'TestStreamSplitAnywhere|TestStreamLongLine' -v
   === RUN   TestStreamSplitAnywhere
   --- PASS: TestStreamSplitAnywhere (0.08s)
   === RUN   TestStreamLongLine
   --- PASS: TestStreamLongLine (0.01s)
   PASS
   ok  	github.com/blackwell-systems/go-agents-book/code/ch06/openrouter	0.929s
   ```

2. Start the example server, which serves a scripted run of the agent.

   ```console
   $ go run ./ssebrowser
   open http://127.0.0.1:8080
   ```

3. In a second terminal, read its stream with curl.
   The `-N` flag turns off curl's own output buffering, which would otherwise hold lines back and hide the streaming.
   Add `-i` to see the response headers too.

   ```console
   $ curl -s -N -i 'http://localhost:8080/events?task=fix'
   HTTP/1.1 200 OK
   Cache-Control: no-cache
   Content-Type: text/event-stream
   X-Accel-Buffering: no
   Date: Fri, 23 Oct 2026 08:54:40 GMT
   Transfer-Encoding: chunked

   : run started

   event: text
   data: {"text":"I'll "}

   event: text
   data: {"text":"run "}
   ```

   The lines keep arriving for about ten seconds, a few words at a time.
   In one run, the stream held 77 `text` events, 6 `call` events, 6 `result` events, 3 `turn` events, and a final `stopped` event.
   Your `Date` line will differ.
   Compare the output with the format section above: the first line is a comment, and each event is an `event:` line, a `data:` line, and a blank line.

4. Stop the server with Ctrl-C in its terminal.
   It shuts down and ends any stream still open.

If port 8080 is taken, add `-addr localhost:8099` to the `go run` command and use that port in the curl URL.

## Where next

- [Streaming model replies](/build/streaming/) builds on this page: tool calls that arrive in fragments, errors in the middle of a stream, and the numbers behind the idle timeout.
- [Streaming an agent's progress to a browser](/run/stream-to-browser/) is the full server: heartbeats, a deadline on every write, and a shutdown that ends open streams.
- [Iterators in an agent](/go/iterators/) explains the `iter.Seq2` shape that `sseData` returns.
- [Context](/go/context/) explains the cancellation that stops a stream.
