---
title: "Errors in a Go agent: wrapping, sentinel families, errors.AsType, and what the model reads"
short: Errors in an agent
description: Error handling in an AI agent written in Go. Wrapping with what each function was doing, sentinel errors grouped into families, error types that carry data and behavior, errors.AsType, errors.Join, the errors a model reads and fixes versus the ones that stop the run, panics recovered at the tool boundary, and the common mistakes.
card: Errors for two readers: the program, which decides what to do, and the model, which fixes its next call.
group: Control and failure
chapters: 2, 5, 7, 9, 12
stage: all
level: intermediate
date: 2026-10-19
weight: 20
art: go-errors
---

Most Go programs have one reader for their errors: the code that called them, and in the end the person who runs the program.
An agent has two.
A model call that failed, a budget that ran out, or a cancelled run goes to the program, which decides whether to retry, stop, or report.
A tool call with bad arguments, or one that names a tool that doesn't exist, goes to the model, as the tool's result, and the model fixes its next call.
Getting each error to the right reader, in a form that reader can act on, is most of the job.

The functions below are standard Go, several of them recent: `errors.Join` and wrapping more than one error with `fmt.Errorf` came in Go 1.20, and `errors.AsType` in Go 1.26.
All the code is from the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), where it is tested.

## Wrap with what you were doing

When a function returns an error from something it called, it adds what it was doing, with `%w` so the original stays inspectable:

```go include=ch06/openrouter/client.go lines=86..107
```

Each layer adds a few words, and the message reads as a path from the outside in.
In a test whose fake transport never answers, the call ends with this error, every part of it added by a different layer:

```text
send request: context deadline exceeded: Post "https://openrouter.ai/api/v1/chat/completions": blocked: openrouter: model call timed out
```

