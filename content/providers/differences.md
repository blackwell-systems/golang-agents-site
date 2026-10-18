---
title: How chat completions and the Messages API differ
short: How two provider APIs differ
description: The same agent request, sent as OpenAI-style chat completions through OpenRouter and as Anthropic's Messages API, differs in twelve places, from where the system prompt goes to how a stream ends. A table of each difference, with the chapter and the code that handles it.
chapters: 2, 4, 6, 8, 11
stage: 2
date: 2026-10-18
weight: 10
---

The book's agent talks to models through one interface, `Model`, with one method.
Behind it sit two clients: one for the chat completions format, which OpenRouter, OpenAI, and many local servers accept, and one for Anthropic's Messages API.
Chapter 11 writes the second as an adapter and lists what an adapter for any API has to decide.
This page collects the answers for the two APIs the book covers into one table, so you can see the size of the job before you start a third.

Facts below are from chapters 2, 4, 6, 8, and 11, checked against the companion code, as of October 2026.
Provider APIs change; each row names the place in the code to check when one does.

## The table

| What differs | Chat completions (OpenRouter) | Messages API (Anthropic) | Where the book handles it |
| --- | --- | --- | --- |
| System prompt | A message with role `system` | A top-level `system` field | `ch11/anthropic/wire.go` |
| Reply length limit | Optional; the provider's default if unset | `max_tokens` is required; the client sends 16,000 when the agent sets none, since thinking counts against it | Chapter 11 |
| Roles | `system`, `user`, `assistant`, and `tool` | `user` and `assistant` only, alternating; tool results and harness notes after them are joined into one user turn | Chapter 11 |
| Tool-call arguments | A string that holds JSON | A JSON object; arguments that aren't valid JSON go as an empty object, and the tool's error explains | Chapters 4 and 11 |
| Tool results | A message with role `tool` and the call's ID; no failure flag, so the client writes "Error:" into the text | A `tool_result` block in a user turn, with an `is_error` flag | Chapters 4 and 11 |
| Reasoning sent back | OpenRouter's `reasoning_details`, returned as received | Thinking blocks returned exactly as received, in order, only to the provider and host that wrote them | Chapter 11 (`ReasoningFrom`) |
| Why the reply stopped | `finish_reason`: `stop`, `tool_calls`, `length`, `content_filter`, `error` | `stop_reason`: `end_turn`, `tool_use`, `max_tokens`, `refusal`, and others, mapped onto the left column | `ch11/anthropic/wire.go` |
| Prompt token count | `prompt_tokens` includes tokens read from the cache | `input_tokens` leaves out cache reads and writes, so the prompt is the sum of three counts | Chapter 11 |
| Cost | OpenRouter reports what it billed in each reply | Not reported by Anthropic's API; the client prices each reply from a table kept in configuration (OpenRouter's Messages endpoint does report it) | Chapter 11 (`prices.json`) |
| Prompt caching | Automatic on some models; Anthropic's models need a `cache_control` marker, which OpenRouter passes on | `cache_control` markers, up to four per request | Chapter 8 |
| Errors | An HTTP status, or an error object inside a 200 reply | An HTTP status, including 529 when overloaded; an error body in a 200 or an error event mid-stream becomes the same error type | Chapters 5, 11, and 12 |
| Streaming | `data:` lines of chunks; the usage arrives after the finish reason; ends with `data: [DONE]` | Named events per content block; the stop reason and final usage arrive together in `message_delta`; ends with `message_stop` | Chapters 6 and 11 |

A few rows need more than a cell.

**Refusals.**
A Claude model that declines a request stops with `refusal`, in a normal reply that can hold some text.
The conversation can't usefully continue from there, so the book's loop counts it among the replies it can't continue from and stops with `ErrRefused`, a new member of the bad-reply family in chapter 11.
Here is the whole mapping, from the [companion code](code:ch11/anthropic/wire.go):

```go include=ch11/anthropic/wire.go tag=finish-reason
```

**Reasoning and hosts.**
OpenRouter routes one Claude model to several hosts, each under its own account.
In chapter 11's experiment, a thinking block sent back to the host that wrote it was accepted in both runs, and sent to any of three other hosts it was refused with a 400.
That is why the adapter records where reasoning came from, and why chapter 13 later pins one host.

**Stream size.**
Over the 13 replies of each recorded run of the same task, the Messages API's stream had a median of 64 events per reply, and chat completions' 31; per output token, about 1.2 times as many.
The adapter turns each event into at most one of the loop's `Delta` values, so the loop never learns which format it is reading.

## Bytes on the wire

Each format writes the same conversation its own way, and the sizes come out close.
Chapter 11 encoded one request, the last of a recorded run, with each client:

| Client | Total bytes | Of it, reasoning |
| --- | --- | --- |
| Chapter 2's, chat completions | 55,162 | 11,156 |
| OpenRouter's Go SDK, chat completions | 55,227 | 11,156 |
| The Messages API's | 43,219 | 0 |

Without its reasoning, the chat completions conversation was within about 1 percent of the Messages API's.
The Messages API request carried no reasoning because that reasoning came from another client, and sending it would break the rule in the reasoning row above.

## Other servers that speak chat completions

The chat completions format is the one many servers accept, and the book's client reads its address from `OPENROUTER_BASE_URL`.
Chapter 2 points it at a local Ollama server, `http://localhost:11434/v1`, with no other change.
Two things differ there: the program needs any non-empty key to stop replaying, and fields only OpenRouter sends, such as each call's cost, read as zero, so a dollar budget does nothing.
[More providers](/providers/more-providers/) lists the native APIs of Gemini, Ollama, Bedrock, and Cohere, with their official documentation.

## Writing a third adapter

For any new API, the rows of the table are the questions.
Answer each one in code, keep the API's format inside its adapter, and test it two ways: `httptest` replies for each error and edge case, and a recorded run replayed through the client.
For scale, chapter 11's Messages API client, `ch11/anthropic`, is {{code lines ch11/anthropic}} lines of Go in {{code files ch11/anthropic}} files, its tests included.
