---
title: "Why did the agent stop? Limits, budgets, and errors a caller can check"
short: Why the agent stopped
description: A production agent loop names every way a run can end. API errors become a type that says whether to retry, sentinels form a tree checked with errors.Is, a dollar budget is checked before each call, replies are checked before they are kept, and cancellation still answers every tool call.
card: Naming every way a run can end: typed errors, budgets, reply checks, and cancellation.
group: Reliability
chapters: 5
stage: 4
date: 2026-10-18
work: true
weight: 10
art: ch05
---

A basic agent loop handles the happy path, where every request succeeds and every reply makes sense.
Production runs are different.
Requests fail for ordinary reasons: a mistyped key, an empty account, an overloaded provider.
Some replies arrive broken, with two tool calls under one ID or with the text truncated at the token limit.
A run that is never told to stop will keep paying for calls, and sooner or later someone hits Ctrl-C in the middle of a test run.

The rest of this page takes these cases one at a time.
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
| Cancelled, or a deadline | `errors.Is(err, context.Canceled)`, `context.DeadlineExceeded` | If a person stopped it, nothing; [`context.Cause`](/go/context/#say-why-it-stopped) gives the reason |
| A bad reply | `errors.Is(err, agent.ErrBadReply)` | Run it again; switch models if it repeats |
| A model call timed out | `errors.Is(err, openrouter.ErrCallTimeout)` | Run it again; slow providers usually recover |
| An API error | `errors.AsType[*openrouter.APIError](err)` | Retry when `Retryable()` says so; otherwise fix the key, the credit, or the request |
| Anything else | None of the checks above match | Read the error, which is usually a dropped connection or a reply that failed to decode |

```diagram
name: turn-flow
alt: A decision flow: if ctx is done, stop with the cancellation and its cause; if the cost so far has reached the budget, stop with ErrBudgetExceeded; call the model, adding its usage even if the call failed, and stop on an error; check the reply, stopping with ErrBadReply if it is cut off, filtered, failed, or has a missing or repeated ID; append it; if it has no tool calls, it is the answer; otherwise run the tools, answering every call even if cancelled, and go to the next turn, or stop with ErrMaxTurns after the last allowed one.

One turn of a hardened loop: every check, and the reason each exit reports.
```

## An error tree

A caller rarely needs to know exactly which limit fired.
It needs to know what kind of problem it has.
A [sentinel error](/glossary/#sentinel-error) is an error value declared once, at package level, so callers can compare against it with `errors.Is`.
The sentinels are grouped: each specific error wraps a family error with `%w`.

```go include=ch05/agent/errors.go tag=tree
```

With that tree, `errors.Is(err, ErrBudget)` matches every budget stop, and `errors.Is(err, ErrMaxTurns)` matches only the turn limit.
The checks keep working after more wrapping, because `errors.Is` walks the full chain; a caller that returns `fmt.Errorf("run agent: %w", err)` breaks nothing.
When an error has to carry data, it is a type, and callers pull it out with [`errors.AsType`](/go/errors/) (new in Go 1.26).
Its form is `errors.AsType[*openrouter.APIError](err)`: it is a generic function, so the type you want goes in square brackets (see [type parameters](/glossary/#type-parameter)), and it returns that value and whether the chain held one.

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

The client reports a failed call as an `*APIError`.
It holds the numeric status, the message from the API, the `Retry-After` delay if the server sent one, and a `Retryable` method that encodes the right-hand column above.
Matching "429" inside an error string stops working as soon as someone rewords the message; a method on a type keeps working.
`Retryable` can cover one more case, OpenRouter's temporary in-flight 402, so the decision lives in one method you can extend.
`openai-go` and `anthropic-sdk-go` take the same approach with `*openai.Error` and `*anthropic.Error`.
This loop only classifies; the [retrying, with backoff and jitter](/production/retries-and-observability/), is a separate step.

## A budget in dollars, checked before each call

A turn limit caps how many times the loop goes around.
A cost budget caps what you actually care about, the money:

```go include=ch05/agent/loop.go tag=before-turn stages=all:4
```

The check runs at the top of each turn, since nobody knows what a call costs until it returns.
That choice has two effects, and both are wanted here.
First, the overshoot is bounded: a run can exceed its budget by one call at most.
Second, if the call that crosses the line is the final answer, the run succeeds anyway, because you have already paid for that answer and discarding it would waste the money.

Every reply's usage counts toward the total, including replies the loop rejects and calls that failed after the provider charged for them, when the error comes back with the usage it was billed.
Whatever happens, `Run` gives back the conversation, the spend, and the reason it ended, so a run cut short by a limit is still there to read and save.

You might ask why the harness tracks spending when the provider already shows a balance.
The balance covers everything on the account.
A per-run total lets you stop one run and leave the rest of the account alone.
The total depends on the usage numbers being right, though.
In recorded live runs with a very small `MaxTokens`, `openai/gpt-5.6-luna` reported no tokens and no cost for the replies that hit that limit.
Check the account's spending separately to audit the budget.

## Check a reply before it joins the conversation

Every reply goes through two checks before the loop appends it:

- **The finish reason.** A reply that ended at the length limit, was blocked by a filter, or was broken off by the provider can't count as an answer. Its final tool call might even stop in the middle of its arguments.
- **The tool-call IDs.** Results are paired with calls by ID, so each call must have one, and an ID can't appear twice anywhere in the conversation.

When either check fails, the loop returns an error in the `ErrBadReply` family and leaves the conversation untouched.
The order matters.
If the loop appended the reply first and refused it afterwards, the conversation would hold a tool call that never got a result, and the API would reject every later request in that run.

In live use these failures almost never show up.
In recorded runs without a token limit, 686 replies contained 617 tool calls, and none of them hit any of these problems.
That rarity is the reason to test them.
A long-running agent will eventually get such a reply, and handling code that never runs is where the bugs hide.
Without the check, a duplicate ID is hard to trace: the tools run normally, and only the following request fails, with a 400 that looks like your client's fault.

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

## Picking the limits

Both limits need numbers, and recorded runs give a way to choose them.

```result
figure: 18 → 4
unit: median turns
task: the `duration` bug fix, with an `edit_file` tool that failed on every call; two wordings of its error
models: `anthropic/claude-haiku-5.5`, `openai/gpt-5.6-luna`, `z-ai/glm-5.3-flash`, `deepseek/deepseek-v4.1-flash`
runs: three per model and wording, with a 20-turn limit
date: October 2026
source: Chapter 4, and chapter 5's advice on limits
link: /findings/measured-mistakes/#locked
more: The finding

With an error that said the file was locked, three of the four models kept going for a median of 18 to 20 turns, and one stopped only because the limit ended its runs. Reworded to say the change couldn't be made and to report the fix instead, the same error let every model finish in four or five turns.
```

In 40 healthy runs of this fix, every one finished within six turns.
A limit of 10 leaves those runs room and cuts the stuck ones off at about half the cost they ran up at 20 turns, so `-max-turns 10` fits this task even though the program defaults to 20 to leave room for harder ones.
The rule generalizes: run the task a few times, note the turns a good run takes, and set the limit a little above that.
Set the dollar budget from what a run is worth to you, not from what it usually costs.

## Steps for your own loop

1. Give your client a typed error with the status, the server's delay, and a `Retryable` method, and have every caller ask the type.
2. Group your sentinels into families with `%w`, so a caller can test for a whole class of stop.
3. Add a turn limit and a dollar budget to every run, checked before each model call, and count usage from every reply, refused ones included.
4. Check the finish reason and the tool-call IDs before a reply joins the conversation.
5. Answer every tool call of a reply, with a "cancelled" result for the ones a cancellation kept from running.
6. Return the conversation and the spend with every error, so a stopped run can be read and saved.
7. Write one [replayed fixture](/production/testing-offline/) per stop reason, and a table test that checks each error's family.

## When to use it

For this page, "it" is a loop that names its stop reasons and enforces limits.

```fit
reach: Any run you won't watch. With a tool that kept failing, one model stopped only at the turn limit, and three used a median of 18 to 20 turns on a fix that healthy runs finished in six or fewer.
reach: Replies you can't check by eye. A repeated tool-call ID passes unnoticed until the next request fails with a 400, so the loop refuses the reply before appending it.
leave: Turning a tool's failure into a stop. An unknown tool name or bad arguments go back to the model as a tool message, and the run continues.
leave: The run's own total as your only check on spending. A model reported zero cost for replies cut off by a 16-token limit, so audit the account's spending too.
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), with no key set, replay the failures:

```console
$ go run ./ch05 -replay dup-id
$ go run ./ch05 -replay status-429
$ go run ./ch05 -replay truncated
$ go run ./ch05 -max-cost 0.0002
$ go run ./ch05 -replay circles -max-turns 4
```

Near the end of each run's output, a `stopped:` line names the stop reason.
The duplicate-ID replay prints `stopped: a reply the loop can't use` along with `bad tool call: the ID "call_dupid_1" is already in use`; the 429 prints `API error 429, worth retrying`; the budget run prints `stopped: the cost budget` and how far past $0.0002 its last call went; and `circles`, a saved run that runs the tests every turn and never edits anything, prints `stopped: the turn limit`.
To run the tests for every saved failure, use `go test ./ch05/...`.
