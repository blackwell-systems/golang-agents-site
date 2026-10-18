---
title: Glossary
description: The vocabulary of building AI agents, from agent and harness to idempotency key and tool poisoning, each in a sentence or two, with the chapter of the book that covers it.
layout: glossary
updated: 2026-10-18
---

Short definitions of the terms this guide uses.
Each entry names the chapter of *Building AI Agents in Go* where the term comes up, and links to a guide where one covers it.

<ul class="letters"><li><a href="#letter-a">A</a></li><li><a href="#letter-b">B</a></li><li><a href="#letter-c">C</a></li><li><a href="#letter-d">D</a></li><li><a href="#letter-e">E</a></li><li><a href="#letter-f">F</a></li><li><a href="#letter-g">G</a></li><li><a href="#letter-h">H</a></li><li><a href="#letter-i">I</a></li><li><a href="#letter-j">J</a></li><li><a href="#letter-l">L</a></li><li><a href="#letter-m">M</a></li><li><a href="#letter-n">N</a></li><li><a href="#letter-o">O</a></li><li><a href="#letter-p">P</a></li><li><a href="#letter-r">R</a></li><li><a href="#letter-s">S</a></li><li><a href="#letter-t">T</a></li><li><a href="#letter-u">U</a></li><li><a href="#letter-w">W</a></li></ul>

## A {#letter-a .letter}

### Ablation {#ablation}

Running the agent with a single component removed, next to the complete agent, so the difference in results shows what that component is worth.
<span class="cite-line">Chapter 13</span>

### Adapter {#adapter}

A thin layer that lets code written against one interface use something with a different one. In the book, an adapter makes each tool an MCP server offers look like an ordinary agent tool, so the loop needs no changes.
<span class="cite-line">Chapter 10</span>

### Agent {#agent}

A program where the model picks the next tool to call, one turn at a time, using what earlier tool calls returned, and keeps going until the job is finished or the harness hits a limit.
<span class="cite-line">Chapter 1 · [The agent loop in Go](/foundations/agent-loop/)</span>

### Agent event {#agent-event}

A notice the harness emits while a run is in progress, at the moment each thing happens: the model produced some text, a tool call is complete, a tool returned, a turn ended, or the run ended.
<span class="cite-line">Chapter 6</span>

### Agentic loop {#agentic-loop}

What the harness does each turn, in four steps: assemble the request, send it to the model, execute any tools the model asked for, then decide whether another turn is needed.
<span class="cite-line">Chapter 1 · [The agent loop in Go](/foundations/agent-loop/)</span>

### Approval {#approval}

A yes from a person, which the harness records, allowing a single specific action to proceed.
<span class="cite-line">Chapter 14</span>

### At least once {#at-least-once}

A delivery guarantee under which an action is never lost but may run more than once. You get it by retrying until a reply comes back.
<span class="cite-line">Chapter 15</span>

### At most once {#at-most-once}

A delivery guarantee under which an action never runs twice but may not run at all. You get it by never retrying.
<span class="cite-line">Chapter 15</span>

### Attempt marker {#attempt-marker}

A note the harness writes right before a call that could change the outside world, saying the call is starting. A marker with no saved result beside it means the call may or may not have happened.
<span class="cite-line">Chapter 15</span>

### Attention {#attention}

The part of a model that lets each position in the text look back over itself and all earlier positions, score them by relevance, and mix in information in proportion to those scores.
<span class="cite-line">Chapter 3</span>

### Audit log {#audit-log}

A record that is only ever appended to, saying who made each decision and when, so anyone can verify the decision afterward.
<span class="cite-line">Chapter 14</span>

## B {#letter-b .letter}

### Backoff {#backoff}

Increasing the wait before each retry, commonly doubling it, so a client that keeps failing puts less load on the server over time instead of more.
<span class="cite-line">Chapter 12</span>

### Barrier {#barrier}

A point that work can't pass in either direction: all the work before it must finish first, and none of the work after it starts until then.
<span class="cite-line">Chapter 9</span>

## C {#letter-c .letter}

### Chat template {#chat-template}

The rules a specific model uses to flatten a list of messages, plus any tool definitions, into the single stream of text it saw during training. Different model families use different templates.
<span class="cite-line">Chapter 3</span>

### Checkpoint {#checkpoint}

A saved snapshot of a run's state, written in a way that survives a crash at any instant: what's on disk afterward is always a whole snapshot, either the old one or the new one.
<span class="cite-line">Chapter 15</span>

### Circuit breaker {#circuit-breaker}

A guard placed in front of a dependency. After a string of consecutive failures it trips and fails calls immediately without sending them; after a pause it allows a single probe call through to find out whether the dependency is back.
<span class="cite-line">Chapter 12</span>

