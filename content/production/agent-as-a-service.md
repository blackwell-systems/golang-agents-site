---
draft: true
title: Running an AI agent as a long-running service in Go
short: An agent as a service
description: Turning an agent into something you leave running. A static binary that resumes its own runs, shutdown that cancels and saves instead of waiting, a container that keeps the key out of the environment, and a service that takes reports over HTTP, queues work in a directory, keeps its spending on disk, and waits for people without blocking anything.
chapters: 15, 17
stage: all
date: 2026-10-18
work: true
weight: 80
---

An agent that runs while you watch can rely on you: you restart it, approve its risky calls, and notice when it spends too much.
An agent left running has to do all of that itself.
Chapters 15 and 17 of the book turn the crash-triage agent into a command you can run unattended and then a service that takes crash reports as they happen.
This page collects the decisions, with the code in [`ch15`](code:ch15) and [`ch17/service`](code:ch17/service).

## One binary that resumes its own work

The triage agent uses no C code, so `CGO_ENABLED=0 go build` gives one static binary.
Running the same command twice must resume, since that is what a platform does when it restarts a container: without an explicit run ID, the command resumes the task's newest unfinished run, or starts a new one.

A new run's ID is the task, the time, and eight random characters, and is never reused, because [idempotency keys](/glossary/#idempotency-key) are built from it.
When old runs are pruned, a small tombstone file stays behind so the ID is remembered.
A new run is saved to disk before it does anything: accepting work means promising to finish it.

Some state lives in the tools, not the loop, and has to survive too: which files the model has read (the patch tool refuses to edit unread files), the agent's notes, the random data marker that the saved conversation's rules name, and which changes a person denied, which are rebuilt from the audit log when one is kept.

## Shutdown: cancel and save, don't wait

The usual advice is to stop accepting work and wait for what is running to finish.
For an agent, waiting doesn't work.
A platform sends SIGTERM, then SIGKILL after a grace period: Kubernetes waits 30 seconds by default, `docker stop` 10.
In the book's measured runs, model calls took 2.35 seconds at the median, and the longest took 27.5.
So the command cancels instead:

```go include=ch15/durable.go tag=shutdown
```

The first signal cancels the run's context with `ErrShutdown` as the cause; the model call in progress stops, unstarted tool calls are answered "cancelled", and the loop saves.
A cancelled model call is repeated after the restart; every call before it was saved.
A second signal exits at once, for a person who doesn't want to wait for the save.

One kind of work gets a few seconds: a call that may change something and is already talking to the tracker is better finished than cut off, since its answer settles it.
A tool middleware gives such calls five seconds after a shutdown, with `context.WithoutCancel` keeping the call's values and dropping the run's cancellation, and its tests run on `testing/synctest`'s fake clock.

## A container that keeps the key out of reach

The agent runs `go test` and builds git workspaces, so its image needs the Go toolchain and git; the book's image is 1.34 GB, nearly all toolchain, and runs as a user without root, with the state directory as a volume.

The key goes in a file, mounted read-only and named by `OPENROUTER_API_KEY_FILE`, not in the environment.
`docker inspect` shows a container's variables to anyone who can run Docker commands, and even after the program removes the variable at startup, the kernel keeps the first process's starting environment, readable in `/proc/1/environ` by any process of the same user, which includes the test code the agent runs.
The chapter checked: with a made-up key passed as a variable, another process read it back.

## From a command to a service

Crashes arrive one at a time, at any hour, from a service that must not slow down while it reports them.
`triage serve` takes each crash as it happens, routes it to a group, and runs the group's investigation when it can.

**Ingest from a file, not from inside the crashing process.** The practice service already writes each crash report as a JSON line.
A small shipper follows the file and posts each new line, saving its position after each post.
The crash path stays local, the file is the durable record if the crashing process dies, and the service can move or be down for an hour without a new release of the crashing program.

**Authenticate and limit the door.** The ingest endpoint spends your money: anyone who can post can start runs and put text of their choosing in front of the agent.
It takes a secret of its own, separate from reviewers' tokens, compared in constant time; limits each source; caps the body with `http.MaxBytesReader`; and redacts credentials from every report before anything is written.

**Deduplicate.** The shipper resends after a timeout, so a report seen before gets a 200 and changes nothing: [deduplication](/glossary/#deduplication), the receiving side of an idempotency key.
The service can't trust the crashing program's report IDs, which restart at 1 with every process, so it builds its own key from the report's ID, time, commit, and service: a retry repeats them all, and a restarted process can reuse the ID but not the time.

**One run per group; late reports become comments.** A run can't take new input mid-run, and a second report for the same crash usually says nothing new except how often it happens.
So a report for a running group is added to the group's issue as a comment when the run ends, through the same egress check and approval policy as the agent's own comments.
If the issue was closed, the crash came back after a fix, and the report starts a new run.

## The queue is a directory

There is no message broker.
Every file in the state directory is written the way a checkpoint is (temporary file, sync, rename, sync the directory), so after a crash each file holds its old contents or its new ones.
The order of writes matters: the report first, then the run that names it, then the 202.
On startup, the service looks for reports no run names, and requeues runs that were going.

A broker would add a second place where the truth about a run lives, and every crash would need the two to agree.
When the service needs more than one machine, the directory and its file locks stop being enough, and that is the time for a broker or a database with leases.

Runs start when there is a free slot (two by default) and money left for the day; a full queue answers 503 with `Retry-After`.
Each workspace checks out the commit that crashed, not the latest, cloned without hard links and without running hooks.

## Spending that survives a restart

An in-memory budget resets on restart, and a service that crashes in a loop would spend its daily budget again each time.
The service keeps a ledger on disk: a run takes its whole budget from the day when it starts, saved before it does anything, and gives back what it didn't spend when it stops.
A process killed mid-run can't give money back, so the next one counts it, using the run's own record of what it spent plus its costliest turn.
The service refuses to start without a daily limit.

## Contexts in a service

The request that brought a report ends as soon as the service answers 202.
A run whose context came from the request would be cancelled the moment it started, so every run's context comes from the service's root context instead, which only shutdown cancels.
The request's context still bounds work done for the request itself, such as a tracker lookup during ingest.

## People, without blocking anything

Approvals and unknown outcomes wait in a review queue, answered with a small command over HTTP with a reviewer's token.
Nothing waits on a decision but the run that asked; other groups keep running.
A pending approval has a deadline, and a missed one is a denial.
An unknown outcome has none, because a guess either way risks a duplicate or a lost action; after four hours the service flags it as stale, which decides nothing but says someone should look.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch17/service/ -v -run 'Secrets|RedactedOnDisk|RepeatedIDs|Restart|DailyBudget|LedgerCrash'
$ go test ./ch15/agent/ -run Grace -v
```

The service's tests run every case against `httptest` with a scripted stand-in for the agent, so none calls a model.
