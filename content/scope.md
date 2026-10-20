---
title: What this guide and the book leave out, and why
short: Scope
description: The ground this site and Building AI Agents in Go cover, the topics the site covers beyond the book, such as retrieval and memory between sessions, the topics both leave out on purpose, such as multi-agent planning, and why, so you can tell before reading whether they answer your question.
layout: page
---

A guide is easier to trust when it says where it stops.
This page lists what this site covers, the topics it leaves out on purpose with the reason for each, and, where it matters, what was measured instead.

## What this site covers

This site follows one agent built in Go, from an empty loop to a service you can leave running, in the order you need the pieces.
The site is organized by what you want to do, one question per page:

- **[Basics](/basics/):** what an agent is, the loop at its center, calling a model from Go, tool calls on the wire, tokens and cost, and what happens inside the model, from tokens to sampling.
- **[Build](/build/):** a workflow or an agent, tools a model uses well, streaming, answers in a fixed shape, keeping a long run's context in check, parallel tool calls, sub-agents, MCP on both sides, and retrieval from your own documents.
- **[Production](/production/):** stop reasons and budgets, retries, traces and cost, offline tests and evals, sandboxing and prompt injection, a person's approval, and runs that survive a crash. The [launch checklist](/production/launch-checklist/) sums these up.
- **[Go ecosystem](/ecosystem/):** why Go, the chat completions API and Anthropic's Messages API in depth, official SDKs, and five Go agent frameworks compared on one task.
- **[Examples](/run/):** every program in the companion code, with the command that runs it offline.
- **[Research](/findings/):** measured results from recorded runs, each with its method, including [the mistakes](/findings/measured-mistakes/) and [the finished agent across three models](/findings/the-finished-agent/).

The book, *Building AI Agents in Go*, goes further on most of these topics: the full build with its tests, the reasoning behind each decision, and the exercises.

## Covered here, beyond the book

**Retrieval (RAG), embeddings, and vector search.**
[RAG in Go](/build/rag/) builds retrieval with the standard library: documents split into chunks, embedded through an API, and searched in memory.
[RAG or agentic search?](/build/rag-or-agentic-search/) measures it against search as a tool and against an agent that reads the files, on the same 16 questions.
The book doesn't cover retrieval; these pages and their example, `rag` in the companion code, are the site's own.

The crash-triage agent itself doesn't use embeddings.
It finds related crashes by comparing the functions in their stack traces, because the frames every stack shares make up most of each trace, so embeddings would place most crashes close together, while matching functions costs less, needs no network, and its accuracy can be measured on the crash catalog.
Searching and opening files is enough for a service of a few thousand lines; documentation, runbooks, or a long history of incidents are where retrieval earns its place.

**Memory between sessions.**
[Memory between sessions](/build/memory/) builds notes an agent saves with a tool, replaces when a fact changes, and recalls by meaning at the start of each conversation, and measures them against carrying nothing and carrying the whole conversation.
The book doesn't cover memory across conversations: within a run, the crash-triage agent keeps notes per crash group and a context policy decides what each request holds; across a crash, a checkpoint restores the run.

## Several agents: what was measured, and what wasn't

The crash-triage agent does use more than one agent, and the recorded runs measure what that buys, so it helps to separate the shapes.

**Measured: agents that code starts.**
One recorded setup starts one agent per crash group from ordinary Go, so the number of agents is fixed before the run, and gives a parent agent sub-agents for questions whose reading would otherwise fill its own context.
On a batch of 99 crash reports, four agents at once [took about as long as one agent and cost 25 to 60 percent more](/findings/measured-mistakes/#fan-out); what the split bought was a steadier cost and smaller requests.
A second model reviews each diagnosis and can send it back, and [the finished agent's results](/findings/the-finished-agent/) report how often that review worked.

**Not measured: a model that hands out the work.**
A lead model that decides during the run how many agents to start, what to ask each, and how to merge what they return is orchestration.
It has failure modes of its own: deciding between agents that disagree, keeping the total cost bounded, and testing a system in which every part varies.
The crash-triage setup stops before orchestration on purpose, the table of shapes names it as the last step up, and the framework comparison notes the Go frameworks that offer graphs and multi-agent patterns, if you need them now.

**When it's worth measuring yourself.**
The research-assistant case study describes the shape where orchestration tends to pay: a question whose answer is spread across many separate sources, where each part can be read in its own context.
Anthropic's [report on its multi-agent research system](https://www.anthropic.com/engineering/multi-agent-research-system), which the case study cites, found such a system used about 15 times the tokens of a chat and cut research time by as much as 90 percent on complex questions.
Crashes in one codebase were rarely independent enough for that, which is why the fan-out bought steadiness rather than speed.
If your parts are as independent as separate documents, measure the lead-and-helpers shape against one agent on your own tasks, with the same budget for both; [beyond crash triage](/beyond-crash-triage/) sketches the parts it would need.

## Out of scope here

**A user interface.**
The agent is a command and an HTTP service with a review queue for approvals.
The agent also streams its work as events in Go, and the [ssebrowser example](/run/stream-to-browser/) shows them in a browser; wiring the agent into a chat tool is left to you.

**Fine-tuning and training.**
Everything here works with models as they come from a provider's API.
One small example builds a tiny language model to show how one picks its next token; it is there to explain the agent's behavior, and training your own model is out of scope.

**Deployment beyond a container.**
The agent ships as a static binary in a container and handles a platform's stop signal and grace period; serverless platforms and managed agent runtimes aren't covered.

**Being a manual for what the agent touches.**
HTTP, JSON, concurrency, and machine learning each get the explanation a step needs, with pointers for going further, and no more.

## Who it suits

This site is written for Go developers who are new to models and agents: if you can build and run Go code and are at home with structs, interfaces, errors, and packages, you have what it asks; goroutines and `context.Context` are explained at each use. Nothing about models or machine learning is assumed.
If you already build agents in another language, this site also works as a tour of how each part is done in Go.
If what you need is on the list above, a framework that ships it, or a book on that subject, will serve you better than these pages.
If it isn't, [start here](/start/) and pick the reading path that fits why you came.
