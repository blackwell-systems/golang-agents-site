---
title: "One agent, several providers: OpenAI-compatible APIs, the Messages API, and official SDKs in Go"
short: Providers and SDKs
description: How to run one agent loop against providers whose APIs disagree on almost everything. An official SDK wrapped in your own interface with its retries off and its errors translated, a hand-written client for Anthropic's Messages API, reasoning that has to return to the provider and host that produced it, and one task run through three clients.
card: One agent loop on providers whose APIs disagree, with each SDK behind your own interface.
group: Providers
chapters: 11
stage: 2
date: 2026-10-18
weight: 10
art: ch11
---

Sooner or later an agent written for one API needs to reach another.
Maybe a team has an Anthropic account but no OpenRouter account, maybe a model is served by only one provider, maybe someone wants to point it at a server on their own machine.
When the loop knows one API's wire format, each new provider is a change to the loop.
When the loop knows only its own `Model` interface, each new provider is one more adapter, and the loop stays as it is.

Putting that idea under real load means writing two extra clients for the same loop.
One wraps an official SDK for the chat completions API already used here; the other is written by hand for Anthropic's Messages API, which does almost everything differently.
The code is in [`ch11/orsdk`](code:ch11/orsdk) and [`ch11/anthropic`](code:ch11/anthropic).

## Why test with a native API

