---
title: Go libraries and SDKs the book uses or compares
short: Libraries and SDKs
description: The provider SDKs, the MCP SDK, OpenTelemetry, and the five Go agent frameworks that the book's code imports or measures, each with its pinned version, what it does well, what it costs, and the part of a harness it stands in for.
card: Every SDK and framework the book's code uses, with its version, its strengths, and its costs.
chapters: 4, 10, 11, 12, 16
date: 2026-10-18
weight: 30
art: ch16
---

The book's agent is built on the standard library through chapter 10, and every part it builds by hand has a library that does the same job.
That is a choice made for teaching: you can only judge what a library decides for you once you have seen the decisions written out, and the libraries below make many of them well.
This page lists every library the companion code imports, and every one the book compares against, with the version that was checked and what the book found.

Versions and counts were checked in October 2026, from the modules' source and the companion code's `go.mod` files.
Libraries at this stage change quickly, so read a version here as the one the book used and check the current release before you choose.

## What the code imports

The main module, `code/go.mod`, gains its first dependency in chapter 11.
Everything else that needs a library lives in a nested module, a directory with its own `go.mod`, so its dependencies never reach the agent's packages.

| Library | Version | Where | Stands in for |
| --- | --- | --- | --- |
| OpenRouter's Go SDK, `github.com/OpenRouterTeam/go-sdk` | v0.9.59 | `code/go.mod`, chapter 11 | Chapter 2's hand-written chat completions client |
| The MCP Go SDK, `github.com/modelcontextprotocol/go-sdk` | v1.8.0 | `code/ch10/sdk`, its own module | Chapter 10's hand-written MCP server |
| OpenTelemetry for Go, `go.opentelemetry.io/otel` and its SDK | v1.47.0 | `code/ch12/otel`, its own module | Nothing: it carries chapter 12's spans to a tracing system |
| Eino, with its OpenAI model adapter from `eino-ext` | v0.9.21 (adapter v0.1.13) | `code/ch16`, its own module | The loop, client, and approval pause, for one task |
| Genkit for Go, with `openai-go` v1.8.2 | v1.13.1 | `code/ch16` | The same |

### OpenRouter's Go SDK

It sends the same chat completions requests the book's own client sends, which is why chapter 11 uses it: the lesson is the adapter, the code that sits between a library you don't control and the `Model` interface your loop calls.
Two details make it fit.
It accepts any value with a `Do` method as its HTTP client, so the replay transport that runs every example offline works with it unchanged; and its retries can be turned off, so retrying stays in one place, chapter 12's middleware.

The cost is churn.
The SDK is below 1.0 and generated from an API description, so its types can change in a minor release.
Pin it, and read what changed before moving the pin.
The finished adapter is 327 lines of Go, leaving out comments and blank lines, and about half of them convert between the SDK's types and the agent's (chapter 16 counts them).

### The MCP Go SDK

The official implementation of both sides of MCP.
Chapter 10 writes a client and a server with the standard library first, then rebuilds the server on the SDK.
What the SDK does well is the tool boundary: `mcp.AddTool` builds the schema from the arguments' Go type, using struct tags for the descriptions, and checks each call against it before your handler runs.
The book's own registry does the same job by hand from chapter 7 on.

### OpenTelemetry

Chapter 12's recorder writes spans for every model call, attempt, and tool call; sending them to a tracing system takes one function, the only code in the book that imports OpenTelemetry.
Most tracing systems read its format, and its semantic conventions for generative AI name the model, token, and cost attributes so a tool can display them without knowing your code.
Those conventions are still changing, which the chapter flags; keep the attribute names in one place.

## What the book compares

Two more libraries come up beside the parts the book writes by hand:

- **`anthropic-sdk-go`**, Anthropic's Go SDK. Its tool runner, in beta when chapter 4 was written, builds a tool's schema from struct tags; its `message.Accumulate` rebuilds a streamed reply, the job chapter 11's stream state does by hand. Chapter 11 calls the Messages API with a client written over `net/http`, which leaves each difference between the two formats visible in code you wrote.
- **`invopop/jsonschema`**, a third-party schema generator that tool runners use to turn a Go struct into a JSON schema. The book writes schemas by hand in Part 1, then writes its own generator in chapter 7, which reads a struct's tags through `reflect`.

## The frameworks

Chapter 16 sets five Go agent frameworks against the parts the book builds, and ports one piece of the agent to two of them.
The five, with the version checked and the number of modules each requires directly:

| Framework | Version checked | Built around | License | Direct module requirements |
| --- | --- | --- | --- | --- |
| ADK for Go | v1.9.0 | an agent and a runner | Apache-2.0 | 36 |
| Eino | v0.9.21 | a graph | Apache-2.0 | 11 |
| Genkit for Go | v1.13.1 | a flow and `Generate` | Apache-2.0 | 40 |
| LangChainGo | v0.1.15 | a chain and an executor | MIT | 65 |
| trpc-agent-go | v1.11.2 | an agent, a runner, and a graph | Apache-2.0 | 37 |

What they do well, from the chapter:

- **Provider breadth.** Genkit has plugins for Anthropic's, Google's, and Ollama's APIs as well as OpenAI-compatible ones, and Eino's extensions cover many more models. Writing one adapter by hand is chapter 11's whole subject.
- **Development UIs.** ADK and Genkit ship a UI for watching a run and replaying a step; Eino has an IDE plugin.
- **Tracing, ready to use.** ADK, Genkit, and trpc-agent-go emit OpenTelemetry spans; Eino has handlers for Langfuse and LangSmith.
- **Graphs and multi-agent patterns**, which the book leaves out.

What they cost, measured on the ported task (linux/amd64, October 2026):

```result
figure: 10.6 / 25.4 / 29.3
unit: MB binaries
task: the same slice of the triage agent (diagnose one crash group, then pause for a person's approval) built three ways: on the book's harness, on Eino, and on Genkit
models: scripted replies for the comparison, plus one live run of each on `anthropic/claude-haiku-5.5`
runs: one live run per version; the counts come from the code
date: October 2026
source: Chapter 16

Under the two framework versions sit code from 27 other modules for Eino, 97,076 lines, and 29 for Genkit, 132,201 lines. The book's version rests on 5,351 lines you wrote yourself, about 1,500 of them safety code that neither framework provides.
```

Neither number settles anything by itself.
Lines maintained by someone else are tested on cases you'll never meet and cost you nothing to write; they are also code you didn't choose, upgraded on someone else's schedule.
The binary size rarely matters for a long-running service and can matter for a tool people download.

## Choosing

Chapter 16 ends on three answers rather than one:

- **Build** the parts yourself when the agent is what you sell, when you have to decide exactly what is sent and how each failure is handled, or when a small loop would otherwise bring in a framework's worth of modules.
- **Adopt** a framework when its strengths are your needs (several providers, a UI for development, graphs) and its defaults match what you would have chosen.
- **Adopt part of one** behind your own interfaces: an SDK behind your `Model` interface, as chapter 11 does; an exporter behind your tracing middleware, as chapter 12 does.

Two questions rule a framework out for an agent that runs unattended and spends money: can you set a turn limit, and can you pass your own `*http.Client`, without which you can't record and replay its traffic for offline tests.
As checked, all five pass the second; ADK for Go lacks a turn limit and needs one added, and trpc-agent-go's two limits stay off until you turn them on.