### Compaction {#compaction}

Shrinking the request by swapping the oldest part of the conversation for a model-written summary of it.
<span class="cite-line">Chapter 8</span>

### Context policy {#context-policy}

The code that decides which parts of the full conversation go into each request. The book's policies send everything, drop old turns, clear old tool results, or summarize.
<span class="cite-line">Chapter 8</span>

### Context window {#context-window}

The text a model can take into account on one call, and its maximum size in tokens. The prompt and the reply share it, so a long prompt leaves less room for the answer.
<span class="cite-line">Chapter 2</span>

### Cost budget {#cost-budget}

A cap, in dollars, on what a run may spend. The harness checks it before every call to the model.
<span class="cite-line">Chapter 5</span>

### Crash catalog {#crash-catalog}

The 32 triage tasks the book seeds for its examples and evals. Each task bundles crash reports made by sending requests to the practice service, `remind`; the answer the agent ought to reach; hidden tests for its fix; and the issues already on file.
<span class="cite-line">Chapter 7</span>

### Crash group {#crash-group}

A set of crash reports with a single underlying cause. Grouping is a judgment call: the same bug can crash in several different stacks, and two different bugs can produce the same stack.
<span class="cite-line">Chapter 7</span>

### Crash report {#crash-report}

Everything captured about one crash: the panic message and stack trace, which version of the program crashed, the request it was handling, and the last log lines before it.
<span class="cite-line">Chapter 7</span>

## D {#letter-d .letter}

### Data marker {#data-marker}

A random boundary string, new each time the program runs, wrapped around any text that came from outside the harness. The prompt says that anything between the markers is data, which the model must not obey.
<span class="cite-line">Chapter 14</span>

### Deduplication {#deduplication}

Using an ID that the sender attached to a message to spot a repeat of something already handled, so a message delivered twice acts as if it arrived once.
<span class="cite-line">Chapter 17</span>

### Delta {#delta}

A fragment of a reply that arrives over a stream. Each delta carries one of four things: some text, a piece of a tool call, the usage, or the finish reason.
<span class="cite-line">Chapter 6</span>

## E {#letter-e .letter}

### Egress {#egress}

Anything that flows out of a system. For the book's triage agent, that includes the text of each issue and comment it files.
<span class="cite-line">Chapter 14</span>

### Embedding {#embedding}

A vector, a list of numbers, that represents a token. Training adjusts the vectors so tokens that appear in similar contexts end up with similar vectors. Production models use a few thousand numbers per vector.
<span class="cite-line">Chapter 3</span>

### Eval {#eval}

An eval measures how well a model, a prompt, or an agent performs a task by running it many times against a real model, scoring each run, and reporting the result as a rate.
<span class="cite-line">Chapters 5 and 13</span>

### Eval set {#eval-set}

A fixed list of tasks, each with its starting inputs and the checks a result has to pass. Because it runs identically every time, it lets you compare one version of an agent with another.
<span class="cite-line">Chapter 13</span>

### Exfiltration {#exfiltration}

Moving data to an outsider who shouldn't have it, usually through a channel the system has for some other job.
<span class="cite-line">Chapter 14</span>

## F {#letter-f .letter}

### Fan-out {#fan-out}

Splitting one job into parts and having the harness run an agent on each part at the same time, then gathering their results.
<span class="cite-line">Chapter 9</span>

### Fingerprint {#fingerprint}

A short identifier derived from a crash report. The goal is that crashes from one cause share a fingerprint and crashes from different causes don't.
<span class="cite-line">Chapter 7</span>

### Finish reason {#finish-reason}

The field in each reply that says why the model stopped generating: it was done (`stop`), it ran out of room (`length`), it wants tools run (`tool_calls`), a content filter cut it off (`content_filter`), or the provider hit an error (`error`).
<span class="cite-line">Chapter 2 · [Tool calling in Go, on the wire](/foundations/tool-calling/)</span>

## G {#letter-g .letter}

### Graceful shutdown {#graceful-shutdown}

Ending a process without dropping work or leaving it half finished: refuse new work, complete or save whatever is in progress, then exit.
<span class="cite-line">Chapter 15</span>

### Grader {#grader}

Code that tests one run's result against each of its checks and decides whether the run passed.
<span class="cite-line">Chapter 13</span>

## H {#letter-h .letter}

### Hallucination {#hallucination}

A model presenting something false with the same confidence as something true. Nothing has malfunctioned: the model produces likely text, and a false statement can be likely.
<span class="cite-line">Chapter 3</span>

### Harness {#harness}

