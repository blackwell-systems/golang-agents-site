---
title: "encoding/json/v2 for LLM APIs in Go: what changed, and what an agent needs"
short: JSON v2 for model APIs
description: Using Go's encoding/json/v2 and jsontext for model APIs and tool calls. Why switch, with a benchmark, what changed from encoding/json, each behavior checked in Go 1.27, wire types with omitzero and jsontext.Value, invalid UTF-8 from tools, strict tool arguments, a field with two shapes, key order, deterministic output, and options for saving a run.
card: What changed from encoding/json, and the parts of v2 an agent leans on, each checked in Go 1.27.
group: Go in depth
chapters: 2, 4, 7, 8, 10, 15
stage: all
date: 2026-10-19
weight: 84
art: go-json
---

Everything an agent says to a model API is JSON: the request, the reply, each tool's definition, and the arguments of every [tool call](/basics/tool-calling/), which a model wrote and which are sometimes wrong.
Go's newer JSON package, `encoding/json/v2`, with its lower-level companion `encoding/json/jsontext`, changes several defaults in ways that matter for exactly this kind of traffic.
This page covers what changed and the parts of the package an agent uses, with code from the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), where it is tested.

## Which Go has it

In Go 1.27, `encoding/json/v2` is available by default; `GOEXPERIMENT=nojsonv2` turns it off.
Go 1.26 ships it behind `GOEXPERIMENT=jsonv2`, and without that setting the package doesn't build.
The original `encoding/json` is still there, so a program can move one package at a time.
The companion code needs Go {{code go-version}}, and `go` downloads it when it reads the module's `go.mod`.

## Why switch

For an agent, the case for v2 is mostly about what it refuses to accept.
A model's output is untrusted input, and v2's defaults treat it that way:

- **Duplicate keys are an error.** In v1 the last one wins; when two parsers disagree about which duplicate counts, a value can get past a check that read the other one.
- **Invalid UTF-8 is an error,** in both directions, instead of passing through.
- **Names match exactly,** so `{"PATH": "..."}` can't land in a field tagged `path` by accident.

The rest is control:

- **Options per call,** such as `RejectUnknownMembers` for strict tool arguments, `AllowInvalidUTF8` for tool output, and `Deterministic` for stable bytes. v1's only option of this kind was `DisallowUnknownFields`, and only on a `Decoder`.
- **Marshalers for types you don't own,** passed with `WithMarshalers` for one call, instead of a wrapper type used everywhere.
- **Streaming marshalers,** `MarshalJSONTo` and `UnmarshalJSONFrom`, which read and write token by token, so a field that can take two shapes needs no buffering or guessing.
- **Errors with locations:** `SemanticError` and `SyntacticError` carry a JSON Pointer to the part of the input that failed, which is what a model needs to fix its arguments.
- **Zero values handled better:** `omitzero`, which respects an `IsZero` method, and nil slices written as `[]`, which APIs that expect arrays prefer to `null`.

It is also faster at decoding, though for an agent that hardly matters.
On a MacBook Pro with an M1 Pro chip and Go 1.27, decoding into typed structs and encoding a request, five runs each:

| Payload | `encoding/json` | `encoding/json/v2` |
| --- | --- | --- |
| a model's reply, 3.8 KB | 9.7 to 10.4 µs | 7.2 to 8.5 µs |
| a 16-message request, 18 KB | 51 to 61 µs | 38 to 41 µs |
| encoding that request | 18.5 to 19.9 µs | 18.6 to 19.5 µs |

Decoding was about a quarter faster; encoding took the same time with both.
A model call takes seconds, so tens of microseconds either way don't change how long a run takes.

The costs are real too: code that relied on case-insensitive names or untagged fields stops decoding them, with no error; map keys are no longer sorted unless you ask; error text varies on purpose; and before Go 1.27 the package needs `GOEXPERIMENT=jsonv2`.

## What changed, checked

Each row below was checked with a small program built with Go 1.27, decoding and encoding the same values with both packages:

| Input or value | `encoding/json` | `encoding/json/v2` |
| --- | --- | --- |
| `{"TOOL_NAME": "x"}` into a field tagged `tool_name` | matches, ignoring case | doesn't match; the field stays empty, with no error |
| `{"tool_name": "x"}` into an untagged field `ToolName` | matches | doesn't match |
| a nil slice or map | `null` | `[]` or `{}` |
| `omitempty` on an `int` that is 0 | left out | written as `0` |
| a string with invalid UTF-8 | accepted | an error, on encode and decode |
| the same key twice in an object | the last one wins | an error |
| an unknown key | ignored | ignored, unless `RejectUnknownMembers` is set |
| map keys | sorted | in no fixed order, unless `Deterministic` is set |

The first two rows cause the most trouble, because nothing reports them.
A reply field whose name differs in case from the tag, or a struct field with no tag at all, decodes as its zero value, and the program carries on with an empty string where the model's answer should be.
The fix is a habit: every field decoded from an API or a model gets an explicit `json` tag with the exact wire name.

## Wire types

These are the request and message types of a chat completions client.
Every field is tagged, and most use `omitzero`:

```go include=ch06/openrouter/wire.go tag=wire-request
```

```go include=ch06/openrouter/wire.go tag=wire-message
```

`omitzero` leaves a field out when it holds its type's zero value, which for an `int` is 0, the case `omitempty` no longer covers.
Without it, `MaxTokens` would go out as `"max_tokens": 0` on every request, a request for an empty reply.
Where zero is a meaningful setting, the field is a pointer: `Temperature` is `*float64`, so "not set" is `nil` and a temperature of 0 is sent.

