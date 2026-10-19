---
title: What this guide and the book leave out, and why
short: Scope
description: The ground this site and Building AI Agents in Go cover, the topics both leave out on purpose, such as retrieval, memory across runs, and multi-agent planning, and why, so you can tell before reading whether they answer your question.
layout: page
---

A guide is easier to trust when it says where it stops.
This page lists what the book behind this site covers, the topics both leave out on purpose with the reason for each, and, where it matters, what was measured instead.

## What they cover

The book builds one agent in Go, from an empty loop to a service you can leave running, in the order you need the pieces.
The site follows the same plan, one question per page, and publishes its guides in batches; each section page lists the guides that are out.

- **[The loop](/loop/):** what an agent is, the request and the reply, tool calls on the wire, tokens and cost, streaming, and what happens inside the model, from tokens to sampling.
- **[Design](/design/):** tools a model uses well, answers in a fixed shape, choosing a workflow or an agent, keeping a long run's context in check, parallel tool calls, sub-agents, and MCP on both sides.
- **[Production](/production/):** stop reasons and budgets, retries, traces and cost, offline tests and evals, sandboxing and prompt injection, a person's approval, and runs that survive a crash. The [launch checklist](/production/launch-checklist/) sums these up.
- **[Providers and frameworks](/providers/):** the chat completions API and Anthropic's Messages API in depth, official SDKs, and five Go agent frameworks compared on one task.
- **[Findings](/findings/):** measured results from the book's recorded runs, each with its method, including [the mistakes](/findings/measured-mistakes/) and [the finished agent across three models](/findings/the-finished-agent/).

The book goes further than the site on every topic: the full build with its tests, the reasoning behind each decision, and the exercises.

## Several agents: what was measured, and what wasn't

The book does use more than one agent, and measures what that buys, so it helps to separate the shapes.

**Measured: agents that code starts.**
Chapter 9 starts one agent per crash group from ordinary Go, so the number of agents is fixed before the run, and gives a parent agent sub-agents for questions whose reading would otherwise fill its own context.
On a batch of 99 crash reports, four agents at once [took about as long as one agent and cost 25 to 60 percent more](/findings/measured-mistakes/#fan-out); what the split bought was a steadier cost and smaller requests.
Chapter 13 adds a second model that reviews each diagnosis and can send it back, and [the finished agent's results](/findings/the-finished-agent/) report how often that review worked.

**Not measured: a model that hands out the work.**
A lead model that decides during the run how many agents to start, what to ask each, and how to merge what they return is orchestration.
It has failure modes of its own: deciding between agents that disagree, keeping the total cost bounded, and testing a system in which every part varies.
Chapter 9 stops before it on purpose, chapter 5's table of shapes names it as the last step up, and chapter 16 notes the Go frameworks that offer graphs and multi-agent patterns, if you need them now.

**When it's worth measuring yourself.**
The book's research-assistant case study (chapter 9) describes the shape where orchestration tends to pay: a question whose answer is spread across many separate sources, where each part can be read in its own context.
Anthropic's [report on its multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system), which the case study cites, found such a system used about 15 times the tokens of a chat and cut research time by as much as 90 percent on complex questions.
Crashes in one codebase were rarely independent enough for that, which is why the fan-out bought steadiness rather than speed.
If your parts are as independent as separate documents, measure the lead-and-helpers shape against one agent on your own tasks, with the same budget for both; [beyond crash triage](/beyond-crash-triage/) sketches the parts it would need.

## Out of scope for this edition

**Retrieval (RAG), embeddings, and vector stores.**
The book's agent finds related crashes by comparing the functions in their stack traces, not by embedding them.
Chapter 8 explains the choice: the frames every stack shares make up most of each trace, so embeddings would place most crashes close together, while matching functions costs less, needs no network, and its accuracy can be measured on the crash catalog.
Searching and opening files is enough for a service of a few thousand lines; a codebase many times that size, or a long history of incidents, calls for a searchable index that is kept up to date and evaluated as carefully as any tool, and this edition doesn't build one.
The sign you need it: the material the agent must consult is too large to search with tools inside one run.

**Memory across unrelated runs.**
Within a run, the agent keeps notes per crash group and a context policy decides what each request holds (chapter 8); across a crash, a checkpoint restores the run (chapter 15); the finished service shares a record of filed issues between runs (chapter 17).
A store the agent writes to and recalls from across unrelated tasks, with rules for what to forget and how to keep untrusted text out of it, isn't built in this edition.
The sign you need it: what one task learns must change how a different task is done later, rather than be looked up again.

If your agent needs either, the rest of the book still applies unchanged: the turn and cost limits, retries, evals with ranges, the sandbox and approval, and durable runs work the same way around a retrieval tool or a memory store as around any other tool, and a retrieval tool needs its own eval like the rest.

**A user interface.**
The agent is a command and an HTTP service with a review queue for approvals.
Chapter 6 streams its work as events in Go, and the [ssebrowser example](/run/stream-to-browser/) shows them in a browser; wiring the agent into a chat tool is left to you.

**Fine-tuning and training.**
The book works with models as they come from a provider's API.
Chapter 3 builds a tiny language model to explain how one picks its next token; it is there to explain the agent's behavior, and training your own model is out of scope.

**Deployment beyond a container.**
Chapter 15 ships the agent as a static binary in a container and handles a platform's stop signal and grace period; serverless platforms and managed agent runtimes aren't covered.

**Being a manual for what the agent touches.**
HTTP, JSON, concurrency, and machine learning each get the explanation a step needs, with pointers for going further, and no more.

## Who it suits

The book is written for Go developers who are new to models and agents: if you can build and run Go code and are at home with structs, interfaces, errors, and packages, you have what it asks; goroutines and `context.Context` are explained at each use. Nothing about models or machine learning is assumed.
If you already build agents in another language, the book also works as a tour of how each part is done in Go.
If what you need is on the list above, a framework that ships it, or a book on that subject, will serve you better than this one.