Everything in an agent program except the model. The harness carries out the tool calls the model requests, chooses what goes into each request, and decides when the run is over.
<span class="cite-line">Chapter 1 · [The agent loop in Go](/foundations/agent-loop/)</span>

### Hidden test {#hidden-test}

A test kept out of the agent's view that tries its fix on inputs other than the one in the crash report. A patch that merely prevents the reported panic doesn't pass.
<span class="cite-line">Chapter 13</span>

### HMAC {#hmac}

A keyed hash: producing or verifying one requires a secret key, so nobody without the key can forge it. Chapter 14 uses it so an edited audit log can be detected.
<span class="cite-line">Chapter 14</span>

### Host {#host}

A company that actually runs a model behind OpenRouter. OpenRouter routes each request to one of them, and each serves the model with its own account.
<span class="cite-line">Chapter 11</span>

## I {#letter-i .letter}

### Idempotency key {#idempotency-key}

A unique name the sender attaches to a request and the receiver stores. When a request arrives with a key the receiver has seen before, it returns the earlier result and doesn't act again.
<span class="cite-line">Chapter 15</span>

### Idempotent {#idempotent}

Safe to repeat: a second call leaves things exactly as one call did. In the triage agent, `run_tests` is idempotent; `issue_file` is not, since a second call files a second issue.
<span class="cite-line">Chapter 7</span>

### Idle timeout {#idle-timeout}

How long a client will wait on a stream that has gone quiet before abandoning it.
<span class="cite-line">Chapter 6</span>

### Interrupt {#interrupt}

A pause requested by a tool partway through a run, so the run can wait on something external, like a person's decision, and later pick up from the same point.
<span class="cite-line">Chapter 16</span>

### Issue tracker {#issue-tracker}

Where a team keeps its list of known problems, for example GitHub Issues or Jira. The companion code ships a small stand-in, `tracker`, which stores issues in a JSON file.
<span class="cite-line">Chapter 7</span>

## J {#letter-j .letter}

### Jitter {#jitter}

A random amount added to each retry's wait, so clients that failed at the same moment spread their retries out instead of all retrying at once.
<span class="cite-line">Chapter 12</span>

### JSON Schema {#json-schema}

A standard vocabulary for describing JSON values: the fields an object has, each field's type, and which fields are required. A tool definition uses one to describe its arguments.
<span class="cite-line">Chapter 4 · [Tool calling in Go, on the wire](/foundations/tool-calling/)</span>

### JSON-RPC {#json-rpc}

A small protocol for remote method calls carried in JSON. A request has an ID, the response repeats that ID so the caller can match the two, and a notification gets no response at all. MCP uses it as its message format.
<span class="cite-line">Chapter 10</span>

## L {#letter-l .letter}

### Language model {#language-model}

A program that takes text and returns a probability for every possible next piece. To generate, you choose a piece, add it to the end, and ask again, as many times as you want text.
<span class="cite-line">Chapter 3</span>

### Least privilege {#least-privilege}

Granting a program the capabilities its task requires and nothing else, so a bug or an attacker has nothing extra to misuse.
<span class="cite-line">Chapter 14</span>

### Logprobs {#logprobs}

For each token the model wrote, the logarithm of the probability it assigned to that token, and optionally the top alternatives it considered at the same spot. Exponentiate to get the probability back: a logprob of -1.11 is about 0.33.
<span class="cite-line">Chapter 3</span>

## M {#letter-m .letter}

### MCP {#mcp}

The Model Context Protocol: an open standard that lets one program expose tools and data, and lets an agent in another process or on another machine find and call them.
<span class="cite-line">Chapter 10 · [One sentence in a tool description decided 79 of 80 first moves](/findings/tool-description-poisoning/)</span>

### Message {#message}

A single entry in a conversation with a model, tagged with a role for its sender: `system` for instructions, `user` for whoever is asking, `assistant` for the model, and `tool` for a tool's output. The conversation is the ordered list of messages so far.
<span class="cite-line">Chapter 1</span>

### Middleware {#middleware}

A wrapper around each model or tool call that runs extra code before and after it, such as retrying or enforcing a timeout. Neither the wrapped call nor the loop has to know it is there.
<span class="cite-line">Chapter 12</span>

### Model {#model}

Short for large language model (LLM): a system trained to continue text. Your program sends it text through an API and gets back its prediction of what follows.
<span class="cite-line">Chapter 1</span>

### Model-as-judge {#model-as-judge}

Asking a model to score another model's output against a written rubric, for outputs that code can't check.
<span class="cite-line">Chapter 13</span>

## N {#letter-n .letter}

### Native API {#native-api}

