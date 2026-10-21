---
title: "Agent patterns by name: ReAct, routing, orchestrator-workers, reflection, and the rest, in Go"
short: Patterns by name
description: The names people use for agent designs come from a handful of sources, among them Anthropic's "Building effective agents", the ReAct and Reflexion papers, Andrew Ng's four design patterns, and OpenAI's guide to building agents. This page maps each name to the Go code and the measured runs on this site that implement it, and says plainly which ones the site doesn't build.
card: ReAct, chaining, routing, reflection, orchestrator-workers, and the rest, each mapped to Go code and measured runs.
group: Start building
chapters: 1, 5, 9, 13
stage: all
date: 2026-10-20
weight: 12
art: patterns-by-name
---

Most of the vocabulary for agent designs comes from a few places.
A research paper named ReAct in 2022, Anthropic's ["Building effective agents"](https://www.anthropic.com/engineering/building-effective-agents) named five workflow shapes and the agent in late 2024, and framework authors and vendor guides added their own names since.
Several names describe the same design, and a few describe designs that look alike but put the decision in a different place.

This page lists the names you will meet, says where each comes from, and points to the Go code and recorded runs on this site that implement it.
Where the site doesn't build a pattern, it says so.
[Workflow or agent?](/build/workflows-vs-agents/) explains how to choose between them; this page is the index by name.

## The names, and where they live here

| Name | Source | What decides the next step | On this site |
| --- | --- | --- | --- |
| ReAct, or the agent | Yao et al., 2022; Anthropic's "agents" | The model, turn by turn | [The agent loop](/basics/agent-loop/), [`ch01`](/run/ch01-scripted-loop/) |
| Prompt chaining | Anthropic | Your code, in a fixed order | [`shapes/chain.go`](code:shapes/chain.go) |
| Routing | Anthropic | Your code, from one model call's label | [`shapes/route.go`](code:shapes/route.go) |
| Parallelization: voting | Anthropic | Your code, which asks several times and counts | [`shapes/vote.go`](code:shapes/vote.go) |
| Parallelization: sectioning | Anthropic | Your code, which splits the work and merges it | [Fan-out](/findings/fan-out/), `ch09` |
| Evaluator-optimizer, reflection | Anthropic; Self-Refine and Reflexion, 2023; Ng, 2024 | A critic, in code or a model | [`shapes/revise.go`](code:shapes/revise.go), the reviewer in [evals](/production/evals/) |
| Tool use | Ng, 2024 | The model, choosing from tools you define | [Tool calling](/basics/tool-calling/), [tool design](/build/tool-design/) |
| Agents as tools, the manager pattern | OpenAI, 2025 | The lead model, one tool call at a time | [Sub-agents](/build/sub-agents/) |
| Orchestrator-workers, supervisor | Anthropic; LangGraph, Eino | The lead model, which plans and splits the whole job | Not built here |
| Plan-and-execute, planning | Wang et al., 2023; Ng, 2024 | A plan the model writes before acting | Not built here |
| Handoffs, the decentralized pattern | OpenAI, 2025 | Each agent, passing control to the next | Not built here |
| Human in the loop | Common usage | A person, for the risky calls | [Approval](/production/approval/) |

## ReAct: the agent loop