`ReasoningDetails` is a `jsontext.Value`, raw JSON kept exactly as it arrived.
Some models return reasoning that must be sent back unchanged on the next turn; the client never needs to know its structure, so it stores the bytes and writes them back as they were.

## Text a tool printed: invalid UTF-8

A tool's output goes into the next request, and tool output is whatever the tool printed: a test's log, a file's contents, a byte cut off partway through a character.
v2 refuses to encode a string with invalid UTF-8, so one bad byte in a tool's output would make the whole next request fail.
The client encodes requests with one option that changes that:

```go include=ch06/openrouter/client.go lines=88..93
```

With `jsontext.AllowInvalidUTF8(true)`, each invalid byte becomes the Unicode replacement character, `U+FFFD`, and the request goes out.
The same reasoning applies to code that truncates tool output to a size limit: cut at a character boundary, not at an arbitrary byte.

## Tool arguments: strict, with errors the model can use

Tool arguments are the one place where an unknown field is a mistake rather than a newer API: a model that sends `{"file": "a.go"}` to a tool that takes `path` should be told so.
The agent decodes arguments with `json.RejectUnknownMembers(true)`, and builds its own message from the error's fields, because v2 deliberately varies the wording of its error messages between runs; the [errors page](/basics/errors/#errors-the-model-reads) shows that code.
Two error types carry the details: `*jsontext.SyntacticError` for JSON that isn't valid, and `*json.SemanticError` for valid JSON that doesn't fit the Go type, with a JSON Pointer to where it went wrong.

## One field, two shapes

Model APIs often accept a field in more than one shape.
A message's content can be a plain string, or a list of parts, which is how a client marks a prompt-cache breakpoint, the point up to which the provider may [cache](/glossary/#prompt-caching) the request.
v2's streaming marshaler interfaces handle that in one type: `MarshalJSONTo` writes through a `*jsontext.Encoder`, and `UnmarshalJSONFrom` reads from a `*jsontext.Decoder`:

```go include=ch12/openrouter/wire.go tag=content
```

```go include=ch12/openrouter/wire.go lines=137..165
```

The reader looks at the kind of the next value, `'n'` for null, `'"'` for a string, anything else for a list, and decodes it accordingly.
The `IsZero` method matters too: `omitzero` calls it when a type has one, so a content with no text is left out of the message whatever else it holds.

## Order you control

A JSON object's members have no meaning in their order, but a model reads a [tool's schema](/build/tool-design/) top to bottom, so the order of its properties is worth keeping.
A Go map can't keep it, so the agent's schema type holds the properties as a slice and writes them as an object, in order, token by token:

```go include=ch12/agent/schema.go tag=props
```

## Same input, same bytes

v1 sorted map keys when it encoded a map; v2 doesn't, unless asked.
Two things in an agent need the same input to encode to the same bytes every time: a hash, and a prompt cache, which providers match on an identical prefix of the request.
`json.Deterministic(true)` sorts map keys, and the agent uses it where the bytes are hashed, to detect a tool whose definition changed:

```go include=ch10/mcptools/trust.go tag=digest
```

## Options for a whole format: saving a run

Options can be combined once and reused.
The agent's checkpoint format joins three: leave zero fields out, write every `jsontext.Value` as a quoted string instead of as raw JSON, and read those strings back the same way:

```go include=ch15/durable/checkpoint.go tag=format
```

The reason for the second is a model's tool call.
Its arguments are meant to be JSON, and occasionally they aren't.
A run that holds such a call still has to be saved, and raw invalid JSON can't be embedded in a JSON file, while a string holding it can.
`json.WithMarshalers` and `json.WithUnmarshalers` apply these functions to every `jsontext.Value` in the checkpoint, without changing the type itself or any other code that encodes it.

## Smaller things worth knowing

- **`json:",inline"`** promotes a field's members into the parent object. v2 already does this for an embedded struct without a JSON name, and the tag makes it explicit; it works on a named field too. The agent's price table uses it so a long-prompt tier carries the same fields as the base price.
- **`json.MarshalWrite` and `json.UnmarshalRead`** encode to an `io.Writer` and decode from an `io.Reader`.
- **`jsontext.Value`** has methods of its own: `Kind`, `IsValid`, and `Canonicalize`, which the tool digest above uses to put a schema in a standard form before hashing it.

## Mistakes to avoid

- **A decoded field without a tag.** It matches only its exact Go name, and a miss is not an error.
- **`omitempty` on a number or a bool.** v2 writes 0 and `false`; use `omitzero`, or a pointer when zero is a real value.
- **Tool output encoded as it is.** One invalid byte fails the next request; use `AllowInvalidUTF8`, and truncate at character boundaries.
- **Matching v2's error text.** It changes between runs on purpose; inspect the error types instead.
- **Maps in anything you hash or want cached.** Use `Deterministic(true)`, or a slice that keeps the order.
- **Invalid model JSON stored as raw JSON.** Store it as a string, or the save fails on exactly the run you most need to keep.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch06/openrouter -run 'TestGenerateSends|TestToolsOnTheWire' -v
```

1. In `ch06/openrouter/wire.go`, change `MaxTokens`'s tag from `omitzero` to `omitempty` and run the tests again: the request the client sends no longer matches the recorded one.
2. Remove the `json` tag from a field of `chatMessage` and see which test fails, and how.
3. Encode a `map[string]int` twice with and without `json.Deterministic(true)` and compare the output.