The API a provider built for its own models. A compatible API, by contrast, imitates another provider's request and reply format.
<span class="cite-line">Chapter 11</span>

## O {#letter-o .letter}

### OpenRouter {#openrouter}

A service that puts many model providers behind a single API, so one key and one bill cover all of them. You pick a model by ID, and OpenRouter sends the request to a provider that runs it.
<span class="cite-line">Chapter 2</span>

### Orchestration graph {#orchestration-graph}

A program written as nodes, such as a model call, a tool call, or a branch, joined by edges. A framework executes the graph and takes care of checkpointing and resuming it.
<span class="cite-line">Chapter 16</span>

## P {#letter-p .letter}

### Parallel tool calls {#parallel-tool-calls}

More than one tool call in a single reply. The harness can run them concurrently; the model only asks for them and never executes anything itself.
<span class="cite-line">Chapter 9</span>

### Pass rate {#pass-rate}

Of an eval's scored runs, the share that passed every check, averaged per task so each task has equal weight. The book reports it with its range, how many tasks and runs it covers, and how many runs couldn't be scored.
<span class="cite-line">Chapter 13</span>

### Prompt caching {#prompt-caching}

A provider saving the computation it did on the opening tokens of a request, then charging less to reuse that work when a later request begins with the identical tokens.
<span class="cite-line">Chapter 8</span>

### Prompt injection {#prompt-injection}

Instructions planted in an agent's input by someone other than its user, which the model then carries out as though its user had given them.
<span class="cite-line">Chapter 14</span>

### Protected path {#protected-path}

A file or directory the agent can't modify unless a person approves.
<span class="cite-line">Chapter 14</span>

### Protocol version {#protocol-version}

A date that identifies a revision of MCP, for example `2026-07-28`. Starting with that revision, every request includes it in its `_meta` field.
<span class="cite-line">Chapter 10</span>

## R {#letter-r .letter}

### Range {#range}

The interval of plausible values for a measured number, allowing for how far chance could have moved it. The book uses the central 95 percent of results the measurement would give if repeated on a new sample of the same kind.
<span class="cite-line">Chapter 13</span>

### Rate limit {#rate-limit}

A cap a provider sets on the requests or tokens one caller can send per period. Anything beyond the cap is rejected with HTTP 429.
<span class="cite-line">Chapter 9</span>

### Read-only tool {#read-only-tool}

A tool that only reads, such as `code_read` or `crash_list`: calling it has no effect outside the run.
<span class="cite-line">Chapter 7</span>

### Reasoning model {#reasoning-model}

A model trained to produce reasoning tokens before it answers. The reply usually hides those tokens or shows a summary, but you pay for them as output.
<span class="cite-line">Chapter 3</span>

### Retry budget {#retry-budget}

A pool of retries shared by everyone calling a service. Failures draw it down and successes refill it, so when most calls are failing, retrying stops.
<span class="cite-line">Chapter 12</span>

### Run state {#run-state}

All a run needs to carry on once its process has exited: the conversation, the money spent so far, anything it is waiting on, and the step it was in the middle of.
<span class="cite-line">Chapter 15</span>

## S {#letter-s .letter}

### Sandbox {#sandbox}

Limits on what code may read, write, or connect to, enforced from outside that code so it can't lift them itself.
<span class="cite-line">Chapter 14</span>

### SDK {#sdk}

A library a provider publishes for using its API from a particular language. It typically includes request and response types and a client, and often helpers for retries and streaming.
<span class="cite-line">Chapter 11</span>

### Server-sent events {#server-sent-events}

A simple format for streaming messages from a server over an HTTP response that stays open. A message travels on lines beginning `data:` and ends at a blank line; a line beginning with a colon is a comment.
<span class="cite-line">Chapter 6</span>

### Settled {#settled}

Describes a call whose saved answer shows either that it completed or that it never started. The harness never runs a settled call again.
<span class="cite-line">Chapter 15</span>

### Side effect {#side-effect}

A lasting change a tool makes beyond the run, for example an edited file or a filed issue; it is still there when the run is over.
<span class="cite-line">Chapter 7</span>

### Span {#span}

One timed step in a trace. It records its parent span, when it started and ended, its attributes, and an error if there was one. Turns, model calls, attempts, and tool calls each get a span.
<span class="cite-line">Chapter 12</span>

### Stack trace {#stack-trace}

The chain of function calls active at the moment a program panicked, listed from the innermost call outward, each with its file and line number.
<span class="cite-line">Chapter 7</span>

### Stop reason {#stop-reason}

