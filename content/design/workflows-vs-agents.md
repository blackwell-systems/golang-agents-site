---
draft: true
title: "Workflow or agent? Choosing a shape for a program that uses a model"
short: Workflow or agent?
description: Programs that call a model come in five shapes, from a fixed sequence of calls to several cooperating agents. Pick the simplest one that handles the task, go further only when the task forces you to, and mix shapes when a task has a routine part and an open-ended part.
chapters: 1, 5
stage: 4
date: 2026-10-18
weight: 30
---

Calling a model from Go doesn't make a program an agent, and many such programs shouldn't be one.
To tell the shapes apart, ask where the decision about the next step is made.
In some programs your code makes it, at places you chose when you wrote it.
In others the model makes it on every turn, after seeing what the previous step produced.
Handing the model more of those decisions makes the program able to do more, and it also makes it slower, more expensive, and harder to test.

Chapter 5 of the book lays out the shapes, adapted from Anthropic's "Building effective agents".
Chapter 1 built one task both ways and measured the difference.
This page covers both.

## Five shapes

| Shape | Who picks the next step | A typical use |
| --- | --- | --- |
| A fixed chain | Your code, in a sequence where each call receives the previous call's output | A job with a known recipe, like writing a summary and then reviewing it |
| Routing | Your code, after one model call has labeled the input | Mixed input where each category gets its own handler, like a support inbox |
| Parallel calls | Your code, which fires several calls together and merges what comes back | Work that splits into unrelated pieces, or a question you want several answers to |
| A single agent | The model, turn by turn, inside one loop | Work where the next move depends on the last result |
| Several agents | Models in separate loops that pass tasks along or delegate to sub-agents | Big jobs that divide into parts that don't depend on each other |

In the top three rows, a [workflow](/glossary/#workflow), the plan lives in your code and the model only does the steps.
In the bottom two, an [agent](/glossary/#agent), the model owns the plan.

Pick the highest row that can do the job, and go lower only when the task leaves you no choice.

## What moving down costs, measured

Chapter 1 fixed the same bug both ways, on four models with ten runs per model.
As an agent, the model decided for itself how to use four tools.
As a workflow, the program followed three hard-coded steps: run the tests and read the two files involved, ask the model for a single edit in JSON, then apply it and rerun the tests.

On every model the workflow's cost was between one sixth and one quarter of the agent's.
Its single model call finished in somewhere between under one second and 2.5 seconds; the agent's runs took 7.5 to 13.5 seconds.
The agent passed all 40 runs and the workflow passed 39.
[The workflow-versus-agent finding](/findings/workflow-vs-agent/) has the table and the failed run.
That failure is worth a look: the model put a literal tab character inside a JSON string, so the edit failed to decode, and the workflow had no step that could send the error back and ask again.

So for this bug, the workflow wins.
It wins because the author already knew which files mattered and what kind of change was needed.
Move the cause into a third file, or make it something only a test run can pin down, and three fixed steps no longer cover the task.

## What an agent buys

An agent pays for itself when nobody can say ahead of time what the next step should be, because that depends on the result of the step before.
In the bug-fix run, no line of code told the model to open the test file after listing the directory; it decided that after seeing the listing, and it decided on the edit after reading the test output.

The book's crash-triage agent does that kind of work all the time.
The crash catalog is built to mislead: a crash can surface far from the code that caused it, a report can come from an older release whose bug was fixed since, and reports that look alike can have different causes.
There's no fixed list of files to read, since the right one depends on the stack trace, the source, and the outcome of the latest test run.

## Agents need more machinery

Each row you move down also shifts responsibility into the harness.
A workflow can only do what its code spells out.
An agent can keep going indefinitely unless you add limits:

```go include=ch05/agent/types.go tag=limits
```

The turn limit ends a loop that would otherwise run forever, and the cost budget, checked before every model call, puts a ceiling on spending.
Beyond limits, an agent needs its tool calls validated before they execute, tool errors written so the model can respond to them, and tests that judge the result instead of the transcript, because the same task can take a different route each run.
The [production](/production/) pages cover all of that.

## Combine the shapes

In practice the shapes mix, and a good design often nests one row inside another.

Chapter 5's case study resolves support tickets.
The outer layer is routing: one model call labels each ticket, and ordinary code handles the common categories with fixed handlers that send a password reset link, fetch an invoice from billing, or fill in a reply template.
Tickets that don't fit a category are passed to an agent.
It can look up orders, payments, and shipments with three read-only tools, and it has one tool that changes something, `issue_refund`.

Putting the router in front means most tickets cost a single model call, and those tickets never reach the agent, so the agent can't mishandle them.
The two halves also get measured in different ways.
The router's accuracy is a number a program can compute against tickets someone has labeled.
Whether a given refund should have been issued is a business decision, so it takes a human reviewer or a model acting as judge.
And because `issue_refund` moves money, a person approves each call before it runs, and the call carries an idempotency key so a refund that went out just before a crash isn't paid twice.

## A checklist

- **Can you write the steps down?** Use a fixed chain. Wrap each model step in a retry; the workflow's single failure above was a decoding error that one more attempt, with the error shown to the model, might have fixed.
- **Do inputs fall into kinds?** Route them, and give each kind the simplest shape that handles it.
- **Are the pieces independent?** Call the model for each piece in parallel and merge the results in code.
- **Does each step depend on what the last one found?** Use an agent, and give it a turn limit, a cost budget, and validated tool calls from the start.
- **Does the task split into large independent parts?** Several agents may help, but measure first: on the book's 99-report batch, [fan-out finished no faster than one agent](/findings/fan-out/) and cost more.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go run ./ch04
```

This replays the agent version of the bug fix, runs the real `go test`, and prints the cost: $0.0008 across four model calls.
The [ch04 example](/run/ch04-first-agent/) goes through it step by step.
