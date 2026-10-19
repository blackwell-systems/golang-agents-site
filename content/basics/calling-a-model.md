---
title: Calling a model's API from Go with net/http
short: Calling a model from Go
description: Under two hundred lines of standard-library Go make a client for an OpenAI-compatible chat completions API, with a per-call deadline that records its cause, errors checked in every place they can appear, and the finish reason passed back to the caller.
card: A chat completions client in standard-library Go, with deadlines, error checks, and finish reasons.
chapters: 2
stage: 2
date: 2026-10-18
weight: 30
art: ch02
---

To a Go program, a model is a web service: you POST some JSON and decode the JSON that comes back.
`net/http` and `encoding/json/v2` cover all of it.
The big providers do publish official Go SDKs, which bring typed options, built-in retries, and streaming helpers.
The catch for an agent is that an SDK makes some choices on your behalf that an agent should make on purpose, such as the timeout for a call, whether a failure gets retried, and what happens to a reply that ended before it was done.
Building the client yourself once puts each of those choices in front of you; a later project can switch to an SDK where one is useful.

The client shown here comes from the [companion code](code:ch02/openrouter/client.go), and we'll follow one call from request to decoded reply.

## One endpoint

Every request the client sends goes here:

```text
POST https://openrouter.ai/api/v1/chat/completions
```

[OpenRouter](/glossary/#openrouter) is a single front door to many model providers: one API, one key, and one invoice, with the model named in the request and OpenRouter passing it to a provider that hosts it.
The request format is OpenAI's chat completions format, which has become the common one: most other hosted services and local model servers accept it as well.
That makes the base URL just another setting.
Change it to a local server's address, for example Ollama's `http://localhost:11434/v1`, and the client works against that server unchanged.

The request body names the model and carries the conversation.
The response carries the model's message plus metadata: the reason it stopped writing and what the call used.

## Wire types, kept apart

The JSON the API speaks lives in unexported types inside the client package, not in the agent's own `Message`:

```go include=ch02/openrouter/wire.go tag=wire-request
```

Separate types let each side evolve on its own schedule.
The API has its own field names and extra fields an agent never reads, and the agent's types are shaped around the agent.
With the translation in one file, the rest of the program never learns the API's conventions, and swapping in an API with another shape later means writing another translation file.

Two details here come from `encoding/json/v2`, which Go 1.27 ships in the standard library.
Each field carries a `json` tag, since v2 compares names case-sensitively.
And `MaxTokens` is marked `omitzero` rather than `omitempty`.
Under v2, `omitempty` drops a field only when its encoded JSON is empty, like `""` or `[]`, and the number `0` doesn't count as empty.
With `omitempty`, every request would include `"max_tokens": 0`, asking the model for a reply no longer than zero tokens.

## The call

The client's zero values are useful: leave the base URL empty and it uses OpenRouter, leave the `*http.Client` nil and it uses `http.DefaultClient`, leave the timeout at zero and it allows two minutes.
`Generate` is the single method that makes the client a `Model` the agent loop can use:

```go include=ch02/openrouter/client.go tag=generate
```

Everything else is standard `net/http`.
`NewRequestWithContext` binds the request to `ctx`, so a cancelled run abandons the call, including while the body is still arriving.
The API key travels as a bearer token in `Authorization`.
`defer resp.Body.Close()` gives the connection back no matter how the function returns.

## A deadline with a reason

The default `http.Client` will wait forever, and a healthy model call can take over a minute, so the client needs some limit.
`http.Client.Timeout` is the obvious place to put one, but it applies to every request through that client, and clients are usually shared by much of a program.
It also gives no way to say which limit expired.

The client instead derives a deadline from `ctx` for each call.
That deadline belongs to one call only, it stacks with any deadline the caller already set (the earlier one fires), and it can carry a reason.
`context.WithTimeoutCause` attaches `ErrCallTimeout` to the cancellation, and `context.Cause` retrieves it.
A caller can then test the error two ways: `errors.Is(err, context.DeadlineExceeded)` says a deadline fired, and `ErrCallTimeout` says it was this client's.
`net/http` doesn't consistently preserve both, and which one survives depends on whether the connection is HTTP/1.1 or HTTP/2.
So the client wraps in whatever is missing using `%w: %w`, which wraps two errors in one, and its tests verify the result over both protocol versions.

The two-minute figure is measured.
Across the recorded runs in the companion code, the longest single call lasted 76 seconds, `z-ai/glm-5.3-flash` producing 2,880 reasoning tokens, and in the bug-fix runs 90% of calls returned in under three seconds.
Two minutes covers the slow reasoning case and still catches a call that is stuck.
The two official Go SDKs pick very different defaults: `openai-go` has no timeout unless you set one, and `anthropic-sdk-go` allows a non-streaming call ten minutes.

## Reading the reply

There are three distinct ways a call can fail, and the client looks for each one:

- **A status other than 200.** The API turned the request down, for reasons like an invalid key (401), an empty account (402), a request bigger than the model accepts, or a provider outage. The returned error includes the status and the message the API sent.
- **An error object inside a 200.** OpenRouter took the request, but the provider broke down before it produced a reply.
- **A finish reason of `error`.** The provider started replying and then broke down.

The client reads the body through `io.LimitReader` with a 16 MB ceiling, so a misbehaving server can't keep it reading without end.

Once the reply checks out, the client copies the parts the agent uses: the message, the [usage](/glossary/#usage) (input and output tokens, cached and reasoning tokens, and the dollar cost OpenRouter includes in every reply), and the [finish reason](/glossary/#finish-reason):

| Value | Meaning |
| --- | --- |
| `stop` | The model finished its reply |
| `length` | It reached the output limit; the text is cut off |
| `tool_calls` | It is asking for tools |
| `content_filter` | A safety filter blocked or cut the reply |
| `error` | The provider failed partway through |

A `length` reply is returned normally, with the reason in a field.
The text is truncated, but it is still text, and only the caller knows whether to keep it, ask again with a larger limit, or stop.
The agent loop built on top should be stricter than the client and refuse to treat a truncated reply as a finished answer.

## What it deliberately leaves out

Each call gets exactly one attempt.
Failures are routine with model APIs: 429 rate limits, 502 and 503 from overloaded providers, connections that drop halfway through a reply.
The official Go SDKs handle this by retrying twice by default.
That's handy, but the caller can't see it happen: one call just looks slow, and any budget that counts calls misses the extra attempts.
Put retries in the agent harness instead, so every attempt is visible in the logs, counts against the budget, and stops when the run is cancelled.

## Run it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go run ./ch02
```

If `OPENROUTER_API_KEY` is unset, the program plays back a three-turn chat recorded from the real API and prints the cost of each call; an HTTP transport serves the recorded responses from disk, and to the client they look exactly like the network.
If the key is set, you get a live chat with the default model.
For a local server, set `OPENROUTER_BASE_URL` and `OPENROUTER_MODEL`, and give `OPENROUTER_API_KEY` any non-empty value, because an empty key means replay.
