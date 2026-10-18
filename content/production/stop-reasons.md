---
draft: true
title: "Why did the agent stop? Limits, budgets, and errors a caller can check"
short: Why the agent stopped
description: A production agent loop names every way a run can end. API errors become a type that says whether to retry, sentinels form a tree checked with errors.Is, a dollar budget is checked before each call, replies are checked before they are kept, and cancellation still answers every tool call.
chapters: 5
stage: 4
date: 2026-10-18
work: true
weight: 10
---

A 30-line agent loop is enough for the happy path, where every request succeeds and every reply makes sense.
Production runs are different.
Requests fail for ordinary reasons: a mistyped key, an empty account, an overloaded provider.
Some replies arrive broken, with two tool calls under one ID or with the text truncated at the token limit.
A run that is never told to stop will keep paying for calls, and sooner or later someone hits Ctrl-C in the middle of a test run.

Chapter 5 of the book goes through these cases one at a time.
When it is done, each way a run can end has its own name, its own test, and its own one-line report.
The code is in [`ch05/agent`](code:ch05/agent).

## The reasons, in one table

A [stop reason](/glossary/#stop-reason) explains why the harness ended the whole run.
A [finish reason](/glossary/#finish-reason) is narrower: it explains why a single reply ended.
These are the main stop reasons the hardened loop reports, with the check a caller writes for each and the usual response:

| Stop reason | How to check | What to do |
| --- | --- | --- |
| The model answered | `err == nil` | Use the answer, then verify what it claims |
| The turn limit | `errors.Is(err, agent.ErrMaxTurns)` | Read the saved conversation for a loop |
| The cost budget | `errors.Is(err, agent.ErrBudgetExceeded)` | Give the run more money, or give it less to do |
| Cancelled, or a deadline | `errors.Is(err, context.Canceled)`, `context.DeadlineExceeded` | If a person stopped it, nothing; `context.Cause` gives the reason |
| A bad reply | `errors.Is(err, agent.ErrBadReply)` | Run it again; switch models if it repeats |
| A model call timed out | `errors.Is(err, openrouter.ErrCallTimeout)` | Run it again; slow providers usually recover |
| An API error | `errors.AsType[*openrouter.APIError](err)` | Retry when `Retryable()` says so; otherwise fix the key, the credit, or the request |
| Anything else | None of the checks above match | Read the error, which is usually a dropped connection or a reply that failed to decode |

## An error tree

A caller rarely needs to know exactly which limit fired.
It needs to know what kind of problem it has.
So the sentinels are grouped: each specific error wraps a family error with `%w`.

```go include=ch05/agent/errors.go tag=tree
```

With that tree, `errors.Is(err, ErrBudget)` matches every budget stop, and `errors.Is(err, ErrMaxTurns)` matches only the turn limit.
The checks keep working after more wrapping, because `errors.Is` walks the full chain; a `main` that returns `fmt.Errorf("run agent: %w", err)` breaks nothing.
When an error has to carry data, it is a type, and callers pull it out with `errors.AsType` (new in Go 1.26).

Tool errors are the exception: an unknown tool name, or arguments that fail to decode, never reach the caller.
The loop turns each one into a tool message, the model sees it, and the run continues.

## API errors as a type

When a call fails, the program has one decision to make: send the request again or give up.
The HTTP status mostly settles it:

| Status | Usually means | Retry? |
| --- | --- | --- |
| 400 | Something in the request is wrong | No |
| 401 | The key is missing or wrong | No |
| 402 | The account is out of credit | No |
| 403 | Blocked by a moderation check | No |
| 408 | The request timed out | Yes |
| 429 | A rate limit | Yes, after a wait |
| 500, 502, 503 | A failure on the API's or provider's side | Yes |

The chapter's client reports a failed call as an `*APIError`.
It holds the numeric status, the message from the API, the `Retry-After` delay if the server sent one, and a `Retryable` method that encodes the right-hand column above.
Matching "429" inside an error string stops working as soon as someone rewords the message; a method on a type keeps working.
`openai-go` and `anthropic-sdk-go` take the same approach with `*openai.Error` and `*anthropic.Error`.
In the book, chapter 5 only classifies; the retrying, with backoff and jitter, is chapter 12's job.

## A budget in dollars, checked before each call

A turn limit caps how many times the loop goes around.
A cost budget caps what you actually care about, the money:

```go include=ch05/agent/loop.go tag=before-turn
```

The check runs at the top of each turn, since nobody knows what a call costs until it returns.
That choice has two effects, and the chapter wants both.
First, the overshoot is bounded: a run can exceed its budget by one call at most.
Second, if the call that crosses the line is the final answer, the run succeeds anyway, because you have already paid for that answer and discarding it would waste the money.

Every reply's usage counts toward the total, including replies the loop rejects and calls that errored after the provider charged for them.
Whatever happens, `Run` gives back the conversation, the spend, and the reason it ended, so a run cut short by a limit is still there to read and save.

You might ask why the harness tracks spending when the provider already shows a balance.
The balance covers everything on the account.
A per-run total lets you stop one run and leave the rest of the account alone.
The total depends on the usage numbers being right, though.
In the book's live evidence runs with a very small `MaxTokens`, `openai/gpt-5.6-luna` reported no tokens and no cost for the replies that hit that limit.
Check the account's spending separately to audit the budget.

## Check a reply before it joins the conversation

Every reply goes through two checks before the loop appends it:

- **The finish reason.** A reply that ended at the length limit, was blocked by a filter, or was broken off by the provider can't count as an answer. Its final tool call might even stop in the middle of its arguments.
- **The tool-call IDs.** Results are paired with calls by ID, so each call must have one, and an ID can't appear twice anywhere in the conversation.

When either check fails, the loop returns an error in the `ErrBadReply` family and leaves the conversation untouched.
The order matters.
If the loop appended the reply first and refused it afterwards, the conversation would hold a tool call that never got a result, and the API would reject every later request in that run.

In live use these failures almost never show up.
In the book's recorded runs without a token limit, 686 replies contained 617 tool calls, and none of them hit any of these problems.
That rarity is the reason to test them.
A long-running agent will eventually get such a reply, and handling code that never runs is where the bugs hide.
A duplicate ID is also hard to trace: the tools run normally, and only the following request fails, with a 400 that looks like your client's fault.

## Stopping partway through a reply

A cancellation can arrive while the loop is working through a reply's tool calls.
Say a reply asks for three calls, and Ctrl-C comes during the first.
Calls two and three should not run.
They still need tool messages, though, because every call in the conversation needs a result.
So the loop gives each of them a message that says it was cancelled and gives the reason, and runs neither.
The error `Run` returns matches `context.Canceled` and also includes the cause, for example `interrupt signal received`, so the caller can test for the first and log the second.

Tools with side effects make this harder.
Suppose a tool files an issue, and the cancellation lands after the issue exists but before the tool returns.
The tool message reports a failure even though the issue was created.
That situation is called an unknown outcome, and [durable runs](/production/durable-runs/) covers how to resolve it.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), with no key set, replay the failures:

```console
$ go run ./ch05 -replay dup-id
$ go run ./ch05 -replay status-429
$ go run ./ch05 -replay truncated
$ go run ./ch05 -max-cost 0.0002
```

Near the end of each run's output, a `stopped:` line names the stop reason.
The duplicate-ID replay prints `stopped: a reply the loop can't use` along with `bad tool call: the ID "call_dupid_1" is already in use`; the budget run prints `stopped: the cost budget`.
To run the tests for every saved failure, use `go test ./ch05/...`.
