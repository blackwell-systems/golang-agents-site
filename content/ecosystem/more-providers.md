---
title: Gemini, Ollama, Bedrock, and Cohere for Go agents
short: More providers
description: A reference for writing a third adapter. How the native APIs of Google's Gemini, Ollama, Amazon Bedrock, and Cohere carry the system prompt, tool calls, tool results, stop reasons, and usage, whether each offers an OpenAI-compatible endpoint, and its official Go library, from each provider's documentation as of October 18, 2026.
card: How Gemini, Ollama, Bedrock, and Cohere carry tools and stop reasons, for writing an adapter.
group: Providers
chapters: 2, 11
stage: 2
date: 2026-10-18
weight: 20
art: more-providers
---

The agent in the companion code speaks two formats: chat completions, through OpenRouter, and Anthropic's Messages API.
A natural next step is a third adapter, for Gemini or a local server, and there are [questions such an adapter has to answer](/ecosystem/providers-and-sdks/#a-checklist-for-the-next-adapter).
This page answers them for four providers, from their own documentation, so you can size the work before you start.

Unlike the rest of this site, nothing here came from a recorded run.
Every fact below comes from the provider's official documentation, linked in each section, as read on October 18, 2026.
These APIs change often; check the linked page before you rely on a detail.

## The short version

| Provider | Native API | Tool-call arguments | Call IDs | OpenAI-compatible endpoint | Official Go library |
| --- | --- | --- | --- | --- | --- |
| Google Gemini | Interactions API (recommended); `generateContent` (still supported) | A JSON object | Yes | Yes, in beta | `google.golang.org/genai` |
| Ollama | `POST /api/chat` on `localhost:11434` | A JSON object | Present in the Go client's type | Yes, `/v1/chat/completions` and more | `github.com/ollama/ollama/api` |
| Amazon Bedrock | Converse (`/model/{modelId}/converse`) | A JSON object | Yes, `toolUseId` | Yes, for some models | AWS SDK for Go v2, `bedrockruntime` |
| Cohere | Chat v2 (`POST /v2/chat`) | A string holding JSON | Yes | Yes, a compatibility API | `github.com/cohere-ai/cohere-go/v2` |

The quickest way to reach one of them from the companion code is through that provider's OpenAI-compatible endpoint, where it has one, by pointing `OPENROUTER_BASE_URL` at it, which the companion code's programs read.
For Ollama, from the `code` directory:

```console
$ OPENROUTER_BASE_URL=http://localhost:11434/v1 \
  OPENROUTER_API_KEY=ollama \
  OPENROUTER_MODEL=<a tool-capable model you have pulled> \
  go run ./ch04
```

This needs Ollama running on your machine and a model that supports tool calls; the key can be any non-empty text, and its only job is to take the program out of replay mode.
This command was not run for this page, so what it prints, and how well a small local model handles the task, is for you to see.
The cost is the compatibility layer's gaps, listed below for each, and the loss of whatever the native API offers that chat completions doesn't.

## Google Gemini

Google now recommends the [Interactions API](https://ai.google.dev/gemini-api/docs/interactions), generally available since June 2026, at `POST https://generativelanguage.googleapis.com/v1beta/interactions`, authenticated with an `x-goog-api-key` header.
Its main difference from chat completions and the Messages API is state on the server: by default it stores each interaction (`store` is true), and a request can continue from `previous_interaction_id` instead of resending the conversation.
The documentation gives a retention of 55 days on the paid tier and 1 day on the free tier.
What the server stores is the conversation's turns; tools, the system instruction, and generation settings apply to one interaction only and must be sent again with each request.

In [function calling](https://ai.google.dev/gemini-api/docs/function-calling) on this API, a tool is declared with `type: "function"`, a name, a description, and a JSON Schema for its parameters.
The model's call arrives as a `function_call` step with an `id`, a `name`, and `arguments` as a JSON object; the result goes back as a `function_result` with the matching `call_id`.
Several calls can come in one response.
The calling modes are `auto`, `any`, `none`, and `validated`.

The older [`generateContent`](https://ai.google.dev/api/generate-content) remains supported, though Google calls it legacy and launches new features on the Interactions API.
Its request takes the system prompt in a separate `systemInstruction` field, and its `contents` use only two roles, `user` and `model`.
Tools are `functionDeclarations`, a call is a `functionCall` part with `args` as an object, and a result is a `functionResponse` part.
Streaming is `streamGenerateContent` with `alt=sse`.
Usage arrives in `usageMetadata`, where `promptTokenCount` includes cached tokens, reported separately as `cachedContentTokenCount`, and thinking is counted in `thoughtsTokenCount`.

**OpenAI compatibility.** Gemini serves chat completions at `https://generativelanguage.googleapis.com/v1beta/openai/`, with tools, streaming, and structured output; the [documentation](https://ai.google.dev/gemini-api/docs/openai) calls it beta and says parameters it doesn't list are ignored without an error.

**Go.** The official library is [`google.golang.org/genai`](https://ai.google.dev/gemini-api/docs/libraries), generally available since May 2025.
[ADK for Go](/ecosystem/go-agent-frameworks/) reaches Gemini through this client and accepts your own `*http.Client` through its configuration.

## Ollama

Ollama runs models on your own machine and serves its [native chat API](https://docs.ollama.com/api/chat) at `POST /api/chat`, by default on `http://localhost:11434`.
Its roles are the familiar four: `system`, `user`, `assistant`, and `tool`.
Tools are declared as in chat completions, with `type: "function"` and a JSON Schema, but a call's `arguments` arrive as a JSON object, not a string.
Streaming is on unless the request turns it off, and the stream is newline-delimited JSON, not server-sent events; the last object has `done` set and a `done_reason`.
Token counts are `prompt_eval_count`, `prompt_eval_cached_count`, and `eval_count`.
There is no cost, since nothing is billed.

**OpenAI compatibility.** Ollama also serves [`/v1/chat/completions`](https://docs.ollama.com/api/openai-compatibility), with tools and streaming, plus `/v1/responses` and others.
The key is required but ignored on a local server.
Among the fields it doesn't support are `tool_choice`, `logprobs`, and `n`.
This is the endpoint the companion code uses, and two consequences follow: the program needs a non-empty key, since without one it [replays recorded replies](/production/testing-offline/) instead of calling the server, and cost reads as zero.

**Go.** Ollama's own repository includes a Go client, [`github.com/ollama/ollama/api`](https://pkg.go.dev/github.com/ollama/ollama/api); `ClientFromEnvironment` reads the server's address from `OLLAMA_HOST`.
Its `ToolCall` type has an `ID` field and its `Message` a `ToolName`, so check which of them your model and server version fill in.

## Amazon Bedrock

Bedrock hosts models from several companies, and its [Converse API](https://docs.aws.amazon.com/bedrock/latest/userguide/conversation-inference.html) gives them one request shape: `POST /model/{modelId}/converse`, or `ConverseStream` for streaming.
The system prompt is a separate `system` field, a list of blocks, and messages take two roles, `user` and `assistant`, each holding a list of content blocks.
Tools go in `toolConfig`; the model's call is a [`toolUse`](https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_ToolUseBlock.html) block with a `toolUseId`, a `name`, and an `input` object; the result goes back as a [`toolResult`](https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_ToolResultBlock.html) block with the same `toolUseId`, and an optional `status` of `success` or `error`, which only some model families accept.

The [`stopReason`](https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_Converse.html) values are `end_turn`, `tool_use`, `max_tokens`, `stop_sequence`, `guardrail_intervened`, `content_filtered`, `malformed_model_output`, `malformed_tool_use`, and `model_context_window_exceeded`; an adapter maps each onto the loop's finish reasons, the same way it maps Anthropic's.
Usage reports `inputTokens`, `outputTokens`, and, with prompt caching, `cacheReadInputTokens` and `cacheWriteInputTokens`.
The stream sends `messageStart`, then for each content block a start, deltas, and a stop, then `messageStop` with the stop reason, and last a `metadata` event with the usage.
A model's reasoning comes with a signature, and the documentation warns that the reasoning must be sent back unchanged, the same rule that applies to Anthropic's thinking blocks.

Requests are signed with AWS credentials, or, as an alternative, authenticated with a [Bedrock API key](https://docs.aws.amazon.com/bedrock/latest/userguide/api-keys.html) as a bearer token; short-term keys last up to 12 hours and are the ones recommended for production.

**OpenAI compatibility.** Bedrock serves OpenAI's [Responses API](https://docs.aws.amazon.com/bedrock/latest/userguide/bedrock-mantle.html) at `https://bedrock-runtime.{region}.amazonaws.com/openai/v1`, and Chat Completions for some models; which API each model supports is on its model card.
The Responses API stores responses for 30 days unless a request sets `store` to false.

**Go.** The AWS SDK for Go v2 has a `bedrockruntime` client with `Converse` and `ConverseStream`.

## Cohere

Cohere's [Chat API v2](https://docs.cohere.com/reference/chat) is `POST https://api.cohere.com/v2/chat`, with a bearer token.
It is the closest of the four to chat completions: roles `system`, `user`, `assistant`, and `tool`; tools declared with `type: "function"` and a JSON Schema; a call with an `id`, a function name, and `arguments` as a string holding JSON; and a result sent as a `tool` message with the call's `tool_call_id`.
Its `finish_reason` values differ: `COMPLETE`, `STOP_SEQUENCE`, `MAX_TOKENS`, `TOOL_CALL`, `ERROR`, and `TIMEOUT`.
Usage reports both the tokens billed (`billed_units`) and the tokens processed (`tokens`), plus `cached_tokens`.
Streaming is server-sent events.

**OpenAI compatibility.** A [compatibility API](https://docs.cohere.com/docs/compatibility-api) at `https://api.cohere.ai/compatibility/v1` accepts chat completions with tools, streaming, and structured outputs; it doesn't support `parallel_tool_calls` or `n`, among others, nor Cohere's own document and citation options.

**Go.** Cohere's Go SDK is [`github.com/cohere-ai/cohere-go/v2`](https://github.com/cohere-ai/cohere-go), with `Chat` and `ChatStream`.

## Writing the adapter

Whichever you pick, the [checklist for the next adapter](/ecosystem/providers-and-sdks/#a-checklist-for-the-next-adapter) applies, and the [difference table](/ecosystem/differences/) shows the answers for chat completions and the Messages API.
Three answers on this page change the most code:

- **Where the conversation lives.** Gemini's Interactions API, and the Responses API that Bedrock serves for OpenAI compatibility, can keep it on the server; Bedrock's native Converse API keeps nothing. The loop in these guides resends the whole conversation every turn and saves it for crash recovery; a server-side copy is a second record to keep in step, or a feature to leave off.
- **How arguments arrive.** Gemini, Ollama's native API, and Bedrock send an object, so an adapter whose loop stores arguments as text has to serialize the object itself, and the text it gets is the adapter's encoding, not the model's.
- **How the stream ends.** Each one ends differently: Ollama's last object has `done` set, Bedrock sends a `messageStop` followed by `metadata`, and the others end with a final server-sent event of their own. Read the final usage from wherever it arrives before treating the reply as complete.

Test each adapter the way the Messages API client is tested: an `httptest` reply for each error and edge case, and a [recorded run replayed](/production/testing-offline/) through it.
