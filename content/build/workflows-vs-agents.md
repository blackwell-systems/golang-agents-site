---
title: "Workflow or agent? The shapes, in Go"
short: Workflow or agent?
description: Programs that call a model come in five main shapes, from a fixed sequence of calls to several cooperating agents. Pick the simplest one that handles the task, go further only when the task forces you to, and mix shapes when a task has a routine part and an open-ended part.
card: Five shapes of program that call a model, and how to pick the simplest one that handles the task.
group: Start building
chapters: 1, 5, 6, 13
stage: 4
date: 2026-10-18
weight: 10
art: ch05
---

Calling a model from Go doesn't make a program an agent, and many such programs shouldn't be one.
To tell the shapes apart, ask where the decision about the next step is made.
In some programs your code makes it, at places you chose when you wrote it.
In others the model makes it on every turn, after seeing what the previous step produced.
Handing the model more of those decisions makes the program able to do more, and it also makes it slower, more expensive, and harder to test.

The shapes below come from Anthropic's ["Building effective agents"](https://www.anthropic.com/engineering/building-effective-agents), which names them.
One task, a small bug fix, was built both ways and measured.
This page covers both, and shows the workflow shapes as small Go functions from the companion code's [`shapes` example](code:shapes).

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

## The workflow shapes in Go

The `shapes` example builds the workflow rows, plus one shape the table leaves out, and each fits in one function.
All of them take `agent.Model`, so any model client plugs in, and all of them treat a reply the model didn't finish (a `finish_reason` of `length`, `content_filter`, or `error`) as an error, never as an answer, which is the rule a [hardened loop](/production/stop-reasons/) follows.

**A fixed chain** runs its steps in order, each step's prompt built from the previous step's output, and checks each reply in code before the next step sees it:

```go include=shapes/chain.go tag=chain
```

The check is what keeps a chain honest.
Without it, a malformed summary from step one becomes the input of step two, and the error surfaces two calls later, if at all.
A chain costs one call per step, made one after another, so its latency is the sum of its steps; in exchange, each call holds only the last output, which keeps it small and easy to test alone.

**A router** asks a small, cheap model for the input's kind and hands the input to that kind's handler, which can be a large model, a small one, or plain code ([`route.go`](code:shapes/route.go)).
It adds one short call in front of every input, and an input it misfiles gets the wrong treatment, so measure the classifier on inputs someone has labeled before you trust it.
The input also reaches the classifier's prompt, so a crafted input, a [prompt injection](/production/prompt-injection/), can choose its own kind: no handler should have a permission that the worst possible input must never get.

