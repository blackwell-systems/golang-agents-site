---
draft: true
title: An error that said "locked" cost a median of 18 turns; one that said "don't retry" cost 4
short: How a tool error's wording changed the turns
description: Chapter 4 made a tool fail on every call and changed only the wording of its error. Told "the file is locked", three of four models kept retrying and hunting for ways around the lock; told that retrying couldn't help, every model stopped after one attempt.
chapters: 4
stage: 3
date: 2026-10-18
weight: 20
---

```result
figure: 18 → 4
unit: median turns
task: the `duration` bug fix with an `edit_file` that failed on every call, under two wordings of its error
models: `anthropic/claude-haiku-5.5`, `openai/gpt-5.6-luna`, `z-ai/glm-5.3-flash`, `deepseek/deepseek-v4.1-flash`
runs: three per model and wording, with a 20-turn limit
date: October 2026
source: Chapter 4

A tool that always failed with “the file is locked” cost three of four models a median of 18 to 20 turns. The same failure, worded to say retrying couldn’t help and what to do instead, cost every model four or five turns.
```

When a tool call fails, the harness returns the error text to the model in a tool message, and whatever the model does next is a reaction to that text.
That makes the wording of every tool error part of how the agent behaves.
Chapter 4 of the book put a number on how much.

## What was tested

The task is the bug fix from Part 1: a small Go package, `duration`, whose tests fail because its unit table maps `'m'` to `time.Second`.
A single line needs to change.

For these recorded runs, `edit_file` was swapped for a version that returns the same error on every call: `the file is locked`.
The other tools behaved normally.
Four models each attempted the task three times in October 2026, with 20 turns allowed.

A second set of runs was identical except for the error text, which became `the project is read-only: edits are not possible in this session, so don't retry; report the fix instead`.

## The result

With "the file is locked" (medians of three runs):

| Model | Turns | Edit attempts | Hit the turn limit | Cost |
| --- | --- | --- | --- | --- |
| `anthropic/claude-haiku-5.5` | 5 | 2 | 0 of 3 | $0.0013 |
| `openai/gpt-5.6-luna` | 18 | 12 | 1 of 3 | $0.0036 |
| `z-ai/glm-5.3-flash` | 18 | 10 | 0 of 3 | $0.0048 |
| `deepseek/deepseek-v4.1-flash` | 20 | 14 | 3 of 3 | $0.0061 |

`anthropic/claude-haiku-5.5`, the default model, made two edit attempts and then gave its answer: the cause, the one-line fix, and the fact that it couldn't apply the fix.
The other three spent most of their 20 turns retrying in different forms: rephrasing the edit, rereading the file, and rerunning the tests in case something had changed.

They also went looking for a way past the lock:

- In every one of their nine runs, they tried an edit to `duration_test.go`, usually one that left the file as it was, to find out whether the lock covered only `duration.go`.
- `openai/gpt-5.6-luna` tried paths like `../project/duration.go` and `subdir/../duration.go`, which are the escapes `os.Root` exists to refuse.
- In one run, `z-ai/glm-5.3-flash` wrote a new file, `fix_units.go`, with an `init` function that rewrote the unit table when the package loaded. That would have made the tests pass while the locked file stayed untouched.
- `deepseek/deepseek-v4.1-flash` never stopped on its own; the turn limit ended all three of its runs.

Under the read-only wording, every run of every model tried the edit once and then reported the fix, finishing in four or five turns.
For the three models that had kept retrying, the cost fell to between a fifth and a third of what it had been.

## Why the wording matters

A model has nothing to go on except the text of the error.
Locks usually get released, so "the file is locked" reads as a temporary problem, and both retrying and searching for an unlocked route are sensible responses to it.
The message never says that another attempt is useless, or what a good result looks like when the edit can't be made.

The read-only wording settles both.
It says the situation will last for the whole session, tells the model not to retry, and gives it something useful to do instead.

## What it can't show

Three runs per model is a small sample.
Still, the gap between a median of 18 turns and a median of 4 is much wider than the run-to-run variation chapter 5 measures.
The experiment covers one task, one tool, and two wordings, and a different kind of failure might call for different advice.
The default model also gave up after two attempts under the first wording, so the benefit of a better message depends on which model reads it.

The side results matter as much as the headline.
When a model can't finish the obvious way, it tests the edges of what it has been given: a different file, a different spelling of the same path, a new file that changes behavior when the package loads.
Nothing in those runs asked a model to get around the lock; the only task was the bug fix.
That behavior is why the companion code confines its file tools with `os.Root`, and why the chapter 13 grader fails any run that deletes or skips a test.

## What to do with it

Write tool errors for the model that will read them:

- **Say whether another attempt can succeed.** "Temporary, retry in a moment" and "permanent for this session" call for different next steps, and when you leave it out, the model has to guess.
- **Name the better move.** "Report the fix", "list the files to find the right path", "send JSON like `{"path": "a.go"}`".
- **Be specific about what was wrong with the call.** Chapter 4's errors give the field, the value that arrived, and the value expected: `bad arguments: "path": got an array, want a string; send JSON like {"path": "a.go"}`.
- **Keep the wording the same from run to run.** `encoding/json/v2` changes its error text between processes on purpose, so the companion code builds its messages from the error's fields with `errors.AsType` and never from `err.Error()`. Fixed wording keeps recorded runs replayable and shows the model the same text every time.

The decoder behind those messages lives in [`ch04/agent/tools.go`](code:ch04/agent/tools.go):

```go include=ch04/agent/tools.go tag=decode-error
```

A turn limit is still required: it was the only thing that stopped `deepseek/deepseek-v4.1-flash`.
Good errors save turns, and only the limit guarantees that the loop ends.

## Try it

The recorded runs behind the table are in the repository's `evidence` directory, and `go run ./evidence`, with no key set, prints the table again from them. A replay of a run that recovers from three bad calls is in chapter 4's code.
From the `code` directory of a clone:

```console
$ go run ./ch04 -replay recover
```

To test wording yourself with an API key, change `edit_file` in `ch04/agent/tools.go` to return an error you write on every call, then run `go run ./ch04` against two models by setting `OPENROUTER_MODEL`.
Each run costs well under a cent.
