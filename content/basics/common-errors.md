---
title: "Common first errors when calling an LLM API from Go"
short: Common first errors
description: The errors a first agent usually runs into, what each one looks like, and the fix. A missing key, no credit, a bad model ID, rate limits and server errors, a reply cut off partway, a reasoning model that spent its whole reply thinking, and two Go traps.
card: A missing key, no credit, a bad model ID, rate limits, cut-off replies, and two Go traps.
group: Talk to a model
chapters: 2, 5, 12
stage: 2
date: 2026-10-19
weight: 35
art: common-errors
---

Most problems with a first agent are problems with a single call.
They come in two kinds: the API refuses the request and says so with an HTTP status, or it accepts the request and the reply isn't what your code expects.
Both are easy to handle once you've seen them.

## When the API refuses

A refused request comes back with a status other than 200 and a small JSON body that says what went wrong.
This one is a real response, recorded with a key that didn't belong to any account:

```json include=ch02/testdata/replies/unauthorized-01.json lines=12..17 title="A 401, as the API sent it"
```

Read the status first; it tells you whether trying again can help.

| Status | Usually means | Fix | Retry? |
| --- | --- | --- | --- |
| 401 | The key is missing, mistyped, or revoked | Check `echo $OPENROUTER_API_KEY` in the same shell that runs the program; an `export` in another terminal doesn't carry over | No |
| 402 | No credit left, or the key reached its own credit limit | Add credit or raise the key's limit | No, with one exception below |
| 400 | The request itself is wrong: a misspelled model ID, a malformed message, or more text than the [context window](/glossary/#context-window), the most a model can read in one request, can hold | Fix the request; for a model ID, copy it from the model's page, in `company/model` form | No |
| 403 | A moderation check blocked the input | Change the input | No |
| 408, 429 | A timeout, or too many requests too fast | Wait, then try again, waiting longer each time | Yes |
| 500, 502, 503 | A failure on the API's or a model host's side | Wait briefly and try again | Yes |

OpenRouter also sends a 402 for a temporary limit on how much a key can have in flight at once, with [a header saying how long to wait](/go/http-client/); that one is worth retrying.
[Retries and observability](/production/retries-and-observability/) covers waiting well, and [why the agent stopped](/production/stop-reasons/) turns these statuses into Go errors your code can check.

A model that can't use [tools](/basics/tool-calling/) is a separate case.
If you send tools to a model that doesn't support them, the request fails; check that the model's page lists tool support before you choose it, as [choosing a model](/basics/choosing-a-model/) describes.

## When the reply is cut off

A request can succeed and still bring back half an answer.
This reply came from a request that allowed the model only 20 tokens:

```json include=ch02/testdata/length-response.json lines=9..22 title="finish_reason: length"
```

The text simply stops, mid-sentence.
Nothing in the content says so; only `finish_reason` does.
An agent that treats this as an answer will act on half of one, so check `finish_reason` on every reply: `stop` is a finished answer, `length` means the reply was cut off and needs more room, and `error` means the provider failed partway.
Raise the reply limit (`max_tokens`), or ask for something shorter.
To know whether the model asked for tools, look at whether the reply holds tool calls rather than at `finish_reason`, since some providers report `stop` even then ([Tool calling on the wire](/basics/tool-calling/) shows why).

## When a reasoning model spends the reply thinking

Some models think before they answer, writing reasoning that counts against the same reply limit.
Given only 12 tokens, this model spent all of them reasoning and returned no answer at all:

```json include=ch02/testdata/replies/length-reasoning-01.json lines=25..31 title="All 12 tokens went to reasoning"
```

`content` is `null`, and `finish_reason` is `length` again.
With a reasoning model, leave a generous reply limit, and treat an empty answer with `length` the same way as a cut-off one.
[Choosing a model](/basics/choosing-a-model/) has more on reasoning models and what they cost.

## When the program hangs

A call to a model can take many seconds, and once in a while it doesn't come back at all.
Give every call a deadline with [`context.WithTimeout`](/go/context/), so a stuck request fails with an error instead of holding the whole agent:

```go
ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
defer cancel()
reply, err := model.Generate(ctx, req)
```

`defer cancel()` releases the timer when the call returns, whether it succeeded or not.
[Calling a model from Go](/basics/calling-a-model/) builds a client that does this and records why a call was cancelled.

## Two Go traps

**A field that never fills in.**
[`encoding/json/v2`](/go/json-v2/) matches JSON names to struct fields exactly, case included.
A field declared as `FinishReason string` without a tag never matches `"finish_reason"`, and it stays empty with no error, so the check above never fires.
Give every field you decode from an API an explicit tag, as the companion code's client does:

```go include=ch02/openrouter/wire.go lines=39..43 title="Every field tagged with its JSON name"
```

**A Go that's too old.**
The companion code needs Go 1.27.
Go 1.21 and later download it automatically when they read the module's `go.mod`; an older `go` stops with an error naming the version the module requires.
[Getting set up](/basics/getting-set-up/) covers installing it.