[ReAct](https://arxiv.org/abs/2210.03629) (Yao et al., 2022) had a model write a line of reasoning, then an action, then read the action's result, over and over until it had an answer.
The point was that each piece of reasoning could use what the last action found.

That is the [agent loop](/basics/agent-loop/) every page on this site builds, with two differences that come from how model APIs work now.
The action is a structured [tool call](/basics/tool-calling/) in the reply rather than a line of text the program has to parse.
And the reasoning is optional: a reasoning model does it before it replies, and an ordinary model often just calls the tool.
Frameworks that ship a "ReAct agent", such as Eino's `react` package, mean this loop.

The recorded bug fix in [`ch04`](/run/ch04-first-agent/) is a complete run of it: four turns, in which the model ran the tests, read two files, made one edit, and answered.

## The workflow shapes: chaining, routing, and parallelization

Anthropic's article calls a design a workflow when your code decides the order of the model calls, and its three simplest workflows are short functions in the companion code's [`shapes` example](code:shapes).

**Prompt chaining** runs calls in a fixed order, each one fed the previous call's output, with a check in code between steps.
Built as a three-step chain, the bug fix passed 39 of 40 runs for a quarter to a sixth of what the agent cost ([the measurement](/findings/workflow-vs-agent/)).
Google's ADK for Go calls the same design a `sequentialagent`.

**Routing** spends one cheap model call on labeling the input and sends it to a handler written for that label, which can be a model, a smaller model, or plain code.
The [support-ticket case study](/beyond-crash-triage/#a-support-ticket-resolver) uses it as its outer layer.

**Parallelization** comes in two forms.
*Voting* asks the same question several times at once and keeps the answer enough of them agree on; `Vote` in [`vote.go`](code:shapes/vote.go) cancels the remaining calls as soon as a quorum is reached.
*Sectioning* splits a job into independent parts and runs them side by side.
On a batch of 99 crash reports, sectioning by cause across 23 agents, four at a time, finished no faster than one agent and cost 25 to 60 percent more, because each agent had to read the same code again ([fan-out on one codebase](/findings/fan-out/)).
ADK's version is a `parallelagent`.
Running several tool calls from one reply at once is a different thing, inside a single agent: [parallel tool calls](/build/parallel-tool-calls/).

## Evaluator-optimizer and reflection

Anthropic's evaluator-optimizer has one call produce something and another judge it, looping until the judge accepts.
The research names for a model critiquing and revising its own output are [Self-Refine](https://arxiv.org/abs/2303.17651) (Madaan et al., 2023) and [Reflexion](https://arxiv.org/abs/2303.11366) (Shinn et al., 2023), and Andrew Ng listed reflection first among his [four agentic design patterns](https://www.deeplearning.ai/the-batch/how-agents-can-improve-llm-performance/) in 2024.
ADK's `loopagent` repeats a step until a condition is met, which is the same loop.

In the companion code, `Revise` in [`revise.go`](code:shapes/revise.go) takes any `Critic`: code that checks a length or a schema, or a model given a rubric.
A critic in code is cheaper and never wrong about what it checks, so use one wherever the check can be written down.

The finished agent tried a model as the critic: a second model reviewing each diagnosis before it was filed, on the ten hard tasks, two runs each.
With its reasoning capped and one host pinned, the reviewer rejected two diagnoses, and both rejections were right; the agent fixed its answer each time and those runs passed.
It also accepted wrong answers four times, failed to answer on 6 of its 25 calls, and added about a fifth to the cost of each run.
Set up without the cap, it rejected three correct answers as well ([the reviewer's numbers](/findings/measured-mistakes/#reviewer-budget)).
A model critic can catch what code can't, and it is a second model that can be wrong, so measure it on your own tasks before trusting it.

The cheapest reflection is already in every agent loop: a tool error goes back to the model, which corrects its next call.
How the error is worded decides how fast that works; an error that said "locked" cost a median of 18 turns and one that said "don't retry" cost 4 ([error wording](/findings/error-wording/)).

## Agents as tools, and orchestrator-workers

OpenAI's guide, ["A practical guide to building agents"](https://cdn.openai.com/business-guides-and-resources/a-practical-guide-to-building-agents.pdf) (2025), describes a *manager* pattern, in which one agent calls other agents as tools.
A [sub-agent](/build/sub-agents/) on this site is that: the crash-triage agent's `explain_package` tool runs a second copy of the loop with an empty history to answer one question about one package, and only the answer comes back.
The lead model decides when to call it, one question at a time.

Anthropic's *orchestrator-workers* goes further: the lead model reads the whole job, splits it into subtasks it decides on, hands them to workers, and combines what they return.
LangGraph and Eino call the lead a *supervisor*.
This site stops short of that on purpose, as [what this covers](/scope/) explains: the shapes above handle the work it measures, and the one multi-agent design it measured, sectioning, didn't pay on a shared codebase.
The [research-assistant case study](/beyond-crash-triage/#a-research-assistant) describes the kind of task where an orchestrator tends to pay.

## Plan-and-execute and handoffs

*Plan-and-execute* has the model write a plan before acting and then carry it out, step by step, revising the plan when a step surprises it.
It comes from work such as [Plan-and-Solve](https://arxiv.org/abs/2305.04091) (Wang et al., 2023), and it is the *planning* in Ng's four patterns.
The agents on this site plan one step at a time instead, which suits work where each step depends on what the last one found.
Eino ships a `planexecute` package if you want the up-front form in Go.

*Handoffs*, OpenAI's decentralized pattern, pass control of the conversation from one agent to another, each with its own instructions and tools, rather than calling the other as a tool and taking back its answer.
The site doesn't build this either.

## Names framework code uses

| Framework name | The pattern |
| --- | --- |
| ADK for Go `sequentialagent` | Prompt chaining |
| ADK for Go `parallelagent` | Parallelization |
| ADK for Go `loopagent` | Evaluator-optimizer |
| Eino `react` | The agent loop |
| Eino `planexecute` | Plan-and-execute |
| Eino `supervisor` | Orchestrator-workers |

[Go agent frameworks](/ecosystem/go-agent-frameworks/) compares these frameworks part by part against a loop you write yourself.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), with no API key:

```console
$ go run ./shapes
$ go test ./shapes
```

The first runs the chain, the router, the vote, and the revise loop on a crash report, against scripted models, and prints what each returned and how many calls it took.
The second runs their tests.
