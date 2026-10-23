---
title: How a language model picks its next token
short: How models pick the next token
description: A language model gives every possible next token a probability and picks one. A toy model in Go shows why answers vary, why temperature matters, why fluent text can be false, why code costs more tokens than English, and why a tool call is text the model writes.
card: A toy model in Go shows why answers vary, what temperature does, and why a tool call is text.
group: Tools and the model
chapters: 3
stage: 2
date: 2026-10-18
weight: 60
art: ch03
---

Four habits of the model shape every agent built on it.
It can give two different replies to one request.
It is billed by the token rather than the word, and one text can come to more tokens on one model than on another.
It sounds just as sure when it is wrong as when it is right.
And it can ask for tools.

A single mechanism accounts for all four, and a few dozen lines of Go, with no machine learning in them, are enough to watch it work.
The experiments described here live in [`ch03`](code:ch03) of the companion code and run offline.

## Next-token prediction

A [language model](/glossary/#language-model) has one job: read some text and assign a probability to every [token](/glossary/#token) that could come next.
A token is a short piece of text, often a whole word and sometimes a fragment of one; the Tokens section below shows where they come from.
To write a reply, the model repeats that step: choose a token, add it to the end of the text, and predict again from the longer text.
That repetition is called next-token prediction.
Everything a model sends back, a paragraph of advice or a tool call, comes out of it one token at a time.

```diagram
name: next-token
alt: A vertical flow: the text "the test failed because it" goes to the tokenizer, which produces token numbers; embeddings turn each into a vector; the transformer's layers of attention and weights produce a score for every token in the vocabulary; softmax, after dividing by the temperature, turns the scores into probabilities; one token is picked, greedily or by sampling; a dashed arrow appends the token " was" to the text and predicts again.

One step of generation: the text becomes tokens, the model scores every possible next token, one is picked, and the step repeats with it added.
```

## A model made of counts

The smallest model that works this way just counts.
The toy used here reads a training text of roughly 7,600 words and records, for each pair of words, which word followed the pair and how often.
The pair "the model" was followed by 38 words in all, 5 of them "sees", so the toy rates "sees" at 5/38, about 0.13.

Generating means looking up the last two words and choosing from what followed them:

```go include=ch03/toy/toy.go tag=generate
```

The choice can be made in two ways, and each matches something real models do.

**Greedy decoding loops.** Taking the most probable word every time, the toy reaches "a key and a 401);" and then repeats it forever, because the likeliest continuation of that phrase leads straight back to it. Small models writing long replies can fall into the same trap, which is one reason APIs sample by default.

**Sampling differs from run to run.** Choose in proportion to the probabilities instead, and three runs from the same starting words go three separate ways. This is the main source of varying answers: every call works out the same probabilities, and the server draws a fresh sample from them.

## Fluent and wrong

Here is the toy's greedy run:

```text include=ch03/testdata/out/toy.txt lines=8..13 title="go run ./ch03 toy, the greedy run"
```

The sentence about `OPENROUTER_API_KEY` reads like the training text, since each run of three words in it occurs somewhere in that text.
It is also false: the saved replies come from the API, and `ToolCall` has nothing to do with them.
The toy has no notion of truth; it writes whatever is probable.

A real model is vastly better at probable text, and probable text is true often enough that models are worth using.
Still, no stage of the mechanism checks facts, so a false statement that happens to be probable arrives in the same assured voice as a true one.
The industry's word for this is [hallucination](/glossary/#hallucination).
The rule it gives an agent is to verify claims with tools.
The crash-triage agent in the companion code must produce a test that fails on the old code and passes with its patch; saying it found the bug isn't enough.

## Temperature

Temperature is a setting applied to the probabilities before the choice.
Values below 1 sharpen them, so the favorite wins more often, and 0 means always taking the favorite, which is greedy decoding.
Values above 1 even them out, and long-shot tokens get chosen more often.
At 0.5 the toy's "sees" climbs from 0.13 to about 0.33.
A real model applies temperature by dividing each token's score by it before the softmax, the step that turns scores into probabilities; the toy gets the same effect another way, raising each word's count to the power 1/temperature, so a low temperature widens the gap between common and rare words.

One recorded experiment put a small open model, `meta-llama/llama-3.1-8b-instruct`, through it: describe Go in exactly three words, ten calls at each of four temperatures.
Temperature 0 produced one reply, ten times over.
At 0.7 and at 1, all ten replies differed and all of them were sensible.
At 1.5, six of the ten fell apart into strings like "Concurrentoons statically": once an improbable token is chosen, it becomes the start that every later token is predicted from.

## Why identical runs differ

Sampling explains most of the variation, but setting the temperature to 0 doesn't remove it.
Providers run many requests together on their hardware, and the arithmetic can come out slightly differently from one batch to the next.
When two tokens are nearly tied, that difference is enough to flip the choice, and the text diverges from that token on.
A router such as OpenRouter can also pass two requests for one model to two different providers, whose hardware differs.
In one recorded comparison, one provider serving the Llama model at temperature 0 returned the same reply in ten calls out of ten; another, serving the same model at the same setting, returned two different replies across nine calls.

In an agent the effect compounds.
A different token early in a turn can mean a different tool call, and every later turn starts from that one.
Evals that run each task many times and report rates are built around this, and [Two identical runs scored 15 points apart](/findings/identical-runs/) shows what happens when you compare single runs.
So check what an agent accomplishes, such as the tests passing or the right issue filed, and never the exact words it wrote.

## Tokens

Real models work in neither words nor letters.
A frequent word gets a token of its own, and a rare one is assembled from fragments.
Most [tokenizers](/glossary/#tokenizer) are built with byte-pair encoding: begin with the 256 possible byte values, then again and again merge the most common neighboring pair into a new token.
The companion code implements one in Go and trains it on the toy's text.
An English sentence comes out at about a token per word, while `ParseDuration` splits into `P`, `ar`, `se`, `D`, `uration`.

Inside the model, each token becomes an [embedding](/glossary/#embedding), a list of numbers.
A separate embedding model does the same for a whole passage of text, and texts with similar meaning get lists that sit near each other.
[Cosine similarity](/glossary/#cosine-similarity) measures how near two lists are, which is how [retrieval](/build/rag/) finds the passages closest to a question.

Production tokenizers handle code far better than that, but the rule survives: text unlike the tokenizer's training data costs more tokens.
In October 2026, recorded runs measured the same two texts on five models.
A 1,008-byte Go file came to between 336 and 502 tokens, and a page of English to between 450 and 598.
Four of the five landed within 7 percent of each other.
The fifth, the default model in the companion code, `anthropic/claude-haiku-5.5`, needed about a third more tokens than the others for the English and about 50 percent more for the Go.
A price per token can mislead, then; compare models by what a whole task costs.
[Tokens, cost, and the context window](/basics/tokens-and-cost/) follows that through an agent's bill.

## Attention, briefly

The toy looks two words back; a real model needs a word from thousands of tokens earlier.
[Attention](/glossary/#attention) works like a fuzzy map lookup: each position scores its query against every earlier position's key and takes a blend of their values, weighted by the match.
Because the weights come from a softmax that hands out a fixed total of 1, every token added to the context leaves less attention for the ones that matter.
In one small attention example, putting a hundred more words between "test" and "it" cut the weight on "test" to 0.03.
That is part of why a model's accuracy falls as its input grows.

## A tool call is text

A model consumes and produces a single sequence of tokens.
Your request is a list of messages, so the provider first flattens it into one text with a [chat template](/glossary/#chat-template), using special tokens to mark where each role's message starts and ends.
The `chatml` package in the companion code renders the start of the bug fix the way Qwen3's template does:

```text include=ch03/testdata/out/template.txt lines=3..28 title="go run ./ch03 template"
```

The tools are in that text too.
`read_file` appears as one line of JSON in the system message, after English instructions that say to call a function by writing a JSON object between `<tool_call>` tags.
The assistant's turn shows the result: the tool call is nothing more than text the model wrote in that format, and the provider parses it back out into the reply's `tool_calls` field.
The tool's result returns the same way, as a user message wrapped in `<tool_response>` tags.
These tags are Qwen3's; each model family has its own template, and closed models keep theirs private, but every one of them turns tool calls into text and back.

Three consequences follow for an agent:

- **A tool call fails the ways text fails.** The JSON can be malformed, an argument can have the wrong type, and the name can belong to no tool at all. The [harness](/glossary/#harness), the Go program around the model, validates each call and replies with an error that says how to correct it.
- **Instructions and data travel together.** A file the agent opened or a crash report it was handed ends up in the same stream as your system prompt. If that text tells the model to ignore its instructions, the model weighs it like any other text. This is [prompt injection](/glossary/#prompt-injection), and special tokens can't prevent it.
- **Calls don't teach the model anything.** A call leaves the model's weights unchanged, so whatever it should know about your task must be sent again in each request.

## Run it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go run ./ch03 toy
$ go run ./ch03 bpe
$ go run ./ch03 template
$ go run ./ch03
```

The first three run the toy model, the tokenizer, and the chat template.
The last runs every experiment in order, ending with the probe that asks a real model for its token probabilities; with `OPENROUTER_API_KEY` unset, the probe replays recorded calls.
