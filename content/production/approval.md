---
draft: true
title: Asking a person before a risky tool call
short: Human approval
description: Approval as tool middleware the model can't skip, a request that shows the raw evidence, a yes bound to a hash of exactly what the person saw, denials remembered for the run, runs that pause for a decision and resume later, and an audit log that notices edits.
chapters: 14, 15
stage: 3
date: 2026-10-18
work: true
weight: 60
art: ch14
---

Some tool calls should wait for a person: filing an issue people will read, changing authorization code, issuing a refund.
Getting that right is mostly a matter of details: where the gate lives, what the person sees, what a yes covers, and what happens when nobody answers.
Chapter 14 of the book builds an approval gate for the crash-triage agent, and chapter 15 makes the pause survive a restart.

## Where the gate lives

[Approval](/glossary/#approval) is a person's decision, recorded by the harness, that one specific action may go ahead.
In the companion code it is tool middleware, around the tool, outside the retries and timeouts so the wait for a person is never retried or timed out as a tool call.

It is not a tool the model calls to ask permission.
That would leave the decision to ask with the model, which can forget, or be told not to by an injected instruction.
A tool for asking a person a question is useful for questions; it is never how permission is given.

A policy decides which calls need a decision.
On by default: everything that sends text to people (filing and commenting, through the agent's own tools or an MCP server), and any write to a [protected path](/glossary/#protected-path), such as authorization code or existing tests.
Each tool says what its call would change, its targets, since only the tool knows which argument is a file.

## What the person sees

```go include=ch14/agent/approve.go tag=types
```

The request shows the raw evidence, not only the model's explanation, since that is written by the party the approval is meant to check: for an issue, the first crash report of its group; for a patch, the old and new text exactly as the tool will write them.
The evidence is an attacker's text too, and a report can argue for its own approval, so a person should judge the change, not the reasons given for it.

The text is made safe to show.
A terminal escape sequence in a crash report can rewrite the lines above it, and a right-to-left override can make `auth.go` read as something else.
Each such character is shown as, say, `<U+202E>` rather than deleted, so the person can see that someone tried.

## What a yes covers

A decision names a hash of the call and of what the person was shown: the agent's name, the call's ID, the tool, its arguments (with object keys sorted, so the same arguments in another order hash the same), the reason, the evidence, and the current content of every file the call targets.

Each part is there for a reason.
The call ID: two identical `issue_file` calls in one run must each be approved, or a yes for the first would let the model file the same issue any number of times.
The files' content: a yes for a patch to a file can't approve the same patch after the file changed underneath it.
After the decision the gate hashes again, and if anything changed while the person decided, the request goes back to them instead of running.

## The defaults lean one way

- **No answer in time is a denial**, with the reason "no decision in time".
- **A denial is remembered for the rest of the run**, by the call's hash and by every target, under a key that ignores case and spelling, so the same change can't come back through another tool, in other words, or as `AUTH/auth.go`. Issue calls target their crash group, so a refused filing can't come back reworded.
- **A denial reaches the model as an error result**, `ErrDenied`, with the reason, so it can try something else. The run goes on.
- **A cancelled run is a cancellation, never a denial**: the person didn't say no.

The chapter's recorded runs show why remembering by target matters.
Denied an edit to `auth/auth.go`, one model tried `auth/../auth/auth.go`, `auth//auth.go`, and `./auth/auth.go`; the path checks refused the first, and the gate's memory refused the other two without asking anyone.
No attacker was involved; a model trying to finish its task treats a refusal as an obstacle.

## Pausing a run for a person

A person may answer in minutes or tomorrow, and a run can't hold a model call open that long.
So when the approver has no decision yet, the run stops, returning `*ApprovalPending`:

```go include=ch14/agent/approve.go tag=pending
```

The calls before the waiting one have run; it and the calls after it go into `Result.Pending`, unanswered.
To resume, the caller runs the agent again on the saved conversation, with the earlier `Result` and an approver that has the decisions.
Before its first model call, the loop runs the waiting calls through the gate again; no model call is made for them, and the turns and cost already spent count against the resumed run's limits, so a run that pauses ten times can't spend ten budgets.

Adding a message after the waiting calls, say "approved, and check the logs too", gets `ErrBadResume` rather than a request the API would refuse: the answers to the waiting calls have to come first.

Chapter 15 makes this survive a restart: the run's state goes into a [checkpoint](/glossary/#checkpoint), the gate's denials are rebuilt from the audit log when one is kept, and a crash while the person decides is one of the points its crash sweep tests.

## An audit log that notices edits

Every outcome at the gate goes into an append-only [audit log](/glossary/#audit-log): an approval, a denial, a call refused because it was denied earlier, a pause, a decision that went stale, and, for a call that went ahead, whether it ran or failed.
Each line is written to disk before the call goes ahead.

A log in a file can be edited by anyone who can write the file, so each line carries an [HMAC](/glossary/#hmac) over the line and the one before it, with a key the agent and its tools can't reach.
A test changes a denial into an approval, removes a line, and builds a whole new chain with another key, and all three fail verification.
The chapter states the limit: an HMAC shows nobody without the key changed the log, not that a person rather than a script made a decision.

## What it costs in decisions

Replayed over the book's 30 recorded runs, the default policy would have stopped 104 calls for a person, about three and a half a run: 78 filings and comments, and 26 writes to protected paths.
A run on one task would have asked up to seven times; a 99-report batch, 22 times.
That is why the finished agent in chapter 17 puts decisions in a review queue, and why approval can be turned off for input you trust.

Others use the same shape.
The OpenAI Agents SDK lets a tool declare that it needs approval, returns from a run with pending interruptions, and serializes the run's state so it can resume later.
What the companion code adds is binding each yes to a hash of exactly what the person saw, and remembering denials by the file they target.

## When to use it

```fit
reach: The agent sends text to people or changes something you protect. Replayed over the book's 30 recorded runs, the default policy would have stopped 78 filings and comments and 26 protected writes for a person (chapter 14).
reach: A model may try another route after a refusal. One model asked for a file it had been denied under three other spellings, and a gate that remembers denials by file refused each (chapter 14).
leave: Read-only tools. A call that waits for approval runs alone, so gating reads would give up parallel reads for nothing; the book's policy leaves them ungated (chapters 9 and 14).
leave: Input you trust, in volume. The recorded 99-report batch run would have asked 22 times under the default policy, which is why approval can be switched off and chapter 17 adds a review queue (chapters 14 and 17).
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch14/safety/ -run 'Gate|Audit|PauseResume|DenialByTarget' -v
```

The tests cover the hash, stale decisions, denials remembered by target, pausing and resuming a run, and tampering with the audit log.
