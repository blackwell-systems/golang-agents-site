---
title: "An HTTP client for LLM APIs in Go: timeouts, limits, errors, and Retry-After"
short: An HTTP client for model APIs
description: The net/http details that a model API client in Go gets right or wrong. One shared client, why the deadline belongs on the context and not on http.Client.Timeout, reading replies with a size limit, errors inside a 200, API errors with Retry-After, connection reuse, and RoundTrippers for tests and fault injection.
card: One shared client, deadlines on the context, size limits, errors inside a 200, and honoring Retry-After.
group: Calling APIs
chapters: 2, 5, 6, 9, 12
stage: 2
level: intermediate
date: 2026-10-19
weight: 40
art: go-http
---

A model API is an ordinary HTTPS endpoint with unusual traffic.
A reply can take half a minute to start, and a streamed one can take minutes to finish; the body can be large; rate limits are part of normal operation; and a request can fail after it has succeeded, with an error inside a `200 OK`.
Go's `net/http` handles all of it, with a few choices made deliberately.
This page goes through them, with code from the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), where it is tested.
For the chat completions request itself, start with [calling a model from Go](/basics/calling-a-model/); for when and how to retry, see [retries and observability](/production/retries-and-observability/).

## One client, shared

An `http.Client` and the `http.Transport` under it are safe for concurrent use (many goroutines can call them at the same time), and the transport keeps a pool of open connections.
Make one and share it; a new client or transport per request throws the pool away and pays for a new TCP and TLS handshake every time (the round trips that open a connection and set up encryption before any request is sent).

The model client takes an `*http.Client` as a field, and uses `http.DefaultClient` when it is nil:

```go include=ch12/openrouter/client.go tag=client
```

Taking the client as a field, instead of building one inside, lets the caller decide what it is: a client with a proxy, one shared by every agent in a process, or one whose transport replays recorded replies in tests.

The default transport is a sensible start: a 30-second dial timeout, a 10-second TLS handshake timeout, HTTP/2 where the server offers it, and idle connections kept for 90 seconds.
One default matters when an agent runs many calls to one provider in parallel: the transport may keep too few idle connections.

<div class="hood">

Over HTTP/1.1, the default transport keeps at most two idle connections per host (`DefaultMaxIdleConnsPerHost`).
A program running many calls to one provider in parallel reopens connections constantly unless it uses a transport with a higher `MaxIdleConnsPerHost`.
Over HTTP/2, one connection carries many requests at once, and the limit matters less.

</div>

## Deadlines go on the context

`http.Client` has a `Timeout` field, and it is the wrong tool here.
It covers the whole exchange, including reading the body, so it can't tell a slow start from a long reply: a timeout long enough for a streamed reply that legitimately takes three minutes is far too long to notice a request that never got an answer.

