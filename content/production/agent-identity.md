---
title: "Agent identity: who an AI agent acts for, and with what rights"
short: Agent identity
description: Every action an agent takes involves three identities, the user it acts for, the agent as software, and the run. How to give an agent narrow, short-lived credentials the model never sees, keep permission in the tools, give sub-agents less than their parent, and record who did what, with Go code you can run.
card: Whose rights an agent uses, how to hand them over safely, and how to prove afterward who did what.
group: Safety
stage: all
level: intermediate
date: 2026-10-23
weight: 70
art: agent-identity
---

Picture a support agent that handles refunds.
A customer writes in, the agent looks up the order, and it calls a `refund` tool.
One afternoon it refunds the wrong customer, perhaps because a message it read contained an instruction, perhaps because it mixed up two order numbers.
The money is gone, and the questions start.
Who asked for this refund?
Which program made the call?
Which of the many conversations running that day was it?
What allowed a support agent to refund any customer at all?
If the agent used one shared API key, the service's log says only that the key was used, and nobody can answer any of the four.

The cause is usually that three different identities were treated as one:

- **The user** the agent acts for: the customer or employee whose request started the work.
- **The agent**: the software itself, with its own name, version, and place in your system.
- **The run**: this one session or task, with a start and an end.

An action is correct only when all three are known and each is limited on its own terms.
The user's rights set the ceiling.
The agent's role may lower it.
The run's task lowers it further, to the few things this task needs.
This page walks through how to do that with Go from the [companion repository](https://github.com/blackwell-systems/go-agents-book-code)'s `identity` example, which runs without a model or an API key: `go run ./identity`.

## The confused deputy

A **confused deputy** is a program that holds authority and is tricked into using it for someone who should not have it.
The program is the deputy: it has the power, and it acts on requests.
It is confused when it cannot tell whose request it is serving, so it applies its own power to a stranger's wishes.

A [prompt injection](/production/prompt-injection/) is mostly this attack.
Someone other than the user plants text in a page, a ticket, or a crash report, and the model, which holds the user's authority for the session, acts on that text as if the user had written it.
The attacker has no rights of their own.
They borrow the agent's.
The defense is the same as for any deputy: make the authority small and specific, so that what a confused deputy can do is small too.
The rest of this page is about doing that.

## Acting on someone's behalf

An agent that acts for a user needs a **delegated credential**: a token that says "this program may do these things for this user, until this time."
Four properties matter:

- **Narrow.** It lists **scopes**, named permissions such as `refund:order-1042`, and only the ones this task needs.
- **Short-lived.** It expires in minutes, so a leaked copy soon becomes useless.
- **Tied to the user.** The service can see which user the action is for and apply that user's limits.
- **Never the user's password**, and never one long-lived key shared across all users. A password gives the agent everything the user can do, forever, and a shared key makes every action look the same.

The general pattern for getting such a credential is **token exchange**.
The agent's platform holds a token that proves who the user is, and it asks an authorization server (the service that issues tokens) to swap it for a new token with a narrower audience, narrower scopes, and a short life.
OAuth 2.0 Token Exchange, [RFC 8693](https://www.rfc-editor.org/rfc/rfc8693.html), defines this request.
It separates two cases.
In *impersonation*, the new token lets A be treated as B, indistinguishable from B.
In *delegation*, A keeps its own identity, and it is understood that A acts on B's behalf.
For agents, delegation is the right case, because the service can then log both parties.

<div class="hood">

An RFC 8693 request is an HTTP POST to the authorization server's token endpoint with `grant_type` set to `urn:ietf:params:oauth:grant-type:token-exchange`.
The `subject_token` parameter carries a token for the party the request is made on behalf of.
The optional `actor_token` identifies the acting party, which here would be the agent.
The `audience` parameter names the service the new token is for, and `scope` asks for the permissions wanted.
The issued token can carry an `act` claim naming the actor, which is how a receiving service sees "the agent, acting for this user".
A separate `may_act` claim lets a token say which parties are allowed to become the actor.
The authorization server decides whether to grant the request, so the narrowing happens where the user's rights are known.

</div>

Most teams do not need the full protocol on day one.
The shape is what counts: a signed statement of user, agent, run, scopes, and expiry, issued by one place and checked by the service.
The `identity` example uses that shape, signed with `crypto/ed25519` from the standard library.
A credential's claims name the user, the agent, the run, the scopes, and the expiry:

```go include=identity/token.go tag=claims
```

Only the harness holds the signing key, in an `Issuer`:

```go include=identity/token.go tag=mint
```

The service holds the public key, in a `Verifier`, and checks the signature and the expiry on every call.
Each failure is a sentinel error the caller can test with `errors.Is`:

```go include=identity/token.go tag=verify
```

A real system would use a standard token format such as a JWT, issued by an authorization server, with an audience claim; the example keeps only what the argument needs.

When a run starts, the harness mints a credential for exactly what the user asked for, and builds the tool with it:

```go include=identity/main.go tag=start
```

## The model never holds the secret

The credential is attached by the harness, at the point where a tool call leaves for the service, after the model has asked for the action.
The model sees a tool named `refund` and its arguments.
It never sees the token.

This matters because anything in the context window can come back out.
The model can repeat it in a reply to a user, a log can record it, and an injected instruction can ask the model to pass it as an argument to some other tool.
A secret the model never received cannot be leaked by the model.
The [prompt injection page](/production/prompt-injection/) lists the other ways text leaves an agent and the checks that sit on them.
The [service page](/production/agent-as-a-service/#a-container-image-that-keeps-the-key-out-of-reach) shows a related leak: a key held in an environment variable can still be read from `/proc/1/environ` by any process of the same user, even after the program removes the variable.

In Go, an `http.RoundTripper` is a natural place to attach a credential, because every request a tool makes passes through it:

```go include=identity/transport.go tag=attach
```

The tool itself names the customer and nothing else; its client was built by the harness with that run's transport.
The model's arguments and the tool's result text never contain the token.
Make sure error messages and debug logs do not print request headers.

## Permission lives in the tools

A system prompt that says "only touch this customer's data" is a request to the model.
The model may follow it, and an injected instruction may talk it out of it.
A check in code decides what happens, whatever the model was persuaded to want.

So the service, or the tool in front of it, checks the credential on every call.
An action outside the credential's scopes fails.
If the action is one a person should be able to approve, it goes to a person instead, as described in [human approval](/production/approval/).

The example's refund service does both checks, and records the attempt whether or not it was allowed:

```go include=identity/service.go tag=refund
```

If the agent is talked into refunding customer 43 while holding only `refund:42`, the call fails with an ordinary error, which goes back to the model as a tool result like any other failure.
The prompt can still say what the agent should do.
The credential decides what it can do.

## Sub-agents get less, never more

When an agent starts a [sub-agent](/build/sub-agents/) for part of the work, the sub-agent should receive a credential narrower than its parent's.
The idea is called **attenuation**: each handoff can remove permissions and none can add them.
A sub-agent that only reads orders gets `read:` scopes and no `refund:` scope, and it cannot get the refund permission back, because it has no way to ask for more than it was given.

```go include=identity/narrow.go tag=narrow
```

The child keeps the same user and run, gets its own credential ID and agent name, holds only scopes the parent held, and never outlives the parent.
The harness calls `Narrow` and then `Mint`:

```go include=identity/main.go tag=sub-agent
```

Only the harness holds the signing key, so a sub-agent cannot build a wider token for itself.
The sub-agent's tools, in turn, check that narrower token exactly as in the previous section.

## The agent's own identity

The software also needs an identity, separate from any user's.
A **workload identity** is an identity issued by the platform to a running piece of software, so the software proves what it is without a stored password.
Cloud IAM roles work this way: the platform gives the process short-lived credentials tied to a role.
So does SPIFFE, an open standard whose identity documents (SVIDs) are short-lived X.509 certificates or JWTs that a workload obtains from a local Workload API, with automatic rotation.
The [SPIFFE overview](https://spiffe.io/docs/latest/spiffe-about/overview/) describes the standard.

The alternative is one shared API key in a config file.
Everything that holds the key looks like the same caller, and a leak means replacing it everywhere.
With a workload identity, each agent is a distinct caller whose credentials expire and rotate on their own.
Keep any remaining long-lived keys where tools and test code cannot read them: files with strict permissions or a secret manager, and out of the environment.
Rotate them on a schedule, and keep rotation cheap enough that you actually do it.

## Telling the world you are an agent

When your agent calls someone else's website or API, the other side may want to know that an automated agent is calling, and whose.
They can then give agents different limits, apply different rules, or block abuse without blocking people.

A `User-Agent` header does not prove this, because anyone can copy any string.
A signed request does.
HTTP Message Signatures, [RFC 9421](https://www.rfc-editor.org/rfc/rfc9421.html), let a sender sign chosen parts of an HTTP message, such as the method, the target URI, and selected headers.
The signature travels in a `Signature` header, and a `Signature-Input` header says which components it covers and gives details such as a key identifier and creation time.
The receiver verifies the signature with the sender's public key.

The standard is careful about its limits: a signature shows that someone holding a key made the request, and who that key belongs to depends on how the receiver finds and trusts it, which the standard leaves to applications.
The IETF has a working group on this subject, [Web Bot Auth](https://datatracker.ietf.org/wg/webbotauth/about/).
Its charter covers standardizing ways for automated clients, including AI agents retrieving content for users, to authenticate to websites.
The charter excludes authenticating the end user and excludes APIs and agent-to-agent interfaces, so check its current documents before relying on it.

## How agent protocols handle it

The [Model Context Protocol](/build/mcp/) (MCP) makes authorization optional.
For HTTP-based transports, its authorization specification builds on OAuth 2.1 (a draft consolidation of OAuth 2.0 practice): the MCP server acts as a resource server that accepts access tokens, the MCP client acts as an OAuth client making requests on behalf of the user, and a separate authorization server issues the tokens.
The specification requires clients to send a `resource` parameter naming the server the token is for (RFC 8707), and requires servers to accept only tokens issued for them.
Clients must not send an MCP server any token other than one issued by that server's authorization server, and servers must not accept or pass along tokens meant for other services.
Those rules block one common confused-deputy path, where a server forwards a token it received to a different service.

For a local server started over stdio, the specification says implementations should not follow that flow and should instead take credentials from the environment.
That puts the secret in the server process's environment, which is why the previous sections about where keys live apply to it as well.
These details come from the [current MCP authorization specification](https://modelcontextprotocol.io/specification/latest/basic/authorization), which was dated 2026-07-28 when this page was written; read the version you target, since it is still changing.

## The audit trail

When something goes wrong, you need one record per action that answers the four questions from the opening story.
Write it from the harness, at the moment of the call:

```go include=identity/audit.go tag=record
```

`User`, `Agent`, `Run`, and `Credential` come from the verified claims, so the record names the credential used as well as the person.
`Approver` is filled when a person approved the call, and `Reason` says why a call was refused.
Record denials too, since a pattern of refused calls is often the first sign of an injection.
Log the credential's ID and never the token itself.
To make the log itself trustworthy, use the signed, chained log described in [human approval](/production/approval/#keep-a-log-that-shows-tampering), so edits and deletions show up.

## Run it

From the `code` directory of the companion repository, with no API key:

```console
$ go run ./identity
run run-7: agent support acting for u-42, scopes refund:42 read:orders
  refund 42: refunded customer 42
  refund 43: refund: refused: identity: scope not held: refund:43
narrow to refund:43: identity: scope not held: parent lacks refund:43
sub-agent reader, scopes read:orders, expires with its parent: true
  refund 42: refund: refused: identity: scope not held: refund:42
audit log:
  u-42    support run-7  cred-1 refund:42 allowed=true
  u-42    support run-7  cred-1 refund:43 allowed=false
  u-42    reader  run-7  cred-2 refund:42 allowed=false
```

The first refund is the user's own and goes through.
The second names another customer and is refused by the service, whatever the model was told.
The sub-agent can't be granted a refund scope the parent lacks, and the one it holds, `read:orders`, can't refund anything.
Every attempt is in the audit log, refused ones included.
`go test ./identity` checks the rest: an expired token, a token from another key, a tampered signature, and a request the transport must leave unchanged.

## Where each identity and credential lives

One request, followed from the user to the service:

| Step | Who or what acts | Identity in play | Credential held here | What is recorded |
| --- | --- | --- | --- | --- |
| 1. Request | The user, in a signed-in session | The user | The user's own session | The user starts a run |
| 2. Harness | The agent's loop, outside the model | The agent (workload identity) and the run | A narrow, short-lived delegated token for this user and run, minted here | Run ID, user, agent, credential ID |
| 3. Model | The model, inside the loop | None; it only proposes calls | None; the model never sees a secret | The call it proposed |
| 4. Tool | The tool wrapper | The agent acting for the user | Attaches the token to the outgoing request | Whether a person approved |
| 5. Service | The API being called | Reads user, agent, run from the token | Checks signature, expiry, and scope on every call | An audit record: user, agent, run, credential, approver, allowed or denied |

Read it top to bottom: the user's authority narrows at step 2, the secret is attached at step 4, and the check that matters happens at step 5.

## Questions to ask before an agent acts for real users

1. **Can you name the user, the agent, and the run for every action?** If a log line has only an API key, the answer is no.
2. **Is each credential narrow?** It lists the scopes this task needs and no more.
3. **Does each credential expire in minutes or hours, and is it tied to one user?** Check that no password or shared long-lived key is in use.
4. **Does the model never see a secret?** Look in the system prompt, tool results, logs, and error messages.
5. **Does the service check the credential on every call?** A rule that exists only in the prompt does not count.
6. **Do sub-agents get a narrower credential than their parent, and can they widen it?** They should not be able to.
7. **Does the software have its own workload identity, and are leftover keys out of tools' reach and rotated?**
8. **Is there an audit record with user, agent, run, approver, and credential ID, and can edits to it be detected?**

## Where next

- [Prompt injection and sandboxing](/production/prompt-injection/): the attack that most often confuses an agent, and the defenses outside the model.
- [Asking a person before a risky tool call](/production/approval/): the gate for actions that need a decision, and the tamper-evident audit log.
- [Sub-agents](/build/sub-agents/): how a fresh agent is started from inside a tool.
- [The Model Context Protocol](/build/mcp/): how tool servers are reached, and what runs in each process.
- [An agent as a service](/production/agent-as-a-service/): keeping keys out of the environment.
- [Launch checklist](/production/launch-checklist/): the wider list to check before leaving an agent running.
