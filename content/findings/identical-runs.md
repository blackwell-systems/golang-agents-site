---
title: Two identical runs of the same agent scored 15 points apart
short: Two identical runs, 15 points apart
description: Chapter 13 ran the same agent twice on each of 20 tasks and scored the second run against the first, as though one were a new version. The second came out 15 points ahead, with nothing changed. How to tell a real improvement from that, with ranges, paired comparisons, and counts.
card: The same agent, run twice, scored 15 points apart. How to tell a real improvement from chance.
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

Run twice on each of 20 tasks, the same agent’s second run beat its first by 15 points: 5 tasks better, 2 worse, 13 the same. The agent, prompt, and model were identical both times. The 95 percent range, −10 to +40 points, includes zero, and so it reports no difference.
```

An agent's answers vary from run to run.
Part of that is sampling: each token is drawn from a distribution.
Part remains even at temperature 0, because providers batch requests and the order of the arithmetic shifts slightly with load.
So any [eval score](/production/evals/) is one draw from what the agent might have done, and before trusting it you need to know how far it drifts when nothing has changed.

The measurement below does exactly that.

## What was tested

The agent is [the crash-triage agent](/run/ch07-crash-triage/) from the companion code: given crash reports from a small Go web service, it groups them, finds each cause, writes a reproducer test, fixes the code, and files an issue.
Each task in the crash catalog comes with a grader, code that checks the result: the grouping, the root cause, the reproducer failing before the fix and passing after, hidden tests the agent never sees, and more.
No model judges the result.

The confirmation set is 20 tasks that were never used to tune the prompt.
The same agent, with the same prompt, ran twice on each, on `claude-haiku-5.5` in October 2026.
Then the two runs were compared task by task, as if they were two versions of the agent.

## The result

Scored against the first run, the second came out 15 points ahead: 5 tasks better, 2 worse, 13 the same, and a 95 percent range from -10 to +40.

Between the two runs, nothing was changed.
The 15 points are chance.
So a prompt change that gains 15 points, judged on a single run of each task on a set this size, has proved nothing so far.

Cost and turn count swing just as widely between identical runs.
Under one [context policy](/build/context-management/), a 99-report batch took 62 turns and cost $0.42 on one run, then 30 turns and $0.06 on the next.

## Pair the comparison

To compare two versions, compare them task by task, and resample the tasks:

```go include=ch13/evals/stats.go tag=compare
```

Pairing cancels out everything the two versions have in common.
A task that both pass contributes zero to the difference, and so does one that both fail; only the tasks where they disagree move the number.
The range comes from resampling the tasks with replacement, ten thousand times, and taking the middle 95 percent of the averages, because the tasks, not the runs, are what a small eval set has too few of.
Below twelve tasks, a resampled range comes out too narrow, so the code reports no range at all, only the counts of tasks better, worse, and tied, and each task's results.

Here is what pairing can separate, from comparisons recorded on a separate sweep of 29 tasks:

| Comparison | Difference, task by task | Tasks better, worse, tied |
| --- | --- | --- |
| A prototype’s prompt against a plain one | +32 points, +20 to +45 | 20, 3, 6 |
| `deepseek-v4.1-flash` against `claude-haiku-5.5` | +10 points, 0 to +22 | 8, 2, 19 |

The first is a real difference: its range is far from zero.
The second just touches zero, so it can't rank the two models.

## Rates have ranges too

A pass rate of k out of n is an estimate of a rate nobody can see directly.
The companion code reports it with a Wilson score interval, which, unlike the familiar formula, stays between 0 and 1 and keeps a width when every run passed or every run failed, which small evals hit often.
Its width falls as one over the square root of n, so halving it takes four times as many runs.

An eval has two kinds of n, runs and tasks, and extra runs help with only the first.
More runs of each task pin down how the agent does on those particular tasks.
They can't make up for a small set of tasks: a change that helps on these 20 may not help on the next 20.
So when several runs of each task are pooled, the range has to come from resampling tasks.
A range computed over runs counts every run as independent evidence and comes out too narrow, because two runs of the same task resemble each other more than runs of two different tasks do.

## Every time, or at least once

A pass rate hides a difference that matters in production.
With two runs per task on the confirmation set, the agent passed 72 percent of runs on average; it passed 55 percent of tasks on both runs (pass^2) and 90 percent on at least one (pass@2).
A good share of what it fails on one attempt, it gets right on the next.

Two agents with the same pass rate can be very different to run: one passes every task two times in three, while the other passes the same two thirds of the tasks on every run and fails the remaining third on every run.
If you'll run the agent once, the second is the one you want, provided you can tell which tasks those are.

## What it can't show

This is one comparison on 20 tasks with one run of each version, on one model.
The range would narrow with more runs per task and more tasks; that is the point.
And chance can go the other way: a real improvement can come out flat or negative on one run.

## What to do with it

- **Before believing a change, measure the spread of the unchanged agent.** A difference smaller than two identical runs' difference means nothing yet.
- **Compare task by task, and report the range.** "Better on 5, worse on 2, tied on 13, -10 to +40" says far more than "+15".
- **Report counts when there are too few tasks for a range.** On five tasks, say which ones moved; one task going from 0 of 5 runs passing to 5 of 5 is a strong signal even when an average across tasks isn't.
- **Keep a confirmation set out of tuning.** A prompt tuned until every task passes is fitted to the set, and the set stops measuring anything else.
- **Watch cost and turns too.** They often separate versions when pass rates don't.

## Try it

The eval runs were live, and their records ship in the repository, under `code/ch13/testdata/evals/`. A test, `TestLive`, recomputes this comparison from them offline and checks it against `ch13/evals/testdata/live.txt`, and the statistics themselves are plain Go with tests.
From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch13/evals/ -v -run 'Live|Compare|Range'
```

The functions are in [`ch13/evals/stats.go`](code:ch13/evals/stats.go): `Rate.Range`, `Compare`, and the pass@k and pass^k estimators.
