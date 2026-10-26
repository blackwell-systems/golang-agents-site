---
title: Start here
description: Four reading paths through the site, each an ordered list of guides, for learning how agents work, for shipping one at work, for leading a team that builds them, and for the Go an agent depends on.
layout: page
---

This site is for Go developers who want to build AI agents and understand what happens inside them.
It assumes you can write and run Go code; nothing about models or machine learning is assumed, and none of the examples needs an API key.
New to Go? Finish the [Tour of Go](https://go.dev/tour/) first; [Getting set up](/basics/getting-set-up/#new-to-go) lists what to brush up on.

Pick the path that matches why you're here; every guide on it links to the next step.

<ul class="path-choose"><li><a href="#newcomer">{{art path-learning card}}<span class="choose-title">Learning how agents work</span><span class="choose-desc">You write Go and have never built an agent.</span></a></li><li><a href="#builder">{{art path-shipping card}}<span class="choose-title">Shipping an agent at work</span><span class="choose-desc">You know the loop and have a deadline.</span></a></li><li><a href="#lead">{{art path-leading card}}<span class="choose-title">Leading a team</span><span class="choose-desc">You decide whether a team spends its time here.</span></a></li><li><a href="#go">{{art path-go card}}<span class="choose-title">The Go underneath</span><span class="choose-desc">You know Go from the Tour and want the parts agents lean on.</span></a></li></ul>

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

For a Go developer who has never built an agent: set up, make a first model call, build an agent from scratch, then learn what it costs, how to design its tools, and how to test it. If a guide's Go is unfamiliar, the Go underneath path covers the parts it uses.
```

```path
id: builder
title: Shipping an agent at work
phase: Decide the shape | A workflow or an agent, and what a launch needs.
step: /build/workflows-vs-agents/
step: /production/launch-checklist/
step: /findings/measured-mistakes/
phase: Design it | Tools and context, which decide most of the quality and the cost, and replies in the shape your code needs.
step: /build/tool-design/
step: /build/context-management/
step: /build/structured-output/
step: /build/streaming/
phase: Make it reliable | Named stops, retries, and traces.
step: /production/stop-reasons/
step: /production/retries-and-observability/
phase: Test it | Offline tests, and evals that tell a real change from chance.
step: /production/testing-offline/
step: /production/evals/
phase: Make it safe | Sandboxes, injection defenses, a person's approval, and credentials scoped to one user and one run.
step: /production/prompt-injection/
step: /production/approval/
step: /production/agent-identity/
phase: Run it for real | Runs that survive a crash, a service, and the provider client, libraries, and framework to pick.
step: /production/durable-runs/
step: /production/agent-as-a-service/
step: /ecosystem/providers-and-sdks/
step: /ecosystem/libraries/
step: /ecosystem/go-agent-frameworks/
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
step: /findings/workflow-vs-agent/
phase: The risks | What goes wrong, whose authority an agent acts with, and what a launch needs.
step: /findings/measured-mistakes/
step: /findings/tool-description-poisoning/
step: /production/agent-identity/
step: /production/launch-checklist/
phase: The results | What the finished agent achieved, how it was measured, and where else it applies.
step: /findings/the-finished-agent/
step: /beyond-crash-triage/
step: /production/evals/
step: /findings/identical-runs/
phase: The fit | What's left out, and why Go.
step: /scope/
step: /ecosystem/why-go/

For whoever decides whether a team spends its time here: what the work costs, what the team ends up able to build, the risks and how they're handled, and what's left out, mostly without code.
```

```path
id: go
title: The Go underneath
phase: How the parts fit | Small interfaces, wrappers around them, and tools typed by their arguments: the shapes the rest of the code is built from.
step: /go/interfaces-and-generics/
phase: Control and failure | How a run is stopped, and how every failure is named and routed.
step: /go/context/
step: /go/errors/
phase: Talking to a model | The JSON, the HTTP client, and the streams every model call is made of.
step: /go/json-v2/
step: /go/http-client/
step: /go/streams/
phase: Doing several things at once | Goroutines with an owner, and iterators for streams and runs.
step: /go/goroutines/
step: /go/iterators/
phase: Touching the machine | Running commands and reading files without letting a model escape its workspace.
step: /go/commands-and-files/

For a Go developer who knows the language from the Tour and wants the parts an agent depends on: interfaces and generics, cancellation, errors, JSON, HTTP, streams, concurrency, and files, each shown in tested agent code.
```

## Going further

<div class="start-close">

{{art path-complete wide}}

<div class="start-close-text">

The paths are the shortest routes through the site.
When you finish one, these guides go further, grouped by subject:

- **More tools, more agents:** [Patterns by name](/build/patterns-by-name/), [parallel tool calls](/build/parallel-tool-calls/), [sub-agents](/build/sub-agents/), and [MCP in Go](/build/mcp/), with [an agent that files issues over MCP](/run/ch10-mcp-tracker/) to run.
- **Knowledge and memory:** [RAG in Go](/build/rag/), [RAG or agentic search?](/build/rag-or-agentic-search/), and [memory between sessions](/build/memory/).
- **Providers and frameworks:** [how two provider APIs differ](/ecosystem/differences/), [more providers](/ecosystem/more-providers/), and [one task built three ways](/run/ch16-framework-ports/): by hand, with Eino, and with Genkit.
- **More to run:** [four crash reports, triaged](/run/ch07-crash-triage/), and [an agent's progress streamed to a browser](/run/stream-to-browser/).
- **The measurements:** [Research](/findings/) holds every measured result behind the guides, each with its method, including [910 crashes with no duplicates](/findings/crash-sweep/), [how a tool error's wording changed the turns](/findings/error-wording/), and [fan-out, measured](/findings/fan-out/).

[All guides](/guides/) lists every page, filtered by the stage of the loop you're working on.

</div>

</div>

## Browsing instead

To look something up instead of following a path, start at [All guides](/guides/): every guide on the site, grouped by section, with its skill level, and a filter that shows the guides for one stage of the loop first.
Each section in the header collects the guides on one subject, [search](/search/) finds a page by its title or headings, and the [glossary](/glossary/) defines the vocabulary.
Every runnable program is listed in [Examples](/run/), with the command that runs it offline.
