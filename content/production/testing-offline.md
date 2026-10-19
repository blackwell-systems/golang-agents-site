---
title: Testing an AI agent offline with recorded replies
short: Testing an agent offline
description: An agent's tests should never call a live model. Use a scripted model for the loop's rules, an http.RoundTripper that replays recorded API calls for the client and tools, hand-written fixtures for failures you can't trigger, outcome checks in place of transcript checks, and testing/synctest for timeouts.
card: Testing an agent without a live model: scripted models, replayed HTTP, and outcome checks.
chapters: 1, 2, 4, 5
stage: all
date: 2026-10-18
work: true
weight: 30
art: ch05
---

Live models make poor test dependencies.
The same prompt can produce a different reply on every call.
Every call is billed and takes seconds.
Providers also retire models, so a test written against one of them stops working when it goes away.
`go test` should depend on none of that.
In the book's companion code, all tests run without a network, and each example program falls back to recorded replies when it finds no API key.
This page walks through how that works, using [`ch02/replay`](code:ch02/replay) and the tests that use it.

## Three kinds of model and what each one shows

| Kind | What it tests | What it can't |
| --- | --- | --- |
| A scripted model | The loop's rules, exactly and for free | Anything about choosing tools |
| Recorded replies | The client and the tools against real JSON, real token counts, and real mistakes, with no network | How a model behaves on a new input |
| Live runs | How a model actually behaves | Anything exact: they vary and cost money |

The test suite uses only the first two rows.
The third row belongs to [evals](/production/evals/), where you deliberately run a real model many times and score the runs as a pass rate.

## A scripted model

The loop only knows a model through one method, `Generate(ctx, req)`.
Chapter 1's `Fake` implements it with a list of canned replies:

```go include=ch01/agent/fake.go tag=fake
```

To choose which reply comes next, `Fake` counts how many assistant messages the request already holds.
A real model also sees nothing but the request, so this keeps the fake honest.
It also turns the fake into a check on the loop: drop a message or put two in the wrong order, and the fake answers with the wrong reply, so the test fails.
That is enough to pin down each rule of the loop with its own test.
An answer ends the run.
So does reaching the turn limit.
Each tool call gets one tool message, never zero or two.
A failing tool's result goes back flagged as an error.

## Replaying recorded calls

The scripted model sits above the HTTP client, so it never exercises the client.
To test the client, the companion code replaces what sits below it.
In Go, every HTTP request passes through an `http.RoundTripper`.
Hand `http.Client` a round tripper that reads its responses from files, and the client behaves exactly as it would on the network:

```go include=ch02/replay/replay.go tag=transport
```

Recorded calls belong to scripts, and each script is a numbered series of files such as `fix-01.json` and `fix-02.json`.
A single client can serve calls from several scripts, which rules out storing the script name on the transport.
The program puts the name in the request's context instead, under a key of an unexported type so that no other package can use the same key.
This is exactly the case context values exist for: a value scoped to one request that has to pass from the program, through the book's client and `http.Client`, down to the transport.

The transport keeps the `RoundTripper` rules.
It closes the request body, leaves the request unmodified, and returns an error for a request whose context is already cancelled, just as a real network round trip would.
When a test makes more calls than a script holds, the transport returns an error that names both the call and the script.
An unexpected extra call then fails the test immediately; it can't hang and it can't go out to the network.

## Recording a live run

For capturing new recordings, the package also has a recording transport.
It passes each request through to the real API and writes three things to disk: the request body, the response status, and the response body.
Headers are never saved, so the key can't end up in a recording.
You name the run and pick a directory, run it once with a key, and copy the files into `testdata/replies`.
From then on, with no key set, the program replays that run while its tools execute for real.

## Fixtures for failures you can't produce

Many failures can't be triggered on request.
There is no way to ask the API for a 503, or to get a model to reuse a tool-call ID, at the moment a test needs one.
Chapter 5 writes those saved runs by hand.
Each is a short list of turns, and each turn is either an HTTP status or a reply: some text, some tool calls, an odd finish reason, or two calls sharing an ID.
An opt-in test turns the lists into recordings.
It serves each one from an `httptest` server that speaks OpenRouter's format, points the real program at that server, and captures the traffic with the recording transport, so the saved requests match what the client really sends.
That produces a saved run for every stop reason and for every status code, and a table test replays each one and checks that the error belongs to the expected family.

