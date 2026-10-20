---
title: Streaming model replies in Go
short: Streaming replies
description: A streamed chat completion arrives as server-sent events. This page reads one with a bufio.Scanner and an iterator, reassembles tool calls from their fragments, and stops a stream that stops sending.
card: Reading server-sent events with bufio and an iterator, and rebuilding tool calls from fragments.
group: Tools and output
chapters: 6
stage: 2
date: 2026-10-18
weight: 30
art: ch06
---

An agent that waits for whole replies gives its user long stretches of nothing.
The terminal sits blank for a few seconds, prints an entire turn, and goes blank again.
From the outside a slow model, a hung connection, and a crashed program all look the same.
With streaming, text appears while the model is producing it, and the [agent loop](/basics/agent-loop/) gets a much earlier signal when a connection has died.

Here is how a stream looks on the wire, how to consume one in Go, and the details that catch out a stream reader written too quickly.

## What streaming buys, measured

In one recorded experiment, each of four models answered one question ten times, half streamed and half not.
With the default model, `anthropic/claude-haiku-5.5`, streamed replies started showing text at a median of 1.6 seconds; the same reply without streaming appeared all at once after 2.7.
Total time barely moved: for three of the four models, streamed and unstreamed replies finished within half a second of each other.

What streaming improves, then, is what the user sees and how fast the program notices a failure; it doesn't make the model finish any earlier.
Reasoning models complicate the picture.
`z-ai/glm-5.3-flash` delivered its first chunk in under a second but its first word of the actual answer only after a median of 12.2 seconds, since everything before that was reasoning.
If your display shows only the answer, the user stares at an empty screen for twelve seconds while data is flowing the entire time.

## Server-sent events

Models produce output a token at a time, which lets a provider forward each piece the moment it exists.
You opt in with `"stream": true` in the request, and add `"stream_options": {"include_usage": true}` if you still want token counts at the end.