Adding another OpenAI-compatible endpoint would show very little.
The hand-written client can already reach any such server; you change the base URL and nothing else.
The real test is a [native API](/glossary/#native-api), meaning a provider's own request and reply format.
The Messages API differs from chat completions in these ways:

- the system prompt goes in its own request field instead of the message list;
- no message has a `tool` role; tool results travel as blocks inside a user message, with every result from one reply packed into that single message;
- the arguments of a tool call arrive as a JSON object, where chat completions sends a string you have to decode;
- reasoning comes back as thinking blocks, and each block carries a signature;
- stop reasons, usage counts, and error bodies each have their own names and layout.

Because OpenRouter also serves the Messages API, every run recorded here used a single key.
Change the base URL to `https://api.anthropic.com/v1` and the same client goes to Anthropic directly.

## An official SDK, behind your own interface

OpenRouter has a Go SDK for chat completions, produced by a code generator from its API description.
Adding it makes it the first dependency of the main module in the companion code, pinned at `v0.9.59`: a generated library before 1.0 changes its types every time the description changes.
Two settings in the adapter's constructor do most of the work:

```go include=ch11/orsdk/orsdk.go tag=client
```

**Supply your own HTTP client.** Any value with a `Do` method will do, so the replay transport plugs straight in, and so does a thin wrapper that notes each response's status code and `Retry-After` header.

**Switch the SDK's retries off** if your harness already retries, or every harness retry will contain a full round of SDK retries.
Left unconfigured, this SDK retries all 5xx responses and connection errors with backoff and keeps going for as long as an hour.
In one test against a local server that returned 503 for every request, a single call with a 10-second deadline tried five times and stopped only when the deadline expired.

**Translate its errors.** When a call fails, the SDK returns one of fourteen error types, and none of them exposes the HTTP status as a field.
The adapter converts each one into the `*APIError` that the hand-written client already returns.
Callers go on using `errors.Is(err, agent.ErrBudget)` and `Retryable()` and never learn which client sent the request.
Had the adapter passed the SDK's types upward, every caller would have had to import the SDK as well.

**Set each field on every message type.** In this SDK, a message is a union of generated types, one per role.
The adapter's first version added the cache marker to system and user messages but missed tool messages.
In that run the cache supplied just 30 percent of the input, and the run cost $0.0208.
After the fix, the same task got 91 percent from the cache and cost between $0.0057 and $0.0108.

The SDK also alters a few bytes on the wire.
Schemas are stored as Go maps, so their properties are sent in alphabetical order, and reasoning records are re-serialized with the fields rearranged.
Every distinct reasoning record from recorded runs, 443 of them, was pushed through the SDK's types.
Each came out with identical fields and values, and OpenRouter took them back in every run that used the SDK.

## Reasoning stays with the provider and host that wrote it

Within a turn that uses tools, thinking blocks have to be returned unchanged, and Anthropic refuses a block that has been edited, moved, or rebuilt.
The Messages API adapter therefore saves the full content of each reply, all its blocks in the order the API sent them, and returns exactly that.
It must also make sure that content never reaches a different client's API.
For that, the agent's `Message` gets one new field:

```go include=ch11/agent/types.go tag=reasoning-from
```

The value records the server as well as the API, since the model alone doesn't decide where a block is accepted.
OpenRouter can send one model's requests to several hosts, and each host runs under its own account.
An experiment on `anthropic/claude-haiku-5.5`, two runs in October 2026, showed the effect.
It pinned the first request to one host, Claude Platform on AWS, and then sent the continuation, the block plus a tool result, along five routes, one of them the same host with the block left out.
Returned to the host that produced it, the block was billed as input at its full size.
Routed instead to Anthropic's own API, Azure, or Google Vertex, the request was refused by OpenRouter with a 400 that named the original host.
The rule that follows: store the origin of every piece of reasoning, and drop the reasoning whenever a conversation moves to a different host or provider.

Editing the history raises a related problem.
On that model a thinking block stays valid only as long as nothing earlier in the request changes.
Context policies change earlier content all the time, by clearing an old tool result or dropping old turns, and every thinking block after the edit becomes stale.
Rather than leave it to the account's default, the client sends Anthropic's beta setting that discards stale blocks.

## One task, three clients

Each client ran the `m1` triage task, one crash report from the [crash catalog](/glossary/#crash-catalog) for the agent to triage, three times in October 2026, all through OpenRouter on the default model at its API's default reasoning setting:

| Client | Turns | From the cache | Cost | Checks passed |
| --- | --- | --- | --- | --- |
| Hand-written chat completions | 21, 14, 17 | 91 to 92% | $0.0068 to $0.0132 | 16 of 16, each |
| The OpenRouter SDK | 15, 12, 16 | 91% | $0.0057 to $0.0108 | 16 of 16, each |
| Hand-written Messages API | 16, 18, 14 | 91 to 94% | $0.0086 to $0.0100 | 16 of 16, each |

All nine runs passed, and on every column the three clients' ranges overlap.
With three runs each, the numbers can't rank the clients by cost.
What they do show is that switching clients took nothing away from the agent.
The Messages API runs used an earlier version of its client; the recording the replay below uses comes from the final one, and finished in 13 turns for $0.0080, with 87.5 percent of its input served from the cache and all 16 checks passed.
The two formats also come out close in size.
One request, the last of a recorded `m1` run, was encoded with each client; leaving reasoning aside, the conversation part was 29,695 bytes in chat completions and 29,356 bytes in the Messages API, within about 1 percent.

## A checklist for the next adapter

An adapter for some other API, Gemini or a local server, has to settle the same points the Messages API adapter did:

- Does the system prompt travel as a message or as its own field?
- What roles does the API know, and do user and assistant turns have to take turns?
- Do tool calls carry their arguments as a JSON string or as an object?
- How are tool results sent back, and can they be marked as failures?
- Which content has to be returned untouched, and to which host?
- What does each stop reason become in your types, including a refusal?
- Does the reply include usage, cache counts, and cost, and is cached input part of the prompt count?
- Which errors are worth retrying, and where does the API say how long to wait?
- How does each streaming event turn into part of the assembled reply?
- What do you test with an `httptest` server, and what with a recorded run?

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), with no API key set:

```console
$ go run ./ch11 -provider anthropic
$ go run ./ch11 -provider sdk
```

Each command replays a recorded `m1` run through the chosen client, runs every tool for real, and prints the tokens, the cost, and the catalog's grade at the end; both recordings pass 16 of 16.
The service's tests run many times along the way, so give each command a minute or so.
