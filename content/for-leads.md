---
title: "For leads: what a team's time and money buy here"
short: For leads
description: A short page for whoever approves a team's time on AI agents in Go. What live runs cost, what the team ends up able to build, the risks and how the material handles each one, what it leaves out, and how to get a team started.
layout: page
---

This page is for the person who decides whether a team spends its time on this material, and who may have to explain the choice to someone else.
It has no code to read.
Every figure on it comes from runs recorded for *Building AI Agents in Go*.

## What it costs to try

**Nothing, to start.**
Every example in the [companion code](https://github.com/blackwell-systems/go-agents-book-code) runs offline: without an API key, the programs replay model replies recorded from real runs, while the tools, `go test` among them, really execute.
Each engineer can clone the repository and work through it with no account, no key, and no budget request.
The code is Apache 2.0, and its {{code tests}} test functions run without a network.

**Very little, to run live.**
With an inexpensive model, live runs are cheap, and recorded runs show what each kind of run costs:

| Run | Cost |
| --- | --- |
| One live run of the bug-fix example (40 runs measured) | no more than a fifth of a cent |
| One crash-triage run on the default model, averaged over the confirmation tasks | $0.0081 |
| Two runs of each of 20 triage tasks, one of the exercises | $0.32 |
| The 99-report batch, before the context policy | $1.33 |
| The same batch after the policy, two runs | $0.06 and $0.42 |
| Each passing run of the finished agent on `anthropic/claude-haiku-5.5` | $0.0165 |
| Every run behind the finished agent's results tables, three models | $3.89 |

Live calls go through OpenRouter on prepaid credit, so the worst case has a ceiling: when the credit runs out, calls fail rather than adding to a bill.
The agent's own budgets are a second ceiling: each run has a turn limit and a cost limit, and the finished service keeps a daily budget on disk, so a restart doesn't reset it.

## What the team will be able to build

By the end, the team has [a crash-triage agent](/findings/the-finished-agent/) for a Go web service, running as a command and as a service.
It groups crash reports by cause, writes a test that reproduces each one, fixes the code when a fix is safe, and files one issue per cause, pausing for a person before anything risky.
Measured on 20 tasks it was never tuned on, it passed 70% of runs on its default model, with a range of 53 to 85%, at under two cents per passing run.

Crash triage is the running example, and the skills are general.
Along the way an engineer builds every part an agent needs, with tests, and the same parts carry over to agents that do other work; [beyond crash triage](/beyond-crash-triage/) walks through three.
They come away able to read any agent framework and see what it does underneath, so the team can decide on evidence whether to build its own harness or adopt one.

## The risks, and how they're handled

Each risk below is one measured in recorded runs, and each answer is code the team writes and tests.

**An agent that loops or overspends.**
A turn limit, a cost budget, and a named reason for every stop, which callers check in code.
The finished service keeps its spending on disk across restarts.

**An improvement that is really chance.**
Two runs of one unchanged agent over one set of tasks scored 15 points apart.
The evals put a range around every rate and compare versions task by task, so the team can tell when a prompt or model change helped.

**Text that steers the model.**
One sentence in a tool description decided the model's first move in 79 of 80 replies across four models.
The defenses don't rely on the model obeying: a sandbox for the tools, limits on what may leave the machine, and a person's approval for risky actions.
In the finished agent's runs, no model did what a planted instruction asked.

**An action that can't be undone.**
Protected paths need a person's approval, and the agent starts on a new codebase in diagnose-only mode, where the tool that edits code isn't offered at all.

**A crash halfway through.**
Runs are saved as they go, and every call that changes something carries an idempotency key.
Runs were crashed at every point a crash could land and then resumed: 910 scenarios, with nothing filed twice and nothing lost.

**More agents than the job needs.**
On one batch of crash reports from a shared codebase, splitting the work across four agents at once finished no sooner and cost 25 to 60 percent more than one agent; [workflow or agent](/build/workflows-vs-agents/) shows how to pick the simplest shape that does the job.

**Models and prices that change.**
The model is one setting, the recorded replies keep every test passing after a model is retired, and the finished agent's results table carries its dates and is meant to be measured again whenever any input to it changes.

**Lock-in to a framework.**
A part-by-part comparison of five Go agent frameworks against the harness the team wrote ends with eight questions to put to any framework; it ranks none of them.

The [launch checklist](/production/launch-checklist/) collects these into items a design review can check, and [twelve measured mistakes](/findings/measured-mistakes/) shows what happened in the runs where one was missing.

## What it leaves out

Memory kept across unrelated tasks and a model that plans work and hands it to other agents at run time are out of scope, along with user interfaces and fine-tuning.
Retrieval from a team's own documents is covered on the site, beyond the book: [RAG in Go](/build/rag/), and [a measured comparison](/build/rag-or-agentic-search/) with agentic search.
[What this covers](/scope/) gives the reason for each and the signs that a team's task needs one.

## Getting a team started

1. **The free code.** Clone the [repository](https://github.com/blackwell-systems/go-agents-book-code) and run `go run ./ch01` from its `code` directory; [Examples](/run/) lists every program and its command.
2. **The reading paths.** [Start here](/start/) has an ordered path for engineers new to agents and one for engineers shipping soon.
3. **The free sample.** The introduction and the first part of chapter 1, as a [PDF](/sample/building-ai-agents-in-go-sample-light.pdf).
4. **The book.** [*Building AI Agents in Go*](/book/) has the full build, the tests, the exercises, and the reasoning behind each decision. Team licenses are available; ask through [Work with me](/work-with-me/#contact).
5. **A workshop.** A hands-on workshop in which the team builds an agent in Go from the loop up; [Work with me](/work-with-me/) describes it.
