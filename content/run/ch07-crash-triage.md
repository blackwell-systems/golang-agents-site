---
title: One crash report, triaged by an agent
short: "ch07: One crash report, triaged"
description: The crash-triage agent's first run on a real Go web service, replayed offline. Four crash reports, two already filed, one bug that lives away from the line that crashed; fourteen tools, a final answer in a fixed shape, and a grade from the crash catalog.
chapters: 7
stage: all
date: 2026-10-18
weight: 30
art: ch07
---

From chapter 7 on, the companion code's agent has a real job: triaging crash reports from `remind`, a small web service for reminders (twenty packages, roughly 4,800 lines of Go), with bugs seeded in it on purpose.
This program runs one task from the crash catalog, replayed from a run recorded on `anthropic/claude-haiku-5.5` in October 2026, with every tool working for real against a fresh copy of the service.

## What it shows

- An agent with fourteen tools grouped by prefix: `crash_*` to read reports, `code_*` to read and patch code, `issue_*` for the tracker, `run_tests`, and a final tool, `submit_diagnosis`. [Designing tools a model uses well](/design/tool-design/) explains the choices.
- A final answer as typed data, one `Diagnosis` per crash group, checked before it is accepted: [structured output](/design/structured-output/).
- A reproducer test that must fail on the original code and pass on the fix.
- A grade from the crash catalog's grader, check by check.

## Run it

From the `code` directory of a clone of [go-agents-book-code](https://github.com/blackwell-systems/go-agents-book-code), with no `OPENROUTER_API_KEY` set:

```console
$ go run ./ch07
```

The task is `m1-done-after-delete`, which hands the agent four crash reports.
The tracker already has issues for two of them.
The other two crashed in one place because of a mistake made somewhere else.
The run starts like this:

```text include=ch07/testdata/out/m1.txt lines=2..12 title="go run ./ch07, the first turns"
```

First the model gets the list of reports and opens all four in a single turn; then it goes to the code that each stack points to.
Over 17 turns it searches the tracker, writes a reproducer test for each of the three causes, runs them to see them fail, fixes all three, runs the whole suite, comments on the two filed issues, files one new issue, and submits.

## What to look for

**The root cause it found.** The service panicked in `api.owned`, dereferencing a nil reminder.
The real fault is upstream, in `store.Get`: asked for a deleted reminder, it returns a nil reminder together with a nil error.
Patching `api.owned` with a nil check would have ended this particular panic, but every other function that calls `Get` would still be open to the same mistake.
The agent's evidence, in its final answer, cites the stack, the code, and the log line showing the reminder was deleted.

**The grade:**

```text include=ch07/testdata/out/m1.txt lines=210..223 title="The end of the run, and the catalog's grade"
```

The run passed 12 of 16 checks.
The four failures say the agent marked both already-filed crashes as `fixed`, treating them as its own to fix, instead of as duplicates of issues #101 and #102.
The first system prompt never tells the model how to handle a report the tracker already knows about, and chapter 8's prompt work starts from exactly this failure.
A typed answer is what made the mistake a failed check instead of a paragraph to read.

**The cost.** 17 turns, $0.034, and every request is bigger than the one before, growing from roughly 3,800 tokens to roughly 29,600.
[Context management](/design/context-management/) is what brings that down.

## Things to try

**Check whether the model picks the right tool.** A cheap smoke test, replayed offline:

```console
$ go run ./ch07/evals/toolselect
$ go run ./ch07/evals/toolselect -distractors
```

The first replays fifteen short requests, three times each on four models, and prints each answer that scored below full marks; the second adds thirty plausible extra tools.
Several "misses" are the system prompt winning: asked to file an issue, models search the tracker first, as the prompt tells them to.

**Read a tool's output the way the model does.** The recorded tool outputs under `ch07/testdata/out/tools` show what concise results look like: `crash_read` shows the service's own stack frames, one line each, and `crash_goroutines` condenses an 85 KB goroutine dump, 46 goroutines, into 8 groups that take 7 KB.

**Run it live**, with a key and `OPENROUTER_MODEL` set to another tool-capable model, and compare its grade with the recorded one.
This makes real model calls; the recorded run cost $0.034.
Expect a different path, and possibly a different grade; one run says little, which is what [evals](/production/evals/) are for.

## Key files

| File | What it holds |
| --- | --- |
| [`ch07/triage/tools.go`](code:ch07/triage/tools.go) | The fourteen tools and their safety flags |
| [`ch07/triage/diagnosis.go`](code:ch07/triage/diagnosis.go) | The `Diagnosis` type, `submit_diagnosis`, and the reproducer check |
| [`ch07/triage/crashtools.go`](code:ch07/triage/crashtools.go) | Reading, searching, and summarizing crash reports |
| [`ch07/triage/redact.go`](code:ch07/triage/redact.go) | Removing credentials from tool output before the model sees it |
| [`ch07/agent/registry.go`](code:ch07/agent/registry.go), [`schema.go`](code:ch07/agent/schema.go) | Tools from typed Go functions, and schemas from structs |
| [`remind`](code:remind) | The practice service, with its seeded bugs |
| [`catalog`](code:catalog) | The crash catalog, its tasks, and the grader |
