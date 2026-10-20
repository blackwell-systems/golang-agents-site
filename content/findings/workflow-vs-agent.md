---
title: The same bug fix as a workflow cost a quarter to a sixth as much as the agent
short: Workflow or agent, measured
description: Chapter 1 fixed one Go bug on four models in two ways, as a three-step workflow written in code and as an agent that picks its own steps. The workflow was cheaper and faster on every model, and its single failure shows what it trades away.
card: One Go bug fixed by a three-step workflow and by an agent, on four models: cost, speed, failures.
chapters: 1
stage: 4
date: 2026-10-18
weight: 30
art: ch01
---

```result
figure: ¼ to ⅙
unit: of the agent’s cost
task: the `duration` bug fix, as a three-step workflow and as an agent with four tools
models: `anthropic/claude-haiku-5.5`, `openai/gpt-5.6-luna`, `z-ai/glm-5.3-flash`, `deepseek/deepseek-v4.1-flash`
runs: ten per model for each version; costs are medians
date: October 2026
source: Chapter 1, and chapter 5’s table of shapes

Written as a fixed three-step workflow, the bug fix cost a quarter to a sixth as much as the agent on each of four models, and its one model call took between under a second and 2.5 seconds, while each agent run took 7.5 to 13.5 seconds.
```

In an [agent](/basics/what-is-an-agent/), the model picks every next step.
In a [workflow](/glossary/#workflow), your code has already picked the steps, and the model is called at the places the code chose.
A common rule of thumb says to prefer a workflow whenever you know the steps.
That rule was checked on a task small enough to build both ways.

## What was tested

The task: a small Go package, `duration`, has failing tests because its unit table maps `'m'` to `time.Second`.

**The agent** is the loop from [the agent loop in Go](/basics/agent-loop/) with a real model client, given four tools (`list_files`, `read_file`, `edit_file`, `run_tests`) and left to find its own way.

**The workflow** is three steps written into the program:

1. Run the tests, and read the package file and its test file.
2. Put all of that into a single request and ask the model for one edit, in JSON.
3. Apply the edit, then run the tests a second time.

Each version ran ten times on each of four inexpensive models in October 2026: `anthropic/claude-haiku-5.5`, `openai/gpt-5.6-luna`, `z-ai/glm-5.3-flash`, and `deepseek/deepseek-v4.1-flash`.

## The result

Costs are medians of ten runs:

| Model | Workflow: tests pass | Workflow: cost | Agent: tests pass | Agent: cost |
| --- | --- | --- | --- | --- |
| `anthropic/claude-haiku-5.5` | 9 of 10 | $0.00013 | 10 of 10 | $0.00080 |
| `openai/gpt-5.6-luna` | 10 of 10 | $0.00022 | 10 of 10 | $0.00089 |
| `z-ai/glm-5.3-flash` | 10 of 10 | $0.00018 | 10 of 10 | $0.00076 |
| `deepseek/deepseek-v4.1-flash` | 10 of 10 | $0.00019 | 10 of 10 | $0.00116 |

On every model, the workflow's cost was between a quarter and a sixth of the agent's, and it finished several times sooner.
The gap comes from the shape of the two programs.
The agent made four to six model calls, and each one resent the conversation so far along with about 270 tokens of tool definitions, plus whatever the provider adds around them ([tokens and cost](/basics/tokens-and-cost/) shows how that adds up).
The workflow made a single call and offered no tools.
On this bug, the workflow is the better choice.

## The one failure

One workflow run failed, on `anthropic/claude-haiku-5.5`.
Go source is indented with tabs, and the model put a literal tab character inside a JSON string, which JSON forbids, so the edit could not be decoded.
Nothing in the three steps allowed a second try, so the run ended there.

An agent handles the same mistake differently.
Its loop would have sent the decoding error back to the model, which could then have corrected its call on the following turn.
The loop's argument decoder writes the reason and an example of the expected shape into that error:

```go include=ch01/agent/tools.go tag=decode
```

That one run shows the trade.
The workflow is cheap because a person fixed the steps in advance, and it is fragile because nothing in the program handles a model reply it can't decode.

## What it can't show

The task is easy on purpose, and that is what lets the comparison work at all.
The workflow succeeds because its author already knew what the bug needed: the package file, its test, and a single edit.
Move the cause into a third file, or make it a bug you can only find by running the tests, and the three steps no longer match the problem.
Ten runs per cell are enough to show a four- to six-fold cost difference, but far too few to measure failure rates: one failure in forty runs doesn't support an estimate.
The task also can't tell the models apart, since every agent run passed in four to six turns.
To compare models, or to learn whether a change to an agent helped, you need tasks that agents sometimes fail.

## What to do with it

- **Begin with the steps you already know.** If you can list them, build a workflow. It costs less, runs faster, and is simpler to test, because every run follows the same path.
- **Use an agent when each step depends on what the previous one found.** That dependence is what the agent's extra calls pay for. A crash whose cause sits far from where it showed up, or a problem where you must collect evidence before you know what to read, won't fit a fixed sequence.
- **Combine the two.** Most real systems fall somewhere between. A support system, for instance, can send each ticket down a workflow and pass only the odd cases to an agent.
- **Let a workflow retry its model step.** Its one failure here was a decoding error that a second attempt could have fixed, and an agent's loop already has that retry built in.

## Try it

The workflow and agent runs behind the table are in the repository's `evidence` directory, and `go run ./evidence`, with no key set, prints the table again from them. The agent is the [`ch04` program](/run/ch04-first-agent/) in the companion code.
From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go run ./ch04
```

This replays the agent's recorded run on the default model, `anthropic/claude-haiku-5.5`, runs `go test` for real, and prints the cost: $0.0008 across four model calls.
