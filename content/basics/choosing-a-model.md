---
title: "Choosing a model for an AI agent"
short: Choosing a model
description: What an agent needs from a model, how to read a model ID, why the price per token misleads and the cost per finished task doesn't, what reasoning models add and cost, when a local model makes sense, and how to compare candidates on your own task.
card: Tool support, cost per finished task, reasoning models, local models, and how to compare.
chapters: 2, 11, 13, 17
date: 2026-10-19
weight: 70
---

There are hundreds of models to choose from, and the leaderboards change every month.
An agent narrows the choice quickly, because it needs a few specific things from a model, and because the one number that matters, what it costs to finish a task, only comes from running yours.

## Reading a model ID

On OpenRouter a model is named by the company that made it and the model's name, such as `anthropic/claude-haiku-5.5` or `deepseek/deepseek-v4.1-flash`.
The companion code keeps that ID in one setting, `OPENROUTER_MODEL`, so trying another model needs no code change.

One model can be served by several hosting companies, and OpenRouter picks one per call.
The reply's `provider` field says which.
Hosts of the same model can differ in speed and in price, so for measurements you want to compare, pin the host as well as the model.

## What an agent needs

- **Tool calling.** An agent works by asking for tools, so the model has to support them. OpenRouter's model list can be filtered to models that do, and each model's page lists the parameters it accepts.
- **Room for a whole run.** Every request carries the conversation so far, and an agent's conversation grows with each turn. The model's context window has to hold the longest run you expect, with room to spare; [tokens, cost, and context](/basics/tokens-and-cost/) explains why the advertised size is a poor planning number.
- **Reliable tool calls.** A model that sends malformed arguments, calls tools that don't exist, or stops early costs turns. This is the quality the leaderboards measure least and your agent depends on most.

## Price per token misleads

A model's price is quoted per million tokens, in and out.
An agent's bill is that price times every token of every request in a run, and a run takes as many turns as the model needs.
A model that is cheaper per token but takes twice as many turns can cost more per finished task.

A crash-triage agent was measured on exactly this, with three inexpensive models, 20 tasks each, in October 2026:

| Model | Tasks passed | Cost per passing run | Median turns |
| --- | --- | --- | --- |
| `anthropic/claude-haiku-5.5` | 70% | $0.0165 | 14 |
| `deepseek/deepseek-v4.1-flash` | 63% | $0.0531 | 26 |
| `openai/gpt-5.6-luna` | 20% | $0.1022 | 18 |

DeepSeek's tokens cost less than Haiku's, yet each passing run cost three times as much, because it took about twice the turns.
[The finished agent](/findings/the-finished-agent/) has the ranges and the method behind these numbers.
The figure to compare is the cost of a finished task, counting the runs that failed.

## Reasoning models

Some models write out their reasoning before they answer.
On hard steps that can help; on simple tool calls it mostly adds time and cost.

- **Reasoning is billed as output.** The reply's `usage` reports it in `completion_tokens_details.reasoning_tokens`.
- **It shares the reply limit.** A low `max_tokens` can be used up entirely by reasoning, leaving no answer; [common first errors](/basics/common-errors/) shows a recorded case.
- **It may have to go back.** Some providers return the reasoning in a field your program must send back unchanged with the next request, to the same provider, or the model loses its thread. [Providers and SDKs](/ecosystem/providers-and-sdks/) covers how the companion code does that.

Where a model lets you set how much it may reason, cap it, and check the usage to see that the cap held.

## Local models

[Ollama](https://ollama.com) and similar servers run open models on your own machine and accept the same chat completions requests, at an address such as `http://localhost:11434/v1`.
The companion code's client takes a base URL, so pointing it at a local server is a configuration change.

A local model costs nothing per call and keeps your data on your machine.
The models small enough to run on a laptop are also weaker at choosing and formatting tool calls, so an agent that works on a hosted model may stumble on a local one.
Measure before you switch.

## How to choose

1. Pick two or three models that support tools and fit your budget.
2. Run your own task several times on each. One run per model tells you very little, since the same agent on the same task can pass once and fail the next time.
3. Compare the share of runs that succeeded and the cost per successful run.
4. Keep the cheapest model that does the job reliably, and measure again when a new model appears or a price changes.

[Evals for an agent](/production/evals/) turns these steps into a repeatable test, and [two identical runs, 15 points apart](/findings/identical-runs/) shows how far a single run can mislead.
