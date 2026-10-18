---
title: What this guide and the book cover, and what they leave out
short: Scope
description: The ground this site and Building AI Agents in Go cover, the topics both leave out on purpose, such as retrieval, memory across runs, and multi-agent planning, and why, so you can tell before reading whether they answer your question.
layout: page
---

A guide is easier to trust when it says where it stops.
This page lists what the site and the book behind it cover, and the topics they leave out on purpose, with the reason for each.

## What they cover

One agent, built in Go from an empty loop to a service you can leave running, in the order you need the pieces:

- **[The loop](/loop/):** what an agent is, the request and the reply, tool calls on the wire, tokens and cost, streaming, and what happens inside the model, from tokens to sampling.
- **[Design](/design/):** tools a model uses well, answers in a fixed shape, choosing a workflow or an agent, keeping a long run's context in check, parallel tool calls, sub-agents, and MCP on both sides.
- **[Production](/production/):** stop reasons and budgets, retries, traces and cost, offline tests and evals, sandboxing and prompt injection, a person's approval, and runs that survive a crash. The [launch checklist](/production/launch-checklist/) sums these up.
- **[Providers and frameworks](/providers/):** the chat completions API and Anthropic's Messages API in depth, official SDKs, and five Go agent frameworks compared on one task.
- **[Findings](/findings/):** measured results from the book's recorded runs, each with its method, including [the mistakes](/findings/measured-mistakes/).

The book goes further than the site on every topic: the full build with its tests, the reasoning behind each decision, and the exercises.
The site answers one question per page and points at the code.

## What they leave out, and why

**Retrieval (RAG), embeddings, and vector stores.**
The book's agent finds related crashes by comparing the functions in their stack traces, not by embedding them.
Chapter 8 explains the choice: the frames every stack shares make up most of each trace, so embeddings would place most crashes close together, while matching functions costs less, needs no network, and its accuracy can be measured on the crash catalog.
Retrieval is a large subject of its own, and neither the site nor the book covers it.

**Memory across unrelated runs.**
Within a run, the agent keeps notes per crash group and a context policy decides what each request holds (chapter 8); across a crash, a checkpoint restores the run (chapter 15); the finished service shares an issue record between runs (chapter 17).
A store the agent writes to and recalls from across unrelated tasks, with rules for what to forget, isn't built.

**Multi-agent planning.**
Chapter 9 uses sub-agents for questions that would fill the parent's context, and starts one agent per crash group from code.
A model that plans the work at run time and hands it to other agents is orchestration, a different kind of program with failure modes of its own, and the book stops before it on purpose; chapter 9 says so, and chapter 5's table of shapes names it as the last step up.
Chapter 16 notes that some Go frameworks offer graphs and multi-agent patterns, if you need them now.

**A user interface.**
The agent is a command and an HTTP service with a review queue for approvals.
Chapter 6 streams its work as events in Go; showing them in a browser, or wiring the agent into a chat tool, is left to you.

**Fine-tuning and training.**
The book works with models as they come from a provider's API.
Chapter 3 builds a tiny language model to explain how one picks its next token, as background, not as a way to train your own.

**Deployment beyond a container.**
Chapter 15 ships the agent as a static binary in a container and handles a platform's stop signal and grace period; serverless platforms and managed agent runtimes aren't covered.

**Being a manual for what the agent touches.**
HTTP, JSON, concurrency, and machine learning each get the explanation a step needs, with pointers for going further, and no more.

## Who it suits

The book is written for Go developers who are new to models and agents: if you can build and run Go code and are at home with structs, interfaces, errors, and packages, you have what it asks; goroutines and `context.Context` are explained at each use. Nothing about models or machine learning is assumed.
If you already build agents in another language, the book also works as a tour of how each part is done in Go.
If what you need is on the list above, a framework that ships it, or a book on that subject, will serve you better than this one.