The harness's account of why a whole run ended: it got an answer, hit a limit, was cancelled, or failed with an error. Compare the finish reason, which explains only why a single reply ended.
<span class="cite-line">Chapter 5 · [The agent loop in Go](/foundations/agent-loop/)</span>

### Structured outputs {#structured-outputs}

An API option that forces the model's reply to fit a JSON Schema, so your code can decode it directly instead of parsing free text.
<span class="cite-line">Chapters 4 and 7</span>

### Sub-agent {#sub-agent}

A separate agent loop that a tool of another agent starts, with its own empty context, to handle one self-contained task and return a report.
<span class="cite-line">Chapter 9</span>

### Supply-chain attack {#supply-chain-attack}

An attack that comes in through something a project depends on (a library, a tool, a build step) rather than through the project's own code.
<span class="cite-line">Chapter 14</span>

### System prompt {#system-prompt}

The first message in a conversation, with the role `system`, holding instructions that apply throughout: what the model's job is and how it should respond.
<span class="cite-line">Chapter 2</span>

## T {#letter-t .letter}

### Threat model {#threat-model}

A brief inventory of what in a system is worth protecting, where untrusted input gets in, and what the system is able to do with that input, used to tie each defense to a specific risk.
<span class="cite-line">Chapter 14</span>

### Token {#token}

The unit of text a model reads and writes. A token can be a whole common word, a fragment of a longer word, or a punctuation mark; in English a token is about four characters on average.
<span class="cite-line">Chapter 2</span>

### Tokenizer {#tokenizer}

The program that converts text into tokens and tokens back into text. It has a fixed vocabulary, usually between 100,000 and 250,000 tokens, each assigned a number, and the model works entirely in those numbers.
<span class="cite-line">Chapter 3</span>

### Tool and tool call {#tool-and-tool-call}

A tool is an action your program offers to perform for the model, like reading a file or running tests, which the model knows by a name and a description of its inputs. A tool call is the model's request to run one of those tools with specific inputs, for example `read_file` on `duration.go`.
<span class="cite-line">Chapter 1 · [Tool calling in Go, on the wire](/foundations/tool-calling/)</span>

### Tool definition {#tool-definition}

The description of a tool sent to the model: a name, a plain-language explanation of what it does, and a JSON Schema for its arguments.
<span class="cite-line">Chapter 4 · [Tool calling in Go, on the wire](/foundations/tool-calling/)</span>

### Tool poisoning {#tool-poisoning}

Hostile instructions planted in a tool's description or schema to steer the model that reads the definition. Since definitions reach the model before any tool runs, it is prompt injection that acts before the first call.
<span class="cite-line">Chapter 10 · [One sentence in a tool description decided 79 of 80 first moves](/findings/tool-description-poisoning/)</span>

### Tool result {#tool-result}

What a tool returned, delivered to the model as a `tool` message tagged with the ID of the tool call it responds to.
<span class="cite-line">Chapter 1 · [Tool calling in Go, on the wire](/foundations/tool-calling/)</span>

### Trace {#trace}

A record of one unit of work, such as a single agent run, organized as a tree of steps, each with its timing.
<span class="cite-line">Chapter 12</span>

### Tuning set and confirmation set {#tuning-set}

Two separate groups of tasks. You may try a change against the tuning set while you develop it; the confirmation set judges the finished change and is never used during development.
<span class="cite-line">Chapter 13</span>

### Turn {#turn}

One trip around the loop: the harness sends the model the whole conversation so far, gets a reply, and acts on it.
<span class="cite-line">Chapter 1 · [The agent loop in Go](/foundations/agent-loop/)</span>

### Turn limit {#turn-limit}

A cap on how many turns a run can take before the harness ends it. A loop without one can keep running, and charging you, indefinitely.
<span class="cite-line">Chapter 1 · [The agent loop in Go](/foundations/agent-loop/)</span>

## U {#letter-u .letter}

### Unscored run {#unscored-run}

A run cut short by a failure in the harness or at the provider, so its result tells you nothing about the agent. Such runs are counted and reported, excluded from the pass rate, and rerun.
<span class="cite-line">Chapter 13</span>

### Usage {#usage}

The token counts the API returns with each reply: input tokens, output tokens, and on OpenRouter the dollar cost of the call.
<span class="cite-line">Chapter 2 · [Tool calling in Go, on the wire](/foundations/tool-calling/)</span>

## W {#letter-w .letter}

### Workflow {#workflow}

A program that calls a model at points its code decides ahead of time. In an agent, by contrast, the model picks the next step.
<span class="cite-line">Chapter 1</span>

### Workspace {#workspace}

A separate copy of a codebase made for one run, which the agent reads and edits so the original is never touched.
<span class="cite-line">Chapter 7</span>