## Check outcomes, not transcripts

The same task, run twice on the same model, can follow two different routes, and two models can reach the same fix in different ways.
The tests for the real agent therefore check results and ignore wording.
Chapter 4's outcome test replays every saved run and asks three questions.
Does `duration.go` hold the fix?
Did the tests run after the final edit?
Did the run end with an answer?
It doesn't look at which files the agent opened, the order it opened them in, or the text of its answer.
A live run on some other model that takes yet another route is judged by the same three questions.

That test has a gap worth noticing.
It doesn't check that the test file is unchanged, and an agent told to make failing tests pass has an easy shortcut: weaken the tests.
The book's eval grader covers this.
A run that deletes or skips a test fails its task, and separate hidden tests, which the agent never sees, check that the fix actually works.

## Timeouts without waiting

The client gives each call two minutes.
Nobody runs a test that sits idle for two minutes.
`testing/synctest`, added in Go 1.25, solves this by running the test inside a bubble that has its own fake clock.
Once all of the bubble's goroutines are durably blocked, the fake clock moves forward.
The test's transport never responds; it just waits on the request's context.
Inside the bubble, the two-minute deadline arrives immediately.
The test then checks the returned error and confirms that the fake clock moved forward by exactly two minutes, and the whole test takes a few milliseconds.

Later chapters use the same approach for streams that stop sending and for retries with backoff.
Streams get one extra test that is worth borrowing.
The network can split a response into pieces at any byte, so the test feeds one recorded stream to the reader again and again, split at every possible position, and expects the same result each time.

## What the tests cover

By the end of chapter 5, the following are all tested with no network calls:

- the request the client builds, checked against an `httptest` server
- every saved reply and every saved status
- the per-call deadline and caller cancellation, over both HTTP/1.1 and HTTP/2
- scripts that fail when a test makes one call too many
- recordings, checked to contain no API key
- the real `go test` tool on projects that pass, fail, and don't build, with the key hidden from the code under test
- every stop reason, replayed

## How rare the failures are

Chapter 5 counted how often live runs produce the replies these fixtures fake.
Across the book's evidence runs that set no token limit, 686 replies carried 617 tool calls, and none of the replies was truncated, blocked by a filter, broken off by its provider, or missing or repeating an ID.
That is why the fixtures are written by hand: a test that waits for the real thing never runs, and the handling code it should cover is the code most likely to be wrong when the day comes.

## Steps for your own agent

1. Put the model behind a one-method interface, and test the loop's rules with a scripted fake that picks its reply from the request.
2. Give your HTTP client an `http.RoundTripper` that replays saved responses by script and call number, and fails on a call the script doesn't have.
3. Record a few real runs with a recording transport that never saves headers, and replay them in every test.
4. For each failure you can't trigger on demand, write its replies by hand, serve them from `httptest`, and record them through the real client.
5. Check outcomes (the file holds the fix, the tests ran after the last edit, the run ended with an answer), never the model's wording.
6. Run anything that waits on a clock under `testing/synctest`.
7. Keep live runs out of `go test`: they belong to evals, which measure a rate on purpose.

## When to use it

```fit
reach: Every change to the harness. The loop's failure handling covers replies that live runs almost never send: none of 686 recorded replies was cut off, filtered, or carried a bad ID (chapter 5).
reach: More than one model or route. A test that checks outcomes holds for any run that fixes the bug, whichever files it read on the way (chapter 4).
leave: Measuring how good the agent is. Saved replies show your code handles a reply, not how often a model sends a good one; that takes many live runs, scored (chapters 5 and 13).
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch05/...
$ go run ./ch05 -replay dup-id
```

The first command runs every test for the hardened agent without a network or a key; the `run_tests` tool runs the real `go test`, so expect it to take a minute or two on a cold build cache.
The second replays a hand-written failure: a reply whose two tool calls share an ID.
Making a recording of your own is the one step that needs a key and spends money: chapter 4's program takes `-record` with a directory and `-replay` with a name for the script, and writes each call as a numbered file under that name.
