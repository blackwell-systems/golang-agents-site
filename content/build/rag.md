---
title: Retrieval-augmented generation (RAG) in Go, with the standard library
short: RAG in Go
description: Answer questions from your own documents by finding the passages that match a question and sending them to the model with it. Chunking by heading, embeddings through OpenRouter, an in-memory vector search with cosine similarity, and the prompt, in standard-library Go, measured on 16 questions.
card: Split documents into chunks, embed them, search in memory, and answer from what the search found.
group: Knowledge and memory
stage: 1
beyond: true
date: 2026-10-19
weight: 90
---

A model knows what was in its training data, and nothing about your service's documentation, your runbooks, or last week's design notes.
When the answer to a question is in documents like those, the program has to find the right passages and put them in front of the model.

```definition
term: retrieval-augmented generation

**Retrieval-augmented generation (RAG)** answers a question in two steps: search a set of documents for the passages closest to the question, then send those passages to the model with the question and ask it to answer from them alone.
```

The search is usually by meaning rather than by keyword.
An embeddings model turns each passage into a vector, a list of numbers, placed so that passages about similar things have similar vectors; the question is turned into a vector the same way, and the closest passages win.
Everything on this page is standard-library Go: the embeddings come from an HTTP API, and the search is a loop.

## Run it first

The program is `rag` in the [companion code](code:rag).
It answers questions about the documentation of `shortlink`, a small URL shortener made up for the purpose: 25 Markdown files, about 53 KB.
From the repository's `code` directory, with no API key:

```console
$ go run ./rag
```

Without a key, it replays a recorded run: the replies of `xiaomi/mimo-v2.6-pro` for the answers and of `voyageai/voyage-4-lite` for the embeddings, recorded on October 19, 2026.
The chunking and the search run for real.
It asks 16 questions three ways, and this page builds the first, `rag`; the others are the subject of [RAG or agentic search?](/build/rag-or-agentic-search/).

```text include=rag/testdata/out.txt title="go run ./rag"
```

With `OPENROUTER_API_KEY` set, ask your own question:

```console
$ go run ./rag -ask rag "How do I make redirects permanent?"
```

There are four pieces: split the documents into chunks, embed every chunk, search the chunks for each question, and answer from what the search found.

## Split the documents into chunks

A whole document is the wrong unit to search.
A long page about deployment matches many questions a little, and sending all of it spends tokens on paragraphs that don't matter.
A sentence is too small: it loses the context that says what it is about.
Documentation is written in sections, so the sections are the chunks here:

```go include=rag/retrieve.go tag=chunk
```

Each chunk is the text under one heading, cut at paragraph breaks when a section runs past 700 bytes.
The 25 files make 140 chunks.

A chunk carries its document's path and its heading.
"Set trusted_proxies" means little on its own; under "Behind a reverse proxy" in `docs/deploy.md`, it answers a question.
So the text that is embedded, and later shown to the model, starts with both:

```text
[docs/deploy.md: Behind a reverse proxy]
When shortlink runs behind a reverse proxy such as nginx, every request
appears to come from the proxy's address, so every user shares one rate
limit. ...
```

The headings also let the model cite where each fact came from, which the prompt below asks for.

## Embed the chunks

OpenRouter serves embeddings models from the same API, with the same key, as the chat models.
The request is a model and a list of texts, and the reply holds one vector per text:

```go include=rag/retrieve.go tag=embed
```

All 140 chunks go in one request, which cost $0.00028 with `voyageai/voyage-4-lite`.
Embedding costs little next to the answers: the 16 questions' answers cost about 20 times as much as building the index.

The request asks for 256 numbers per vector, through the `dimensions` field; this model makes 1,024 by default.
Shorter vectors take a quarter of the storage, and a quarter of the arithmetic in every comparison.
The questions must be embedded by the same model, with the same number of dimensions, as the chunks, or the vectors can't be compared.

## Search in memory

To compare two vectors, the program uses cosine similarity: it measures the angle between them and ignores their length.
It is 1 for vectors that point the same way and near 0 for unrelated ones.
The search computes it against every chunk and keeps the best:

```go include=rag/retrieve.go tag=index
```

Comparing the question with every chunk is a brute-force search, and for a few thousand chunks it needs no database.
Larger sets keep their vectors in a store with a vector index, which finds close vectors without comparing every one; the rest of the program stays the same.

## Answer from the passages

The question is embedded, the four closest chunks are found, and one chat call gets the passages and the question:

```go include=rag/answer.go tag=rag
```

The system prompt makes three rules: answer only from the passages, name the document each fact comes from, and say so when the passages don't answer the question.
The last rule matters as much as the first two.
Search always returns four passages, whether or not any of them is relevant, and a model given passages tends to answer from them.
The 16th question asks whether shortlink can make QR codes, which no document mentions; `rag` answered:

```text
The documentation doesn't say. None of the provided passages mention
QR codes ...
```

```result
figure: 16
unit: of 16 questions answered right, for $0.0064 in all
task: 16 questions about shortlink's 25 documents, from a single setting's default to a two-step upgrade and one question the documents don't answer, graded by the phrases a right answer must contain
models: xiaomi/mimo-v2.6-pro for the answers, voyageai/voyage-4-lite for the embeddings, through OpenRouter
runs: one run of the 16 questions; 32 API calls, two per question, after one call to build the index
date: October 2026
source: code/rag
link: /build/rag-or-agentic-search/
more: The comparison with agentic search

The cost includes embedding each question. One run on one model is a single sample: it shows that the pipeline works on these documents, not a rate you can expect elsewhere.
```

## Retrieval as a tool

The same search can be a tool instead, so that a model in an agent loop decides when to search and what to search for:

```go include=rag/answer.go tag=search-tool
```

With the tool, the model can search again with other words when the first results miss, and on these questions it searched more than once on 13 of 16.
That costs calls and tokens; [RAG or agentic search?](/build/rag-or-agentic-search/) compares it with one-call RAG and with an agent that reads the files itself.

## What to watch

**Retrieval can miss.**
When the right chunk isn't among the four returned, the model can't answer, and with the prompt above it says it can't.
Check retrieval on its own: for each test question, note which document holds the answer, and count how often a chunk from it comes back.
That tells you whether a wrong answer came from the search or from the model, which an [eval](/production/evals/) of the answers alone can't.

**The index goes stale.**
Vectors describe the documents as they were when they were embedded.
Rebuild the index when the documents change, or embed only the files that changed.

**Retrieved text is untrusted input.**
The passages come from documents, and anyone who can edit a document can write instructions into it.
Treat them as data in the prompt, the way [prompt injection](/production/prompt-injection/) describes for any tool result.

**Chunk size is a trade-off.**
Larger chunks carry more context and spend more tokens on each answer; smaller ones are more precise and lose context.
Splitting by heading suits documentation; for text without headings, split by paragraph and keep a little overlap between neighbors.

## Try it

1. Run `go run ./rag` and read `testdata/run.json` for one question: the embedding request, the four passages in the chat request, and the answer.
2. With a key, ask a question whose answer spans two documents, such as which setting turns webhooks on and which metric counts failed deliveries, and see whether four chunks are enough.
3. Change `topK` in `answer.go` to 2, record a run with `-record`, and compare the scores and the cost.
4. Change `embedDims` in `retrieve.go` to 1,024, and compare the size of the recording and the answers.
