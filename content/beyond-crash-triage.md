---
title: "Beyond crash triage: the same parts in three other agents"
short: Beyond crash triage
description: The book's agent triages crashes in a Go service, and its parts carry over to agents with other jobs. A support-ticket resolver, a SQL analyst, and a research assistant, each with the shape it needs, what goes wrong without the right parts, and how to measure it.
layout: page
---

The running example on this site is one agent with one job: it triages the crash reports of a small Go web service.
What the material teaches is the program around the model, the [harness](/glossary/#harness): limits that end a run, tools that report their own mistakes, [evals](/production/evals/) that report a range around every score, a sandbox and a person's approval, and runs that survive a crash.
None of that is specific to crashes.

Three case studies follow.
Each is an agent these guides don't build, and each way it could fail is tied to the technique that prevents it.
Here are the three, described briefly.
Each starts by asking what shape the job needs, using the ladder in [workflow or agent](/build/workflows-vs-agents/): fixed code where the steps are known, an agent where the next step depends on what the last one found, and several agents only where the parts are truly independent.

## A support-ticket resolver

**The job.**
A support team receives every kind of ticket.
Most are routine, such as a forgotten password or a request for an old invoice.
A handful mean digging through orders, payments, and delivery tracking, and a few of those finish with money going back to the customer.

**The shape: a workflow in front of an agent.**
A single model call labels each ticket, and plain Go routes the routine labels to handlers written in advance: send a reset link, attach the invoice, reply from a template.
Only tickets that get no routine label reach an agent, which has three lookup tools that only read (orders, payments, shipments) and one that moves money, `issue_refund`.
This follows a simple division of labor: code for whatever someone could have written down beforehand, the model for the rest.

**What keeps it safe:**

- A puzzling ticket can keep the agent circling the same order, so the run needs a turn limit, a cost budget, and [a named reason for stopping](page:/production/stop-reasons/).
- A refund against an order ID that doesn't exist should return to the model as [a tool error it can fix](/basics/tool-calling/#a-wrong-call-gets-an-error-the-model-can-fix), and the program should confirm the refund itself instead of believing a final message that claims it.
- Customers write the tickets, so a ticket that tells the agent to refund everything is [prompt injection](/glossary/#prompt-injection). A model can be talked out of its instructions; [a person approving each refund](page:/production/approval/) can't be, so that check sits in code in front of `issue_refund`.
- A refund can go out moments before the process dies, with no record that it went. On resume it must not go out twice, so [the attempt is written down first](page:/production/durable-runs/), and an idempotency key travels with the call when the payment service supports one.
- Every tool carries two marks, whether it only reads and whether repeating it is harmless, and those marks decide what may be retried. `issue_refund` has neither.

**How to measure it.**
The two halves need different measurements.
Whether a ticket got the right label is a rate you can compute against a set of tickets people have labeled, much as you would measure whether a model picks the right tool.
Whether a particular refund should have been granted is a question of company policy that no test settles, so it goes to a person, or to a model judging against a rubric.

## A SQL analyst

**The job.**
Staff want answers from the company database without writing SQL, for questions like whether refunds rose after prices went up.
The agent drafts a query, runs it through a `query` tool, looks at the rows or the error message, and revises until it has an answer.
That one tool executes text the model wrote against data the business relies on.

**The shape: a single agent.**
Each query is shaped by the last result, so no script of steps fits, and the queries aren't independent enough to farm out.
Besides `query`, the agent gets a few tools that describe tables and columns.

**What keeps it safe: three layers of limits, all outside the model.**

- **The database.** The agent logs in as a role allowed to `SELECT` from the tables it needs and nothing more, and each query runs inside a read-only transaction (`sql.TxOptions{ReadOnly: true}` in `database/sql`). Those rules hold no matter what the query text says, even a `SELECT` that calls a function that tries to write. A statement timeout on the role kills a query that would sort an enormous table to return ten rows, a case a row cap can't catch.
- **The tool.** Before running anything, the tool parses the text and refuses all but a single `SELECT`, with a message explaining the refusal. It also limits how many rows come back and says when it trimmed them, because rows in the conversation are resent, and paid for, on every turn after.
- **The call.** The tool's context carries a deadline, so a query still running when the agent's time is up is cancelled and reported to the model as an error.

Two risks survive all three layers.
Read-only access is still access to everything the database role can see, so the agent should query with the asking person's permissions rather than through one account that sees every table.
And the rows are untrusted: a free-text field holding "ignore your previous instructions" is prompt injection that arrived through a query result.
Because the agent can't write, the worst case is an answer that is wrong or reveals too much, so it is given nothing that can send data anywhere.

As long as it can only read, it needs no approval step; removing the dangerous action can remove the reason to ask, and [approval](page:/production/approval/) becomes necessary the day a tool that writes is added.

**How to measure it.**
Most data questions have a correct answer, so code can grade the agent by comparing its result with the output of a reference query, the same way the [crash catalog](/glossary/#crash-catalog) grades a fix with tests the agent never saw.

## A research assistant

**The job.**
A question whose answer is scattered across many documents, such as how three databases compare on one kind of workload.
One agent working alone reads source after source, so the run lasts as long as all that reading, and its context fills up with pages it is done with.

**The shape: several agents.**
A lead agent breaks the question into parts and hands each to a [sub-agent](/glossary/#sub-agent).
Each sub-agent searches and reads with a fresh context and reports back a compact summary that names its sources; the lead merges the reports and sends out another round when there are gaps.
Anthropic has published [an account of a research system built this way](https://www.anthropic.com/engineering/multi-agent-research-system): its agents used about 4 times the tokens of a chat, its multi-agent setups about 15 times, and running sub-agents in parallel cut research time by as much as 90 percent on complex questions.

**What keeps it safe:**

- The lead and every sub-agent draw on one account, so a single budget must bound all of them together, and they compete for the same provider's rate limits.
- When a sub-agent hits its turn limit, the lead should receive that as an ordinary failed result it can respond to; reporting it as a budget error would wrongly say the lead itself ran out.
- Letting sub-agents start their own sub-agents puts no ceiling on the cost.
- Every page a sub-agent reads is untrusted, and an instruction hidden in one can travel inside its summary up to the lead, so sub-agents get tools that only read.

**Where the book stops.**
A lead that decides during the run how many helpers to start and what each should do is orchestration, which is left out here; the page on [what this site covers](/scope/) explains where the line is drawn.
The companion code's fan-out sidesteps that choice: it starts one agent per crash group, so the number of agents is fixed before the run begins.
Measured on a batch of crash reports, that fan-out [took as long as one agent and cost more](/findings/measured-mistakes/#fan-out), because crashes in one shared codebase are seldom independent; separate documents often are, which is why research can gain from parallel agents where crash triage didn't.

**How to measure it.**
There is no single right research answer, so scoring takes a model judge with a rubric, and that judge has to agree with people's grades before its scores mean anything.

## What carries over

The same handful of rules runs through all three:

- **Use the narrowest shape that works.** A ticket the classifier handles costs one model call and leaves the agent no room for error.
- **Put each limit where the model has no say:** in the database, in the tool's code, in the context's deadline, and never only in the prompt.
- **Label every tool by what it can do,** and let the labels decide what is retried, what needs a person, and what a sub-agent may touch.
- **Treat whatever the agent reads as untrusted,** whether it came in as a ticket, a row, or a web page.
- **Make each action that changes something safe to repeat,** with an idempotency key, before a crash makes the point for you.
- **Grade with code wherever an answer can be checked,** and with a judge you have checked against people where it can't.

To try the parts on a service of your own, [the finished agent](/findings/the-finished-agent/#pointing-it-at-your-own-service) has the steps for pointing the agent in the companion code at another Go codebase, starting in a mode where it can't change your code.
