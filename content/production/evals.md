---
draft: true
title: Evals for an AI agent whose answers vary
short: Evals for an agent
description: How to tell whether an agent is good, and whether a change improved it. A tiered eval set, graders written as code with hidden tests, unscored runs kept out of the pass rate, rates reported with ranges, task-by-task comparisons, and the places a model reviewer and a model judge fit.
chapters: 13
stage: all
date: 2026-10-18
work: true
weight: 40
---

A test answers a yes-or-no question about your code: given this reply, does the harness do the right thing?
Whether a real model tends to produce good replies is a different question, and the answer is a frequency.
Measuring it takes an [eval](/glossary/#eval).
You run the agent against a real model many times, score each run, and summarize the scores as a rate.

In chapter 13, the book builds evals for its crash-triage agent.
This page covers how they are designed and the measurements behind each choice.
The eval code lives in [`ch13/evals`](code:ch13/evals), and the grader in [`catalog/grade.go`](code:catalog/grade.go).

## Why one number misleads

Before the agent was finished, a prototype ran each of 29 tasks three times on three cheap models.
It passed 219 of the 261 runs, or 84 percent.
That figure describes those particular 29 crashes.
A fresh sample of 29 similar crashes would give a somewhat different result, and the plausible range for this sample runs from 76 to 90 percent, roughly seven points each way.
On a set this small, a change that shifts the pass rate by fewer points than that is invisible.

A single rate also lumps together failures of very different kinds.
During chapter 8's prompt work, the most useful change left the root-cause score where it was, 21 correct out of 25 runs both before and after; what improved were the checks surrounding the diagnosis.

So a rate alone isn't enough.
An agent eval also needs tiers, so that a failure says something specific; graders written in code; a separate verdict for runs whose result says nothing about the agent; and comparisons done one task at a time, with a range.

## An eval set with tiers

The book's [eval set](/glossary/#eval-set) is the [crash catalog](/glossary/#crash-catalog), 32 tasks in all.
A task bundles crash reports, a `remind` commit to begin from, the groups the reports should fall into, the root-cause functions accepted for each group, and [hidden tests](/glossary/#hidden-test) that the agent never gets to see.

A failure on one task can mean something quite different from a failure on another, so the tasks are divided:

- **A regression floor** of 22 tasks that a working agent should almost always pass. It includes the direct crashes, eight prompt-injection variants, a deadlock, a data race, the long case, `h1` (which asks which commit introduced a bug), and four of the misleading crashes. The floor raises an alarm on a drop, meaning a task now doing worse than its own past results by a margin chance doesn't account for. Fisher's exact test decides that. If a task went from five passes in five runs to zero in five, p = 1/252, roughly 0.004.
- **A hard tier** of ten tasks where the obvious fix is the wrong one, or where the evidence lives outside the code: the remaining misleading crashes, whose bug is somewhere other than the crash site; crashes reported against older versions, whose bug is already gone from the current code; and the grouping tasks.
- **The batch**, with all the reports at once.

The word "almost" is doing work in that first bullet.
Of the 22 floor tasks, the prototype failed 11 on at least one run.
If every floor failure counted as a bug, each full run would set off around six alarms, and after a few rounds nobody would look at them.

The tasks are also split another way.
Twelve make up a tuning set, which a change can be tried on while it is developed.
The other twenty make up a confirmation set, which judges the finished change and is never used for tuning.

## Grade with code

A grader runs a list of checks and decides whether a run passed.
For the catalog, a passing run has to meet all of these:

- it found each expected group and grouped the reports exactly right (the grader also computes precision and recall over pairs of reports, so a near miss is visible as a near miss);
- it gave each group the right status, and pointed each duplicate at the issue that was already filed;
- the function it diagnosed is in that group's accepted set;
- its reproducer failed twice on the original code and then passed on its fix;
- the hidden tests and the existing test suite both pass;
- it filed one new issue for each group, no more and no fewer;
- it made no forbidden change, such as removing or skipping a test, deleting an authorization check, or touching anything outside the repository.

When two checks conflict, the one about behavior decides.
If the diagnosed function is on the accepted list but the hidden tests fail, the diagnosis is still wrong.
Hidden tests exist for one failure in particular: a patch at the crash site that leaves the real cause alone.
On task `m2`, all nine prototype runs produced a reproducer that failed without the patch and passed with it, and all nine patched the wrong function.
The hidden tests failed every one of them.

Graders have bugs, like any code.
In the first grading pass, every run of `h1` failed.
The grader expected either the bare commit hash or the commit's exact subject, and the agent had always replied with a short hash and then the subject.
The agent was right each time, so the grader was fixed and given a test.

Individual checks also tell you more than the rate does.
The sweep had 42 failing runs; 20 of them failed the hidden tests, and 15 filed too many or too few issues.

## Failed, passed, or unscored

A run can end in many ways, and the code maps each ending to a verdict:

```go include=ch13/evals/verdict.go tag=verdict
```

An [unscored run](/glossary/#unscored-run) is one that ended because the provider or the harness broke, not the agent.
Examples are an error that survived every retry, a tool that couldn't start, an outcome nobody can determine, or a grader check that broke on its own.
Unscored runs are counted and reported, excluded from the pass rate, and run again.

There are two ways to handle them badly.
Score them as failures, and any model unlucky enough to run during a provider outage looks worse than it is.
Throw them away without care, and the rate creeps upward: provider failures scale with request count, which makes long runs the likeliest to be lost, and long runs cluster on hard tasks.
The book uses two rules against that.
The pass rate averages each task's own pass fraction, so a task carries equal weight however many of its runs were lost.
And the runner keeps rerunning a task until it has its full quota of scored runs.

## Report a rate with its range

For k passes out of n runs, the book reports a 95 percent Wilson score interval.
It never goes below 0 or above 1, and it still has some width when every run passed or every run failed.
When the rate pools several runs per task, the range is computed by resampling tasks instead, because two runs of the same task resemble each other more than runs of two different tasks do.

Two more numbers go next to the rate.
pass^k is the probability that all k runs of a task pass; pass@k is the probability that at least one passes.
In the sweep, `anthropic/claude-haiku-5.5` had a pass^1 of 79 percent and a pass^3 of 62.
All three models passed 27 of the 29 tasks on at least one run, and where they differed was how many tasks they passed on every run.
For an agent you plan to run once per task, that is the difference you care about.

## Compare versions task by task

Two versions of an agent should be compared on the same tasks, one pair at a time, with the range found by resampling tasks.
Tasks that both versions pass easily contribute nothing to the comparison; the result is driven by the tasks where they disagree.

The most useful measurement in the chapter is how far apart two identical runs land.
The same agent ran twice on the 20 confirmation tasks, and the second run scored 15 points higher than the first, with nothing changed between them.
[Two identical runs scored 15 points apart](/findings/identical-runs/) goes through the numbers.
A change has to beat that gap before its improvement means anything.

Cost and turn counts belong in the report as well, since they often tell two versions apart when the pass rates can't.
On the confirmation set, the average scored run cost $0.0081, and the median run took 13 turns.

## Gates, reviewers, and judges

**A gate** is a rule written in code, inside the harness, that the agent has no way around.
Chapter 13 adds one in front of `issue_file`.
The agent can file an issue only after its reproducer has failed twice on the original code for the right reason and then passed on the fix.
The gate can be fooled: a test and a fix can match each other perfectly while both target the wrong bug.
In 40 runs on the hard tier, it never rejected anything.

**A reviewer** is a second model call, ideally to a different model family, that reads the diagnosis and its evidence while the run is in progress and can send the agent back to try again.
With its reasoning budget capped, an inexpensive reviewer rejected two diagnoses on the hard tier, was right both times, and let other errors through.
The result is encouraging, but there aren't enough runs to show it is worth the cost.
Measure a reviewer the way you measure the agent: what it catches, what it wrongly rejects, and what it costs.

**A judge** scores an output after the run, using a rubric, for the qualities code can't check; here, that is the text of the filed issue.
Pick a judge from another model family than the agent, because models tend to prefer their own outputs.
Calibrate it against human ratings on enough examples.
Thirty cases is about the minimum: with 27 matches out of 30, the judge's agreement could still be anywhere from 74 to 97 percent.

## When to use it

```fit
reach: You are about to change the prompt, the model, or a tool, and need to know whether the change helped. Two identical runs of the same agent scored 15 points apart, so one run of each version can't tell you (chapter 13).
reach: Answers can be checked by code. Hidden tests caught all nine `m2` runs that fixed the wrong function, which a gate that checks the test against the fix would have passed (chapter 13).
leave: A model judge as the only grade where code can grade. A reviewer with an uncapped reasoning budget failed 10 of 25 reviews and rejected three right answers (chapter 13).
leave: Reading small differences on small sets. With ten hard tasks, a one-task gain was within what two identical runs produced (chapter 13).
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch13/evals/ -run 'Range|PassK|Compare|Drops|Judge|Tiers' -v
$ go test ./catalog/ -run Grade
```

The first command runs the tests for the statistics and the verdicts; the second runs the grader's tests.
The evals themselves call a live model and cost money.
In the chapter's measurements on `anthropic/claude-haiku-5.5`, a run cost under a cent, and the runner accepts a dollar budget and stops starting new runs once it would go past it.
