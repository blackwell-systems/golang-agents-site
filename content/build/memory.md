---
title: "Memory between sessions for a Go agent: notes, recall, and what to forget"
short: Memory between sessions
description: Give an agent a memory across conversations in standard-library Go. Short notes it saves with a tool and replaces when facts change, recalled into the prompt by meaning, with limits on what it may save. Measured over eight sessions against no memory and against carrying the whole conversation.
card: Notes an agent saves, replaces, and recalls by meaning, measured against no memory and the whole history.
group: Knowledge and memory
stage: 1
beyond: true
date: 2026-10-19
weight: 92
---

Every call to a model starts from nothing but the request.
Within one conversation, the program resends the history, so the model seems to remember; when a new conversation starts, the history is gone, and so is everything the user said in it.
An assistant that someone uses for weeks needs to carry some of that across: their proxy's address, where their database lives, what they changed last Tuesday.

```definition
term: memory

**Memory**, for an agent, is what it keeps from one conversation to the next: text saved outside the conversation, and a rule for which of it goes into the next one.
```

There are two common ways to build it.
The simplest is to carry the whole conversation forward.
The other is notes: the agent saves short facts as it learns them, and each new conversation starts with the notes that matter to it.
This page builds the notes, then measures them against carrying everything and against carrying nothing.

## Run it first

The program is `memory` in the [companion code](code:memory).
It is an assistant for `shortlink`, the small made-up URL shortener from [RAG in Go](/build/rag/), answering from the same 25 documents with `list_files` and `read_file`.
From the repository's `code` directory, with no API key:

```console
$ go run ./memory
```

Without a key, it replays a run recorded on October 19, 2026, with `xiaomi/mimo-v2.6-pro` for the answers and `voyageai/voyage-4-lite` for the embeddings; the store, the file tools, and the grading run for real.
The run is eight sessions, each a new conversation:

```go include=memory/sessions.go tag=sessions
```

```text include=memory/testdata/out.txt title="go run ./memory"
```

With a key, `-store` runs one session with a notes file, the way an assistant would between real conversations:

```console
$ go run ./memory -store notes.json "We run shortlink behind Caddy at 10.0.0.2."
$ go run ./memory -store notes.json "Everyone shares one rate limit. What do I set?"
```

The second command starts a new conversation that never saw the first; in a test run, it answered with `TRUSTED_PROXIES=10.0.0.2`, from the note the first one saved.

## A note is a fact with a vector

A note is one short fact, the session that saved it, and its embedding, so notes can be found by meaning the way [RAG](/build/rag/) finds chunks:

```go include=memory/memory.go tag=store
```

Recall sorts the notes by their cosine similarity to the user's message and keeps the closest.
The store is a slice in memory, saved to a JSON file between runs; a few hundred notes need nothing more.

## Recall at the start of a session

Before the agent runs, the notes closest to the user's first message go into the system prompt:

```go include=memory/memory.go tag=recall
```

Recall happens in code, not as a tool the model may or may not call, so a relevant note is in front of the model on its first turn.
The notes are labeled as a record of what the user said, and each carries its ID, which the model needs to replace or delete one.

## Saving and replacing notes

The agent writes notes through two tools:

```go include=memory/memory.go tag=tools
```

`remember` takes the fact and, when it supersedes an earlier note, that note's ID in `replaces`.
Replacing matters as much as saving.
When the user moves from Caddy to nginx, a store that only adds keeps both addresses, and recall may well put the old one first.

The checks around `remember` are in code, where the model can't talk past them: a note holds at most 300 characters, the store holds at most 100 notes, and a note to replace must exist.
The new note is embedded before the old one is deleted, so a failed embedding call loses nothing.

The prompt says what to save:

```go include=memory/answer.go tag=prompt
```

## What the run showed