`send request` is this function, `Post "…"` is `net/http`, `blocked` is the fake transport, and the rest is the cancellation and its cause, joined as the [context page](/go/context/#say-why-it-stopped) describes.
The conventions keep that path readable: lowercase, no trailing punctuation, no "failed to" or "error:" prefixes, and the name of what was being done, not of the function.

Use `%w` when callers may need the cause, and `%v` to deliberately hide it, at a boundary where the inner error is an implementation detail callers shouldn't depend on.
Handle each error once: either log it or return it.
Code that does both prints the same failure at every layer it passes through.

## Errors callers check: sentinels in families

Conditions a caller needs to recognize are exported sentinel errors, values compared with `errors.Is`.
An agent has enough of them that grouping helps, so a caller can ask about a family without listing its members:

```go include=ch12/agent/errors.go tag=tree
```

Each member wraps its family with `fmt.Errorf("%w: ...")`, so `errors.Is(err, ErrBudget)` is true for both the turn limit and the cost budget, and `errors.Is(err, ErrMaxTurns)` still tells them apart.
A caller that just wants to know "did a limit I set stop this?" checks the family; one that reports which limit checks the member.

## Errors that carry data: types and behavior

Some failures need more than a name.
An API error has a status, the API's own message, and sometimes a delay the server asked for.
Those are a type, with methods that answer the questions callers ask:

```go include=ch06/openrouter/errors.go tag=api-error
```

`errors.AsType` finds the first error in a chain with a given type and returns it, typed: `e, ok := errors.AsType[*APIError](err)`.
The type in square brackets (a [type parameter](/glossary/#type-parameter)) says which kind of error to look for; `AsType` returns it already converted to that type, with `ok` false if the chain has none.
It is the generic form of `errors.As`, added in Go 1.26; it needs no variable declared in advance, and it can't be handed a pointer of the wrong shape, a common mistake with `errors.As`.

The retry code goes one step further and asks about behavior, not type.
It defines small interfaces for what an error can report about itself.
Each interface names one method; any error that has a `Retryable` method counts, whatever its type.


```go include=ch12/agent/classify.go tag=reports
```

and checks for them with the same function, given an interface instead of a concrete type:

```go include=ch12/agent/classify.go tag=classify
```

Any error that has a `Retryable` method, from either provider's client or from the retry code itself, is classified the same way, and the retry code imports none of those packages.
The order of the cases matters: a cancelled run is never retried, whatever the error says, and an error that reports itself is believed before the guesswork of timeouts and network errors.

One type uses that behavior to prevent a mistake.
A call retried until the policy gave up returns a `*RetryError`, which says it is not retryable, so a retry loop further out doesn't start the whole series again:

```go include=ch12/agent/retry.go tag=retry-error
```

Here is the whole chain chapter 12's code returns when a test server answers every model call with a 429, under a retry policy of three attempts.
Step through how it is built, and what each of the two searches finds:

{{errchain}}

## Errors the model reads

Most tool errors don't stop the run.
Such an error becomes the tool's result, marked as an error, and the model reads it on its next turn.
The text has to be written for that reader: what was wrong, and how to make the call correctly.
The agent keeps the text for the model separate from the kind of error the program checks:

```go include=ch12/agent/errors.go tag=tool-errors
```

`Error()` returns only the message, so the model reads something like `no tool named "read"; the tools are edit_file, list_files, read_file, run_tests`, not a chain of package prefixes, while `Unwrap` still lets the program count unknown-tool errors with `errors.Is`.
Here is that error being made, with the list of real tools the model needs to fix its call:

```go include=ch12/agent/loop.go tag=lookup
```

Arguments that don't decode get the same treatment: the model reads a short message saying which part of its arguments was wrong, followed by the shape of a correct call.

<div class="hood">

One more detail: `encoding/json/v2` deliberately varies the wording of its error messages between runs, so the agent writes its own stable message from the error's fields, through `errors.AsType`:

```go include=ch06/agent/tools.go tag=decode
```

</div>

Stable wording matters twice here: a model reads it, and [recorded runs used as test fixtures](/production/testing-offline/) would otherwise change from one run to the next.

## Errors that stop the run

Some tool failures can't be fixed by a better call: the workspace the tool works in is gone, or the server behind it died.
Sending those to the model just spends turns.
They are marked with a sentinel of their own:

```go include=ch12/agent/errors.go tag=tool-fatal
```

The code that runs a tool sorts every result into one of three outcomes: an answer, an error for the model, or an error that also stops the run:

```go include=ch12/agent/runner.go tag=run-tool
```

Even a fatal error produces a tool message, because every tool call in a conversation needs an answer; the run then stops with the error, wrapped with the tool's name.

The second case in the listing, `ErrOutcomeUnknown`, is for a tool that may have changed something outside the run, such as sending a request, and then stopped without saying whether it did, for example by timing out.
Running it again could do the thing twice, so the run stops instead; [Durable runs](/production/durable-runs/) shows how a resumed run settles the question.

## Panics at the tool boundary

A panic is for a programmer's mistake, not for a failure the program expects.
But a tool is code too, and a bug in one tool shouldn't crash an agent partway through a long run.
The tool boundary is the one place the agent recovers, and it turns the panic into an error the model reads:

```go include=ch12/agent/loop.go tag=run-safely
```

Recovering anywhere else would hide bugs; recovering here keeps one broken tool from losing the run, and the error still names the tool and the panic, for the person reading the log.

## Several failures at once: errors.Join

Sometimes two things fail and both matter: a write fails, and putting the files back fails too.
`errors.Join` returns one error that holds both, and `errors.Is` and `errors.AsType` search every branch.
The excerpt is the end of `apply`, a function in the companion code that writes several files into a tree and puts the originals back if any write, or the build afterward, fails:

```go include=ch12/groups/merge.go tag=apply-write
```

[Parallel tool calls](/build/parallel-tool-calls/) are the other common case: when several calls in one reply can't run at all, the runner joins their errors, so the run reports every one instead of whichever finished first.

The retry code has to look inside joined errors too, when it checks whether any error in a chain is a timeout.

<div class="hood">

A joined error's `Unwrap` returns a slice, `Unwrap() []error`, so code that walks a chain by hand has to follow both shapes.
The retry code does, because it looks for any error in the chain with a `Timeout` method that returns true, which `errors.AsType` can't express, since it stops at the first error in the chain that has the method, whatever that method returns:

```go include=ch12/agent/classify.go tag=timed-out
```

</div>

## Mistakes to avoid

- **Comparing error strings.** Use `errors.Is` and `errors.AsType`; message text changes between versions and, as with `encoding/json/v2`, sometimes between runs. The retry code makes one exception: `net/http` reports an HTTP/2 stream reset and a closed idle connection with errors it doesn't export, so the only way to recognize them is their text.
- **A nil pointer returned as an error.** A function that returns `error` and ends with `var e *APIError; return e` returns a non-nil error holding a nil pointer, and `err != nil` is true. Return `nil` itself.
- **Logging and returning.** The same failure appears at every layer. Pick one.
- **`%w` at a boundary you don't want to commit to.** Wrapping makes the inner error part of your API: callers will check for it. Use `%v` where it is an implementation detail.
- **A Go error string sent to the model as is.** "json: cannot unmarshal string into Go struct field" tells a model little. Say what was wrong and what a right call looks like.
- **Recovering panics everywhere.** Recover at the tool boundary, where a failure has somewhere useful to go, and let real bugs elsewhere crash in tests.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch12/agent -run 'TestClassify|TestToolPanic|TestToolFatal' -v
```

1. In `TestClassify`, add a case for an error that wraps a `*RetryError` in another `fmt.Errorf`, and check that it still isn't retried.
2. Make a tool panic on purpose and read the tool message the model would get. `TestToolPanic` in `ch12/agent/final_test.go` already does it: a tool that panics with `"boom"`, run through the loop by a scripted model. Add `t.Log(got.Content)` after the line `got := res.Messages[2]` and run `go test ./ch12/agent -run TestToolPanic -v`. You should see `echo failed with an internal error (boom); try another way, or report it`.
3. Change `toolError.Error` to return `e.kind.Error() + ": " + e.msg`, and look at what the model would read for an unknown tool.

Undo your edits with `git restore .` in the `code` directory.
