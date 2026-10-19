---
title: What an AI agent is, in terms a Go programmer can build
short: What an agent is
description: An agent is a program where the model picks each next step and your Go code carries it out. This page follows one agent through a failing test, compares it with chatbots and workflows, and lists the parts you write.
card: What makes a program an agent, followed through one run that fixes a failing test.
chapters: 1
stage: all
date: 2026-10-18
weight: 10
art: ch01
---

"Agent" now gets attached to nearly anything that calls a model.
A Go programmer needs a definition narrow enough to implement and test, so here is the one this site uses: an agent is a program that lets a model pick its next action, one turn at a time, looking at the results of the actions so far, and keeps going until the job is finished or a limit ends the run.

Rather than argue for that definition, this page shows one agent working and then names the pieces you would write to build it.

## One agent, four turns

Start with a tiny Go package, `duration`, whose job is turning a string such as `"1h30m"` into a `time.Duration`.
Two of its tests fail.
The agent is told that the tests fail and asked to find the cause and fix it, and it has four tools to work with: `list_files`, `read_file`, `edit_file`, and `run_tests`.

A recorded run has `anthropic/claude-haiku-5.5` doing exactly this in October 2026.
Each row below is one round trip to the model:

| Turn | The model asked for | The program did |
| --- | --- | --- |
| 1 | `run_tests`, `list_files` | Ran the tests (two failures), listed the two files |
| 2 | `read_file` on both files | Sent back their contents |
| 3 | `edit_file`: `'m': time.Second` becomes `'m': time.Minute`; then `run_tests` | Made the edit; the tests pass |
| 4 | Nothing: an answer explaining the typo | Stopped and showed the answer |

Look at the middle column: it holds nothing but requests.
The model has no access to the files and never could have, because it is a service on someone else's computer that receives text and replies with text.
The right-hand column is where things happen, and all of it is your Go program: the code that gives `read_file` its meaning, runs the tests, applies the edit, and notices that the job is over.

One run proves little, so the task was repeated 40 times: ten runs each on `anthropic/claude-haiku-5.5`, `openai/gpt-5.6-luna`, `z-ai/glm-5.3-flash`, and `deepseek/deepseek-v4.1-flash`.
All 40 fixed the bug, each in four to six turns, and no run cost more than a fifth of a cent.
That price is counted in [tokens](/glossary/#token), the pieces of text a model reads and writes: a common word, a fragment of a longer word, or a punctuation mark.
A provider charges for every token in the request and every token in the reply, at a price per million.
The routes differed more than the outcomes.
Two of the models repeated an identical sequence of calls in every run; the other two wandered by a turn.
In some order, every run surveyed the project, read the source, edited it, and ran the tests to confirm.

## Three ways to use a model

The useful question for telling model programs apart is: who chooses the next step?

| Shape | Who decides the next step | Example |
| --- | --- | --- |
| Chatbot | The person, one reply at a time | A chat window that answers questions |
| [Workflow](/glossary/#workflow) | Your code, at points fixed in advance | Summarize each support ticket, then route it by the summary |
| [Agent](/glossary/#agent) | The model, on each turn, from what the last step returned | The bug fix above |

Nothing in the bug-fix program told the model to open the test file once it had seen the file list.
The listing prompted that choice, and the test output prompted the edit.
That is what puts it in the third row.
The chatbot, workflow, and agent split is the one Anthropic draws in "Building effective agents", and most real systems mix the shapes: a ticket system can send all tickets through a fixed workflow and pass only the odd cases to an agent.

Often the workflow is the right answer.
If you already know the steps, fixing them in code means fewer model calls, the same path on every run, lower cost, and tests that are easy to write.
An agent pays for itself only when the step after this one can't be known until this one has run.

## The model only asks

Simon Willison defines an LLM agent as something that runs tools in a loop to achieve a goal.
That is accurate as far as it goes, but the model runs nothing.
It produces requests, and some program has to carry them out, hold on to the conversation, and judge when the work is finished.
That program is the [harness](/glossary/#harness), and it does two jobs:

- **Run the tools the model asks for**, safely, and send back each result.
- **Decide what the model sees on every call.** A model remembers nothing from one call to the next. Every request has to include the conversation so far, so whatever you put in the request is the model's entire knowledge of the task. It also means every turn's request is bigger than the last, and every token of it is billed again; [Tokens, cost, and the context window](/basics/tokens-and-cost/) follows that growth through a run.

The second job is every bit as important as the first.
Later pages cover it under the name context management: deciding, as a run gets longer, what stays in the request, what gets cut, and what gets summarized.

## Tools have two sides

A [tool](/glossary/#tool-and-tool-call) faces two directions.
Toward the model it is a name and a description, plus a JSON Schema for its arguments.
Toward your program it is a Go function, which the model never sees.
The companion code keeps both halves in a single value:

```go include=ch01/agent/tools.go tag=read-file
```

From this, the model learns that a tool called `read_file` exists, what it is for, and what arguments it takes, so it can ask for it with something like `{"path": "duration.go"}`.
When that request arrives, `Run` unpacks the JSON into a struct and opens the file.
Arguments that fail to unpack produce an error that shows a correct example, which gives the model what it needs to send a better call next time.

Tag every struct field that a model fills with an explicit `json` name.
The companion code uses `encoding/json/v2`, part of the standard library in Go 1.27, and it matches names case-sensitively: without a tag, `"path"` does not land in `Path`, decoding reports no error, and the tool receives an empty path.

## The parts you write

An agent in Go comes down to a handful of plain pieces:

- **A model interface** with one method, `Generate(ctx, req)`, so a scripted fake and a real HTTP client are interchangeable.
- **Messages**: the conversation as a slice, each with a role (`system`, `user`, `assistant`, or `tool`).
- **Tools**: a definition the model reads and a function the harness calls.
- **The loop**: assemble a request, send it, act on any tool calls in the reply, then either finish or take another turn. [The agent loop in Go](/basics/agent-loop/) shows it in about 30 lines.
- **Limits**: a cap on turns from the very first version, and later a cap on spending, since nothing else stops a loop that never gets an answer.

You don't need a framework for any of this, and you don't need a model to test it.
The loop in the companion code runs against a scripted list of replies, so its rules are checked by tests that finish in milliseconds and never touch the network.
Frameworks wrap the same loop with extra features, and they are easier to judge once you have built the parts yourself.

## When not to build one

Each turn is a network call that takes seconds and is billed.
An early mistake carries forward, since every later turn builds on what the model concluded from it.
And the same task can follow a different route on a different run, so an agent is harder to test than a function that always does the same thing.
When you can list the steps before the program starts, build a workflow.
Use an agent for problems where you only learn the next step by taking the current one.

## Try it

You need Go {{code go-version}}; a `go` command from 1.21 on downloads that toolchain the first time it reads the module's `go.mod`, so the first run pauses for a moment.
From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go run ./ch01
```

This plays back the run from the table through a loop short enough to read in one sitting, with no API key and no network.
The [ch01 example](/run/ch01-scripted-loop/) explains the output line by line.
