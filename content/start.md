---
title: Start here
description: Three reading paths through the site, each an ordered list of guides, for learning how agents work, for shipping one at work, and for leading a team that builds them.
layout: page
---

This site is for Go developers who want to build AI agents and understand what happens inside them.
It assumes you can write and run Go code; nothing about models or machine learning is assumed, and none of the examples needs an API key.

Pick the path that matches why you're here; every guide on it links to the next step.

<ul class="path-choose"><li><a href="#newcomer"><span class="choose-title">Learning how agents work</span><span class="choose-desc">You write Go and have never built an agent.</span></a></li><li><a href="#builder"><span class="choose-title">Shipping an agent at work</span><span class="choose-desc">You know the loop and have a deadline.</span></a></li><li><a href="#lead"><span class="choose-title">Leading a team</span><span class="choose-desc">You decide whether a team spends its time here.</span></a></li></ul>

```path
id: newcomer
title: Learning how agents work
step: /basics/getting-set-up/
step: /basics/what-is-an-agent/
step: /run/ch01-scripted-loop/
step: /basics/agent-loop/
step: /basics/first-model-call/
step: /basics/messages-and-roles/
step: /basics/calling-a-model/
step: /basics/common-errors/
step: /basics/tool-calling/
step: /run/ch04-first-agent/
step: /build/first-agent/
step: /basics/tokens-and-cost/
step: /basics/how-models-work/
step: /basics/choosing-a-model/
step: /build/tool-design/
step: /production/stop-reasons/
step: /build/workflows-vs-agents/
step: /production/testing-offline/
step: /findings/the-finished-agent/

For a Go developer who has never built an agent: set up, make a first model call, build an agent from scratch, then learn what it costs, how to design its tools, and how to test it.
```

```path
id: builder
title: Shipping an agent at work
step: /build/workflows-vs-agents/
step: /production/launch-checklist/
step: /findings/measured-mistakes/
step: /build/tool-design/
step: /build/context-management/
step: /production/stop-reasons/
step: /production/retries-and-observability/
step: /production/testing-offline/
step: /production/evals/
step: /production/prompt-injection/
step: /production/approval/
step: /production/durable-runs/
step: /production/agent-as-a-service/
step: /ecosystem/libraries/
step: /findings/the-finished-agent/

For an engineer who knows the loop and has a deadline: decide between a workflow and an agent, then work down the launch checklist, one guide per item.
```

```path
id: lead
title: Leading a team
step: /for-leads/
step: /build/workflows-vs-agents/
step: /findings/measured-mistakes/
step: /production/launch-checklist/
step: /findings/the-finished-agent/
step: /beyond-crash-triage/
step: /production/evals/
step: /scope/
step: /ecosystem/why-go/

For whoever decides whether a team spends its time here: what the work costs, what the team ends up able to build, the risks and how they're handled, and what's left out, mostly without code.
```

## Browsing instead

If you'd rather look something up, each section in the header collects the guides on one subject, [search](/search/) finds a page by its title or headings, and the [glossary](/glossary/) defines the vocabulary.
Every runnable program is listed in [Examples](/run/), with the command that runs it offline.
