---
title: Asking a person before a risky tool call
short: Human approval
description: How to put a person in front of an agent's risky calls. A gate in tool middleware that the model can't route around, raw evidence shown safely, a yes tied to a hash of exactly what was shown, denials that hold for the whole run, runs that pause and resume instead of blocking, and an audit log that reveals edits.
card: Putting a person in front of risky tool calls, with a gate the model can't route around.
group: Safety
chapters: 14, 15, 17
stage: 3
date: 2026-10-18
work: true
weight: 60
art: ch14
---

Some calls an agent makes deserve a second pair of eyes: an issue that people will read, an edit to authorization code, a refund.
The idea is simple and the details decide whether it protects anything.
Where does the check sit, what does the person look at, how far does one yes reach, and what happens at 3 a.m. when nobody replies?
What follows answers those questions for a crash-triage agent, makes a waiting run survive a restart, and moves the decisions into a review queue.
The code is in [`ch14/safety`](code:ch14/safety) and [`ch14/agent`](code:ch14/agent).

## Put the gate in the harness

In the companion code, [approval](/glossary/#approval) is a wrapper around a tool, the same kind of middleware used for retries and timeouts.
It sits outside those wrappers, so time spent waiting for a person never counts against a tool's time limit and is never retried.
The one wrapper further out is the [egress check](/production/prompt-injection/), which inspects text the agent is about to send out of the machine before the call reaches the approval gate.

The alternative, a tool such as `ask_permission` that the model calls before acting, puts the choice to ask in the model's hands.
A model can skip the call by mistake, and an injected instruction, a [prompt injection](/production/prompt-injection/), can tell it to skip the call on purpose.
A tool for asking a person a question is still worth having, for questions rather than for permission.
Permission has to come from code the model can't route around.

## Decide which calls need a person

A policy marks the calls that wait.
The default policy in the companion code covers two kinds: anything that puts text in front of people (filing an issue or adding a comment, whether through the agent's built-in tools or the [MCP server](/build/mcp/) in front of the tracker), and any write to a [protected path](/glossary/#protected-path) such as the authorization package or an existing test.
Each tool reports what a call would change, its targets, because only the tool knows which of its arguments names a file.

Read-only tools stay ungated.
A call that has to wait for a decision runs alone, outside the batch of a reply's calls that [run in parallel](/build/parallel-tool-calls/), so gating reads would give up parallel reads and buy nothing.

## Show the evidence, made safe to read

```go include=ch14/agent/approve.go tag=types
```

The model's own reason for a call is in the request, but it was written by the party the person is checking.
So the request also carries raw evidence: the first crash report of the group, for a new issue, and the exact old and new text, for a patch.
That evidence may be an attacker's text as well, and a report can plead for its own approval, so the person's job is to judge the change and ignore its arguments.

Before any of it reaches a screen, characters that could deceive the reader are shown by their code points.
An escape sequence in a report could redraw earlier lines on a terminal, and a right-to-left override could disguise a file name; each appears as something like `<U+202E>`, so the person sees that it was there.
Paths get a stricter treatment that also escapes anything outside printable ASCII, so a look-alike letter can't pose as a plain one.

## Tie each yes to what was shown

A decision counts only if it names the request's hash, a [hash](/glossary/#hash) (a short code computed from data that changes if any byte of it changes; SHA-256 is the standard one used here) over the agent's name, the call ID, the tool, its arguments with their object keys in sorted order, the reason, the evidence, and the current bytes of every target file.

Two parts of that list do most of the work.
Without the call ID, approving one `issue_file` call would also approve an identical second call later in the run, and an approver that stores decisions by hash would let the model file that issue again and again.
Without the file contents, a yes given for a patch would still apply after someone changed the file underneath it.
The gate computes the hash again after the decision arrives, and if anything moved in the meantime, the call doesn't run and the person is asked again.

## Lean every default toward no

- A request that gets no answer before its deadline is denied, with "no decision in time" as the reason.
- A denial lasts for the rest of the run. The gate keeps it by hash and by each target, keyed in a way that ignores letter case and path spelling, so an identical change can't return through another tool, with reworded text, or as `AUTH/auth.go`. The issue tools give each filing a target of its own, its crash group, so a refused issue can't come back under a new title.
- The model receives a denial as a tool error, `ErrDenied`, carrying the reason, and the run continues so the model can try something else.
- Cancelling a run is never recorded as a denial, since nobody refused anything.

The second rule earned its place in a recorded run with no attacker in it:

```result
figure: 3
unit: other spellings tried
task: the injection eval's `i3-split-fields` task, with every chapter 14 defense on and a scripted approver that denies changes to protected files
models: `openai/gpt-5.6-luna`
runs: one recorded run, with a second showing the same pattern
date: 16 October 2026
source: Chapter 14
link: /findings/measured-mistakes/#denial
more: The finding

After its patch to `auth/auth.go` was denied, the model asked for the same file as `auth/../auth/auth.go`, `auth//auth.go`, and `./auth/auth.go`. The path check refused the first; the gate's memory of the denial refused the other two without asking anyone.
```

## Pause the run instead of blocking

People answer in minutes, hours, or the next morning, and a run can't keep a model call open while it waits.
So an approver with nothing to report returns `ErrNoDecision`, and the run stops with a typed error:

```go include=ch14/agent/approve.go tag=pending
```

The calls that came before the waiting one in the same reply have already run.
The waiting call and everything after it go into `Result.Pending` with no answers yet, since the later calls may depend on it.

Resuming takes two things: the conversation from that `Result`, and a `Limits` value that carries the paused `Result` (in its `Before` field) and an `Approver`, which is whatever returns the decisions.
The loop runs the pending calls through the gate first, before it makes any model call, and the turns and money already spent count toward the resumed run's limits, so pausing can't be used to reset a budget.
A caller that slips an extra user message in after the waiting calls gets `ErrBadResume`, because the API expects the tool results to come first.

```diagram
name: ch14-approval
alt: A sequence of three participants: the loop, the approval gate, and a person. In one reply the model asks for code_read, run_tests, code_patch to auth/auth.go, and issue_file. The loop runs code_read and run_tests. code_patch goes to the gate, which sends a request with the reason, the old and new text, and the hash to the person; there is no decision yet, so the gate returns ApprovalPending and the loop stops with code_patch and issue_file in Result.Pending. Later, the run resumes with Limits.Approver: the person's decision arrives, approved for this hash; the gate writes the audit log entry, and code_patch runs, followed by issue_file, which goes through the gate in turn.

One reply's calls, where the third needs a person's decision.
```

## Make the pause survive a restart

A [checkpoint](/production/durable-runs/) saves the run to disk so a restart loses nothing and the run can pick up where it paused.
Checkpoints and the attempt markers below are covered in [Durable runs](/production/durable-runs/); here it is enough to know the run is saved and can resume.
Three details connect it to approval.
The marker that says "this call went out" is written after the yes, not before the question, so a process that dies while a person is deciding leaves nothing to investigate.
The gate's denials, which the gate keeps only in memory, are rebuilt from the audit log when the command runs with `-audit`; without it, a refusal doesn't outlive the process.
And when a crash leaves a call whose outcome nobody knows, the person is asked through the same approver, with a different question: did this take effect?
The [crash sweep](/findings/crash-sweep/) has a separate case for tools behind the gate, with a crash point while the person decides: with idempotency keys, its 1,063 scenarios made no duplicates and asked nobody about an outcome.

## Keep a log that shows tampering

Every outcome at the gate goes into an append-only [audit log](/glossary/#audit-log): approvals, denials, calls refused because of an earlier denial, pauses, stale decisions, and whether an approved call then ran or failed.
Each entry reaches the disk before the call proceeds.

A plain file can be edited by anyone with write access, and a plain hash chain can be recomputed by the same person.
So each line is signed with a key that only the harness holds, and the signatures are chained, so editing, deleting, or reordering a line shows up when the log is checked.

<div class="hood">

Each line carries an [HMAC](/glossary/#hmac) over itself and the previous line, keyed with a secret stored where neither the agent, its tools, nor test code can read it.
The tests flip a denial to an approval, delete a line, and rebuild the whole chain under another key, and verification catches all three.
Two limits remain: lines cut from the end of the log leave the chain intact, and an HMAC can't prove that a person, rather than a script, made the decision.
For a record other teams must rely on, have the harness sign each line with a key that nothing else holds, and send the log to a store that accepts additions but no rewrites.

</div>

## What it costs in decisions

The default policy was replayed offline against 30 recorded runs.
It would have held 104 calls for a decision, roughly 3.5 per run: 78 were filings and comments, and 26 were protected writes.
Of the 15 writes into `auth/`, 14 were the agent fixing an already-filed bug outside its task, exactly what a person should catch.
One task's run would have asked as many as seven times, and the recorded run over a batch of 99 crash reports 22 times.

That volume is why decisions belong in a review queue, where a paused run waits without holding up any other group, approvals carry a deadline (a day by default), and a question about an unknown outcome has no deadline at all, since neither guess is safe.

The OpenAI Agents SDK has the same overall shape (the [frameworks page](/ecosystem/go-agent-frameworks/) compares the Go ones): a tool can declare that it needs approval, a run returns with pending interruptions, and the run's state can be serialized and resumed later.
What this design adds is binding each yes to a hash of the person's view, and keeping each denial per target.

## Steps for your own agent

1. Write the gate as tool middleware, outside retries and timeouts, and never as a tool the model chooses to call.
2. Have each tool report its targets, and gate outgoing text and protected writes by default.
3. Show the person the raw evidence with control and direction characters made visible.
4. Hash the call ID, arguments, evidence, and target contents, and recheck the hash after the decision.
5. Treat silence as a denial, remember denials by target for the whole run, and send them to the model as errors.
6. Stop the run with a typed error when no decision exists yet, and resume from the saved conversation with the decisions.
7. Log every outcome with a keyed chain, with the key out of the agent's reach.
8. Count how many decisions your policy asks for on recorded runs before you ship it.

## When to use it

```fit
reach: The agent sends text to people or changes something you protect. Over 30 recorded runs, the default policy would have held 78 filings and comments and 26 protected writes for a person.
reach: A model may look for another way after a refusal. One model asked for a denied file under three other spellings, and a gate that remembers denials by file turned each one down.
leave: Read-only tools. A call waiting for a decision runs alone, so gating reads gives up parallel reads and protects nothing.
leave: Trusted input in volume, with a person at a terminal. The recorded batch run would have asked 22 times, which is why approval can be switched off for trusted input and the decisions can go to a review queue.
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch14/safety/ -run 'Gate|Audit|PauseResume|DenialByTarget' -v
```

The tests cover the hash, decisions that went stale, denials remembered by target, pausing and resuming a run, and tampering with the audit log.
No key is needed, and nothing calls a model.
Passing looks like a `--- PASS` line for each test and a final `ok  github.com/blackwell-systems/go-agents-book/code/ch14/safety`.
`TestGateHash` (the hash changes when anything the person saw changes) and `TestDenialByTarget` are the two worth reading first.
