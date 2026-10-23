---
title: How chat completions and the Messages API differ
short: How two provider APIs differ
description: The same agent request, sent as OpenAI-style chat completions through OpenRouter and as Anthropic's Messages API, differs in twelve places, from where the system prompt goes to how a stream ends. A table of each difference, with the chapter and the code that handles it.
card: The same request on OpenAI-style and Anthropic APIs differs in twelve places. Here is each one.
group: Providers
chapters: 2, 4, 6, 8, 11
stage: 2
level: intermediate
date: 2026-10-18
weight: 15
art: ch11
---

The agent talks to models through one interface, `Model`, with one method.
Behind it sit two clients: one for the chat completions format, which OpenRouter, OpenAI, and many local servers accept, and one for Anthropic's Messages API.
The second is an adapter, and it shows what an adapter for any API has to decide.
This page collects the answers for the two APIs covered here into one table, so you can see the size of the job before you start a third.

Facts below are checked against the companion code, as of October 2026.
Provider APIs change; each row says where to check in the companion code when one does.

## The table

| What differs | Chat completions (OpenRouter) | Messages API (Anthropic) | Where to check |
| --- | --- | --- | --- |
| System prompt | A message with role `system` | A top-level `system` field | `ch11/anthropic/wire.go` |
| Reply length limit | Optional; the provider's default if unset | `max_tokens` is required; the client sends 16,000 when the agent sets none, since thinking counts against it | the package `ch11/anthropic` |
| Roles | `system`, `user`, `assistant`, and `tool` | `user` and `assistant` only, alternating; tool results and harness notes after them are joined into one user turn | the package `ch11/anthropic` |
| Tool-call arguments | A string that holds JSON | A JSON object; arguments that aren't valid JSON go as an empty object, and the tool's error explains | the package `ch11/anthropic` |
| Tool results | A message with role `tool` and the call's ID; no failure flag, so the client writes "Error:" into the text | A `tool_result` block in a user turn, with an `is_error` flag | the package `ch11/anthropic` |
| Reasoning sent back | OpenRouter's `reasoning_details`, returned as received | Thinking blocks returned exactly as received, in order, only to the provider and host that wrote them | `ReasoningFrom` |
| Why the reply stopped | `finish_reason`: `stop`, `tool_calls`, `length`, `content_filter`, `error` | `stop_reason`: `end_turn`, `tool_use`, `max_tokens`, `refusal`, and others, mapped onto the left column | `ch11/anthropic/wire.go` |
| Prompt token count | `prompt_tokens` includes tokens read from the cache | `input_tokens` leaves out cache reads and writes, so the prompt is the sum of three counts | the package `ch11/anthropic` |
| Cost | OpenRouter reports what it billed in each reply | Not reported by Anthropic's API; the client prices each reply from a table kept in configuration (OpenRouter's Messages endpoint does report it) | `prices.json` |
| Prompt caching | Automatic on some models; Anthropic's models need a `cache_control` marker, which OpenRouter passes on | `cache_control` markers, up to four per request | the package `ch11/anthropic` |
| Errors | An HTTP status, or an error object inside a 200 reply | An HTTP status, including 529 when overloaded; an error body in a 200 or an error event mid-stream becomes the same error type | the package `ch11/anthropic` |
| Streaming | `data:` lines of chunks; the usage arrives after the finish reason; ends with `data: [DONE]` | Named [events](/build/streaming/) per content block; the stop reason and final usage arrive together in `message_delta`; ends with `message_stop` | the package `ch11/anthropic` |

A few rows need more than a cell.

**Refusals.**
A Claude model that declines a request stops with `refusal`, in a normal reply that can hold some text.
The conversation can't usefully continue from there, so the loop counts it among the replies it can't continue from and stops with `ErrRefused`, an error the companion code added for replies the loop can't continue from.
Here is the whole mapping, from the [companion code](code:ch11/anthropic/wire.go):

```go include=ch11/anthropic/wire.go tag=finish-reason
```

**Reasoning and hosts.**
OpenRouter routes one Claude model to several hosts, each under its own account.
In one recorded experiment, a thinking block sent back to the host that wrote it was accepted in both runs.
Sent to any of three other hosts, it was refused with a 400.
That is why the adapter records where reasoning came from, and why the agent pins one host (the client asks OpenRouter to use only the host you name).

**Stream size.**
Over the 13 replies of each recorded run of the same task, the Messages API's stream had a median of 64 events per reply, and chat completions' stream had 31; per output token, the Messages API sent about 1.2 times as many events.
The adapter turns each event into at most one of the loop's [`Delta` values](/glossary/#delta) (the pieces of a streamed reply), so the loop never learns which format it is reading.

```diagram
name: ch11-messages
alt: On the left, chat completions: a system message, a user message, an assistant message with two tool calls whose arguments are strings, and two tool messages. On the right, the Messages API: a system field, a user turn, an assistant turn with a signed thinking block and two tool_use blocks whose input is an object, and one user turn holding two tool_result blocks, one with is_error set.

The same turn in both formats: chat completions on the left, Anthropic's Messages API on the right.
```

## Bytes on the wire

Each format writes the same conversation its own way, and the sizes come out close.
One request, the last of a recorded run, was encoded with each client:

| Client | Total bytes | Of it, reasoning |
| --- | --- | --- |
| The companion code's client, chat completions | 55,162 | 11,156 |
| OpenRouter's Go SDK, chat completions | 55,227 | 11,156 |
| The Messages API's | 43,219 | 0 |

Without its reasoning, the chat completions conversation was within about 2 percent of the Messages API's.
The Messages API request carried no reasoning because that reasoning came from another client, and sending it would break the rule in the reasoning row above.

## Other servers that speak chat completions

The chat completions format is the one many servers accept, and the client reads its address from `OPENROUTER_BASE_URL`.
Point it at a local Ollama server, `http://localhost:11434/v1`, with no other change:

```console
$ OPENROUTER_BASE_URL=http://localhost:11434/v1 \
  OPENROUTER_API_KEY=ollama \
  OPENROUTER_MODEL=<a tool-capable model you have pulled> \
  go run ./ch04
```

This needs Ollama running on your machine and a model that supports tool calls.
How well it handles the task depends on the model you pull.

Two things differ there: the program needs any non-empty key, since without one it [replays recorded replies](/production/testing-offline/) instead of calling the server, and fields only OpenRouter sends, such as each call's cost, read as zero, so a [dollar budget](/production/stop-reasons/) does nothing.
[More providers](/ecosystem/more-providers/) lists the native APIs of Gemini, Ollama, Bedrock, and Cohere, with their official documentation.

## Writing a third adapter

For any new API, the rows of the table are the questions.
Answer each one in code, keep the API's format inside its adapter, and test it two ways: `httptest` replies for each error and edge case, and a recorded run replayed through the client.
For scale, the Messages API client, `ch11/anthropic`, is {{code lines ch11/anthropic}} lines of Go in {{code files ch11/anthropic}} files, its tests included.
