---
title: Tokens, cost, and the context window
short: Tokens, cost, and context
description: An agent's requests grow with every turn. This page shows that growth in recorded numbers, what a run costs, which tokens you pay for before the model reads the task, and why the size of the context window is a poor planning target.
chapters: 1, 2, 4
stage: 1
date: 2026-10-18
weight: 50
art: ch02
---

Model APIs measure everything in tokens.
Your request is counted in them, the reply is counted in them, the size limits are stated in them, and the price list is written per token.
For an agent, one consequence matters more than any other.
The model remembers nothing between calls, so each request has to resend the conversation from the beginning, and an agent's conversation never shrinks on its own.

Below, that growth is traced through runs the book recorded, followed by what it does to cost and to the context window.

## Tokens and prices

A [token](/glossary/#token) is a piece of text the model treats as one unit.
Short, common words are usually one token each; longer words get split into several, and punctuation marks are often tokens of their own.
For English text, figure about four characters to a token.
The request you send on a call is called the prompt, and the text the model sends back is the completion.

Providers set two prices per million tokens, one for input and a higher one for output, since producing a token is much more work for the model than reading one.
In October 2026 the book's default model, `anthropic/claude-haiku-5.5`, cost $0.10 per million input tokens and $0.50 per million output tokens.

Each reply comes with a usage report, and the agent should hold on to it:

```go include=ch02/agent/types.go tag=reply
```

Two fields in that report deserve explanation.
Cached tokens are input that matches the opening of a request the provider handled shortly before, and the provider bills them at a discount.
Reasoning tokens are text the model writes to work through the problem before giving its answer; they are billed as output even when you never see them.
In one of the book's recorded chats, a four-line answer used 700 output tokens, and 642 of those were reasoning.

## The same question, four prices

A low price per token doesn't guarantee a low price per answer.
Chapter 2 asked four inexpensive models the same short question about nil and empty slices, four times on each.
The question was 19 words, but the input counts ran from 29 to 53 tokens, mainly because every provider adds its own formatting around a message.
`z-ai/glm-5.3-flash` produced three times the output tokens of the default model, mostly reasoning, and the amount of reasoning swung between 193 and 807 tokens across runs of the identical question.
`deepseek/deepseek-v4.1-flash` had the cheapest input on the price list, yet its median answer cost more than the default model's, because it wrote longer replies and its output price was higher.
Before picking a model from a price table, run it on your own task and look at the bill.

## Every request carries everything

In a chat, each new request is the full history plus one more question.
Here are the three requests from a recorded chat:

| Turn | Tokens in | Tokens out | What was sent |
| --- | --- | --- | --- |
| 1 | 43 | 122 | System prompt and question 1 |
| 2 | 175 | 253 | Turn 1's input, answer 1, and question 2 |
| 3 | 439 | 700 | Turn 2's input, answer 2, and question 3 |

Look at answer 1: you buy it as 122 output tokens in turn 1, then buy it again as input in turn 2, and again in turn 3, and in every turn after.
A turn's cost therefore rises as the conversation lengthens, and the total for a conversation of n turns rises about as fast as n squared.

Three turns hide this; twenty don't.
The book also recorded a 20-turn chat on the default model.
By the last turn, a question of roughly twelve words went out inside a 13,785-token request.
Over all twenty turns the requests added up to 124,673 tokens, nine times the size of the finished conversation, and turn 20 cost ten times what turn 1 did.

## An agent grows faster

People add to a chat a sentence at a time.
An agent adds whole tool outputs: each file it opens and each test run it does becomes part of the history, and it may take thirty turns with no person involved.
Here are the four requests from chapter 4's bug fix, in which a model repairs a failing Go test using four tools:

| Call | Tokens in | Tokens out |
| --- | --- | --- |
| 1 | 583 | 94 |
| 2 | 834 | 99 |
| 3 | 1,846 | 245 |
| 4 | 2,164 | 131 |

Call 3 jumps by about a thousand tokens because turn 2 read two source files, and from then on both files ride along in every request.

The more surprising number is call 1.
The task itself is a single sentence of 26 tokens, but the request is 583.
The four [tool definitions](/glossary/#tool-definition) account for roughly 270 of those.
About 290 more are added by the provider whenever tools are present: it rewrites your definitions into its own format and wraps them in its own usage instructions, which you never see and are billed for on every call.
The overhead depends on the provider.
Measured on the identical first request, `deepseek/deepseek-v4.1-flash` came to 441 tokens and `z-ai/glm-5.3-flash` to 317, while `openai/gpt-5.6-luna` needed only 126, since its provider stores tool definitions in a format more compact than the JSON you send.

On a task this small the overhead hardly shows; the whole run cost $0.0008.
Crash investigation is different, because the agent reads much more.
Chapter 8's batch of 99 crash reports cost $1.33 in a run with no context policy; with a policy that marks the cache and clears old tool results once a request passes 60,000 tokens, two runs cost $0.06 and $0.42.

## Caching changes the bill, not the size

With [prompt caching](/glossary/#prompt-caching), the provider saves the work it did on the beginning of one request and reuses it, for less money, when a later request begins with the exact same tokens.
Agents benefit naturally, since each request is the one before it with a little added at the end.

In the 20-turn chat, `openai/gpt-5.6-luna` and `deepseek/deepseek-v4.1-flash` cached without being asked and charged a tenth of the normal rate for most of each request; for `deepseek/deepseek-v4.1-flash`, the final turn ended up a little cheaper than the first.
The default model supports caching as well, but through OpenRouter you have to request it by putting markers in the request.
Chapter 8 adds two markers, one at the end of the part every request starts with (the system prompt and the task) and one at the end of the request, and measured the difference on its largest task:

```result
figure: $0.23
unit: for the batch, down from $1.33
task: chapter 8's batch of 99 crash reports from 23 causes, triaged by the crash-triage agent with its whole history in every request, once without cache markers and once with them
models: `anthropic/claude-haiku-5.5`
runs: one per condition
date: October 2026
source: Chapter 8

With the markers, 95 percent of the input tokens were read from the cache, the cost fell about sixfold, and the agent passed the same 86 of the grader's 94 checks. The two runs took 35 and 33 turns, so most of the gap comes from what each token cost rather than from different choices by the model. Part of it is the long-context price tier described below: priced at the lower rate throughout, the markers would still have made the batch about four and a half times cheaper.
```

Either way, a cached history costs less while staying exactly as long, and length has a cost of its own, covered next.

## The context window, and why it's the wrong number

The [context window](/glossary/#context-window) is the most tokens a single call can involve, counting the prompt and the reply together.
The default model allows 1,000,000; other models go from a few thousand to a few million.
Send more than fits and the provider will usually reject the request with an error.

Quality drops long before that limit.
A 2023 paper by Liu and others, "Lost in the Middle", found that models reliably found a fact placed near the beginning or end of a long input and often missed the same fact placed in the middle.
In 2025, Chroma's "Context Rot" report ran 18 models through simple tasks and saw every one of them get less accurate as the input lengthened, well short of its window.
Length can raise the price per token too: on the default model, once a request passes 100,000 tokens, the whole request is billed at five times the usual rate, and about half the requests in chapter 8's uncached batch run crossed that line.

So plan around a budget, not the window.
A file the agent opened many turns back and doesn't need any more is still billed on each call, and it still takes some of the model's attention away from what matters now.

## What to do about it

- **Count from the first call.** Record the usage that comes back with each reply and total it for the run; the [ch04 example](/run/ch04-first-agent/) does it by wrapping the model.
- **Bound the spend.** A turn limit catches a loop that won't stop, and a dollar budget checked before every call limits the cost itself.
- **Keep tool output small.** Whatever a tool returns is resent on every call after it. Chapter 4's `go test` tool reduces 1,767 bytes of JSON events to a 158-byte report.
- **Keep the start of the request stable.** The cache discount only applies to an identical prefix, so put instructions and tool definitions first and leave them alone during a run.
- **Decide what the model sees.** Eventually the history needs rules: drop old turns, clear old tool results, or summarize. That is stage 1 of the loop and the topic of chapter 8.

## Run it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go run ./ch02
$ go run ./ch04
```

`./ch02` plays back the three-turn chat and prints the tokens and cost of every call.
`./ch04` plays back the bug fix and ends with the run's totals: 4 model calls, 5,427 tokens in, 569 out.
