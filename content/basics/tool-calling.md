---
title: Tool calling in Go, on the wire
short: Tool calling on the wire
description: What a tool call looks like as JSON on an OpenAI-compatible chat completions API, how a Go client sends tool definitions and reads calls back, and the rules that keep a conversation valid.
card: A tool call as raw JSON, how a Go client sends and reads one, and the rules that keep it valid.
chapters: 4, 2
stage: 3
date: 2026-10-18
weight: 40
art: ch04
---

A model never runs a tool.
It writes a request to run one, and your program decides what that request means.
The request travels as JSON, and getting the JSON right in both directions is most of what "tool calling" means for the program that does it.

This page shows the real JSON from one recorded run of chapter 4's agent, on `anthropic/claude-haiku-5.5` in October 2026: the request that offers tools, the reply that asks for them, and the messages that answer.
Then it shows the Go types that carry them, from the companion code's [`openrouter` package](code:ch04/openrouter).
The API is OpenRouter's chat completions endpoint, `POST https://openrouter.ai/api/v1/chat/completions`, which uses the OpenAI-compatible format that many providers and local servers share.

## Three additions to a chat request

Without tools, a request is just a model ID plus messages, each a role and its text.
Tool calling extends that JSON in three places:

- a `tools` array in the request, one definition per tool;
- a `tool_calls` array in assistant messages, where the model asks for tools;
- a `tool_call_id` in tool messages, naming the call each result answers.

## Every request carries the tool definitions

Here is the first request of the run, cut to one of its four tools:

```json include=ch04/testdata/out/request.json title="The first request, with one of its four tools"
```

