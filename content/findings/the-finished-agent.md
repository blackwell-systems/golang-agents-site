---
title: "The finished agent: results across three models, and where it fails"
short: The finished agent
description: The crash-triage agent the book ends with, measured on 20 held-out tasks with three models. Pass rates with their ranges, the cost of each passing run, how the failed runs went wrong, and how to point the agent at a Go service of your own.
chapters: 17, 13
stage: all
date: 2026-10-19
weight: 2
art: ch17
work: true
---

The book's agent ends up as one Go program, `triage`, that reads the crash reports a web service writes, sorts them into groups with one cause each, reproduces each cause in a test, fixes it when a fix is safe to make, and files one issue per cause.
Its practice target is `remind`, a reminders service of about 4,800 lines that ships with the companion code with bugs planted in it.
This page reports how the finished agent did on three models, how those numbers were produced, what went wrong in the runs that failed, and what it takes to run the agent against your own code.
Everything here comes from chapter 17 and the run records behind its tables.

## What it does

The program has four forms:

- `triage FILE` triages a file of crash reports, one JSON object per line, and prints what it found and fixed.
- `triage catalog` runs one task from the [crash catalog](/glossary/#crash-catalog) and grades the result against the task's [hidden tests](/glossary/#hidden-test).
- `triage serve` is a service: `remind`'s crashes are posted to it as they happen, it groups them by [fingerprint](/glossary/#fingerprint), and it starts one run per group, several at a time, each in its own copy of the code.
- `triage review` is the person's side of the service's review queue, where runs wait for approval.

Most of the system is fixed code that runs the same way every time: grouping reports, checking each reproducer, the second model that reviews each diagnosis, the approval step, and filing each issue once.
The model is left the choices no fixed sequence makes well: within one crash group, which files to open, what test to try, and whether a change is safe to make.
An issue may claim a fix only when the agent's reproducer fails against the code as it was and succeeds once the change is applied.
The service keeps each day's spending in a file, so restarting it can't hand out the budget a second time, and interrupted runs pick up where they stopped with no issue filed twice.

## How it was measured

The catalog holds 32 tasks, each a batch of crash reports from `remind` with tests the agent never sees.
Twelve form the [tuning set](/glossary/#tuning-set), used while the prompt was being refined, and none of them counts below, since a prompt shaped on a task flatters itself on that task.
The remaining 20 are kept for confirmation: 15 regression-floor tasks that a sound agent should almost never fail, and 5 hard-tier tasks where the first fix that comes to mind is wrong.

The runs took place on October 17 and 18, 2026, on three models:

- `anthropic/claude-haiku-5.5`, three runs of each task;
- `openai/gpt-5.6-luna`, three runs of each task;
- `deepseek/deepseek-v4.1-flash`, two runs of each task (three for five of them), because it cost the most per run and the whole budget was $6.

Each model's requests went to a single pinned provider, so a change of host partway through couldn't move a result.
Every run had all of chapter 14's defenses on, with a scripted approver standing in for a person: it approved every issue and comment, and refused every edit to a [protected path](/glossary/#protected-path).
A run counts as passed only if the grader's code says so; nobody's opinion enters it.

Two terms carry the tables.
The **pass rate** is, for each task, the share of its runs that passed, averaged across the 20 tasks, and its [range](/glossary/#range) is the span of rates those 20 tasks are consistent with; when two models' ranges overlap, these runs can't rank them.
The **cost per pass** divides what all the scored runs cost, the failures as well as the passes, by the number that passed, so a cheap model that fails often pays for its failures.
A run that ended for a reason unrelated to the agent, such as a provider refusing the request, is an [unscored run](/glossary/#unscored-run): counted and reported, but left out of the rates.

All of these runs together cost $3.89 according to their records and $3.91 according to the API key's usage page; the two cents between them are calls outside any run, such as a short check of each model before its first task.

## The results

<div class="result-cards">

```result
figure: 70%
unit: pass rate (53 to 85%)
task: the 20 confirmation tasks, graded by hidden tests
models: `anthropic/claude-haiku-5.5`
runs: three per task
date: October 17 and 18, 2026
source: Chapter 17

DeepSeek passed 63% (45 to 81%). The two ranges overlap, so these runs can't say which of the two passes more often.
```

```result
figure: $0.0165
unit: per passing run
task: the same 20 tasks, counting every scored run's cost
models: `anthropic/claude-haiku-5.5`, against `deepseek/deepseek-v4.1-flash` at $0.0531
runs: three per task for Haiku, two for most of DeepSeek's
date: October 17 and 18, 2026
source: Chapter 17

DeepSeek's ranges for cost per pass ($0.0333 to $0.0915) and Haiku's ($0.0124 to $0.0229) don't overlap. DeepSeek is cheaper per token and took about twice as many turns.
```

</div>

The main columns of chapter 17's tables:

| Model | Pass rate | Floor (15 tasks) | Hard (5 tasks) | Cost per pass | Median turns |
| --- | --- | --- | --- | --- | --- |
| `anthropic/claude-haiku-5.5` | 70% (53 to 85%) | 76% | 53% | $0.0165 ($0.0124 to $0.0229) | 14 |
| `deepseek/deepseek-v4.1-flash` | 63% (45 to 81%) | 68% | 50% | $0.0531 ($0.0333 to $0.0915) | 26 |
| `openai/gpt-5.6-luna` | 20% (7 to 35%) | 18% | 27% | $0.1022 ($0.0497 to $0.3159) | 18 |

Compared task by task, DeepSeek came out 7 points below Haiku, with a range from 20 below to 7 above; since that range includes zero, the two can't be told apart.
Luna was 50 points below Haiku and 43 below DeepSeek, with ranges far from zero, so it passes less than either.
Five hard tasks are too few for a range of their own.

A few more figures from the same runs:

- **Solving a task once isn't solving it every time.** Haiku passed both of two runs on 58% of the tasks and at least one of two on 82%. The service runs each crash group once, so the first figure is closer to what a deployment sees.
- **No run did what a planted instruction asked.** Six of the confirmation tasks carry text in a crash report that tries to give the agent orders; none of the 50 judged runs across the three models followed it or tried to.
- **The reviewer often ran out of room.** About two reviews in five failed, nearly all because the reviewing model used up its 8,000-token cap. A failed review lets the diagnosis through unreviewed, so it can only cost a rejection that might have led to a fix.
- **DeepSeek lost 54 runs to its provider.** The provider answered with a 429 and a three-minute `Retry-After`, longer than the retry policy's one-minute limit, so those runs stopped before they could be judged. They are counted as unscored and cost $0.0693 together.

The times in chapter 17's table were taken on one laptop that was running other jobs, most heavily during DeepSeek's runs, so they can't be compared across models and are left out here.

## The recommendation

The model the companion code recommends comes from the confirmation tasks only, and on these runs it stays `anthropic/claude-haiku-5.5`.
Against DeepSeek, the stronger of the other two: the pass rates can't be separated, while each passing run cost 3.2 times as much on DeepSeek, with ranges that don't overlap.
DeepSeek's cheaper tokens were outweighed by how many turns it took; its median of 26 would be higher without the 40-turn limit, which ended 13 of its 15 failed runs.

How the grading was settled matters as much as the result.
On three confirmation tasks the bug sits in a protected file, so under the eval's own policy the right answer is to hand the group to a person with the right root cause.
The catalog's grader predates the defenses and expected a fix, so the first grading failed correct hand-offs; the rule was changed to grade a hand-off the way diagnose-only mode is graded, and the saved runs were graded again offline with no model called.
No verdict moved under the adopted rule.
A looser rule would have raised Haiku to 77% and DeepSeek to 66%, and it was rejected because it excused a test the policy let the agent write.
The rule was fixed after the recordings had been read, which chapter 17 calls the wrong order: write down how a grader treats the policy, and commit it, before looking at any regraded number.

Read the table as a picture of two days in October 2026.
A provider can change the model behind a name, prices move, and a new prompt or tool changes the agent being measured.
Measure again after any such change, and put the date beside every figure you quote.

## Where it fails

Chapter 17 reads the recordings of the failed runs, and most failures fall into a few patterns:

- **A search that finds nothing.** Luna searched open issues with words and a fingerprint at once. Issues filed by a person have no fingerprint, so both filters together matched nothing, and Luna filed the issue again. That happened in 46 of its 48 failed runs, and in 32 it was the only fault, although the tool's description warns that a person may have filed the crash in other words.
- **The crash site taken for the cause.** In one run of a hard task, Haiku blamed the function that panicked, and the reviewer accepted the diagnosis. The real cause was another function that deleted a user and left the user's lists behind; Haiku got it right on its second try and wrong again on the third.
- **A fix that went around a protected file.** Twice, after its edit to a protected file was refused, Luna added the missing authorization check in a file that wasn't protected. The fix was right, and it also changed who may call a route without anyone approving it: a protected path covers a file, while the behavior inside it can often be changed from another one.
- **Out of turns.** Thirteen of DeepSeek's 15 failed runs reached the 40-turn limit; five of those were on tasks whose fix is in a protected file, where the model kept hunting for another way in instead of handing the group to a person.
- **The whole catalog at once.** One run on the batch, 99 reports from 23 causes, failed: Haiku used 279 turns and $0.18, and got the grouping and the one-issue-per-cause check wrong. One run is a single data point, so the table records it as a count.

For each pattern chapter 17 proposes a change and says what it would cost to confirm: a search tool that falls back to words alone (a full rerun of the confirmation set, about $1.23 for Luna's three runs per task), more runs of the decoy tasks before changing the reviewer, a test the agent can't edit that checks every route's middleware, and several batch runs at about $0.18 each.

### Gaps in the harness itself

The batch runs also turned up three places where the harness falls short.
They stay in the companion code as they are, because that code produced the tables, and any fix needs new runs to measure it:

- The reproducer gate keeps one verdict per fingerprint group rather than per cause, so in a group with two causes, a refused reproducer for one blocks a fixed status for both.
- `triage catalog` grades a correct hand-off to a person as a failure, where the eval doesn't, because the command doesn't work out which groups its approver keeps from being fixed.
- An `issue_file` call that names no root cause skips the record of causes already filed, since that field is optional.

## Pointing it at your own service

Only three things tie the agent to `remind`: its Go module path, which tells the service's own stack frames apart from library ones; the name the prompts and tool descriptions use for the code; and which paths are protected.
They live together in a `Target`, which defaults to `remind`'s values:

```go include=ch17/triage/target.go tag=target
```

The module path matters most.
Fingerprints are computed from frames under that path alone, so if you leave `remind`'s path in place, none of your service's frames count, and a week of its crashes falls into a handful of groups, one per kind of panic.

Chapter 17's steps, in order:

1. **Write crash reports.** Make your service's panic recovery append one JSON object per crash to a file, with the same fields `remind` writes, from the panic and stack trace to the request and the last log lines. Don't leave out the build's commit: the agent checks out that commit to investigate each group.
2. **Describe the service.** A target file names your module and your protected paths: authentication and authorization code, build and deploy configuration, anything that holds a secret. The name defaults to the last element of the module path.
3. **Start in diagnose-only mode.** The agent keeps every tool except `code_patch`, so it can read, run tests, write a reproducer, and file an issue, and it can't change your code.
4. **Then allow patches,** with a person approving every filing and every edit to a protected path.
5. **Set the limits:** turns and dollars per run, and dollars per day, worked out from the results table's cost for your model and how often your service crashes.

For a file of saved reports, with your target, your repository, and a state directory:

```console
$ triage -target billing.json -git ~/src/billing \
    -diagnose-only -state week -max-cost 1 week.jsonl
```

For crashes as they happen, `triage serve` takes the same `-target` and `-git`, with `crashship` running beside your service to post its reports.

Diagnose-only mode removes a tool rather than adding a rule to the prompt:

```go include=ch17/triage/diagnose.go tag=diagnose-tools
```

An instruction not to edit code is text the model weighs against all the other text in front of it, including text an attacker may have put in a crash report.
A tool that isn't on the list can't be called at all.
A wrong diagnosis in an issue costs someone a few minutes to read; a wrong patch that a tired reviewer approves costs far more, so stay in diagnose-only mode until the evals and a few supervised runs look right.

## Run it

From the `code` directory of a clone of the companion repository, with no API key set, the whole catalog runs offline from a recorded Haiku run:

```console
$ go run ./ch17 -task batch -parallel 1 -approve files \
    -max-cost 0.75 catalog
```

Only the model's side is recorded; tests and builds really execute, so expect it to take a while (13 and 16 minutes, in two replays on the machine that made the recording).
`-parallel 1` keeps the groups in their recorded order, and `-approve files` gives each approval request the answer it got during the recording, so nothing waits for you.