```result
figure: 8/8
unit: sessions answered right with notes, for 7 percent more than with no memory
task: eight sessions with an assistant for shortlink's documentation; the user mentions their proxy and database in sessions 1 and 2, changes the proxy in session 5, and pastes a log line that asks to be remembered in session 7; four sessions need an earlier fact
models: xiaomi/mimo-v2.6-pro for the answers, voyageai/voyage-4-lite for the embeddings, through OpenRouter
runs: one run of the eight sessions per approach
date: October 2026
source: code/memory

With no memory, the assistant missed exactly the four sessions that needed an earlier fact. With notes, it answered all eight, ended with two notes, both right, replaced the Caddy note when the user moved to nginx, and didn't save the log line's instruction. One run is a single sample.
```

**No memory** failed where it had to.
In session 6, asked which proxy the user runs, it read six documents looking for an answer that only the user had, and spent 10,369 input tokens finding nothing.

**Notes** cost about what no memory cost, 7 percent more, for the embedding calls and the turns that saved notes.
They didn't save money, because the notes hold only what the user said: each session still read the documents it needed.
Session 6 was the exception: the answer was in a note, and the assistant answered in one call with 592 input tokens.

**Carrying the whole conversation** answered all eight too, and was the cheapest by far, $0.0031 against $0.0113.
It carried more than the user's facts: the documents it read in session 1 stayed in the conversation, so it rarely read them again.
That is the right choice while the history is short, and this run is short.

## When the history stops being enough

The input tokens per session, in the output above, show the cost of carrying everything: from session 2 on, each new session starts by resending all the earlier ones, from 1,396 tokens at session 2 to 3,229 at session 8.
In eight short sessions that stays small.
It grows with every session, while notes grow only when the user says something worth keeping.

Size isn't the only cost.
The carried history kept everything: the old proxy address beside the new one, and session 7's log line with its planted instruction, sent again with session 8 and with every session after.
The model noticed that the line was suspicious, and it still had to be sent.
The notes kept two facts and dropped the rest.

```fit
reach: Notes, when an assistant serves the same people over many conversations, more than fit in a context window, or for a long time.
reach: Notes, when what is carried must be checked and corrected: one fact per note, replaced when it changes, with limits enforced in code.
reach: The whole history, for a few short sessions; here, eight of them cost less than a third of what notes cost.
leave: Notes, when one conversation is enough; within a run, a context policy does this job (see [context management](/build/context-management/)).
leave: Carried history, for text you wouldn't want resent forever, such as pasted logs, documents from outside, or anything a user later corrects.
```

## What to watch

**Memory is a place for injected text to persist.**
A note is sent with every future session that recalls it, so an instruction saved by mistake acts again and again.
In this run, the prompt was enough to keep the log line's instruction out; a prompt is one layer, and [prompt injection](/production/prompt-injection/) explains why it shouldn't be the only one.
A second layer is a check on each note before it is saved, by code or by a separate model asked a narrow question: is this a fact about the user's setup, or an instruction?

**Notes go stale.**
Replacing a note needs the model to notice that a new fact contradicts an old one.
It did here, because the user said so plainly.
Store the session or date with each note, so recall can prefer newer notes, and give people a way to see and delete what the assistant remembers.

**Recall by meaning can miss.**
The five closest notes may not include the one that matters, and they always include five, relevant or not.
For a large store, weigh how recent a note is beside how similar, and test recall on its own, as for any [retrieval](/build/rag/).

**Each user needs their own store.**
An assistant for many people keeps one store per person and never recalls one person's notes into another's session.

## Try it

1. Run `go run ./memory` and find, in `memory/testdata/run.json`, the `remember` call in session 5 that replaced note 1.
2. With a key, run two sessions with `-store notes.json`, then open `notes.json` and read what was saved.
3. Remove the last sentence of `memoryPrompt`, the one about never saving instructions, record a run with `-record`, and see whether the log line's instruction gets saved.
4. Add sessions to `sessions.go` until the history row passes 10,000 tokens a session, and compare the cost of `history` and `memory` again.
