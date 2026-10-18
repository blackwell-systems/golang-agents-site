# Content plan

The information architecture of golang-agents.com: every planned page, with its URL, its title, the search question it answers, the chapters of *Building AI Agents in Go* it draws on, and the public code it links to (paths under `code/` in [go-agents-book-code](https://github.com/blackwell-systems/go-agents-book-code)).

The build skips this file; it is not a page.

## Rules for every page

- One question, answered well, in 800 to 1,500 words of original prose. Never paste chapter text or long passages: a page teaches the idea and shows one result, and the book has the full build, the tests, and the depth.
- Code comes only from the public repository, through an include block (`include=` and `tag=`), so every excerpt is tested code and its caption links to the exact lines. Excerpts stay small: one or two per page, under about 30 lines each.
- Every number comes from the book or the code, with its chapter cited. No new measurements.
- The prose rules of the book apply: plain and specific, no em dashes, no hype, none of the banned words, nothing disparaging about other languages or frameworks.
- The book appears in the nav, on its own page, and in one line at the end of each guide. Nowhere else.
- Status: **published** pages are built; **draft** pages (`draft: true` in front matter) are written and kept on disk but left out of the build until the author has reviewed them. As of 2026-10-18 the first batch is published: home, book, glossary, the section pages, the agent loop, tool calling, the 79-of-80 finding, and the ch01 and ch04 examples, plus Work with me.

## Pages that aren't guides

| URL | Title | Status |
| --- | --- | --- |
| `/` | AI Agents in Go (home) | published |
| `/book/` | Building AI Agents in Go (the book) | published |
| `/glossary/` | Glossary | published |
| `/404.html` | Page not found | published |
| `/loop/`, `/design/`, `/production/`, `/run/`, `/providers/`, `/findings/` | Section pages, listing their guides | published |

### The glossary: one page with anchors

The glossary is one page, `/glossary/`, with an anchor per term (`/glossary/#harness`), not one page per term.
Most entries are one or two sentences: as separate pages they would be thin pages that search engines rank poorly and readers leave at once, while one page of about 80 short definitions is a reference people bookmark and scan.
Guides link terms to their anchors, so each definition is still one click from where it is used, and a search for a term lands on the page that holds it and its neighbors.
If a term later earns a full explanation, it gets a guide, and its glossary entry links to it.

## The loop (`/loop/`, formerly Foundations)

| URL | Title | Search question | Chapters | Code | Status |
| --- | --- | --- | --- | --- | --- |
| `/loop/what-is-an-agent/` | What an AI agent is, in terms a Go programmer can build | what is an AI agent; agent vs chatbot vs workflow | 1 | `ch01/agent/types.go` | draft |
| `/loop/agent-loop/` | The agent loop in Go | how to write an agent loop in Go | 1, 4, 5 | `ch01/agent/loop.go`, `ch04/main.go` | published |
| `/loop/calling-a-model/` | Calling a model's API from Go with net/http | call an OpenAI-compatible chat completions API from Go without an SDK | 2 | `ch02/openrouter/client.go`, `ch02/openrouter/wire.go` | draft |
| `/loop/tool-calling/` | Tool calling in Go, on the wire | how does tool calling (function calling) work in JSON | 4, 2 | `ch04/openrouter/wire.go`, `ch04/openrouter/client.go` | published |
| `/loop/tokens-and-cost/` | Tokens, cost, and the context window | how much does an agent run cost; why does every request grow | 1, 2, 4 | `ch02/agent/types.go`, `ch04/main.go` | draft |
| `/loop/how-models-work/` | How a language model picks its next token | how do LLMs work, for programmers | 3 | `ch03/toy/toy.go`, `ch03/bpe/bpe.go` | draft |
| `/loop/streaming/` | Streaming model replies in Go | parse server-sent events from an LLM API in Go | 6 | `ch06/openrouter/sse.go`, `ch06/openrouter/stream.go`, `ch06/agent/events.go` | draft |

## Design (`/design/`, formerly Patterns)

| URL | Title | Search question | Chapters | Code | Status |
| --- | --- | --- | --- | --- | --- |
| `/design/tool-design/` | Designing tools a model uses well | how to write tool descriptions and errors for an LLM | 7, 4 | `ch07/agent/registry.go`, `ch07/agent/schema.go`, `ch07/triage/` | draft |
| `/design/structured-output/` | Structured output: an agent's answer in a fixed shape | LLM structured output in Go | 7 | `ch07/agent/final.go` | draft |
| `/design/workflows-vs-agents/` | Workflow or agent? Choosing a shape | when to use an agent and when a workflow | 1, 5 | `ch05/agent/loop.go` | draft |
| `/design/context-management/` | Keeping a long run's context in check | agent context compaction, trimming, prompt caching | 8 | `ch08/agent/policy.go`, `ch08/agent/trim.go`, `ch08/agent/compact.go` | draft |
| `/design/parallel-tool-calls/` | Running tool calls in parallel with goroutines | parallel tool calls in Go | 9 | `ch09/agent/runner.go`, `ch09/agent/slots.go` | draft |
| `/design/sub-agents/` | Sub-agents and fan-out in Go | multi-agent and sub-agent patterns | 9 | `ch09/fanout.go`, `ch09/agent/fanin.go` | draft |
| `/design/mcp/` | MCP in Go, both sides | write an MCP client and server in Go | 10 | `ch10/mcp/`, `ch10/mcptools/`, `ch10/sdk/main.go` | draft |

## Production

| URL | Title | Search question | Chapters | Code | Status |
| --- | --- | --- | --- | --- | --- |
| `/production/stop-reasons/` | Why did the agent stop? Limits, budgets, and errors callers can check | agent max turns, cost budget, error handling | 5 | `ch05/agent/errors.go`, `ch05/agent/loop.go` | draft |
| `/production/retries-and-observability/` | Retries, traces, and cost for model calls | retry an LLM API on 429 and 503 in Go; trace an agent with OpenTelemetry | 12 | `ch12/agent/retry.go`, `ch12/agent/classify.go`, `ch12/agent/trace.go`, `ch12/agent/genai.go` | draft |
| `/production/testing-offline/` | Testing an agent offline with recorded replies | test LLM code without calling the API | 2, 4, 5 | `ch04/replay/replay.go`, `ch05/agent/fake.go` | draft |
| `/production/evals/` | Evals for an agent whose answers vary | how to evaluate an LLM agent honestly | 13 | `ch13/eval.go`, `catalog/grade.go` | draft |
| `/production/prompt-injection/` | Prompt injection and sandboxing: defenses that don't rely on the model | defend an agent against prompt injection; sandbox tool execution | 14 | `ch14/safety/mark.go`, `ch14/safety/egress.go`, `ch14/safety/sandbox.go` | draft |
| `/production/approval/` | Asking a person before a risky tool call | human-in-the-loop approval for agents | 14, 15 | `ch14/safety/approve.go`, `ch14/agent/approve.go` | draft |
| `/production/durable-runs/` | Durable runs: checkpoints and idempotency keys | resume an agent after a crash without repeating actions | 15 | `ch15/durable/checkpoint.go`, `ch15/agent/once.go`, `ch15/agent/resume.go` | draft |
| `/production/agent-as-a-service/` | Running an agent as a service | deploy an agent as a long-running service | 15, 17 | `ch17/service/`, `ch15/Dockerfile` | draft |

## Run it (`/run/`, formerly Examples)

Each example page says what the program shows, the exact commands (offline, from recorded replies), what to look at in the output, and the key files.

| URL | Title | Commands | Chapters | Code | Status |
| --- | --- | --- | --- | --- | --- |
| `/run/ch01-scripted-loop/` | A scripted agent loop, no model needed | `go run ./ch01`, `go test ./ch01/...` | 1 | `ch01/` | published |
| `/run/ch04-first-agent/` | A real agent that fixes a failing test | `go run ./ch04`, `-replay fix-again`, `-replay recover` | 4 | `ch04/` | published |
| `/run/ch07-crash-triage/` | One crash report, triaged | `go run ./ch07` | 7 | `ch07/`, `remind/`, `catalog/` | draft |
| `/run/ch10-mcp-tracker/` | An agent filing issues through an MCP server | `go run ./ch10` | 10 | `ch10/`, `ch10/trackermcp/` | draft |
| `/run/ch16-framework-ports/` | One task three ways: the harness, Eino, and Genkit | `cd ch16 && go run ./cmd/harness` (and `eino`, `genkit`) | 16 | `ch16/` | draft |

## Providers and frameworks (`/providers/`, formerly Ecosystem)

| URL | Title | Search question | Chapters | Code | Status |
| --- | --- | --- | --- | --- | --- |
| `/providers/providers-and-sdks/` | One agent, several providers | use the Anthropic Messages API and OpenAI-compatible APIs from one Go agent; official Go SDKs | 11 | `ch11/anthropic/`, `ch11/orsdk/` | draft |
| `/providers/go-agent-frameworks/` | Go agent frameworks compared | Eino vs Genkit vs ADK for Go vs trpc-agent-go vs LangChainGo | 16 | `ch16/compare/`, `ch16/eino/`, `ch16/genkit/` | draft |

The frameworks page follows chapter 16's standard: the five frameworks mapped onto the parts of a harness, checked against their source with a date, what each does well, and the eight questions, with no ranking.

## Findings

Each finding states the result, the method (task, models, runs per cell, date, cost), what it can and can't show, and what to do with it. The `evidence/` programs that produced them are not in the public repository, so a finding links to the code under test and cites the chapter, and never tells readers to run `go run ./evidence`.

| URL | Title | Chapters | Code | Status |
| --- | --- | --- | --- | --- |
| `/findings/tool-description-poisoning/` | One sentence in a tool description decided 79 of 80 first moves | 10 | `ch10/mcptools/trust.go` | published |
| `/findings/error-wording/` | An error that said "locked" cost 18 turns; one that said "don't retry" cost 4 | 4 | `ch04/agent/tools.go` | draft |
| `/findings/workflow-vs-agent/` | The same bug fix as a workflow cost a quarter to a sixth as much | 1 | `ch01/` | draft |
| `/findings/fan-out/` | Four agents at once finished no faster and cost 25 to 60 percent more | 9 | `ch09/fanout.go` | draft |
| `/findings/identical-runs/` | Two identical runs scored 15 points apart | 13 | `ch13/eval.go` | draft |
| `/findings/crash-sweep/` | 910 crash scenarios, no duplicate issues | 15 | `ch15/durable/`, `ch15/agent/once.go` | draft |

## Count

35 guides (7 The loop, 7 Design, 8 Production, 5 Run it, 2 Providers and frameworks, 6 Findings), plus the home page, the book page, the glossary, the six section pages, and the 404 page: 45 pages in all.
Candidates to merge if the count should come down: `structured-output` into `tool-design`, and `agent-as-a-service` into `durable-runs`.

## Next candidates

- More findings from the book's measured results: prompt caching's sixfold cut on the batch (chapter 8), the backoff simulation (chapter 12), the reviewer on the hard tier (chapter 13), and the agent that went around a denial (chapter 14).
- More examples: chapter 6 (streaming, `-replay stream-error`), chapter 12 (`-trace -faults 503`), and chapter 15 (`-die-after code_patch`).
- The finished agent (chapter 17): `triage FILE`, `triage serve`, and the results table across models, once the author decides how much of it belongs on the site.
