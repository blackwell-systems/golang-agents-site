---
title: "RAG or agentic search? A measured comparison in Go"
short: RAG or agentic search?
description: Three ways to answer questions from a set of documents, each asked the same 16 questions. One-shot RAG, retrieval as a tool in an agent loop, and an agent that lists and reads the files itself. All three answered every question; what differed was the cost, the calls, and the tokens, and why.
card: One-shot RAG, search as a tool, and an agent reading files, measured on the same 16 questions.
group: Knowledge and memory
stage: all
beyond: true
date: 2026-10-19
weight: 91
art: rag-or-agentic-search
---

There are two common ways to give a model your documents.
[Retrieval-augmented generation](/build/rag/) searches them first and sends the closest passages with the question.
Agentic search gives an agent tools, such as listing and reading files or searching an index, and lets it decide what to look at, turn by turn.
Many coding agents work the second way: they search and read files the way a developer would, without an index of meanings.

Which one to use depends on the documents and the questions, and the useful thing is to measure it on yours.
This page shows one such measurement and how to run it.

## Three approaches, one set of questions

The [`rag` example](code:rag) asks 16 questions about the documentation of `shortlink`, a small URL shortener made up for the purpose: 25 Markdown files, about 53 KB, split into 140 chunks for search.

| Approach | How it answers |
| --- | --- |
| `rag` | Turns the question into an [embedding](/build/rag/) (a list of numbers that stands for its meaning), finds the four closest chunks of the documents, and answers in one chat call |
| `rag-tool` | An [agent loop](/basics/agent-loop/) with one tool, `search_docs`, that runs the same search for any query the model writes |
| `files` | An agent loop with `list_files` and `read_file`, and no index |

The questions range from one setting's default to an upgrade that needs facts from two documents, and the last is one the documents don't answer: can shortlink make QR codes?
Each answer is graded by phrases a right answer must contain, such as `redirect_status` and `301` for "How do I make redirects permanent?".
The three approaches share the model, `xiaomi/mimo-v2.6-pro`, and the agents share the loop and a limit of eight turns.

From the `code` directory of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code) (clone steps are on the [examples page](/run/)), `go run ./rag` replays the recorded run:

```text include=rag/testdata/out.txt title="go run ./rag"
```

## What the run showed

All three answered all 16 questions, including the last: each said the documents don't mention QR codes instead of describing a feature that doesn't exist.
On these documents, the approach didn't decide whether the answer was right.
It decided what the answer cost.

```result
figure: 2.4×
unit: the cost of one-shot RAG, for the agent that read the files
task: 16 questions about shortlink's 25 documents, each asked three ways; all three approaches answered all 16 right
models: xiaomi/mimo-v2.6-pro for the answers, voyageai/voyage-4-lite for the embeddings, through OpenRouter
runs: one run of the 16 questions per approach
date: October 2026
source: code/rag

`rag` cost $0.0064 for the 16 questions, `rag-tool` $0.0128, and `files` $0.0151. With one run each, small differences between the two agents could change on another run; the gap between one chat call and a loop is the part to expect again.
```

**`rag` made two calls a question:** one to embed the question and one to answer it.
It sent 8,714 [input tokens](/basics/tokens-and-cost/) in all, because each answer saw only four chunks.

**`rag-tool` searched more than once on 13 of the 16 questions.**
It searched once on three, twice on nine, three times on three, and five times on the QR question, trying other words before it concluded the documents don't cover it.
Each search is an embedding call, and every chat turn resends the conversation so far.
A reply can ask for several searches at once, so there were fewer chat turns than searches plus answers: its 72 calls were 35 searches and 37 chat turns, and it sent 25,834 input tokens.

**`files` listed the files and picked what to read by name.**
It read 1 to 6 files a question, 39 reads in all, and opened `README.md`, the one file that lists the others, only once.
File names like `docs/rate-limits.md` and `docs/upgrading.md` told it where to look.
Every file it read stayed in the conversation for the following turns, so it sent 43,201 input tokens, five times as many as `rag`.
Its 54 calls were all chat turns, more than `rag-tool`'s 37.

## When each one fits

```fit
reach: One-shot RAG, when the questions are many and alike, the answers are in a passage or two, and cost per question matters. Here it was the cheapest by half, with two calls a question.
reach: Search as a tool, when some questions need a second look, or the answer is spread across documents. The model writes its own queries and retries with other words; on these questions it did so on 13 of 16.
reach: An agent that reads files, when there is no index to build or keep up to date, the files have names that say what they hold, and reading whole files matters more than cost.
leave: An index, while your documents are small enough to read. Here the agent without one answered everything; what it cost was 2.4 times as much per question.
leave: An agent loop, for a high-volume question service where one search answers most questions. The loop's extra turns cost more on every question, including the easy ones.
```

The three combine.
A common design is an agent with both kinds of tools: a search tool to find candidates, and a read tool to open the whole file once it knows which.

## What this run doesn't show

**Larger document sets.**
At 53 KB, the files agent could read whatever it needed in a few turns.
With thousands of documents, or files with names that say nothing about their content, it would have to read more to find the same answer, and that is where an index earns its upkeep.
This run doesn't measure where that point is.

**Putting everything in the prompt.**
53 KB of text fits in one request to this model, and with prompt caching, sending the same documents with every question can be cheap.
That is a fourth approach, and it isn't measured here.

**Speed.**
The run recorded calls and tokens, not time.
Fewer calls usually means a faster answer, since each call waits on the model, but this run doesn't show it.

**Variation.**
Each approach ran once.
A model's answers vary between runs, so treat the scores as one sample; [evals](/production/evals/) explains running each case several times and reporting a range.

## Getting the questions right

Two things changed in this question set before the run above, and both are worth knowing before you build your own.

In an earlier recording, the unanswerable question asked whether shortlink supports custom domains.
`rag-tool` answered that you can serve short links under your own domain by setting `base_url`, which is true of a self-hosted service.
The grader marked it wrong, but the answer was right and the question was ambiguous, so the question was replaced with one about QR codes, which no document touches.

In the run above, the grader first marked the files agent wrong on the QR question.
Its answer, "the docs don't describe any way for shortlink to generate one", was right, but the grader's list of ways to say "the documents don't say" had "doesn't describe" and not "don't describe".
The list was widened, and the recorded answers were graded again; the answers themselves are as recorded.

A grader that matches phrases is quick to write and quick to run, and it misreads answers worded in ways you didn't expect.
Read the answers it marks wrong before you trust a score, and see [evals](/production/evals/) for graders that check more than wording.

## Try it

1. Run `go run ./rag` (no key needed) and look in `rag/testdata/run.json` for the five searches `rag-tool` made on the last question.
2. With `OPENROUTER_API_KEY` set (see [your first model call](/basics/first-model-call/); steps 2 to 4 make live calls), run `go run ./rag -ask files "What does a 429 mean?"` and `-ask rag` with the same question, and compare the calls and the cost each prints.
3. Rename the 25 files in `rag/testdata/docs` to `doc1.md`, `doc2.md`, and so on, record a run with `-record my-run.json`, and see what happens to the `files` approach. To undo it, run `git restore rag/testdata` and then `git clean -fd rag/testdata` from the `code` directory (the second command deletes the renamed files).
4. Add a question of your own to `rag/questions.go` with the phrases a right answer must contain, and run it all three ways.
