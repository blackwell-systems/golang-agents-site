---
title: "Interfaces and generics in a Go agent: small interfaces, wrappers, and typed tools"
short: Interfaces and generics
description: How a Go agent's parts fit together with interfaces and generics. A one-method Model interface with four implementations, interfaces declared where they are used, adapters between a whole reply and a stream, a function type that satisfies an interface, wrappers for retries and tracing, a slice type that is a policy, optional behavior found with a type assertion, compile-time checks, and generic tools whose schema comes from a struct.
card: The shapes the rest of the code is built from: one-method interfaces, wrappers, and tools typed by their arguments.
group: How the parts fit
chapters: 1, 6, 7, 8, 12, 15
stage: all
level: intermediate
date: 2026-10-26
weight: 5
art: go-interfaces
---

An agent's code is a few small parts that other code plugs into: a model it calls, tools it runs, and a policy that decides what each request holds.
Go gives you two ways to make a part pluggable.
An interface says what a value can do, and any type that can do it fits, with no declaration that it does.
A generic function works for many types at once, and the compiler checks each use.
This page shows both in the companion code, from the one-method interface the loop calls to tools whose JSON schema is generated from a Go struct.

## One method: the Model interface

All the loop asks of a model is a reply to the conversation so far.
That is the whole interface:

```go include=ch12/agent/types.go tag=model
```

A type satisfies `Model` by having a `Generate` method with that signature, and nothing else.
This is how every interface in Go works, and it is called implicit satisfaction: a type never declares which interfaces it implements, as a class does with `implements` in Java or C#, and the compiler checks the methods wherever the type is used as the interface.
So a type can satisfy an interface written after it, in a package it has never heard of.
In chapter 12's code, four types produce replies that way, and the wrappers later on this page satisfy it too:

- `*agent.Fake`, which replays a script of replies, for tests and for runs with no API key;
- `*openrouter.Client`, the hand-written client for OpenRouter's API;
- `*anthropic.Client`, the same for Anthropic's API;
- `*orsdk.Client`, an adapter that puts OpenRouter's official Go SDK behind the same method.

None of them mentions `Model`.
The loop takes a `Model` and never learns which one it has, so a test can run the real loop with no network:

```go include=ch12/agent/fake.go tag=fake
```

The fake is short because the interface asks for one method.
An interface with ten methods would need ten in every fake, and every new client would have ten chances to get something wrong.
Keep interfaces small enough that writing a second implementation is easy; the standard library's `io.Reader` and `io.Writer`, with one method each, are the usual examples.

## Declare an interface where it is used

An interface belongs to the code that calls it, and it can list exactly what that code needs.
The program in chapter 12's `main.go` needs a client that can do two things, return a whole reply for a compaction summary and stream a reply for the loop, so it declares that, by embedding the two interfaces the `agent` package defines:

```go include=ch12/main.go lines=339..344
```

The lowercase name keeps it private to `main`: nothing else needs it.
Both hand-written clients satisfy it, because each has both methods.
The SDK adapter has only `Generate`, so it isn't a `model`, and the compiler says so if you pass one where a `model` is wanted.

The same file declares an interface for errors:

```go include=ch12/main.go lines=452..457
```

The OpenRouter client's `*APIError` and the Anthropic client's are different types in different packages.
Both have a `Retryable` method, so one check finds either: `errors.AsType[retryable](err)` searches the chain for any error with that method, and [Errors in an agent](/go/errors/) shows how the chain is searched.
Asking for behavior instead of a concrete type is what lets the check work for a client the code doesn't import.

## Adapters: from a whole reply to a stream and back

The streaming loop calls a second interface, from [Streams in Go](/go/streams/):

```go include=ch12/agent/stream.go tag=stream-model
```

A scripted fake returns whole replies, and the SDK adapter can't stream at all.
`Streamed` turns any `Model` into a `StreamModel` whose stream holds the whole reply at once:

```go include=ch12/agent/stream.go tag=streamed
```

`Generator` goes the other way, collecting a stream into one reply for calls that don't stream:

```go include=ch12/agent/middleware.go lines=60..70
```

Both follow the same pattern: an unexported struct that holds the value it adapts, a method that satisfies the other interface, and an exported function that returns the interface.
The usual Go advice is to accept interfaces and return concrete types, and the code mostly does.
An adapter is the exception: its struct is an implementation detail with nothing for a caller to use, so returning the interface keeps it out of the package's API.

## A function that is an interface

Some implementations are a single function.
Go lets a function type have methods, so a function can satisfy an interface, as `http.HandlerFunc` does for `http.Handler`:

