---
title: "Before you leave an agent running: a checklist"
short: Launch checklist
description: Twenty-three checks for an agent that will run unattended and spend money, grouped by what can go wrong, each with the guide that covers it and, where a recorded run measured one, the failure that shows why it matters.
card: Twenty-three checks for an agent that runs unattended and spends money.
group: Before launch
chapters: 4, 5, 10, 11, 12, 13, 14, 15
stage: all
date: 2026-10-18
weight: 90
---

An agent that works on your laptop while you watch is a different program from one that runs at night with a budget and a key.
The production concerns on this site cover the distance between them.
This page boils them down to a list you can check a design against, one line each, and, where recorded runs showed one, the failure behind it.

The list is ordered by what goes wrong.
Items marked with a measured failure link to [measured mistakes](/findings/measured-mistakes/), where each has its method.

## It stops, and says why

Guide: [why the agent stopped](/production/stop-reasons/).

1. **A turn limit that every caller sets.** No default can know your task. In recorded runs, one model never stopped on its own; only the turn limit ended its runs ([the "locked" error](/findings/measured-mistakes/#locked)).
2. **A dollar budget per run.** Count what each reply cost and stop before the next call would pass the cap. Two runs of the same [context policy](/build/context-management/) on the same batch cost 42 cents and 6 cents, so size the cap from many runs, not one.
3. **Stop reasons callers can test.** [Sentinel errors grouped into families](/basics/errors/#errors-callers-check-sentinels-in-families), checked with `errors.Is`, so a caller can separate running out of budget, an unusable reply, and a cancellation.
4. **Prices kept in configuration.** A cost budget priced from an old table stops at the wrong place; load the table at startup and check it against the provider's bills.

## It survives a bad network

Guides: [retries, traces, and cost](/production/retries-and-observability/), and [testing offline](/production/testing-offline/) for the fault-injecting transport.

5. **Errors sorted into retry and stop.** 408, 429, 500, 502, and 503, and dropped connections, are worth another attempt; 400, 401, 402, and 403 are not. A dropped connection [ended a 41-turn run](/findings/measured-mistakes/#dropped-connection) before such a policy existed.
6. **Backoff with jitter, honoring the server's delay**, and a final error that marks itself as not retryable, so neither the loop nor its callers retry it again.
7. **Limits shared across agents.** A retry budget that runs down as calls fail and a circuit breaker that stops sending to a dependency that keeps failing, so many agents don't retry a struggling provider in step.
8. **[Streams](/build/streaming/) retried from the start** when they break partway, since a stream can't be resumed from the middle.
9. **Retries tested on purpose.** In live runs recorded for these guides, 209 model calls needed no retry at all, so the only way to know the retries work is a transport that injects failures.

## Its tools can't hang it or repeat harm

Guide: [retries, traces, and cost](/production/retries-and-observability/), whose tools section covers timeouts and safe retries.

10. **A timeout on every tool**, with a cause the loop reports.
11. **Tool retries only where a repeat is harmless.** Mark which tools are safe to run twice and retry only those.
12. **Tool errors that say whether to try again.** [Told a file was "locked"](/findings/measured-mistakes/#locked), three of four models kept retrying for 18 to 20 turns; told it was read-only for the session, they finished in four or five.

## You can see what it did

Guides: [retries, traces, and cost](/production/retries-and-observability/) for logs, traces, and the meter, and [human approval](/production/approval/) for the audit log.

13. **A trace of every model call, attempt, and tool call**, with token counts, exported under the standard generative-AI names if you use a tracing system.
14. **A cost meter that includes failed attempts**, so the bill and the run's own total agree.
15. **An audit log of every approval decision**, chained with a key so that an edited, removed, or rebuilt entry fails the check.

## You know whether a change helped

Guide: [evals for an agent](/production/evals/).

16. **An eval set graded by code**, split into tasks you tune on and tasks you only confirm on, with each result's range and counts of tasks that got better and worse. [Two identical runs scored 15 points apart.](/findings/measured-mistakes/#identical-runs)
17. **Checks the agent can't see, and a grader with tests of its own.** Hidden tests caught the [nine wrong fixes a gate passed](/findings/measured-mistakes/#gate-blind-spot), and the first grading [failed nine right answers](/findings/measured-mistakes/#grader) until the grader was fixed.

## It can't do harm it wasn't asked to do

Guides: [prompt injection and sandboxing](/production/prompt-injection/), and [human approval](/production/approval/).

18. **Tools sandboxed, and keys out of their reach.** Run test code with no network and no access outside its workspace, and read keys from files, not the environment: recorded measurements showed that test code could read the harness's starting environment, key included.
19. **Defenses that don't depend on the model**, strongest first: no tool for an action the agent shouldn't take, protected paths that remember each denial by the file it targets, however the path is spelled, an [egress check](/production/prompt-injection/) on every text that leaves the machine, and untrusted text marked as data. An agent [tried three other spellings](/findings/measured-mistakes/#denial) of a file it had been refused, with no attacker involved. If it uses MCP, pin each server's tool definitions; [one sentence in a description](/findings/measured-mistakes/#poisoned) steered 79 of 80 first moves ([the full finding](/findings/tool-description-poisoning/)).
20. **A person's approval before side effects**, bound to a hash of the exact change they saw, with no answer counted as a denial.

## It survives its own crash

Guides: [durable runs](/production/durable-runs/), and [an agent as a service](/production/agent-as-a-service/) for shutdown and deployment.

21. **A checkpoint after each reply and each tool result**, written so a crash leaves a whole file, with a lock so only one process runs a run.
22. **An idempotency key on every call that changes something**, made from the run ID and the call ID, and an [attempt marker](/glossary/#attempt-marker) written before each such call. [Saving once per turn duplicated work at 8 of 18 crash points](/findings/measured-mistakes/#save-per-turn); keys and markers together left none.
23. **A shutdown that cancels instead of waiting.** On SIGTERM, cancel the run with a cause and save, inside the platform's grace period: Kubernetes allows 30 seconds by default, and one model call in recorded runs took 27.5 seconds.

None of these needs a framework.
Each is a small piece of Go, with its tests written alongside it.
