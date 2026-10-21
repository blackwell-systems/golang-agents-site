---
title: "Go agent frameworks compared: ADK, Eino, Genkit, LangChainGo, and trpc-agent-go"
short: Go agent frameworks compared
description: Five Go agent frameworks set against the parts of an agent harness, as their source stood in October 2026, plus one approval-gated task ported to Eino and Genkit, what the code measured, and eight questions to put to any framework.
card: Five Go agent frameworks set against the parts of a harness, with one task ported and measured.
group: Libraries and frameworks
chapters: 16
stage: all
date: 2026-10-18
weight: 40
art: ch16
---

A framework hands you the loop, tool dispatch, memory, and tracing on day one, and you supply the prompt, the tools, and the policy.
That is a good deal for a lot of teams.
The trouble is choosing between frameworks: a list of features per framework is out of date within a few months.
A steadier way to judge one is to hold it up against the parts of an agent you could write yourself, and ask, part by part, what it does for you, what it does differently, and what is still left for you to write.

This page does exactly that, using a [hand-built agent harness](/build/first-agent/) as the yardstick.
Chapter 16 of the book makes the full comparison; this page gives the short version.
The version numbers and facts below come from reading each module's source in October 2026, so they will age; the questions at the end are meant to outlast them.
A disclosure: the author builds [Bide](https://bide-ai.com), a Go framework for durable agents.
It is left out of the comparison so that this page never grades the author's own work.

## Five frameworks, five central nouns

- **ADK for Go** (Google): an *agent*, plus a *runner* that runs it inside a *session*, which is the framework's saved record of one conversation. It is a port of Google's Python ADK.
- **Eino** (ByteDance's CloudWeGo group): a *graph*; its agents sit on top of graphs.
- **Genkit** (Google): a *flow*, meaning a function you register so Genkit traces it and its developer UI can run it, and a `Generate` call that drives the tool loop.
- **LangChainGo** (an independent port of LangChain's ideas): a *chain*, and an *executor* that drives an agent.
- **trpc-agent-go** (Tencent): an *agent* and a *runner*, with a graph package alongside.

Which noun a framework puts at the center tells you how it models an agent.
An [orchestration graph](/glossary/#orchestration-graph) pays off for a program made of many fixed steps and branches whose progress has to outlive a crash: think of a nightly job that pulls a report, labels it, sends it to one of several queues, and holds until someone signs off.
A tool-calling agent has a different shape.
It is a single loop, and the model decides what happens on each pass.

## The parts, mapped

Every cell records what the framework offers for that part, from a built-in feature to an external package, or `none`.
The comparison ran Eino and Genkit; the other three columns come from their documentation and source, with no run behind them.

| Part | ADK v1.9.0 | Eino v0.9.21 | Genkit v1.13.1 | LangChainGo v0.1.15 | trpc-agent-go v1.11.2 |
| --- | --- | --- | --- | --- | --- |
| Turn limit by default | none | 20, then an error | 50, then an error | 5, then an error | none; two opt-in limits |
| A tool's error goes to the model | has | opt-in wrapper; by default the run ends | none: the run ends | none: the run ends | has |
| A panicking tool | recovered, sent to the model | recovered, ends the run | crashes the process | not recovered: the panic reaches your code | recovered, sent to the model |
| Context policy | none | opt-in summarization and reduction | none; a hook for trimming history yourself | window and token buffers | session summaries |
| Retrying model calls | none (a plugin guides the model on failed tools) | opt-in | opt-in middleware, plus the client's own two retries | none | the client's, plus failover |
| Tracing | OpenTelemetry | Langfuse and LangSmith handlers in eino-ext | OpenTelemetry | none | OpenTelemetry, Langfuse |
| Evals | none in Go | none | evaluators | none | a nested module |
| Pausing for a person | opt-in tool confirmation | opt-in interrupts | opt-in interrupts, approval middleware | none | opt-in graph interrupts |
| Saved state to resume from | sessions in memory, a database, or Vertex AI | a checkpoint interface, no store | experimental sessions | conversation memory only | checkpoints in memory, SQLite, or Redis |
| Takes your `*http.Client` for an OpenAI-compatible API | v2 only, experimental | has | has | has | has |
| Development UI | has | an IDE plugin | has | none | none; an AG-UI server for chat front ends |

One cell rests on less than the others.
Genkit's panic cell comes from its source and from one probe run, described below; no test in the companion code panics a tool.

<div class="hood">

In Genkit's source, each tool runs in a goroutine Genkit starts, so no `recover` in your own goroutine can catch its panic (`recover` is the built-in that stops a panic from crashing the program, and it works only in the goroutine that panicked).
The probe run, made while this table was checked, ended with the process exiting.

</div>

The v1 releases of ADK for Go speak only to Gemini models; the v2 line adds an OpenAI-compatible adapter, still marked experimental.
There is nothing odd about a model provider's framework favoring that provider's models, but if your agent may need to change providers, check this first.

## One task, three versions, the same replies

For the comparison, one job was taken from the [triage agent](/run/ch07-crash-triage/), diagnosing a single crash group with three tools, and written again in Eino and in Genkit.
The job was picked with a hard step on purpose.
Filing an issue with `issue_file` [requires a person to approve it](/production/approval/), so each version has to halt, hold its state, and pick up again after the decision.
This tilts things toward the hand-built harness, because the approval gate, error handling, and retries are parts that harness supplies.
A task built around what the frameworks offer and the harness doesn't (many providers, a development UI, a fixed multi-step pipeline) would show their strengths instead.

Eino and Genkit both let you hand them your own `*http.Client`.
That made it possible to feed all three versions the same scripted replies, written by hand in the harness's [replay format](/production/testing-offline/), through one transport that also saved a copy of every request each version made.
In Eino, the approval is an interrupt raised from inside the tool: the tool stops the run and asks to be resumed later with the person's answer.

```go include=ch16/eino/eino.go tag=issue
```

In Genkit it is middleware set on the generate options, which has the same shape as the hand-built approval gate:

```go include=ch16/genkit/genkit.go tag=setup
```

Here is what the code measured (`go run ./compare` prints the table, and a test checks it):

| Measure | Hand-built agent | Eino | Genkit |
| --- | --- | --- | --- |
| Lines of code for the task | 80 | 156 | 116 |
| Lines under it, outside the standard library and the shared task code | 5,351 | 97,076 | 132,201 |
| Modules under it | 0 | 27 | 29 |
| Model calls to finish the scripted run | 3 | 3 | 4 |
| A tool's error | sent to the model, which corrected the call | ends the run | ends the run |
| A 503 on the first call, as ported | retried | ends the run | retried by the underlying client |
| Bytes kept while waiting for the person | 1,692 | 45,562 | 1,322 |

Be careful with the line counts.
The hand-built version is only 80 lines because 5,351 lines from fourteen earlier stages of the harness sit beneath it, so the row doesn't show the frameworks needing more code for the same job.
Turned around, those 5,351 lines are roughly what adopting a framework would spare you.
Not all of them, though: around 1,500 are the sandbox (the limits on what a tool may touch), the egress check (which network destinations are allowed), and the audit log (a record of what the agent did), and neither framework supplies those.

The first requests from all three versions were almost identical, within 70 bytes of each other, and neither framework added a prompt of its own.
Where the versions did differ, each difference traced back to a default that somebody chose, and each default is defensible: ending the run on a tool error or not, retrying a 503 or not, sending a string result as plain text or as a JSON-encoded string, marking schemas strict or not.
Finding one difference meant reading two layers below the framework.
Genkit's plugin is built on the OpenAI Go library, and that library waits between retries with `time.Sleep`, which ignores the context.
When a run was cancelled 100 milliseconds into a retry, Genkit took 421 to 485 milliseconds to return; the hand-built version took 100 to 103.
A batch job won't notice.
A handler serving a request with a deadline will, because its goroutine stays busy until the sleep ends.

Each version was also run once against a live model, `anthropic/claude-haiku-5.5`; each filed one issue and submitted a diagnosis, and the three runs cost $0.0028 between them.

## What frameworks give

These are things you get from a framework that the hand-built harness never builds:

- **Provider breadth.** Genkit ships plugins for Anthropic's, Google's, and Ollama's APIs; eino-ext covers Claude, Gemini, DeepSeek, Qwen, Ollama, and others.
- **Context management you can switch on**, in the form of Eino's opt-in summarization and reduction middleware.
- **Development UIs** for stepping through a run, replaying one step, and changing a prompt in place. ADK and Genkit include one, and Eino offers an IDE plugin.
- **Tracing integrations**: ADK, Genkit, and trpc-agent-go emit OpenTelemetry, and trpc-agent-go also has a Langfuse handler; Eino has Langfuse and LangSmith handlers.
- **Small parts done well.** Genkit's own retry middleware stops waiting as soon as the context is done, unlike the OpenAI library beneath it. Eino's error wrapper turns tool errors into results without swallowing interrupts. ADK's `retryandreflect` plugin coaches the model after a tool call fails.
- **Multi-agent patterns and graphs** that the harness would need several more stages to write.

## What they cost

- **Churn.** These frameworks are new and change quickly. Pin the version, read the changelog before upgrading, and keep a replay test, so a behavior change breaks a test instead of a production run.
- **Defaults you didn't pick.** You find them by recording what the framework sends and reading it.
- **Their own error vocabularies.** To separate "ran out of budget" from "the model replied badly" from "a tool failed", you learn each framework's names for those cases.
- **Behavior you can't see into**, like the retry sleep described above. Your own code from a year ago can surprise you the same way; a framework's loop is simply larger, and the eight questions below are a quick way to read it.

The vendor SDKs sit between the two paths.
Anthropic's Go SDK has a tool runner that repeats the model call and the tool calls until the model is done; it has no turn limit until you set `MaxIterations`, and it returns tool errors to the model.
For an agent on one provider with a loop as simple as the minimal one in the companion code, an SDK's runner is a reasonable place to stop.

## Eight questions for any framework

1. **Where is the loop?** When a run goes wrong, this is the code you'll be reading.
2. **How many turns does a run get, and what happens when it runs out?**
3. **Where do a tool error, a call to a tool that doesn't exist, and a panic end up?**
4. **Who retries a failed call, and how long is each wait?** Check the provider client underneath the framework too.
5. **What does cancellation do?** Cancel once mid-tool-call and once mid-retry, and measure how long the return takes.
6. **Can you pass your own `*http.Client`?** If yes, every request can be recorded, replayed, and read.
7. **What goes over the wire?** Open the very first request and read it.
8. **Where does a paused run's state live, and how does it resume?**

Two of these answers are disqualifying for an agent that runs on its own and spends money: no way to set a turn limit, and no way to supply your own HTTP client.
None of the five fails either test, although ADK needs a turn limit added and trpc-agent-go's limits do nothing until you turn them on.
The other answers are mostly costs, and you can plan for those.

That leaves three choices.
**Build** if the agent is your product and you need a say in every byte sent and every failure path.
**Adopt** if your needs line up with the framework's strengths and you agree with its answers to the questions.
**Adopt for part of the job** if one piece is worth borrowing: a provider adapter that sits behind your own model interface, a tracing exporter, or a graph runner wrapped around a loop you keep.
This page reads these questions from the builder's side, and says so; a team whose agent is one feature of a larger product could weigh the same answers differently.
One worked example of each of the first two follows.
A triage service that has to survive restarts while approvals are pending comes out as build.
A support bot that writes draft replies for a person to send, wants to try several providers and use a development UI, and loses little when a run fails, comes out as adopt.

## Try it

The ports live in their own module in the companion code.
From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ cd ch16
$ go run ./compare
$ go run ./compare -cancel
```

The first command runs the three versions on the scripted replies and prints the comparison table; the second measures how long each takes to return when cancelled during a retry.
See the [ch16 example](/run/ch16-framework-ports/) for more.
