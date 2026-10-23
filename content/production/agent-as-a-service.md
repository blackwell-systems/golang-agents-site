---
title: Running an AI agent as a long-running service in Go
short: An agent as a service
description: What changes when nobody is watching the agent. A static binary that picks up its own unfinished runs, a shutdown that cancels and saves inside the platform's grace period, a container that keeps the key out of every environment, and a service that takes crash reports over HTTP, queues work in a directory, keeps its spending on disk, and parks decisions in a review queue.
card: Running an agent unattended: restarts, shutdown, secrets, a work queue, budgets, and reviews.
group: Running it
chapters: 12, 15, 17
stage: all
level: advanced
date: 2026-10-18
work: true
weight: 80
art: ch17
---

While you watch an agent, you are part of its harness.
You restart it when it dies, you say yes or no to its risky calls, and you notice when the bill climbs.
Leave it running and the program has to do those jobs itself.
The crash-triage agent becomes a command that can run unattended, and that command becomes a service that takes crash reports as they happen.
This page lists the decisions in the order you meet them, with code from [`ch15`](code:ch15) and [`ch17/service`](code:ch17/service).

## A binary that finds its own work

The agent has no C dependencies, so `CGO_ENABLED=0 go build -o triage ./ch15` produces a single static binary.
A static binary does not depend on the system's C library or any other shared library, so it runs on a machine with nothing installed.
`CGO_ENABLED=0` turns off cgo, the part of the `go` command that lets Go code call C, so the build cannot pull in C code.
Its runs live in one state directory, named with `-state` or the `TRIAGE_STATE` variable, so a service manager sets it once.

