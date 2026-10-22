---
title: Work with me
description: Dayna Blackwell of Blackwell Systems helps teams build, review, measure, and run AI agents in Go. Architecture and production reviews, eval design, AI cost optimization, hands-on team workshops, and AI opportunity assessments.
layout: work
---

I'm Dayna Blackwell, and through Blackwell Systems I help teams build AI agents in Go and run them in production.
The method is the one this guide uses: measure on your own data, say what the evidence can and can't show, and recommend only what holds up.

## What I can help with

**Architecture and production reviews of agent systems.**
A review of an agent you have built or are about to build, covering the loop and its limits, tool design, context management, retries, safety and approval, and what happens when a run crashes halfway through.
You get findings ranked by risk, each traced to code or to a run, and what to change first.

**Eval design and measurement.**
[Evals for an agent whose answers vary](/production/evals/): task sets with tiers, graders written as code, a range around every measured rate, and comparisons that tell a real improvement from chance.
Then you can tell whether the next prompt or model change helped.

**AI cost optimization.**
An audit of what your LLM calls cost and why, across prompt caching, model routing, batch processing, context and tool-output size, and the shape of the agent loop, with every saving measured before and after on your workload.

**A hands-on workshop for your team.**
One or two days in which your engineers build an agent in Go, following the [book's](/book/) path: a loop driven by a scripted model, then a real model with tools that fixes a failing test, then the turn and cost limits, the [stop reasons](/production/stop-reasons/) a caller can check, and the [offline tests](/production/testing-offline/) that together make it safe to run.
The rest of the time goes to the parts your own agent needs most, chosen with you beforehand, such as tool design, evals, approval and sandboxing, or surviving a crash.
Everything runs offline from recorded model replies, so nobody needs an API key to take part, and each engineer leaves with working, tested code of their own to build on.

**AI opportunity assessment.**
A fixed-scope, vendor-neutral read on where AI fits your business and where it doesn't, in plain language, for teams that want a straight answer before they build anything.

[Full list of services](https://blackwell-systems.com/services/), including fractional CTO work and technical due diligence.

## Background

- **The book.** [*Building AI Agents in Go*](/book/), which builds a crash-triage agent from a 30-line loop to production, with every result measured.
- **[bide](https://bide-ai.com)**, a Go framework for durable agents, whose side effects fire at most once across crashes and which keeps a verifiable audit trail of every run.
- **[GCF](https://gcformat.com)**, a data format that cuts the tokens LLMs spend on structured data. It ships in Google's Chrome DevTools [MCP](/build/mcp/) server and Opera's DevTools MCP, and in 28 projects outside Blackwell Systems in all, with more than 580,000 downloads.
- **[agent-lsp](https://agent-lsp.com)**, code intelligence for agents, used by 41 repositories; NVIDIA Labs includes all 24 of its agent skills in Skill2Env, its research collection of agent skills.
- **mcp-assert**, a testing standard for MCP servers, running in CI across 39 of WYRE's server repositories.
- **Upstream work:** a top-10 contributor to mcp-go, with fixes merged into the official MCP SDKs for Go, Python, and PHP, and over 50 pull requests merged in all, into projects from Google, GitHub, and HashiCorp, among others.
- **Research:** 11 published papers, including a theory of when replicated systems converge, with 4,797 results machine-checked in Coq.
- **Production:** more than four years as the technical owner of Best Western's loyalty platform, serving tens of millions of members at thousands of hotels in over 100 countries.