The reply is an HTTP 200 whose body keeps arriving, formatted as [server-sent events](/glossary/#server-sent-events).
The format is line-based.
A line beginning `data:` holds the payload, an empty line separates one event from the next, and a line beginning with a colon is a comment to be ignored.
Every `data:` line here is one JSON chunk of the reply, and the final one is the literal `data: [DONE]`.
OpenRouter sends comments like `: OPENROUTER PROCESSING` while a slow model warms up, so the connection never looks idle.

The opening of a recorded stream, one chunk per line:

```text include=ch06/testdata/out/stream.txt lines=1..11 title="Recorded from claude-haiku-5.5: the reasoning record, then text in pieces"
```

The pieces don't respect word boundaries; one chunk ends with "mapped to `time." and the next starts "Second`".
Where a complete response has a `message`, each chunk has a `delta` instead: only what was added to the message since the previous chunk.

## Reading it with an iterator

A `bufio.Scanner` plus a handful of rules is enough to parse the format.
The `sseData` helper hands back the payload of each `data:` line through a [range-over-func iterator](/basics/iterators/), an `iter.Seq2` of bytes and an error, so the code that decodes chunks is an ordinary `for` loop:

```go include=ch06/openrouter/sse.go tag=sse highlight=10,24-27
```

Two lines in this function each head off a real failure.

First, the scanner's buffer is allowed to grow to 16 MB.
Left at its default, a `bufio.Scanner` refuses any line longer than 64 KB.
Chunks can be long: when a model puts an entire file into a tool call's arguments, that can come through as a single line.
With the default buffer, the reader passes every test and every ordinary run until the first long tool call breaks it.
The companion code has a test that feeds it a 100 KB line.

Second, if the body ends before `[DONE]` shows up, the iterator yields `io.ErrUnexpectedEOF`, and the client decides what that means for the reply.

## Three things that don't fit the pattern

Most of a stream is just more text in each chunk.
Three cases break that pattern:

- **Usage comes last.** One chunk carries the [finish reason](/glossary/#finish-reason), the field that says why the model stopped; the usage comes in a chunk after it. Stop reading at the finish reason and you never find out what the call cost, so the client keeps going until `[DONE]`.
- **Errors can arrive mid-stream.** A provider that fails after it has started writing sends an error object as an ordinary chunk. The client treats it as a failure, never as a normal end to the reply.
- **The connection can just end.** If the body stops before any finish reason or `[DONE]`, the reply is incomplete and the client reports an error. If the finish reason already arrived, the reply itself is complete and accepted; the only thing missing is its usage.

## Tool calls arrive in fragments

A tool call is spread over several chunks.
The first one for a call carries its `index` within the reply, its `id`, and the tool name, with the arguments still empty.
After that, each chunk for the call holds just the `index` and another slice of the argument text.
The reader therefore gathers calls by index:

```go include=ch06/agent/stream.go tag=add-call
```

Indexes are checked before use: anything negative or 128 and above means the stream is broken, and the assembler rejects it rather than panicking on a bad index or growing a slice without bound.
It also rejects a fragment whose ID disagrees with the one already recorded at that index.
The argument fragments are appended as raw bytes, and nothing tries to read them yet.

That last choice is the one to copy.
Until the stream is over, the arguments are an unfinished string such as `{"path": "duration.g`, which isn't JSON, and a harness that guessed how it ends could run a tool on a path nobody asked for.
Decoding waits until the whole reply has arrived and passed its checks.
A reply that the token limit truncated ends the run with an error; it never turns into a tool call with half its arguments.

## Stopping a stream that goes quiet

A fixed deadline works for a whole-reply call, but a healthy stream can run longer than any deadline you would choose.
What a stream needs is a cap on how long it may go without sending anything, an [idle timeout](/glossary/#idle-timeout).
A `time.AfterFunc` timer restarts every time bytes come in (comment lines count), and if it ever fires it cancels the context of that stream, recording `ErrStreamIdle` as the reason.
It defaults to 30 seconds, which is deliberately loose.
Across the twenty timed streams, no gap between pieces after a reply began was longer than 0.64 seconds, but one run spent 10.9 seconds before sending its first piece.
A shorter timeout would trip on providers that hold a request in a queue before starting, and would discard replies that were on their way.

The reason travels with the [context](/basics/context/), so the resulting error matches `context.Canceled` and also `ErrStreamIdle`.
That lets a caller distinguish a stream that went idle, which is worth retrying, from a person pressing Ctrl-C.
Cancellation can hit at any moment without corrupting the conversation: a reply still in flight is thrown away, so the history never contains a partial reply or a tool call missing its result.

```diagram
name: ctx-tree
alt: A tree of contexts: main's, from signal.NotifyContext with the cause interrupt signal received, at the top; below it the run's, from context.WithCancel in Events; below that, for each reply, the stream's, from context.WithCancelCause, which the idle timer cancels with ErrStreamIdle, and below it the HTTP request; and for each tool call, run_tests's, from context.WithTimeoutCause with ErrTestsTimedOut, and below it the go test command started with exec.CommandContext.

The contexts in a streamed run: ending one ends everything below it, and each records its own reason.
```

## The run as an iterator

The model's stream is a detail of the client.
A program running an agent cares about a different set of happenings: text from the model, a finished tool call, a tool result, the end of a turn, the end of a run.
Those happenings are reported as events, and the agent loop itself becomes `Events`, a function that returns an `iter.Seq` of them.
`Run` and the new `RunStream` both consume `Events`, so there is still exactly one loop.

Making the run an iterator, rather than a goroutine sending events down a channel, keeps ownership simple.
A channel needs a goroutine to feed it, and then the code has to close the channel, make that goroutine exit if the reader walks away, and carry errors across.
`Events` runs in the caller's goroutine, puts the error in the final event, and stops completely when the caller breaks out of its `for` loop.
`TestBreakOutOfEvents` in `ch06/agent/stream_test.go` checks exactly that: it breaks out at the first text event, then asserts that the model's stream returned and that the context the model was given has been cancelled.

## One interface or two?

Streaming adds a second interface next to `Model`:

```go include=ch06/agent/stream.go tag=stream-model
```

A single interface plus a type assertion for streaming support would be shorter, but it fails without telling you.
Wrap the client in a logger that only implements `Model`, and the `Stream` method disappears behind it; the loop switches back to whole replies and every test still passes.
Two interfaces put the choice in the type system.
An adapter wraps any plain `Model` as a `StreamModel`, which is how all the earlier tests keep running, unchanged, through the streaming loop.

## Run it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go run ./ch06 -replay fix-again
$ go run ./ch06 -replay stream-error
```

Replays hand over the recorded stream in one go; set `OPENROUTER_API_KEY` and you see each line as the model produces it.
The second command replays a provider that fails mid-reply: the partial text is printed, is kept out of the conversation, and the error explains what happened.