Platforms restart containers by running the same command again, so the same command has to continue rather than begin anew.
When no `-run` is given, the command looks for the newest unfinished run of its task and resumes it, and only starts a fresh run if there is none.
Fresh run IDs combine the task, the time, and eight random characters.
They are never reused, because [idempotency keys](/glossary/#idempotency-key) are derived from them, and when old runs are pruned (after seven days unless `-keep` says otherwise) a small tombstone file keeps the ID taken.
The command saves a new run to disk before doing anything with it: once a run exists on disk, something will finish it.

Not all of a run's memory is in the loop's state.
The tools remember which files the model has read (the patch tool won't edit a file the model never saw) and the agent's notes, in a `tools.json` beside the checkpoint.
The random boundary the agent puts around untrusted text, which [prompt injection](/production/prompt-injection/) explains, is kept in a file too, since the instructions in the saved conversation name it.
And the approval gate's denials, held in memory during a run, are rebuilt from the audit log on restart when the command runs with `-audit`.

## Shut down by cancelling

The conventional graceful shutdown stops taking new work and waits for current work to finish.
An agent can't wait.
Platforms send SIGTERM and follow it with SIGKILL after a grace period: 30 seconds by default on Kubernetes and 10 for `docker stop` with a Linux container.
Across the 209 model calls of the recorded live runs, the median took 2.35 seconds, the 95th percentile 9.25, and the slowest 27.5, longer than `docker stop` allows and close to Kubernetes' limit.

So the command cancels:

```go include=ch15/durable.go tag=shutdown
```

The `go func()` block runs in the background as a [goroutine](/go/goroutines/), and `select` waits for whichever of its channels receives first: a signal, or the `done` channel that `stop` closes.
The prose below says what it does, so you can read on without tracing it.

The first signal cancels the run with `ErrShutdown` as its cause.
The model call in flight is abandoned and will be made again after the restart; tool calls that hadn't started get a "cancelled" answer that is kept out of the saved state; and the loop saves.
A second signal ends the process at once, for whoever can't wait for the save.

Calls that are already talking to the outside world get a short reprieve.
A call that files an issue in the tracker is settled once its answer arrives, while one cut off halfway leaves an outcome nobody knows, so a tool wrapper gives calls with side effects five more seconds after a shutdown before cancelling them.

<div class="hood">

The wrapper does that with `context.WithoutCancel`, which keeps the call's values, its idempotency key among them, and forwards the cancellation on its own schedule.
Five seconds sits well inside both platforms' limits, and since the wrapper is applied inside the timeout wrapper, each tool's own time limit keeps working.

</div>

## A container image that keeps the key out of reach

The agent runs `go test` and creates each workspace as a git checkout at the crash's commit, so the image needs the Go toolchain and git.
The image is 1.34 GB, almost all toolchain, runs as a user without root (uid 10001, the number that identifies the user a process runs as), and mounts the state directory as a volume.

Give it the API key as a read-only mounted file named by `OPENROUTER_API_KEY_FILE`, readable by that user alone; the command refuses a key file that its group or others can read.
A key passed with `-e` is worse in two ways.
Anyone allowed to run Docker commands can read it with `docker inspect`, the command that prints a container's settings, environment included.
And although the command removes the variable from its own environment at startup, the kernel still holds the environment the first process started with, and any process of the same user can read it from `/proc/1/environ`, the file Linux keeps for the environment of process 1, which in a container is the first process started.
This was tried with a made-up key, and another process in the container read the key back.
Test code runs as that same user inside the image, which also means the tests are at rung 2 of the [sandbox ladder](/production/prompt-injection/) at best.
Stronger isolation for the tests means a separate sandbox beside the agent's container.

<div class="hood">

The reason is that Docker's default settings don't allow the namespaces (the Linux feature that gives a process its own private view of files, processes, or the network) that bubblewrap, a Linux sandboxing tool, needs.

</div>

## From a command to a service

Crashes don't come in batches. They trickle in, day and night, from a program that shouldn't slow down while reporting them.
`triage serve` takes them as they occur, assigns each to a group by its fingerprint, and runs that group's investigation when there is room.

**Ship reports from a file.** The practice service already appends each crash report to a file as a JSON line.
A small shipper, `crashship`, follows that file and posts each complete new line, recording its position after every post.
Report-writing code runs in the middle of a panic, a poor moment to wait on a network, and a line in a local file outlives a process that is about to die.
The service can move or go down for an hour without a new release of the program that crashes; the shipper backs off on 429 and 503 and catches up.
A log shipper such as Vector or Fluent Bit can play the same part.

**Guard the door.** Whoever can post reports can start paid runs and put any text they like in front of the agent.
The ingest endpoint takes its own secret, separate from the reviewers' tokens, so posting reports never grants the power to approve a patch.
Secrets are compared with `subtle.ConstantTimeCompare` after hashing, must be at least 32 characters, and are read from files; a per-source limit answers 429 per one-minute window, `http.MaxBytesReader` caps a report at 256 KiB, and credentials are redacted from each report before it is written anywhere.

**Recognize repeats.** The shipper resends after a timeout, so a report the service has already stored gets a 200 and changes nothing, which is [deduplication](/glossary/#deduplication), the receiver's half of an idempotency key.
The crashing program numbers its reports from 1 in each process, so its IDs repeat after every restart.
The service files each report under its own key instead, a hash of the ID, the time, the commit, and the service name: a resend repeats all four, and a new crash after a restart can't match the time.

**One run per group.** A run takes its messages at the start and has no inbox, and a second report for the same crash mostly says that it happened again.
A report that arrives while its group's run is going is saved as late, and when the run ends, the service gathers the late reports into one comment on the issue the agent filed for that group (each crash group gets one issue in the [tracker](/run/ch10-mcp-tracker/)).
That comment passes through the same [egress check](/production/prompt-injection/) and [approval policy](/production/approval/) as the agent's own comments, since its text came from a crash report.
A report for a group whose issue was closed means the crash returned after a fix, and it starts a new run.

```diagram
name: ch17-architecture
alt: A diagram of the crash-triage system: a shipper sends remind's crash reports to the service, which groups them by fingerprint and queues one run per group; each run is the agent loop wrapped in middleware, with a reproducer gate inside issue_file that each issue must pass to be filed, and a reviewer that reads the diagnosis after filing; pauses go to a review queue that a person answers; each box is labeled with the chapter that built it

The crash-triage service: fixed workflow in the solid boxes, and the agent, where the model decides, in the dashed one.
```

## The queue is a directory

The service has no message broker (a separate program, such as RabbitMQ or Kafka, that holds a queue of work between the parts of a system).
Every file under its state directory is written the way a [checkpoint](/production/durable-runs/) is written, so a crash leaves each file whole, and the writes go in a fixed order: the report, then the run that names it, then the 202 to the shipper.
On startup the service looks for reports no run names and puts runs that were in progress back in the queue, where their checkpoints take them from where they were.
Two runs go at once by default, up to twenty wait, and a report that would start a run when the queue is full gets 503 with `Retry-After`.

Each run's workspace checks out the commit that crashed, not the latest, because some crashes come from old builds whose bug is already fixed.
The clone copies objects instead of hard-linking them and runs no hooks, so test code in the workspace can't reach your repository and the repository can't run code in the service.

A broker would add a second record of where each run stands, and every crash would have to keep the two in agreement.
On one machine, the checkpoints, their locks, and a directory listing already do the job.
Once the service needs several machines, file locks stop being enough, and a broker or a database with leases (a claim on a piece of work that expires, so a crashed worker's claim frees itself) becomes the right tool.

## Spending that survives a restart

A budget kept in memory starts over with each process, so a service stuck in a crash loop would spend its day's money again on every restart.
The service keeps a ledger on disk and refuses to start without a daily limit (`-daily`).
When a run starts, its whole cap is taken from the day and saved before the run does anything; when it stops, the unspent part goes back.
A process killed mid-run can't return anything, so the next process works out what that run spent from its own records, adds the most expensive single turn as an allowance for whatever call was underway, and never counts more than the run held.

## Contexts in a service

The request that delivered a report is over the moment the service answers 202.
A run whose context came from that request would be cancelled as it started, so each run's context comes from the service's root:

```go include=ch17/service/runs.go tag=start
```

Only shutdown cancels the root.
The request's context still bounds work done on the request's behalf, such as the tracker lookup during ingest, so a client that hangs up stops that lookup and nothing else.

When the service stops, it first calls `http.Server.Shutdown` so no report is half-saved, gives in-flight requests two seconds, and then cancels the root with `ErrShutdown`; runs save and go back to the queue for the next process.

## People, without blocking anything

Approvals and unknown outcomes land in a review queue on disk, answered with a small `triage review` command that talks to the service over HTTP with a reviewer's token.
Only the run that asked waits; every other group keeps going.
An approval has a deadline, a day by default, and a missed one counts as a denial.
An unknown outcome has none, because a timeout can't know whether the call took effect and either guess risks a duplicate or a lost action; after four hours the item is flagged stale, which decides nothing and tells someone to look at the tracker.
Answers to the two kinds differ on purpose ("approve" or "deny" for one, "rerun" or "done" for the other), so nobody reruns a side effect by approving out of habit.

```diagram
name: ch17-review
alt: A state diagram of a run: queued, running, paused, done, and failed or unresolved, with a pause going to the review queue and back to queued once every item is decided, and a shutdown sending a running run back to queued for the next process

A run's life in the service, including a pause for a person's review.
```

## The service, run once for real

```result
figure: 0
unit: issues filed twice, after a SIGKILL with two decisions waiting
task: `triage serve` on five crash reports from the practice service, shipped by `crashship`, with a person answering the review queue; the service was killed with SIGKILL while both runs were paused and started again on the same state directory
models: `anthropic/claude-haiku-5.5`
runs: one live run of the service
date: 18 October 2026
source: Chapter 17

Five reports became two runs, two issues (one per cause), and two comments for late reports. The person made six decisions, all approvals. The kill lost nothing: after the restart the review queue held the same two items, and the ledger had counted $0.0096 by the end.
```

This run was made to see the parts work together, not to measure them, and OpenRouter chose which [host](/glossary/#host) served the model, so its numbers aren't comparable with the [eval tables](/findings/the-finished-agent/).

## Steps for your own service

1. Build one static binary that resumes its newest unfinished run when started with no run ID, and save every new run before starting it.
2. On SIGTERM, cancel with a cause and save; give calls with side effects a few seconds; exit on a second signal.
3. Read every secret from a file only the service user can read, and take key variables out of the environment before starting any child.
4. Ingest from a file with a shipper, behind an endpoint with its own secret, a rate limit, a size cap, and redaction.
5. Deduplicate reports by a key you build, and send late reports for a running group to its issue as one comment.
6. Keep the queue, the ledger, and the review items as files written like checkpoints, in an order that a crash can't break.
7. Derive every run's context from the service's root, never from a request.
8. Put decisions in a review queue with deadlines for approvals and none for unknown outcomes.

## When to use it

```fit
reach: Reports arrive at any hour and must not wait for a person to type a command. The service took five live reports, survived a SIGKILL with two decisions waiting, and filed each cause once.
reach: The platform may stop the process at any time. Kubernetes allows 30 seconds and `docker stop` 10, and the slowest model call in the recorded runs took 27.5.
leave: More than one machine, or more than one team. The directory queue and its file locks hold on one machine, and the service has one tracker, one budget, and one set of reviewers.
leave: A batch you already have in hand. The command form runs it and resumes after a crash without the service's moving parts.
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch17/service/ -v -run 'Secrets|RedactedOnDisk|RepeatedIDs|Restart|DailyBudget|LedgerCrash'
$ go test ./ch15/agent/ -run Grace -v
```

The first runs the service's tests for secrets, redaction on disk, repeated report IDs, restarts, and the ledger, all against `httptest` with a scripted stand-in for the agent, so nothing calls a model.
The second checks the five-second grace period on `testing/synctest`'s fake clock.
Passing looks like a `--- PASS` line per test and a final `ok` line for each package (`ok  github.com/blackwell-systems/go-agents-book/code/ch17/service`, and the same for `ch15/agent`); `TestRestart` and `TestLedgerCrash` are the two worth reading, since they stop the service and start it again on the same files.
[Durable runs](/production/durable-runs/) shows a run killed and resumed from the command line.