**Parallel calls with a vote** ask several voters the same question at once and return the first answer a quorum of them gives.
The code runs each call in its own [goroutine](/glossary/#goroutine), a function running at the same time as the rest of the program, and collects the answers through a [channel](/glossary/#channel), which passes values between goroutines ([Goroutines with an owner](/basics/goroutines/)):

```go include=shapes/vote.go tag=vote highlight=17-20
```

Once the vote is decided, the calls still running are stopped, and Vote doesn't return until every call has finished.

<div class="hood">

The deferred `wg.Wait` means no voter's goroutine outlives the call, and cancelling with a cause stops the calls still running once the vote is decided, since their answers can't change it.

</div>

A vote costs one call per voter and takes about as long as the slowest call it waits for.
It only buys something if the voters can disagree: one model at temperature 0 gives the same answer every time.
Answers are compared as strings after trimming space, so ask for a word or a fixed label; in the example's demo, `Yes` and `yes` count as two different answers.

**Check and revise** is the shape the table leaves out, one model judging another's output, also used for [evals](/production/evals/).
A writer drafts, a critic judges the draft, and the critic's notes go back to the writer, up to a limit:

```go include=shapes/revise.go tag=revise-loop
```

The critic can be code (a length limit, a schema, the tests), a model, or both, with the cheap checks first; `AllOf` in the same file runs them in that order.
Each round costs a critic call and a writer call, and the writer's calls grow, since each one carries the conversation so far; `maxRevisions` bounds both.
Recorded measurements show why the critic deserves suspicion too: a model reviewer [spent its whole reasoning budget](/findings/measured-mistakes/#reviewer-budget) (the tokens a [reasoning model](/glossary/#reasoning-model) may use to think before it answers) on 10 of 25 reviews and returned nothing usable.

## What moving down costs, measured

The same bug was fixed both ways, on four models with ten runs per model.
As an agent, the model decided for itself how to use four tools.
As a workflow, the program followed three hard-coded steps: run the tests and read the two files involved, ask the model for a single edit in JSON, then apply it and rerun the tests.

On every model the workflow's cost was between one sixth and one quarter of the agent's.
Its single model call took from under a second to 2.5 seconds; the agent's runs took 7.5 to 13.5 seconds.
The agent passed all 40 runs and the workflow passed 39.
[Measured mistakes](/findings/measured-mistakes/#workflow-tab) has the failed run and its method.
That failure is worth a look: the model put a literal tab character inside a JSON string, so the edit failed to decode, and the workflow had no step that could send the error back and ask again.

So for this bug, the workflow wins.
It wins because the author already knew which files mattered and what kind of change was needed.
Move the cause into a third file, or make it something only a test run can pin down, and three fixed steps no longer cover the task.

## What an agent buys

An agent pays for itself when nobody can say ahead of time what the next step should be, because that depends on the result of the step before.
In the bug-fix run, no line of code told the model to open the test file after listing the directory; it decided that after seeing the listing, and it decided on the edit after reading the test output.

A [crash-triage agent](/glossary/#crash-triage-agent), the companion code's agent that reads crash reports from `remind` (a small practice service with planted bugs), fixes them, and files issues in a tracker, does that kind of work all the time.
The crash catalog is built to mislead: a crash can surface far from the code that caused it, a report can come from an older release whose bug was fixed since, and reports that look alike can have different causes.
There's no fixed list of files to read, since the right one depends on the stack trace, the source, and the outcome of the latest test run.

## Agents need more machinery

Each row you move down also shifts responsibility into the [harness](/glossary/#harness), the program around the model.
A workflow can only do what its code spells out.
An agent can keep going indefinitely unless you add limits:

```go include=ch05/agent/types.go tag=limits
```

The turn limit ends a loop that would otherwise run forever, and the cost budget, checked before every model call, puts a ceiling on spending.
Beyond limits, an agent needs its tool calls validated before they execute, tool errors written so the model can respond to them, and tests that judge the result instead of the transcript, because the same task can take a different route each run.
The [production](/production/) pages cover all of that.

## Combine the shapes

In practice the shapes mix, and a good design often nests one row inside another.

One case study, from [beyond crash triage](/beyond-crash-triage/#a-support-ticket-resolver), resolves support tickets.
The outer layer is routing: one model call labels each ticket, and ordinary code handles the common categories with fixed handlers that send a password reset link, fetch an invoice from billing, or fill in a reply template.
Tickets that don't fit a category are passed to an agent.
It can look up orders, payments, and shipments with three read-only tools, and it has one tool that changes something, `issue_refund`.

Putting the router in front means most tickets cost a single model call, and those tickets never reach the agent, so the agent can't mishandle them.
The two halves also get measured in different ways.
The router's accuracy is a number a program can compute against tickets someone has labeled.
Whether a given refund should have been issued is a business decision, so it takes a human reviewer or a model acting as judge.
And because `issue_refund` moves money, a person [approves each call](/production/approval/) before it runs, and the call carries an [idempotency key](/glossary/#idempotency-key) (an identifier the payment system uses to ignore a repeat of the same request) so a refund that went out just before a crash isn't paid twice.

## When to use it

For this page, "it" is an agent; leaving it out means writing a workflow.

```fit
reach: Each step depends on what the previous one turned up. On `m2`, a crash whose cause was stored long before it, the agent passed three of its six hard-tier runs.
reach: A model reply can fail in ways you can't list in advance. The workflow's only failure in 40 runs was a reply it couldn't decode, which an agent's loop would have sent back to the model.
leave: You can write the steps down. As a three-step workflow, the bug fix passed 39 of 40 runs for a quarter to a sixth of the agent's cost.
leave: Waiting matters. The workflow's single call took at most 2.5 seconds; each agent run took 7.5 to 13.5.
```

## A checklist

- **Can you write the steps down?** Use a fixed chain. Wrap each model step in a [retry](/production/retries-and-observability/); the workflow's single failure above was a decoding error that one more attempt, with the error shown to the model, might have fixed.
- **Do inputs fall into kinds?** Route them, and give each kind the simplest shape that handles it.
- **Are the pieces independent?** Call the model for each piece in parallel and merge the results in code.
- **Does each step depend on what the last one found?** Use an agent, and give it a turn limit, a cost budget, and validated tool calls from the start.
- **Does the task split into large independent parts?** Several agents may help, but measure first: on a recorded 99-report batch, [fan-out finished no faster than one agent](/findings/fan-out/) and cost more.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go run ./shapes
$ go test -race ./shapes
```

The `-race` flag turns on Go's data-race detector, which reports two goroutines touching the same memory without coordination; the second command runs the example's tests with it.
The first runs each workflow shape on scripted models, on a crash report from the `remind` service, and prints what each returned and how many calls it made: two calls for the chain, four to the small model and two to the large one for the router, a vote that reaches its quorum and one that doesn't, and two rounds of writing and critique.
No key is needed.

```console
$ go run ./ch04
```

This replays the agent version of the bug fix, runs the real `go test`, and prints the cost: $0.0008 across four model calls.
The [ch04 example](/run/ch04-first-agent/) goes through it step by step.
