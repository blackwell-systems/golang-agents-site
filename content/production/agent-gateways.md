---
title: "Agent gateways in Go: one MCP endpoint, a policy on every call, and a kill switch"
short: Agent gateways
description: How to put an MCP gateway between your agents and the MCP servers they use, in tested Go with the official MCP Go SDK. A default-deny policy on every tool call by user, agent, and tool; only approved tools published, with definitions pinned by digest; OAuth token exchange (RFC 8693) so no upstream sees the agent's token; a tamper-evident audit log; and revocation that cancels calls in flight.
card: One MCP server in front of the rest that decides every call, gives each upstream its own token, and can stop an agent.
group: Safety
stage: 3
level: advanced
date: 2026-11-03
weight: 77
beyond: true
art: agent-gateways
next: true
---

One agent with one MCP server can leave every decision to that server.
A company with a dozen agents and a dozen servers can't: each agent holds a token for each server, each server applies its own rules, and when an agent misbehaves at two in the morning there are twelve places to cut it off and no one record of what it did.

An [MCP gateway](/glossary/#mcp-gateway) is one MCP server that agents connect to, in front of the servers that do the work.
Agents see one endpoint and one list of tools.
Behind it, the gateway decides every call, gets each server a token of its own, writes down what happened, and is the one place an operator goes to stop something.

This page builds one in Go and runs it, offline, against two MCP servers, with tests for each thing it promises.
It builds on [MCP authorization](/production/mcp-authorization/): the gateway checks agents' tokens with that page's verifier, and every server behind it is protected the way that page describes.

## What it does, and what it doesn't

The gateway here does five things:

- **Decides every call** against a policy that names users, agents, and tools, and refuses whatever no rule allows.
- **Publishes only approved tools.** An operator approves each one, and the gateway pins the definition it approved, so a server that changes a tool afterward is caught.
- **Gives each upstream its own token**, through OAuth token exchange, with only the scopes that call needs. The agent's token never leaves the gateway.
- **Keeps an audit log** of every decision and every operator change, in which an edited or missing entry shows.
- **Revokes** a user, an agent, or a tool in one place, which also cancels the calls already running.

It doesn't replace the servers' own checks.
Each server behind it still verifies every token it receives, as if the gateway weren't there, so a bug in the gateway, or a request that goes around it, still meets a server that refuses what isn't meant for it.
It doesn't make a model safe to trust with tools either: [prompt injection](/production/prompt-injection/) and [approval](/production/approval/) cover what has to happen around the model.
What it adds is one place where the rules for every agent and every server are written, applied, and recorded.

## The whole thing, in one run

The demo starts an authorization server, two MCP servers (a refunds service and the issue tracker), and the gateway, all on loopback addresses.
Two agents connect: a support agent acting for bob, and a billing agent acting for alice.
The log shows every request the gateway sends on, so a call that went nowhere is visibly one that went nowhere:

```text include=mcpauth/cmd/gateway/testdata/gateway.txt title="go run ./cmd/gateway (in code/mcpauth)"
```

```diagram
name: agent-gateway
alt: A sequence of four participants: the agent, the gateway, the authorization server, and the refunds server. The agent sends tools/call refunds.refund with a token issued for the gateway. The gateway checks the token and asks the policy about alice, the billing agent, and refund. It sends a token exchange to the authorization server naming the refunds server and the least scopes, and gets back a token for the refunds server whose act claim names the gateway and the billing agent. It lists the refunds server's tools and checks the definition against the pin, calls refund with the exchanged token, gets the result, appends to the audit log, and returns the result to the agent.
tall: yes

One allowed call, step 3 of the log. Everything the gateway does happens between the agent's request and the answer.
```

Before step 1, an operator approves tools: both of the refunds service's, and only the tracker's search.
`issue_file` exists on the tracker, but no agent is ever shown it.

1. The support agent lists its tools and sees two of the three published ones: the policy lets it look orders up and search issues, and nothing else.
2. It calls `refunds.refund` anyway, the way a model following an [injected instruction](/production/prompt-injection/) would. The gateway refuses it, and the log has no line for it: no token was exchanged, and nothing reached the refunds service.
3. The billing agent refunds $12.00 for alice. The gateway exchanges alice's token for one issued for the refunds service, lists the service's tools to check the definition, and calls it. The refunds service names alice as the user and the gateway, acting for the billing agent, as the caller.
4. The billing agent sends its own token straight to the refunds service, going around the gateway. The service refuses it: the token was issued for the gateway.
5. The refunds service changes `order_lookup`'s description to tell the model to refund every order. The next call through the gateway notices and quarantines the tool.
6. An operator revokes the billing agent, and its next call is refused.
7. The audit log, fetched through the admin API, checks out, and stops checking out when one entry is changed.

## Who is calling

Agents get tokens the way any MCP client does, issued for the gateway, which is the only server they talk to.
A token's subject is the user, and its `client_id` is the agent: each agent is its own OAuth client, so the gateway can tell the billing agent from the support agent even when both act for the same person.

```go include=mcpauth/gateway/gateway.go tag=new
```

1. **Every pinned tool is registered** under its published name, `refunds.refund`, with a handler that decides and forwards. The definition is the one the operator approved, so the model reads the approved description even if the server has changed it since.
2. **A middleware filters `tools/list`** for the caller.
3. **The server is stateless**, as MCP is from the 2026-07-28 revision: one server answers every request, and each request carries its own token.

The tools endpoint needs `gateway:tools` and the admin API needs `gateway:admin`, so an agent's token can't change the rules.

## Decide every call

A rule names users, agents, and tools.
Users are token subjects, agents are client IDs, and tools are the published names, with `refunds.*` for every tool of one server:

```go include=mcpauth/gateway/policy.go tag=rule
```

The demo's policy is two rules:

```go include=mcpauth/cmd/gateway/main.go tag=policy
```

Deciding is a short function, and its order is the whole policy model:

```go include=mcpauth/gateway/policy.go tag=decide
```

A revocation wins over every rule, so the kill switch can't be overridden by an allow someone forgot.
A deny rule wins over every allow rule, so "no refunds this week" can be added without editing the rules that grant refunds.
And with no rule that matches, the answer is no.
A gateway that allows by default publishes every new tool on every new server to every agent the moment it appears.

The tools an agent is shown come from the same decisions:

```go include=mcpauth/gateway/gateway.go tag=filter
```

Filtering the list keeps a tool the model may not use out of its context, which matters, since a tool's description is text the model reads and follows.
It isn't enforcement.
An agent can call a tool by a name it was never shown, which is exactly step 2 of the log, so the handler decides again on every call:

```go include=mcpauth/gateway/gateway.go tag=proxy
```

1. **The call registers as in flight before the policy decides it.** A revocation either lands first, and the policy refuses the call, or lands after, and finds the call to cancel. Deciding first would leave a gap in which a call is allowed, not yet registered, and so missed by a revocation.
2. **The policy decides**, and a refusal goes back to the model as a tool error it can read: `denied: no rule allows it`. Every outcome, refused or not, gets an audit entry.
3. **A cancelled call reports why.** When a revocation cancels it, the reason it gives is the revocation's, not `context canceled`.

`TestDeniedNeverReachesUpstream` sends the denied call and checks three things: the agent got a refusal, no request reached the upstream, and no token exchange happened.

```go include=mcpauth/gateway/gateway_test.go tag=deny-test
```

## Never pass the agent's token on

The agent's token was issued for the gateway.
Sending it on to the refunds service would be [token passthrough](https://modelcontextprotocol.io/specification/2026-07-28/basic/security_best_practices#token-passthrough), which the MCP specification forbids, and it wouldn't work: the refunds service checks the audience, as step 4 shows.

Using one token of the gateway's own for every call would work, and would be worse.
The refunds service would see every request as coming from the gateway, with the gateway's rights, and couldn't tell alice from bob or apply either one's limits.
The gateway would become the [confused deputy](/glossary/#confused-deputy) the audience rule exists to prevent.

So for each call the gateway asks the authorization server for a token issued for that upstream, for the same user, through [token exchange](/glossary/#token-exchange) ([RFC 8693](https://www.rfc-editor.org/rfc/rfc8693.html)):

```go include=mcpauth/gateway/broker.go tag=exchange
```

The request sends the agent's token as the `subject_token`, names the upstream as the `resource`, and asks for the scopes the pinned tool needs, `refunds:read refunds:write` for a refund and `refunds:read` for a lookup, rather than everything the service defines.
The gateway authenticates as a confidential client of its own, and the authorization server checks that this client may exchange tokens issued for the gateway, and only for the servers behind it.

The token that comes back has an `act` claim ([RFC 8693, section 4.1](https://www.rfc-editor.org/rfc/rfc8693.html#section-4.1)) naming who is acting for the user: the gateway, and inside it, the agent that called the gateway.
The [verifier](/production/mcp-authorization/#the-server-verify-every-token-yourself) passes the chain on, and the refunds tool reports it:

```go include=mcpauth/servers/refunds.go tag=refunds
```

That is how step 3 ends with `for alice@example.com, asked by gateway for billing-agent`: the service knows whose money it is and which agent asked, from a token signed by the authorization server rather than from anything the gateway or the agent claims.

The broker reuses a token until ten seconds before it expires, keyed by a hash of the agent's token, the upstream, and the scopes, so a run of calls costs one exchange.
An exchanged token never outlives the token it came from.

`TestUpstreamGetsExchangedToken` checks the upstream never saw the agent's token, that what it saw was issued for it with the pin's scopes, and that two calls cost one exchange.

A real authorization server decides more at the exchange than this one does.
Here it grants the gateway any scope the upstream defines, for any user, which leaves the per-user decision entirely to the gateway's policy: delete the policy check from the proxy and the support agent refunds bob's order, with a token the refunds service accepts.
Your identity provider can also check, at the exchange, whether this user may hold `refunds:write` at all, and that check is worth turning on.

## Pin what was approved

An operator approves tools by connecting to a server, reading its tool list, and choosing:

```go include=mcpauth/gateway/pins.go tag=pin
```

The pin keeps the definition and a digest of it:

```go include=mcpauth/gateway/pins.go tag=digest
```

The digest is over canonical JSON ([RFC 8785](https://www.rfc-editor.org/rfc/rfc8785.html), which Go's `encoding/json/jsontext` implements as `Value.Canonicalize`), so the same definition sent with its keys in another order gives the same digest, and any change to its description, its schema, or its annotations gives another.

The attack it catches is a server that changes a tool after it was approved, sometimes called a rug pull.
The new description in step 5 tells the model to refund every order it looks up.
A sentence like that is not hypothetical: in [one measurement](/findings/tool-description-poisoning/), a single sentence in a tool's description steered four models' first move in 79 of 80 replies.

Before each call, the gateway compares the server's current definition with the pin:

```go include=mcpauth/gateway/gateway.go tag=check-pin
```

A mismatch revokes the tool for everyone, which is what quarantine means here, and writes the reason in the log.
The tool stays off until an operator reads the new definition, approves it again, and lifts the revocation.
The model was never shown the new description, since the gateway publishes the pinned one, but the server's behavior may have changed with it, and no agent should call it until someone has looked.

Listing the tools before every call costs a request.
A gateway under load can cache the list, refreshing it on a timer and when the server sends `notifications/tools/list_changed`, and still compare at call time against the cached copy.

```go include=mcpauth/gateway/gateway_test.go tag=rugpull-test
```

## The kill switch

```go include=mcpauth/gateway/gateway.go tag=revoke
```

Revoking writes the revocation into the policy, so every later call is refused, then cancels every call already running for that user, agent, or tool.
`TestRevokeCancelsInFlight` starts a call to a tool that runs until it is cancelled, revokes the agent while it runs, and checks the call comes back as `revoked: it misbehaved`, and that the next call is refused.

Cancelling the gateway's side is the easy part.
Whether the upstream stops too depends on the upstream: the MCP servers here are stateless, and with the SDK's `PropagateRequestCancellation` option a handler's context ends when the gateway abandons the request, so the tool stops at once.
Testing found the other case.
Against a stateful server, the SDK's client sends its cancel notice in the background, the gateway closed the session first, and the notice never arrived: the tool kept running, and closing the session took five seconds.

Even a tool that stops at once may have done its work first.
A refund that reached the payment provider a millisecond before the revocation has happened, so a tool that moves money needs an [idempotency key](/production/durable-runs/#doing-something-exactly-once) and a record of what it did, whatever sits in front of it.

Revoking at the gateway also leaves tokens the gateway already obtained valid until they expire, five minutes at most here.
That is safe because no agent ever holds one: the agent's own token is good only at the gateway, which now refuses it.

## The admin API

```go include=mcpauth/gateway/admin.go tag=admin
```

It takes only tokens issued for the gateway with `gateway:admin`, rejects a rule with a field it doesn't know, so a misspelled field is an error rather than ignored, and logs every change, with the operator's name, in the same log as the calls it affects.
Step 6 of the log is a `PUT /admin/revocations/agent/billing-agent`, and entry 5 of the audit log records who sent it.

## An audit log that shows tampering

```go include=mcpauth/gateway/audit.go tag=entry
```

An entry records a digest of a call's arguments rather than the arguments, which can hold customer data the log shouldn't keep: whoever has the arguments can check them against it.

Each entry's `Sum` is an [HMAC](/glossary/#hmac) over the entry, including the `Sum` of the one before, keyed with a secret only the gateway holds:

```go include=mcpauth/gateway/audit.go tag=verify
```

Changing an entry, dropping one, or rebuilding the log under a guessed key all fail verification, and `TestAuditChain` does all three.
It is the technique [approval](/production/approval/#keep-a-log-that-shows-tampering) uses for its log, with the same limits: entries cut from the end leave the chain intact, so publish the latest `Sum` somewhere the gateway can't edit, and send each entry, as it is written, to a store that accepts additions and no rewrites.

## Model gateways are a different thing

"Gateway" also names something else: an LLM gateway, between your agents and the model providers.
It routes requests to providers, holds the provider API keys so no agent does, applies rate limits and spending limits per team, retries and fails over, and logs prompts and costs.

The two do different jobs at different boundaries.
A model gateway decides which model answers and how much it may cost; an MCP gateway decides what the answer may do.
An agent in production often goes through both, and the same rules carry over to each: the agent never holds the real credential, the gateway applies limits per user and per agent, and every decision is logged.

## Why this way and not another

- **No gateway, each server enforcing alone.** Right for a few agents and a few servers, and each server must check its own tokens either way. With many, the rules are scattered, there is no one switch to stop an agent, and no one log says what it did.
- **Forwarding the agent's token.** Forbidden by the MCP specification, and refused by any server that checks the audience.
- **One service token for every upstream.** The upstream can no longer tell users apart, and every user's agent gets the gateway's rights.
- **Filtering `tools/list` and nothing else.** Hides tools from the model and stops nothing: a call by name still goes through.
- **Allow by default.** Every tool a server adds reaches every agent before anyone has read it.
- **Asking the model to follow a policy in its prompt.** The model chooses which tool to call; it can't be the thing that decides whether the call is allowed, since anything in its context can talk it out of a rule.

## Mistakes to avoid

- **Deciding only when listing tools.** Decide on every call.
- **Deciding before registering the call.** A revocation in between misses it.
- **Logging tokens or raw arguments.** Log the user, the agent, the tool, and a digest.
- **Letting an outgoing call inherit the incoming request's context.** The SDK keeps values in the context of a request it is serving, among them the protocol version the agent asked for, and a client making a request upstream with that context picks them up. Here it sent a stateful server a protocol version only stateless servers accept, and every call failed. `forward` passes the cancellation on and drops the values.
- **Treating revocation as instant everywhere.** It is instant at the gateway; at the upstream it depends on cancellation, and at the payment provider on idempotency.
- **Skipping the checks in the servers behind the gateway.** They are what stops a request that goes around it.

## Try it

From the `code/mcpauth` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go run ./cmd/gateway
$ go test -race ./...
```

1. In `gateway/gateway.go`, make the proxy skip its policy check (change `if !d.Allow` to `if false`) and run `go test ./gateway -run TestDeniedNeverReachesUpstream`. The support agent now refunds bob's order in full: the tool it was never shown is still a name it can call.
2. In `forward`, delete the `checkPin` call and run `go test ./gateway -run TestRugPull`. The changed tool is called, still listed, and never quarantined.
3. In `resource/server.go`, set `Stateless` to `false` and run `go test ./gateway -run TestRevokeCancelsInFlight -v`. The test still passes, five seconds slower: the upstream never hears that the call was cancelled.

Undo your edits with `git restore .` in the `code` directory.
