---
title: "Before you leave an agent running: a checklist"
short: Launch checklist
description: Twenty-three checks for an agent that will run unattended and spend money, grouped by what can go wrong, each with the chapter that builds it and, where the book measured one, the failure that shows why it matters.
card: Twenty-three checks for an agent that runs unattended and spends money.
chapters: 4, 5, 10, 11, 12, 13, 14, 15
stage: all
date: 2026-10-18
weight: 90
---

An agent that works on your laptop while you watch is a different program from one that runs at night with a budget and a key.
The book's production chapters cover the distance between them.
This page boils them down to a list you can check a design against, one line each, with the chapter that builds the piece and, where the book's runs showed one, the failure behind it.

The list is ordered by what goes wrong, not by chapter.
Items marked with a measured failure link to [measured mistakes](/findings/measured-mistakes/), where each has its method.

## It stops, and says why

Guide: [why the agent stopped](/production/stop-reasons/).

1. **A turn limit that every caller sets.** No default can know your task. In chapter 4, one model only stopped because the limit ended its runs ([the "locked" error](/findings/measured-mistakes/#locked)).
2. **A dollar budget per run.** Count what each reply cost and stop before the next call would pass the cap (chapter 5). Two runs of the same policy on the same batch cost 42 cents and 6 cents (chapters 8 and 13), so size the cap from many runs, not one.
3. **Stop reasons callers can test.** Sentinel errors grouped into families, checked with `errors.Is`, so a caller can separate running out of budget, an unusable reply, and a cancellation (chapter 5).
4. **Prices kept in configuration.** A cost budget priced from an old table stops at the wrong place; load the table at startup and check it against the provider's bills (chapter 11).

## It survives a bad network

Guides: [retries, traces, and cost](/production/retries-and-observability/), and [testing offline](/production/testing-offline/) for the fault-injecting transport.

5. **Errors sorted into retry and stop.** 408, 429, 500, 502, and 503, and dropped connections, are worth another attempt; 400, 401, 402, and 403 are not (chapter 12). A dropped connection [ended a 41-turn run](/findings/measured-mistakes/#dropped-connection) before the book had this.
6. **Backoff with jitter, honoring the server's delay**, and a final error that nothing above it retries again (chapter 12).
7. **Limits shared across agents.** A retry budget that runs down as calls fail and a circuit breaker that stops sending to a dependency that keeps failing, so many agents don't retry a struggling provider in step (chapter 12).
8. **Streams retried from the start** when they break partway, since a stream can't be resumed from the middle (chapters 9 and 12).
9. **Retries tested on purpose.** In chapter 12's own live runs, 209 model calls needed no retry at all, so the only way to know the retries work is a transport that injects failures.

## Its tools can't hang it or repeat harm

Guide: [retries, traces, and cost](/production/retries-and-observability/), whose tools section covers timeouts and safe retries.

10. **A timeout on every tool**, with a cause the loop reports (chapter 12).
11. **Tool retries only where a repeat is harmless.** Mark which tools are safe to run twice and retry only those (chapter 12).
12. **Tool errors that say whether to try again.** [Told a file was "locked"](/findings/measured-mistakes/#locked), three of four models kept retrying for 18 to 20 turns; told it was read-only for the session, they finished in four or five (chapter 4).

## You can see what it did

Guides: [retries, traces, and cost](/production/retries-and-observability/) for logs, traces, and the meter, and [human approval](/production/approval/) for the audit log.

13. **A trace of every model call, attempt, and tool call**, with token counts, exported under the standard generative-AI names if you use a tracing system (chapter 12).
14. **A cost meter that includes failed attempts**, so the bill and the run's own total agree (chapter 12).
15. **An audit log of every approval decision**, chained with a key so that an edited, removed, or rebuilt entry fails the check (chapter 14).

## You know whether a change helped

Guide: [evals for an agent](/production/evals/).

16. **An eval set graded by code**, split into tasks you tune on and tasks you only confirm on, with each result's range and counts of tasks that got better and worse (chapter 13). [Two identical runs scored 15 points apart.](/findings/measured-mistakes/#identical-runs)
17. **Checks the agent can't see, and a grader with tests of its own.** Hidden tests caught the [nine wrong fixes a gate passed](/findings/measured-mistakes/#gate-blind-spot), and the first grading [failed nine right answers](/findings/measured-mistakes/#grader) until the grader was fixed (chapter 13).

## It can't do harm it wasn't asked to do

Guides: [prompt injection and sandboxing](/production/prompt-injection/), and [human approval](/production/approval/).

18. **Tools sandboxed, and keys out of their reach.** Run test code with no network and no access outside its workspace, and read keys from files, not the environment: chapter 14 measured that test code could read the harness's starting environment, key included.
19. **Defenses that don't depend on the model**, strongest first: no tool for an action the agent shouldn't take, protected paths that remember a denial by the file, a check on every text that leaves, and untrusted text marked as data (chapter 14). An agent [tried three other spellings](/findings/measured-mistakes/#denial) of a file it had been refused, with no attacker involved. If it uses MCP, pin each server's tool definitions; [one sentence in a description](/findings/measured-mistakes/#poisoned) steered 79 of 80 first moves ([the full finding](/findings/tool-description-poisoning/), chapter 10).
20. **A person's approval before side effects**, bound to a hash of the exact change they saw, with no answer counted as a denial (chapter 14).

## It survives its own crash

Guides: [durable runs](/production/durable-runs/), and [an agent as a service](/production/agent-as-a-service/) for shutdown and deployment.

21. **A checkpoint after each reply and each tool result**, written so a crash leaves a whole file, with a lock so only one process runs a run (chapter 15).
22. **An idempotency key on every call that changes something**, made from the run ID and the call ID, and a marker written before each such call. [Saving once per turn duplicated work at 8 of 18 crash points](/findings/measured-mistakes/#save-per-turn); keys and markers together left none (chapter 15).
23. **A shutdown that cancels instead of waiting.** On SIGTERM, cancel the run with a cause and save, inside the platform's grace period: Kubernetes allows 30 seconds by default, and one model call in chapter 12's runs took 27.5 (chapter 15).

None of these needs a framework.
Each is a small piece of Go, and the book builds every one of them, with its tests.
