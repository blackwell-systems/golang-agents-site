---
title: "Retries, traces, and cost for an agent's model and tool calls"
short: Retries and observability
description: Which failures deserve a retry and which never do, full-jitter backoff measured against plain backoff, a retry budget and circuit breaker shared across agents, tool retries limited to tools that are safe to repeat, and logs and OpenTelemetry GenAI spans that show where a run's time and money went.
card: What to retry, how to back off, and traces that show where a run's time and money went.
group: Reliability
chapters: 12
stage: 2
date: 2026-10-18
work: true
weight: 20
art: ch12
---

Two failures from recorded runs set up this page.
In the first, a batch run had read 1.7 million tokens when it hit `remote error: tls: bad record MAC` at turn 41.
The harness resent the request once, right away.
The resend failed as well, and the run ended with all of its findings gone.
(How this was measured is written up in the finding [a dropped connection ended a 41-turn run](/findings/measured-mistakes/#dropped-connection).)
In the second, 23 agents were running in parallel, and that same TLS error cut off three of them in the middle of reading a reply.
The task had nothing to do with either failure, and waiting a few seconds and trying again would probably have saved both.
Afterwards there was also no record of which turns had cost the most or which calls had been slow.

This page closes both gaps with retries, timeouts, logs, traces, and cost reporting.
Each one is a wrapper around a call, the pattern called [middleware](/glossary/#middleware); the code is in [`ch12/agent`](code:ch12/agent).

## Middleware, two kinds

Each of these concerns does one thing to a call.
A retry sends it again, a timeout limits how long it may take, a span measures it, and a meter totals what it cost.
Writing all of them into the loop body would make the loop longer with every addition.
Each one is instead a wrapper function that takes a call and returns a call with one added behavior, written once and stacked with the others.

The companion code uses two wrapper types.
Model wrappers wrap the interface the loop calls to get a streamed reply from the model.
Tool wrappers receive the entire tool, including the [safety flags](/build/tool-design/) on it (`ReadOnly`, meaning it only looks things up, and `Idempotent`), and a retry needs those flags to tell whether running the tool twice is harmless.
A tool is [idempotent](/glossary/#idempotent) when a second run changes nothing the first did not.
Model calls can always be sent again; many tool calls can't, and a single shared handler type would lose that distinction.

## Which failures to retry

The short version: retry what is temporary, never what will fail the same way, and stop as soon as the caller gives up.

| Failure | Retry? |
| --- | --- |
| 408, 429, and 5xx statuses | Yes; honor [`Retry-After`](/basics/http-client/#when-the-status-isnt-200) |
| A connection reset, cut off, or ended by a TLS alert | Yes; a new connection usually works |
| A provider error inside a 200 stream | Yes |
| An attempt that ran out of its own time | Yes |
| The caller's context cancelled or past its deadline | No: stop now |
| 400, 401, 403, most 402s | No: it will fail the same way |
| A certificate the client rejects | No: a configuration problem or an attack |
| A reply cut off, filtered, or refused | No: the model finished, and would likely finish the same way |
| A tool that can't run at all (`ErrToolFatal`) | No |

Two rows in that table hide a trap.
A timeout can be one attempt's own or the whole run's, and a 402 can be a temporary limit or an empty account.
The classifier checks the caller's context first and retries only the temporary cases.
The details are below if you want them.

<div class="hood">

**Two kinds of timeout produce the same error.** Both an attempt that ran past its own deadline and a run whose caller's deadline expired end in `context.DeadlineExceeded`.
The attempt deserves another try; the expired caller means the run is over.
The classifier separates them by order: it looks at the caller's context before anything else, and it only treats a timeout as retryable when that context is still live.
Each attempt's own deadline is set with `context.WithTimeoutCause` and a cause of its own, `ErrAttemptTimeout`, which the classifier counts as a timeout and which makes the error message say which deadline fired.

**Some 402s clear up on their own.** While a paid request is running, OpenRouter reserves its estimated cost against the account, and once those reservations add up to the available credit, new requests are refused.
That refusal is a 402 whose body sets `limit_source` to `openrouter_in_flight_budget`, and that one is retryable.
Any other 402 means the credit is gone, and retrying won't help.
There is a catch: an account that really has no credit left can receive the same in-flight `limit_source`, and in that case no number of retries succeeds.
The policy's attempt cap, four by default, is what ends the loop then, and the run fails with the 402.

The classifier doesn't depend on any client package.
It asks the error three questions through small interfaces (an interface of one method that the error type satisfies without naming it): is it retryable, does it carry a retry delay, and is it a timeout.
Each client's error types answer them.

The diagram below shows one call through the retry policy.
The circuit breaker in it is explained under "Many agents, one provider" further down.

```diagram
name: ch12-do
alt: A vertical flow: the breaker's Allow, the policy's Wait hook, the attempt under its own deadline, record, then Classify. Classify leads to returning the result as it is for a success or a failure not worth retrying, to a wait and then back to the breaker for a failure worth retrying, or to a *RetryError when the retries have run out.

One model call through the retry policy: the circuit breaker, the wait, the attempt under its own deadline, then the decision to retry or stop.
```

</div>

## How long to wait

With [backoff](/glossary/#backoff), each retry waits longer than the one before.
[Jitter](/glossary/#jitter) randomizes the wait, which spreads out clients that all failed at the same moment:

```go include=ch12/agent/backoff.go tag=backoff
```

Full jitter was measured against plain backoff in a simulation that runs as an ordinary test:

```result
figure: 2.3 s
unit: until the last client was answered, with full jitter
task: a simulation in which 100 clients send at once and retry against a provider that serves 10 requests in each 100 ms window and refuses the rest with a 429
models: none; the provider is simulated
runs: one per fixed strategy, 200 random seeds per jittered one
date: deterministic; `go test ./ch12/agent -run Contention` repeats it
source: Chapter 12

Backoff without jitter took 32.7 seconds, because every retry wave arrived together. Full jitter sent the fewest requests, 411 on average against 550, and finished in 2.3 seconds on average (1.3 to 6.2 across seeds).
```

The three main strategies side by side:

| Retry strategy | Requests sent | Last client answered |
| --- | --- | --- |
| A fixed 10 ms wait | 4,600 | 0.9 s |
| Backoff from 100 ms, no jitter | 550 | 32.7 s |
| Backoff from 100 ms, full jitter | 411 (401 to 425) | 2.3 s (1.3 to 6.2) |

A fixed short wait gets everyone served quickly, at a price of 46 requests for each of the 100 clients.
Plain backoff cuts the request count sharply but is slow: every retry wave hits the provider at once, and the windows between waves sit idle.
Full jitter sends the fewest requests and is done in about two seconds.
Marc Brooker's AWS experiment, the source of full jitter, saved more requests than this.
Here a rejected request is nearly free for the provider, so the gain from jitter is mostly time.

A server can name its own wait in a `Retry-After` header.
Use that value in place of the backoff, add a small amount of jitter, and cap it.
If a server asks for ten minutes, the retries stop at once with an error that says what the server asked, which beats ten minutes with no output.

## Many agents, one provider

Run 23 agents against the same provider, and retries that make sense for each agent add up to a flood.
Two pieces of state, shared by all the agents, keep that in check:

- **A [retry budget](/glossary/#retry-budget)**, modeled on gRPC's retry throttling. Failures spend credits and successes earn them back, so when most calls are failing, nobody retries.
- **A [circuit breaker](/glossary/#circuit-breaker)**. After a run of consecutive failures it opens and fails calls immediately. After a pause it allows a single probe call, and that call shows whether the provider is back.

## Retrying a stream

There is no way to pick a stream up where it broke, so a retry starts the reply from the beginning.
Anyone watching has already seen part of the first attempt, so before the new attempt's first piece the wrapper sends a reset, and the display clears the partial reply.
Whatever usage a failed attempt did report is carried over and added to the attempt that succeeds, so budgets match the bill.
A chat completions stream reports usage in its final chunk, so a stream that broke usually reports none.
To recover that cost, the client saves each generation's ID, and OpenRouter can report the cost for that ID afterwards.
The lookup isn't immediate: in a recorded measurement, the generation took between 4.5 and 8.5 minutes to appear, and OpenRouter answered 404 until it did.

## Tools: timeouts, and retries only when safe

Each tool gets a `Timeout` field, and one wrapper enforces it for every tool.
A tool that runs out of time fails with `ErrToolTimeout` when it is read-only or idempotent, and the model receives that as an error result it can respond to.

Retrying tools needs more care than retrying model calls, because a tool can change the world.
The tool retry wrapper only repeats tools flagged as read-only or idempotent.
The difficult case is a tool with neither flag that timed out.
It might have finished its work and lost only the reply, for example by filing an issue and then timing out before saying so.
An ordinary error would encourage the model to call it again and file a second issue.
Instead the run stops with `ErrOutcomeUnknown`, which names the call.
[Durable runs](/production/durable-runs/) shows how those get resolved.

## Logs, traces, and cost

**Logs** go through `log/slog`, and each line has the same fields: the agent's name, taken from the context; the model, or the tool and its call ID; and the trace ID.
Metadata is always logged.

<div class="hood">

A structured log line is a message plus named values, so a program can filter and search it by field instead of parsing a sentence.
With `log/slog`, a logger writes each call as one line: the time, a level (`Debug`, `Info`, `Warn`, or `Error`), the message, and the key-value attributes you passed after it.
A logger prints `Info` and above unless you set a lower level.
These calls, run in a small program, print the two lines below them; the `Debug` call is dropped:

```go
log := slog.New(slog.NewTextHandler(os.Stdout, nil))
log.Info("tool call", "tool", "read_file", "call", "call_7",
	"took", 3*time.Millisecond)
log.Error("tool call", "tool", "run_tests", "err", "exit status 1")
log.Debug("not shown")
```

```text
time=2026-10-23T02:45:59.911-07:00 level=INFO msg="tool call" tool=read_file call=call_7 took=3ms
time=2026-10-23T02:45:59.911-07:00 level=ERROR msg="tool call" tool=run_tests err="exit status 1"
```

The agent's logging wrapper, `LoggedTools` in [`ch12/agent/log.go`](code:ch12/agent/log.go), passes the agent, tool, and call attributes this way.

</div>
Content is logged only when you turn it on; it is truncated and then run through the same redaction hook every tool result already passes through.

**Traces** show a run as a tree of [spans](/glossary/#span), where a span is one timed step, such as a model call or a tool call, with its children inside it.
A small recorder built on the standard library prints that tree.
The same spans can then be exported through OpenTelemetry, using the generative-AI naming conventions: `invoke_agent` for a run, `chat {model}` with token counts for a model call, and `execute_tool {name}` for a tool call.
Those conventions put all of a call's retries under one span, so each attempt becomes a child span:

```text
invoke_agent triage 37.968s
  chat anthropic/claude-haiku-5.5 1.324s
    attempt 1.324s
  execute_tool crash_list 191µs
  chat anthropic/claude-haiku-5.5 1.381s
    attempt 1.381s
  execute_tool tracker__issue_search 4ms
```

The full trace of that live run on `anthropic/claude-haiku-5.5` covers 38 seconds.
Thirty of them went to 13 model calls and 5.3 to two test runs; the remaining 28 tool calls together used less than 2.5.

**Cost** is broken down by agent and by call, and failed attempts are included, so every run can answer the question of where its money went.

## Test it without waiting for an outage

You can't count on these failures happening when you want to test them.
Recorded live runs made 209 model calls and none of them needed a retry.
The companion code therefore has a transport that causes failures deliberately: a 429, a 503, a dropped connection, or a stream reset after a chosen number of bytes.
Every behavior on this page is tested against it, and many of the tests use the fake clock from `testing/synctest`.

## Steps for your own agent

1. Write a classifier that checks the caller's context first, then asks the error whether it is retryable, then looks for a timeout, then for a dropped connection; anything else is permanent.
2. Give each attempt its own deadline with a cause you can recognize, inside a total time for all of a call's retries.
3. Back off with full jitter, use the server's `Retry-After` in its place, and give up at once when the server asks for longer than you will wait.
4. When retries run out, return an error that says it is not retryable, so no layer above starts the cycle again.
5. Share a retry budget and a circuit breaker across every agent that calls the same provider.
6. Retry a broken stream from the start, tell the display to clear the partial reply, and carry the failed attempt's usage into the total.
7. Put a timeout on every tool, retry only tools marked read-only or idempotent, and stop the run with `ErrOutcomeUnknown` when a tool that changes things times out.
8. Log metadata always and content only on request, through your redaction hook, and export spans under the generative-AI names.
9. Test all of it with a transport that injects 429s, 503s, and dropped connections, since live runs rarely produce them.

## When to use it

```fit
reach: Runs are long enough that one dropped connection is expensive. A TLS error ended a run at turn 41 after $0.077 of work, and left three groups of a fan-out unresolved.
leave: Retrying permanent errors. A 402 means the account is out of credit, as happened partway through recorded runs; retrying adds requests until someone adds funds.
leave: Retrying a tool that isn't safe to repeat. In the crash sweep, every duplicate came from repeating a filing or comment whose result was never saved; retry only tools marked safe.
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), with no key set:

```console
$ go run ./ch12 -trace -faults 503
$ go run ./ch12 -log >/dev/null
$ go test ./ch12/agent -run Contention -v
```

The first command replays the recorded run with one 503 injected; in the trace, the call's first attempt fails and the second succeeds.
The second prints the log lines and nothing else.
The third runs the backoff simulation and compares its table to a saved copy.
