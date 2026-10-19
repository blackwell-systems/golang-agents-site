---
title: Sub-agents in Go, and when a fresh context helps
short: Sub-agents
description: A sub-agent is your agent loop run again from inside a tool, with an empty history, to answer one self-contained question. This page covers building one in Go, reporting one that ran out of turns, limiting nesting to one level, and a measured case where it didn't pay off.
chapters: 9
stage: 1
date: 2026-10-18
weight: 60
art: ch09
---

As an agent investigates, every file and report it opens stays in its history and is resent each turn.
A context policy that clears and summarizes old turns is one way to keep that in check.
A [sub-agent](/glossary/#sub-agent) is another.
The parent hands a narrow question to a tool, the tool runs a second copy of the loop with an empty conversation, and only the final answer comes back.
Whatever the sub-agent read along the way is discarded with its conversation, so the parent's history grows by one answer.

Chapter 9 of the book adds a sub-agent to the crash-triage agent and then tests it on the task it was designed for.

## A sub-agent is a tool

From the parent's point of view the sub-agent is just a tool, `explain_package`, which takes the name of a package in the service and a question about that package:

```go include=ch09/triage/subagent.go tag=explain
```

The whole sub-agent is set up inside that function.
It gets a brief system prompt, the question as its task, a tool set and limits of its own, and it runs on the same `RunStream` loop the parent uses.
Its answer is truncated to 4,000 bytes before being returned to the parent.

Four of its design choices carry over to other agents.

**Read-only tools.**
The sub-agent can only read code.
Patching, running tests, and filing issues are off its list; it investigates and reports, and any action is the parent's call.

**Its own bookkeeping.**
The parent's `code_patch` won't touch a file unless the parent has read it first.
Sharing that record with the sub-agent would let the parent edit a file it knows only from someone else's summary.
So the sub-agent works in the same workspace but keeps a separate record of what it has read.

**A sub-agent that didn't finish is an error result, never a budget error.**
When the sub-agent hits its turn limit, the parent receives an ordinary tool error, `the helper didn't finish (agent: budget exhausted: max turns reached) after 15 turns; ask a narrower question, or read the code yourself`, which tells it what to try next.
Returning `ErrBudget` here would be wrong.
The parent's loop reads `ErrBudget` as its own turns or money being used up, so it would halt and report a budget problem that belongs to someone else.
The sub-agent's error is formatted into the message with `%v`, which copies its text without wrapping it, so no `errors.Is` check in the parent can match it.
The one exception is a fatal error, a missing workspace for instance: that is wrapped with `%w` and stops the parent as well.

**One level.**
`explain_package` is left out of the sub-agent's tool list, so a sub-agent can never start another one.
Without that rule, spending can grow geometrically, as each question spawns three helpers and each helper spawns three more.
With a single level, the worst case for any question is one sub-agent's turn limit, and since the parent and its sub-agents spend from one shared pool, one cost budget bounds the whole run.

Claude Code makes a different and reasonable choice: its subagents may start their own, down to three levels by default, and the limit is configurable.
That fits an assistant a person is watching as it works.
A triage run that nobody is supervising benefits more from a fixed limit the model can't talk its way past.

## Did it help? Measured

The longest single task in the catalog is a crash in the digest code.
Its cause sits three packages away in `schedule`, and to understand how the next run time is calculated, the agent ends up reading most of that package.
On the default model, `anthropic/claude-haiku-5.5`, chapter 9 ran this task three times with `explain_package` available and three times without it (October 2026):

| Runs | Turns | Parent's largest request (tokens) | Cost, sub-agent included | Checks passed (of 16) |
| --- | --- | --- | --- | --- |
| With the sub-agent | 18, 23, 18 | 24,493; 33,809; 33,891 | $0.0122, $0.0161, $0.0147 | 16, 16, 16 |
| Without | 17, 16, 16 | 30,435; 28,968; 37,862 | $0.0113, $0.0095, $0.0121 | 16, 16, 16 |

The sub-agent didn't help.
Each run that had the tool used it once, for a well-chosen question: can `Next` in `schedule` return a time earlier than its input?
The sub-agent settled that in three or four tool calls, costing between $0.0020 and $0.0032.
After that, the parent opened `schedule/weekly.go` on its own regardless.
The fix belonged in that file, and the patch tool refuses to change a file the parent hasn't read.
So the two agents read much of the same code, and the runs with the tool needed between one and seven extra turns and cost slightly more.
All six runs located the root cause and passed all 16 checks.

The task also never came close to needing context relief.
Its largest request stayed below 38,000 tokens, well under the size at which chapter 8's policy begins to clear old results.

## When a sub-agent pays

The saving comes from questions about files the parent will never open again.
Examples: how some other package gets used, what a third-party library does, or which handful of forty call sites actually matter.
If the sub-agent's answer points to a change in the very files it just read, the parent must open those files to make the change, and the sub-agent's reading was duplicated work.

Research is the stronger case, and chapter 9's case study is about it.
There the answer is scattered across many sources that have nothing to do with each other.
A lead agent breaks the question into parts and starts a sub-agent for each; every sub-agent reads its own sources in a separate context and returns a brief summary.
Anthropic's report on its multi-agent research system covers a design like that.
It reports that its multi-agent systems used roughly 15 times as many tokens as chats, and it names the reason coding sees smaller gains: coding tasks have "fewer truly parallelizable tasks than research".
The rules from the triage agent hold there too.
Use a single budget for the lead and all sub-agents.
Report a sub-agent that ran out of turns as a result the lead can respond to.
And give sub-agents read-only tools, because the pages they read are untrusted, and instructions planted in a page can travel up to the lead inside a summary.

## Sub-agents and fan-out

The model decides when a sub-agent runs, by calling the tool.
In fan-out, the harness itself splits the job ahead of time and starts one agent per part.
Each is a form of concurrency in Go, and the book measured both on the triage agent.
On the 99-report batch, [fan-out finished no faster than one agent](/findings/fan-out/); what it gave instead was steadier cost and smaller requests.
When a model chooses how many agents to start and merges their results, the program is doing orchestration, and the book deliberately stops short of that.

## What the book measured, and what it didn't

Measured, on the default model in October 2026:

- **One sub-agent on one task**, three runs with it and three without, on the catalog's longest single task. The result is in the table above: no context saved, a little more cost.
- **Fan-out over a batch**: 23 group agents run four at once and run one after another, two runs of each, set against three runs of a single agent over the same 99 reports. Four at once beat one after another by about 2.4 times, matched the single agent's wall time, varied less in cost, and cost 25 to 60 percent more than the single agent.

Not measured:

- **A sub-agent on a task where the parent never rereads the files.** That is the case the book expects to pay, and its one task with a sub-agent wasn't one.
- **A research workload.** The research assistant is a case study in the chapter, described from Anthropic's published report; the book ran no research agent.
- **Orchestration**, where a model decides how many agents to start and what each one does. The book leaves it out on purpose.

Three runs a side can show a large effect and can't rank two close ones.
If your agent asks side questions about code it will never edit, or reads many independent sources, the unmeasured cases are the ones that describe it, and a few runs of your own, with and without the sub-agent, will tell you more than this page can.

## When to use it

```fit
reach: A side question needs reading the parent will never use again. On chapter 9's digest task, the sub-agent answered its question in three or four calls for $0.0020 to $0.0032 (chapter 9).
reach: You want a steady bill and small requests across a batch. One agent per crash group cost $0.124 to $0.126 in four runs and never sent a request over 28,275 tokens (chapter 9).
leave: The answer leads to an edit in the files the sub-agent read. On the digest task, the parent reread those files anyway, and runs with the sub-agent took one to seven more turns and cost slightly more (chapter 9).
leave: One agent with a context policy can hold the batch. Four agents at a time finished no sooner and cost 25 to 60 percent more (chapter 9).
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch09/triage/ -run TestExplainUnfinished -v
$ go run ./ch09 -task g3-mixed-batch -groups -parallel 4
```

The test confirms that a sub-agent that exhausts its turns reaches the parent as a tool error and that the parent keeps running.
The second command, with no key set, replays a recorded fan-out run on a six-report task with five fingerprint groups.
Each output line names the group whose agent produced it; after the agents finish come the merge of their workspaces, the filed issues, and a grade of 22 of 22 checks.
The service's tests run many times along the way, so it takes a couple of minutes.
The sub-agent's prompt and tool list are in [`ch09/triage/subagent.go`](code:ch09/triage/subagent.go).