```go include=ch12/agent/middleware.go tag=stream-func
```

`StreamFunc(f)` is a conversion, not a call: it gives the function `f` the type `StreamFunc`, whose `Stream` method calls `f`.
With it, code that needs a `StreamModel` can write a closure instead of declaring a struct.

## Wrappers: retries, spans, and logs around a model

A wrapper takes a model and returns another that does something before and after each call: retry it, trace it, log it, count its tokens.
The agent package names the shape of a wrapper with two function types:

```go include=ch12/agent/middleware.go tag=middleware
```

`WrapModel` applies them from the last to the first, so the first wrapper in the list is the outermost: a call passes through `ws[0]` first.
Here is one, `Traced`, which records a span for each model call; it returns a closure through `StreamFunc`, and the closure calls the model it wraps:

```go include=ch12/agent/traced.go tag=traced
```

Chapter 12's program stacks five of them, and the order is part of the design: the span for the whole call goes outside the retries, so one span covers every attempt, and the meter goes inside them, so a failed attempt's tokens are counted too.

```go include=ch12/reliable.go tag=wrap-order
```

The last one, `w.meter.Wrap`, is a method value.
`Meter.Wrap` takes a `StreamModel` and returns one, so `w.meter.Wrap`, the method bound to `w.meter`, has exactly the type `ModelWare` and goes in the list with no adapter.

Tools are wrapped the same way, but a `Tool` is a struct, not an interface:

```go include=ch12/agent/types.go tag=tool
```

That is deliberate.
A wrapper that retries a tool has to know whether running it twice is safe, which is what `Idempotent` says, and a wrapper that applies a time limit needs `Timeout`.
An interface with only a `Run` method would hide both.
So a `ToolWare` takes the whole struct and returns it with `Run` replaced, keeping the definition and the flags:

```go include=ch12/agent/tooltime.go tag=timeouts
```

## Several implementations, and a slice that is one

What each request holds is decided by a `ContextPolicy`, from [Context management](/build/context-management/):

```go include=ch12/agent/policy.go tag=policy
```

`Trim`, `ClearResults`, `Compact`, and `CacheMarks` each implement it, and a run takes whichever it is given.
To use several, `Chain` returns a slice of policies that is itself a policy:

```go include=ch12/agent/policy.go tag=chain
```

`chain` is a named slice type with a `Build` method, so it satisfies `ContextPolicy`, and the loop calls it like any other policy.
Its method runs each policy in turn on what the one before built.

Sometimes code needs to know which concrete types an interface holds.
`Chain` refuses to hold two policies that both shorten the conversation, and finds them with a type switch:

```go include=ch12/agent/policy.go lines=75..85
```

Each type appears twice, as a value and as a pointer.
`Trim`'s method has a value receiver, `func (t Trim) Build`, and a method with a value receiver belongs to both `Trim` and `*Trim`, so a caller could pass either and the switch has to catch both.
A method with a pointer receiver belongs to the pointer only, which is why `*agent.Fake` is a `Model` and `agent.Fake` is not.

## Optional behavior: ask with a type assertion

A function can take a small interface and still use more when a value offers it.
Chapter 15's audit log writes each entry to an `io.Writer`; when that writer is a file, it also asks the operating system to flush the entry to disk, so a crash can't lose it:

```go include=ch15/safety/audit.go lines=79..83
```

`a.W.(interface{ Sync() error })` asks whether the value in `a.W` also has a `Sync` method.
An `*os.File` does, so the entry is synced; the `bytes.Buffer` a test passes doesn't, and the check is skipped.
The interface type is written inline, with no name, because nothing else needs it.
The standard library works the same way: `io.Copy` checks whether its source is an `io.WriterTo`.

## Checking at compile time

Go checks that a type satisfies an interface only where the type is used as one.
To check it earlier, assign a value to a blank variable of the interface type:

```go include=ch12/main_test.go lines=38..38
```

The line does nothing when it runs.
If `CacheMarks` ever loses its `Build` method, or the method's signature changes, the code stops compiling here, with an error that names the missing method, instead of somewhere far from the cause.

## Generics: one function for every tool's arguments

Each tool decodes its own JSON arguments into its own Go type.
Without generics, every tool would repeat the decoding and the error the model reads when it fails.
With a type parameter, one function does it for every type:

```go include=ch12/agent/tools.go lines=40..51
```

`T` is a type parameter: the caller picks it, as in `decodeArgs[readArgs](args, example)`, and the compiler makes a version of the function where `T` is that type.
`any` is the constraint, the set of types `T` may be; `any` allows every type, which is all this function needs, since it only declares a `T` and passes its address to `json.Unmarshal`.

