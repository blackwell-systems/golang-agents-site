---
title: Start here
description: Three reading paths through the site, each an ordered list of guides, for learning how agents work, for shipping one at work, and for leading a team that builds them.
layout: page
---

This site is for Go developers who want to build AI agents and understand what happens inside them.
It assumes you can write and run Go code; nothing about models or machine learning is assumed, and none of the examples needs an API key.
New to Go? Finish the [Tour of Go](https://go.dev/tour/) first; [Getting set up](/basics/getting-set-up/#new-to-go) lists what to brush up on.

Pick the path that matches why you're here; every guide on it links to the next step.

<ul class="path-choose"><li><a href="#newcomer">{{art path-learning card}}<span class="choose-title">Learning how agents work</span><span class="choose-desc">You write Go and have never built an agent.</span></a></li><li><a href="#builder">{{art path-shipping card}}<span class="choose-title">Shipping an agent at work</span><span class="choose-desc">You know the loop and have a deadline.</span></a></li><li><a href="#lead">{{art path-leading card}}<span class="choose-title">Leading a team</span><span class="choose-desc">You decide whether a team spends its time here.</span></a></li></ul>

```path
id: newcomer
title: Learning how agents work
phase: Get oriented | Set up, and see what an agent is before any model is involved.
step: /basics/getting-set-up/
step: /basics/what-is-an-agent/
step: /run/ch01-scripted-loop/
step: /basics/agent-loop/
phase: Talk to a model | Make a first call, and learn the shape of every request and reply.
step: /basics/first-model-call/
step: /basics/messages-and-roles/
step: /basics/calling-a-model/
step: /basics/common-errors/
phase: Build an agent | Give the model tools, watch a real agent work, then write your own.
step: /basics/tool-calling/
step: /run/ch04-first-agent/
step: /build/first-agent/
phase: Understand the model | What each turn costs, how a model picks its words, and which model to use.
step: /basics/tokens-and-cost/
step: /basics/how-models-work/
step: /basics/choosing-a-model/
phase: Make it dependable | Tools a model uses well, stopping safely, the right shape, and tests.
step: /build/tool-design/
step: /production/stop-reasons/
step: /build/workflows-vs-agents/
step: /production/testing-offline/
phase: See where it leads | A finished agent, measured on real tasks.
step: /findings/the-finished-agent/

For a Go developer who has never built an agent: set up, make a first model call, build an agent from scratch, then learn what it costs, how to design its tools, and how to test it.
```

```path
id: builder
title: Shipping an agent at work
phase: Decide the shape | A workflow or an agent, and what a launch needs.
step: /build/workflows-vs-agents/
step: /production/launch-checklist/
step: /findings/measured-mistakes/
phase: Design it | Tools and context, which decide most of the quality and the cost.
step: /build/tool-design/
step: /build/context-management/
phase: Make it reliable | Named stops, retries, and traces.
step: /production/stop-reasons/
step: /production/retries-and-observability/
phase: Test it | Offline tests, and evals that tell a real change from chance.
step: /production/testing-offline/
step: /production/evals/
phase: Make it safe | Sandboxes, injection defenses, and a person's approval.
step: /production/prompt-injection/
step: /production/approval/
phase: Run it for real | Runs that survive a crash, a service, and the libraries to pick.
step: /production/durable-runs/
step: /production/agent-as-a-service/
step: /ecosystem/libraries/
phase: See the result | The finished agent, measured.
step: /findings/the-finished-agent/

For an engineer who knows the loop and has a deadline: decide between a workflow and an agent, then work down the launch checklist, one guide per item.
```

```path
id: lead
title: Leading a team
phase: The case | What the work costs, and when an agent is the right tool.
step: /for-leads/
step: /build/workflows-vs-agents/
phase: The risks | What goes wrong, and what a launch needs.
step: /findings/measured-mistakes/
step: /production/launch-checklist/
phase: The results | What the finished agent achieved, how it was measured, and where else it applies.
step: /findings/the-finished-agent/
step: /beyond-crash-triage/
step: /production/evals/
phase: The fit | What's left out, and why Go.
step: /scope/
step: /ecosystem/why-go/

For whoever decides whether a team spends its time here: what the work costs, what the team ends up able to build, the risks and how they're handled, and what's left out, mostly without code.
```

## At the end of a path

{{art path-complete wide}}

When you reach the last step of a path, the [Research](/findings/) section holds every measured result behind the guides, each with its method.
The guides on knowledge and memory go past what the paths and the book cover: [RAG in Go](/build/rag/), [RAG or agentic search?](/build/rag-or-agentic-search/), and [memory between sessions](/build/memory/).

## Browsing instead

If you'd rather look something up, each section in the header collects the guides on one subject, [search](/search/) finds a page by its title or headings, and the [glossary](/glossary/) defines the vocabulary.
Every runnable program is listed in [Examples](/run/), with the command that runs it offline.