The client instead puts a deadline on each call's context, two minutes by default, and a [stream](/build/streaming/) gets an idle timer that measures silence instead of total time.
Both set a context cause (`context.WithTimeoutCause`), so the error from a timeout says which deadline ended the call.
The [context page](/go/context/#a-deadline-for-each-model-call) shows both.

## Read with a limit

A response body is read with a cap, so a misbehaving server or proxy can't make the client allocate without bound:

```go include=ch12/openrouter/client.go tag=read-reply
```

The limit is 16 MB, far above any reply, and the client reads one byte past it so it can tell a body that is exactly the limit from one that is over.

The checks after reading are in order, and each catches something real:

1. A status other than 200 becomes an API error, built below.
2. A 200 can still carry an `error` object instead of a reply: the request was accepted, and the provider failed.
3. A reply with no choices is unusable.
4. A choice can carry its own error, when a provider failed partway through writing it; the tokens it did write are still billed, so the reply is returned alongside the error.

A client that only checks the status code treats cases 2 and 4 as answers.

## When the status isn't 200

A refused request becomes an `*APIError` with the status, the API's own message, and the wait the server asked for:

```go include=ch12/openrouter/errors.go tag=status-error
```

The body isn't always the API's JSON.
A proxy or load balancer in between can answer with an HTML error page, so the client keeps the first 200 bytes of whatever came back rather than failing to decode it.

The wait comes from the `Retry-After` header, which has two forms, a number of seconds or an HTTP date:

```go include=ch12/agent/retryafter.go tag=retry-after
```

Both forms are clamped: a value in the past or one that doesn't parse is no wait, and anything over a day is capped at a day.
A server's header is input like any other, and a huge value multiplied into a `time.Duration` would overflow.

## Honoring the wait, within limits

The retry policy uses the server's wait when there is one, and gives up instead when the wait is longer than the caller is willing to spend.

<div class="hood">

The function that decides how long to wait before a retry:

```go include=ch12/agent/retry.go tag=wait
```

</div>

A server's wait replaces the backoff for that retry, plus a random extra of up to a second, so that many clients told to wait the same time don't all come back in the same instant.
A wait longer than `MaxWait`, one minute by default, ends the retries with an error that names the wait, which is more useful than sleeping through it.
Which errors are worth retrying is decided by `Classify`, described on the [errors page](/go/errors/#errors-that-carry-data-types-and-behavior); the backoff itself is on the [retries page](/production/retries-and-observability/#how-long-to-wait).

## Keep connections reusable

A connection goes back to the pool only when its response body has been read to the end and closed.
`defer resp.Body.Close()` after every successful request is the rule, and reading the whole body, as `readResponse` does, is what lets the connection be reused.
Code that gives up on a response early, and expects more requests to the same host, reads what is left with `io.Copy(io.Discard, resp.Body)` before closing it.

## RoundTrippers: where tests and faults plug in

`http.Client` sends every request through its transport, an `http.RoundTripper`: an interface with one method, `RoundTrip(*http.Request) (*http.Response, error)`.
Wrapping or replacing it changes how requests are sent without touching the client code.

The companion code uses that in two ways.
Every program runs offline by default with a transport that answers from recorded replies, which [testing offline](/production/testing-offline/) describes.
And the retry tests use a transport that injects failures, a status, a dropped connection, a body cut short, in front of a real server.

<div class="hood">

The transport that injects them:

```go include=ch12/agent/fault.go tag=fault-trip
```

</div>

When it fakes a lost connection after a real response, it drains and closes that response's body first, so the real connection isn't leaked.
A `RoundTripper` owns the request body it is given, too: one that answers without sending a request closes the body itself.

## Mistakes to avoid

- **A new `http.Client` or `http.Transport` per request.** Each one starts a new connection pool; share one.
- **`http.Client.Timeout` on streamed replies.** It counts the whole stream; use a deadline per call on the context, and an idle timer for streams.
- **`io.ReadAll(resp.Body)` with no limit.** Cap it with `io.LimitReader`.
- **Checking only the status code.** A model API can report a failure inside a `200`.
- **An unclosed body.** The connection is never reused, and a long-running agent runs out of file descriptors.
- **Trusting `Retry-After` without a cap.** Clamp it, and decide how long you'll wait before you sleep.
- **The API key anywhere but a header.** A failed request's error includes its URL, so a key in the query string ends up in error messages and logs; and never log request headers.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch12/agent -run 'TestDoHonorsRetryAfter|TestFaulty' -v
$ go test ./ch06/openrouter -run 'TestErrorInsideA200|TestAPIErrors' -v
```

1. In `TestTimeoutSynctest` (`ch06/openrouter/client_test.go`), give the `http.Client` a `Timeout` of one second and see which deadline ends the call, and what the error says.
2. `TestAPIErrorEdges` in `ch06/openrouter/client_test.go` already points the client at an `httptest` server that returns a `502` with an HTML body; run it with `go test ./ch06/openrouter -run TestAPIErrorEdges -v`, then change the body or the status in the test and read the `*APIError` it produces.
3. Give `RetryAfter` a date in the past, an empty header, and `999999999`, and check each result.