Chapter 7 goes further: the tool's whole definition comes from its argument type.
A tool's arguments are a struct, with a tag for each field's description:

```go include=ch12/triage/codetools.go tag=read-args
```

`NewTool` takes the tool's Go function and generates the JSON schema the model sees from the struct, so the schema and the code that reads the arguments can't drift apart:

```go include=ch12/agent/registry.go tag=new-tool
```

Three things happen in it.
`reflect.TypeFor[A]()` gets the struct's type, which is how the schema is built at run time from the fields and their tags.
The function the caller passes takes an `A`, so inside it `a.Path` is a string and `a.Start` an int, checked by the compiler, with no type assertion.
And the `Tool` it returns isn't generic: its `Run` takes raw JSON, and the closure decodes it into an `A` before calling the typed function.

That last point is what makes it work.
The loop holds its tools in a `[]Tool`, and a slice holds one type.
If `Tool` were generic, `Tool[readArgs]` and `Tool[searchArgs]` would be different types and couldn't share the slice.
So the type parameter lives only in the constructor, and the closure carries the type inside.

You rarely write the type argument yourself.
In `agent.NewTool("code_read", "...", func(_ context.Context, a readArgs) (string, error) {...})`, Go infers `A` from the function's parameter.
For the `read` tool in the package's tests, with a required `path` and an optional `start`, the generated schema is this, checked byte for byte by `TestSchema`:

```json
{"type":"object","properties":{"path":{"type":"string","description":"a file path, relative to the project"},"start":{"type":"integer","description":"the first line"}},"required":["path"],"additionalProperties":false}
```

The `omitzero` on `start` is what leaves it out of `required`.

## Constraints that say more than any

A constraint is an interface, and it can list types or methods.
The code that summarizes the [recorded experiments](/findings/) takes the median of ints in one place and of float64s in another, with one function:

```go include=evidence/summary.go lines=356..365
```

`int | float64` is a union: `T` may be either, and since both support `+`, `/`, and ordering, the body may use them.
With `any`, the compiler would reject `s[i] + s[j]`, because not every type can be added.

A constraint can also require a method, written inline:

```go include=evidence/experiments.go lines=97..111
```

Each kind of saved run is its own struct type, and each has a `model` method that says which model it recorded.
`next` loads any of them and counts the runs for one model, calling `r.model()` on each.

Reach for a type parameter when the function's types matter to its caller, as in `decodeArgs`, which returns a `T`, or `median`, which returns the type it was given.
When a function only calls methods on a value and returns nothing of its type, an interface parameter says the same thing more simply.

## Mistakes to avoid

- **A large interface written before its users.** Declare an interface where it is called, with the methods that code needs; the implementer exports a concrete type.
- **An interface for one type that will never have a second.** A test fake counts as a second, and `Fake` is why `Model` exists; a type no one will ever substitute doesn't need one.
- **A nil pointer in an interface.** An interface holding a nil `*T` is not nil, so `err != nil` is true. [Errors in an agent](/go/errors/#mistakes-to-avoid) shows the case.
- **A pointer receiver, then a value where the interface is wanted.** `*Fake` has the method and `Fake` doesn't, so passing `agent.Fake{}` as a `Model` fails to compile. Pass `&agent.Fake{}`.
- **A generic type where the values must share a slice.** `Tool[A]` for each `A` can't go in one `[]Tool`; keep the type parameter in a constructor, as `NewTool` does.
- **Wrappers in the wrong order.** In `WrapModel`, the first wrapper is the outermost. A span placed inside the retries records each attempt; placed outside them, it records the whole call once.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch12/agent -run 'TestSchema$|TestDecodeStrict|TestGenerator$|TestChainShortening|TestTimeouts' -v
```

1. In `TestSchema` in `ch12/agent/registry_test.go`, add a field `End int` with tags `json:"end,omitzero" desc:"the last line"` to `readArgs`, and run the test. It fails and prints the new schema, with `end` in `properties` and not in `required`.
2. Change the `Fake`'s `Generate` in `ch12/agent/fake.go` to have a value receiver, `func (f Fake)`, and run `go vet ./ch12/...`. Every `&agent.Fake{}` still compiles, since a value receiver belongs to the pointer too; the methods of `Fake` are now in both method sets.
3. In `ch12/main_test.go`, add `var _ agent.StreamModel = &agent.Fake{}` and run `go vet ./ch12`. The compiler refuses it and names the missing `Stream` method; `agent.Streamed(&agent.Fake{})` is the value that fits.

Undo your edits with `git restore .` in the `code` directory.
