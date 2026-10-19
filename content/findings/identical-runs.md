---
draft: true
title: Two identical runs of the same agent scored 15 points apart
short: Two identical runs, 15 points apart
description: Chapter 13 ran the same agent twice on each of 20 tasks and compared the runs as if they were two versions. The second run scored 15 points higher, with nothing changed. How to tell a real improvement from that, with ranges, paired comparisons, and counts.
chapters: 13
stage: all
date: 2026-10-18
weight: 50
art: ch13
---

```result
figure: +15
unit: points, from chance
task: the confirmation set of the crash-triage eval, the same agent run twice and compared as two versions
models: `anthropic/claude-haiku-5.5`
runs: two per task, 20 tasks
date: October 2026
source: Chapter 13

Run twice on each of 20 tasks, the same agent’s second run beat its first by 15 points: better on 5 tasks, worse on 2, tied on 13. Nothing changed between the runs. The 95 percent range, −10 to +40 points, holds zero, and says so.
```

An agent's answers vary from run to run.
Part of that is sampling: each token is drawn from a distribution.
Part remains even at temperature 0, because providers batch requests and the order of the arithmetic shifts slightly with load.
So every eval result is a sample, and the question is how much it could move without anything changing.

Chapter 13 of the book measured exactly that.

## What was tested

The agent is the book's crash-triage agent: given crash reports from a small Go web service, it groups them, finds each cause, writes a reproducer test, fixes the code, and files an issue.
Each task in the crash catalog comes with a grader, code that checks the result: the grouping, the root cause, the reproducer failing before the fix and passing after, hidden tests the agent never sees, and more.
No model judges the result.

The confirmation set is 20 tasks that were never used to tune the prompt.
The same agent, with the same prompt, ran twice on each, on `claude-haiku-5.5` in October 2026.
Then the two runs were compared task by task, as if they were two versions of the agent.

## The result

The second run against the first: +15 points, better on 5 tasks, worse on 2, tied on 13, with a range of -10 to +40.

Nothing changed between the runs.
The 15 points are chance.
A new prompt that scores 15 points higher on one run per task, on a set like this one, has shown nothing yet.

The same thing shows up at the scale of a single run.
Chapter 8 ran the same context policy twice on the same 99-report batch: 62 turns and 42 cents the first time, 30 turns and six cents the second.

## Pair the comparison

To compare two versions, compare them task by task, and resample the tasks:

```go include=ch13/evals/stats.go tag=compare
```

Pairing removes what the two versions share.
A task both find easy adds a difference of zero, and so does a task both find impossible; only tasks where they differ move the result.
The range comes from resampling the tasks with replacement, ten thousand times, and taking the middle 95 percent of the averages, because the tasks, not the runs, are what a small eval set has too few of.
Below twelve tasks, a resampled range comes out too narrow, so the code reports no range at all, only the counts of tasks better, worse, and tied, and each task's results.

Here is what pairing can separate, from chapter 13's comparisons on 29 tasks:

| Comparison | Difference, task by task | Tasks better, worse, tied |
| --- | --- | --- |
| A prototype’s prompt against a plain one | +32 points, +20 to +45 | 20, 3, 6 |
| `deepseek-v4.1-flash` against `claude-haiku-5.5` | +10 points, 0 to +22 | 8, 2, 19 |

The first is a real difference: its range is far from zero.
The second just touches zero, so it can't rank the two models.

## Rates have ranges too

A pass rate of k out of n is an estimate of a rate nobody can see directly.
The companion code reports it with a Wilson score interval, which, unlike the familiar formula, stays between 0 and 1 and keeps a width when every run passed or every run failed, which small evals hit often.
Its width shrinks roughly with the square root of n: four times the runs halve it.

There are two kinds of n, and more runs only help with one.
Running each task more times narrows the uncertainty about those tasks.
It does nothing about having few tasks: a change that helps on these 20 may not help on the next 20.
So when several runs of each task are pooled, the range has to come from resampling tasks; a range over runs treats every run as independent and comes out too narrow, since runs of one task are more alike than runs of different tasks.

## Every time, or at least once

A pass rate hides a difference that matters in production.
With two runs per task on the confirmation set, the agent passed 72 percent of runs on average; it passed 55 percent of tasks on both runs (pass^2) and 90 percent on at least one (pass@2).
Much of what it gets wrong on one try, it gets right on another.

Two agents with the same pass rate can be very different to run: one passes every task two times in three, the other passes two thirds of the tasks every time and the rest never.
If you'll run the agent once, the second is the one you want, provided you can tell which tasks are its.

## What it can't show

This is one comparison on 20 tasks with one run of each version, on one model.
The range would narrow with more runs per task and more tasks; that is the point.
And chance can go the other way: a real improvement can come out flat or negative on one run.

## What to do with it

- **Before believing a change, measure the spread of the unchanged agent.** A difference smaller than two identical runs' difference means nothing yet.
- **Compare task by task, and report the range.** "Better on 5, worse on 2, tied on 13, -10 to +40" says far more than "+15".
- **Report counts when there are too few tasks for a range.** On five tasks, say which ones moved; 0 of 5 becoming 5 of 5 on the same task is a strong signal even when an average across tasks isn't.
- **Keep a confirmation set out of tuning.** A prompt tuned until every task passes is fitted to the set, and the set stops measuring anything else.
- **Watch cost and turns too.** They often separate versions when pass rates don't.

## Try it

The eval runs were live, and their records ship in the repository, under `code/ch13/testdata/evals/`. A test, `TestLive`, recomputes this comparison from them offline, and the statistics themselves are plain Go with tests.
From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch13/evals/ -v -run 'Live|Compare|Range'
```

The functions are in [`ch13/evals/stats.go`](code:ch13/evals/stats.go): `Rate.Range`, `Compare`, and the pass@k and pass^k estimators.
