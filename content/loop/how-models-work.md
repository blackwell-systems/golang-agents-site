---
draft: true
title: How a language model picks its next token
short: How models pick the next token
description: A language model gives every possible next token a probability and picks one. A toy model in Go shows why answers vary, why fluent text can be false, why code costs more tokens than English, and why a tool call can be wrong in every way text can.
chapters: 3
stage: 2
date: 2026-10-18
weight: 60
art: ch03
---

An agent is built on four behaviors of the model inside it.
Ask the same question twice and you can get two answers.
The bill counts tokens, not words, and the same text costs more on one model than another.
The model states wrong things in the same confident tone as right ones.
And it knows how to ask for tools.

All four come from one mechanism, and you can see it in a few dozen lines of Go with no machine learning at all.
This page follows chapter 3 of the book, whose experiments are in [`ch03`](code:ch03) of the companion code and run offline.

## A model made of counts

A [language model](/glossary/#language-model) does one thing: given some text, it gives a probability for each possible next piece of text.
Generating means doing that over and over: pick a piece, append it, predict again.

The simplest version counts.
Take a training text, and for every pair of words, count which words came next.
Chapter 3's toy is trained on an early draft of the book's first two chapters, about 7,600 words.
After "the model", it saw "sees" 5 times out of 38, so "sees" gets a probability of 5/38, about 0.13.

To generate, the toy looks up the distribution for its last two words and picks one:

```go include=ch03/toy/toy.go tag=generate
```

There are two ways to pick, and they explain two behaviors you'll meet in real models.

**Greedy decoding gets stuck.** Always taking the likeliest word, the toy reaches "a key and a 401);" and loops forever, because the likeliest continuation leads back to the same place. Small models writing long outputs can do the same, which is one reason APIs sample by default.

**Sampling varies.** Picking at random in proportion to the probabilities, three runs from the same two words go three different ways. That is the main reason the same question gets different answers: the model computes the same distribution each time, and whoever serves it draws a different sample.

## Fluent and wrong

The greedy run also produced this sentence: "When OPENROUTER_API_KEY is empty, it replays replies saved from the agent's ToolCall."
It reads like the book, because every three-word stretch of it appears in the book, and it is false.
Nothing in the toy checks whether its text is true; it produces likely text.

A real model is enormously better at producing likely text, and likely text is often true, which is why models are useful.
But there is no separate step that checks facts, so false text that is likely comes out in the same confident tone, which the industry calls [hallucination](/glossary/#hallucination).
For an agent the lesson is practical: check claims with tools.
The crash-triage agent in the companion code has to show a test that fails before its fix and passes after, rather than say it found the bug.

## Temperature

Temperature reshapes the probabilities before sampling.
Below 1 it makes likely choices likelier, and at 0 the pick becomes greedy; above 1 it flattens the distribution so unlikely choices come up more often.
In the toy at temperature 0.5, "sees" rises from 0.13 to about 0.33.
In a real model, temperature divides each score before the softmax that turns scores into probabilities, and the toy's exponent turns out to be the same operation written another way.

Chapter 3 asked a small open model, `meta-llama/llama-3.1-8b-instruct`, to describe Go in exactly three words, ten times at each of four temperatures.
At 0 it gave one reply all ten times; at 0.7 and 1, ten different reasonable replies; at 1.5, six of the ten turned to nonsense such as "Concurrentoons statically", because once an unlikely token is picked, everything after it is predicted from a broken start.

Temperature 0 doesn't make a model deterministic, either.
Providers batch requests on their hardware, and the arithmetic can differ slightly between batches; serving the same model at temperature 0, one provider gave the same reply ten times in ten, and another gave two different replies in nine calls.
So test what an agent achieves, such as the tests passing or the right issue filed, not the exact words.

## Tokens

Real models predict neither words nor letters but tokens: common words become one token, and rarer words are built from pieces.
Most tokenizers are built with byte-pair encoding, which starts from the 256 possible bytes and repeatedly merges the most frequent adjacent pair into a new token.
Chapter 3 writes one in Go, trains it on the same text as the toy, and shows the pattern: the English sentence comes out at about one token per word, while `ParseDuration` becomes `P`, `ar`, `se`, `D`, `uration`.

Real tokenizers do much better on code, but the pattern holds: text unlike a tokenizer's training data takes more tokens.
Measured on five models in October 2026, a 1,008-byte Go file took from 336 to 502 tokens, and a page of English from 450 to 598.
Four of the five agreed within 7 percent; the default model's tokenizer used a third more tokens for the English and half again as many for the Go.
So compare models by what a task costs, not by their price per token.
[Tokens, cost, and the context window](/loop/tokens-and-cost/) follows that through an agent's bill.

## Attention, briefly

The toy looks two words back; a real model needs a word from thousands of tokens earlier.
[Attention](/glossary/#attention) works like a fuzzy map lookup: each position scores its query against every earlier position's key and takes a blend of their values, weighted by the match.
Because the weights come from a softmax that hands out a fixed total of 1, every token added to the context leaves less attention for the ones that matter.
In chapter 3's toy calculation, putting a hundred more words between "test" and "it" cut the weight on "test" to 0.03.
That is part of why accuracy falls on long inputs well before the window is full.

## A tool call is text

A model reads and writes one stream of tokens.
Before it sees your list of messages, the provider renders them with a [chat template](/glossary/#chat-template) into one text, with special tokens marking each role.
Chapter 3 renders the start of the bug fix the way Qwen3's template does: the tool definition is a line of JSON inside the system message, followed by English instructions to write a JSON object between `<tool_call>` tags.
The model's tool call is exactly that, text it wrote, which the provider parses back into the `tool_calls` field of the reply.

Three things follow for an agent:

- **A tool call can be wrong in every way text can.** Malformed JSON, a wrong type, a tool name that doesn't exist. The harness checks every call and sends back an error that says how to fix it.
- **Instructions and data share one stream.** A file the agent reads, or a crash report, lands in the same text as your system prompt. If it says "ignore your instructions", the model reads it with the same attention as everything else. That is prompt injection.
- **The model learns nothing from your requests.** Its weights don't change when you call it; everything it knows about your task has to be in the request, every time.

## Run it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go run ./ch03 toy
$ go run ./ch03 bpe
$ go run ./ch03
```

The first two run the toy model and the tokenizer; the last runs every experiment in order, including the probe of a real model's probabilities, replayed from recorded calls.