A [tool definition](/glossary/#tool-definition) is a name, a description in plain language, and a [JSON Schema](/glossary/#json-schema) for the arguments.
The API wraps each definition in an object with a `type`, always `"function"` here, which leaves room for other kinds of tools.
The model reads the name and the description to decide when to call the tool, and the schema to decide what to send, so both are part of your prompt.

The definitions go out with every request, and they are not free.
In this run the first request was 583 tokens, although the task is one sentence: the four definitions came to about 270 tokens, and the provider added about 290 more of its own instructions for using tools, text you never see but pay for.
How much a provider adds varies; across chapter 1's 40 runs (ten on each of four models, October 2026), the same first request came to 126 tokens on `gpt-5.6-luna`, whose provider encodes tool definitions in a compact format of its own, and at 441 on `deepseek-v4.1-flash`.

## What the model actually reads

The JSON above is what your program sends, not what the model sees.
A model reads one stream of text, so before generating, the provider turns the messages and the tool definitions into a single text using the model's [chat template](/glossary/#chat-template), a layout each model family was trained on.
The provider's own instructions for using tools, the roughly 290 tokens counted above, are part of that text.
Chapter 3's `chatml` package renders the start of the book's bug fix the way Qwen3's open models lay it out, with one tool:

```text include=ch03/testdata/out/template.txt lines=3..22 title="One tool, a task, and a tool call, as the model reads them"
```

The definition turns into one JSON line in the system message, and after it come instructions, in English, on how to ask for a call.
In this template the model's tool call is text it writes between `<tool_call>` tags, and the provider parses that text back into the `tool_calls` array you receive.
Other model families use other markers and wording, and closed models don't publish theirs, but the mechanism is the same: the call is text the model writes, which the provider turns back into JSON.
So choosing to call a tool is the same act as writing any other reply: the model continues the text, and a call is the continuation it found most likely, given the descriptions, the task, and everything said so far.

That explains several things a harness has to handle.
Because the call is generated text, its arguments can be malformed JSON, and its name can be a tool that doesn't exist unless the provider restricts the output to the listed tools.
Because the definitions sit in the same text as the conversation, the wording of a tool's name and description steers the choice as much as your prompt does, and text that comes back in a tool result is read with the same attention.
Other model families use other templates, and most hosted providers don't show theirs, but the shape is the same.

## One reply can ask for several tools

The model answered the first request like this (the part of the response that matters):

```json include=ch04/testdata/out/reply.json lines=10..49 title="The first reply: a sentence and two tool calls"
```

Four things in it are worth reading closely.

**One reply, two calls.** The model asked to run the tests and list the files at once.
A reply can hold several tool calls, and text alongside them.
Every model in chapter 1's 40 runs did this in every run.
The `parallel_tool_calls` setting can turn it off, but the name misleads: it decides whether a single reply is allowed to contain more than one call, and says nothing about whether your harness runs them concurrently.
The companion code leaves it on: every turn resends the whole conversation, so asking for two things at once saves a turn's worth of tokens and time.

**The arguments are a string.** `"arguments": "{}"` is a string holding JSON, not a JSON object.
The client has to unwrap it, and it can't assume the JSON inside is valid: nothing forces the model to write arguments that match the schema, or even to write well-formed JSON.

**The ID comes from the provider.** Anthropic's IDs start with `toolu_`.
The harness copies each ID into the message that answers the call.

**`reasoning_details` go back unchanged.** This model thought for 24 tokens before replying, and the provider returned a signed record of that thinking, shortened above.
The harness never reads it, but it must send it back, exactly as it came, with the assistant message it belongs to; without it, the model starts its next turn without the reasoning behind its own call, and some providers reject the conversation.

`finish_reason` is `tool_calls`, OpenRouter's translation of Anthropic's own `tool_use` in `native_finish_reason`.
Don't lean on it.
Chapter 2 found the translation right in all 194 replies of the 40 runs, but other servers that accept the same requests don't all translate as carefully, and some report `stop` for a reply that holds tool calls.
The loop in the companion code decides from the reply itself: a reply with tool calls is a request for tools, whatever its finish reason says.

## Each result answers one call, by its ID

After running the tools, the harness appends one message per call, each with the role `tool` and the ID it answers, and sends the whole conversation again.
Here are three messages from the fourth request of the run, with the file contents cut short:

```json include=ch04/testdata/out/turn4.json title="An assistant message and the two tool messages that answer it"
```

The rule the API enforces: each call needs exactly one tool message, and that message has to come later in the list than the assistant message holding the call.
Drop a result, or send one without its call, and the next request is rejected.
If your loop adds the reply to the history first and then writes one result per call, in the order the calls appear, the rule holds by construction; [the agent loop in Go](/basics/agent-loop/) shows one.

## The wire types stay inside the client

The client keeps the API's JSON in its own unexported types, separate from the agent's `Message`, which keeps everything specific to this API inside one file of the client:

```go include=ch04/openrouter/wire.go tag=wire-message
```

Three details carry most of the weight:

- `Content` is `omitzero`, so when a reply is nothing but tool calls, the field is left out of the JSON entirely, as in the assistant message from turn 4 above.
- `Arguments` is a `string`, matching the wire. When the client reads a reply, it keeps the arguments as raw JSON (`jsontext.Value`) in the agent's own `ToolCall`, valid or not, and each tool decodes its own; a model that sends an empty string for a tool with no arguments gets `{}`.
- `ReasoningDetails` is a `jsontext.Value`, raw JSON the client carries without looking inside.

Every field has an explicit `json` tag because `encoding/json/v2`, in the standard library from Go 1.27, matches field names exactly.
An untagged `Path` field would never receive `"path"`, and the tool would get an empty value with no error.

The API has no field for a failed tool.
The agent's `Message` has an `IsError` flag, and when the client converts a tool message, it turns that flag into a prefix, so the model reads `Error: old text appears 0 times in duration.go, want exactly once` and knows the call failed.
An empty result becomes `(no output)`, since an empty message can trip up some providers and the text tells the model the tool ran and printed nothing.

## A wrong call gets an error the model can fix

Because the arguments are text the model wrote, the harness checks every call before running it: is the tool name real, do the arguments decode into the tool's argument struct, and are there fields the struct doesn't have?
Chapter 4's decoder rejects unknown fields, because models guess names (`old_str` for `old`), and ignoring a guess would leave the real field empty, which can turn an edit into a deletion that reports no error.

Each failed check goes back to the model as a tool message that says what was wrong and what to send instead:

```text include=ch04/testdata/out/recover.txt lines=4..12 title="Two wrong calls and their tool errors, from a hand-written replay"
```

The model can fix its call on the next turn, and the run goes on.
Some APIs can constrain the arguments to match the schema exactly (strict mode, a form of [structured outputs](/glossary/#structured-outputs)), at the price of a restricted schema; even then, a well-formed `path` may still point at a missing file, so the checks in the harness stay on regardless.

## Run it

From the `code` directory of a clone of the repository, with no API key set:

```console
$ go run ./ch04
$ go run ./ch04 -replay recover
```

The first replays the recorded run this page quotes; only the model's replies are saved, and the tools, `go test` included, run for real.
The second replays the hand-written run with three bad calls.
The [ch04 example](/run/ch04-first-agent/) has more to try.
