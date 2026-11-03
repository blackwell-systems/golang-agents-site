---
title: "MCP authorization in Go: resource servers, audience-bound tokens, and step-up"
short: MCP authorization
description: How to put an MCP server behind OAuth 2.1 and connect a client to it, following the MCP authorization specification of 2026-07-28, in tested Go with the official MCP Go SDK. Protected Resource Metadata (RFC 9728), PKCE, the resource parameter (RFC 8707), the iss check (RFC 9207), Client ID Metadata Documents, JWT access token validation (RFC 9068), per-tool scopes with step-up, and why a token issued for one server is refused by every other.
card: An MCP server that accepts only tokens issued for it, a client that gets one, and the checks the SDK leaves to you.
group: Safety
stage: 3
level: advanced
date: 2026-11-03
weight: 75
beyond: true
---

An MCP server started over stdio runs on the user's machine and takes its credentials from the environment.
A remote MCP server, reached over HTTP, is different: many users and many clients reach it over the network, and it has to know who is calling and what they may do.
The MCP specification answers with OAuth 2.1: the MCP server is a resource server that accepts access tokens, the MCP client is an OAuth client acting for a user, and an authorization server, which may be your company's identity provider, issues the tokens.

This page builds both sides in Go and runs them against each other, offline, with tests for the attacks the specification is written to stop.
It follows the [MCP authorization specification](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization) dated 2026-07-28, which changed in ways older write-ups miss: Client ID Metadata Documents are now the preferred way for a client to identify itself, Dynamic Client Registration is deprecated, and clients must check which authorization server answered them.
The MCP transport and most of the client's flow come from the [official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk) (v1.8.0).
What the SDK leaves to the server, verifying the token, is where most of this page goes, because that is where the mistakes are.

## One rule behind the rest

Every access token is issued for exactly one MCP server, and that server accepts no other.

The client names the server it wants a token for in a `resource` parameter ([RFC 8707](https://www.rfc-editor.org/rfc/rfc8707.html)), on both the authorization request and the token request.
The authorization server writes that server into the token's audience, its `aud` claim.
The MCP server refuses any token whose audience isn't its own address, and it never sends a token it received on to anyone else.

The rule matters for agents because an agent connects to many servers at once.
If one token worked everywhere, any server the agent talks to, including a malicious one or a compromised one, could take the token it was sent and use it at every other server the user has connected.
That is a [confused deputy](/glossary/#confused-deputy) and a token replay in one, and audience binding is what prevents both.

## The whole flow, in one run

The example's demo starts an authorization server, two MCP servers, a website hosting the client's metadata, and a stand-in for the user's browser, all on loopback addresses, and prints every HTTP request:

```text include=mcpauth/testdata/demo.txt title="go run . (in code/mcpauth)"
```

```diagram
name: mcp-auth
alt: A sequence of three participants: the MCP client, the MCP server, and the authorization server. The client sends a request with no token and gets a 401 naming the resource metadata; it fetches the resource metadata, which names the resource and its authorization servers, then the authorization server's metadata, with its issuer, endpoints, and S256; it sends the user's browser to authorize with PKCE, the resource, and the scope, and the browser comes back with a code, state, and iss; the client checks state and iss, redeems the code with the verifier and the resource, and gets a token whose audience is the MCP server; it sends the request with the token, and the server checks its signature, type, issuer, audience, and expiry.
tall: yes

A client's first connection, as in step 1 of the log. The authorization request goes through the user's browser, where the authorization server also fetches the client's metadata document.
```

Step 1 is the first connection.
The client calls the server with no token and gets 401, with a `WWW-Authenticate` header that says where the server's metadata is.
From there the client finds the authorization server, sends the user to it, and comes back with a token.
Step 2 uses that token.
Step 3 calls a tool that needs more than the user granted at first, so the server answers 403, and the client asks the user for the extra scope and retries.
Step 4 is the attack: the same token, sent to a second MCP server, is refused.

The rest of the page goes through the parts that make each step work.

## The server: say where tokens come from

A protected MCP server publishes [Protected Resource Metadata](https://www.rfc-editor.org/rfc/rfc9728.html) (RFC 9728): a small JSON document naming the server and the authorization servers it trusts.
The specification requires it, and requires clients to use it to find the authorization server.

```go include=mcpauth/server.go tag=server
```

The metadata lives at a well-known address built by inserting `/.well-known/oauth-protected-resource` between the host and the server's path, so the server at `/mcp` publishes it at `/.well-known/oauth-protected-resource/mcp`, and every 401 and 403 names that address in `resource_metadata`.
The handler, the SDK's `ProtectedResourceMetadataHandler`, serves whatever it is given without checking it, so the values come from one function:

```go include=mcpauth/server.go tag=prm
```

Its `resource` must be the URL clients call, byte for byte.
A client that fetches metadata through `resource_metadata` must check that `resource` is identical to the URL it requested, and discard the document if not ([RFC 9728, section 3.3](https://www.rfc-editor.org/rfc/rfc9728.html#section-3.3)).
Without that check, a malicious server could publish metadata claiming to be another server and get the client to fetch a token for the other one.
`TestImpersonatedMetadata` runs that attack: the client refuses the metadata, and the victim's authorization server never hears from it.

`scopes_supported` lists only `issues:read`.
The specification asks for the smallest set a client needs to start, with more requested only when an operation needs it, so a user who only searches never grants write access.

## The server: verify every token yourself

The SDK's `RequireBearerToken` middleware takes the token out of the `Authorization` header and passes it to a `TokenVerifier`, a function you write.
It checks the scopes and expiry you return, but deciding whether the token is genuine, whose it is, and who it was issued for is entirely the verifier's job.

The tokens here are JWT access tokens as [RFC 9068](https://www.rfc-editor.org/rfc/rfc9068.html) profiles them, signed with ES256 by the authorization server.
The verifier holds what it must check against:

```go include=mcpauth/verify.go tag=verifier
```

`Resource` is the server's own canonical URI, the audience a token must name.
`Issuer` is the authorization server's issuer identifier, the one value an `iss` claim may have.
The signature is checked with [go-jose](https://github.com/go-jose/go-jose), a maintained JOSE library; the claim checks around it are written out, because those are the rules this specification adds and the ones a library won't apply unless you ask:

```go include=mcpauth/verify.go tag=verify
```

In order:

1. **A verifier with no issuer or resource refuses to run.** go-jose's `Expected` skips any check left empty: an empty `Issuer` accepts every issuer, an empty audience accepts every audience. A configuration mistake would turn verification off with no error, so the verifier fails with a server error instead.
2. **The algorithm is pinned to ES256.** `ParseSigned` takes the list of algorithms to accept, which rejects a token whose header says `"alg": "none"` and a token signed with HS256 using the server's public key as the secret, the algorithm-confusion attack against libraries that let the token choose how it is verified.
3. **The token must be an access token.** RFC 9068 gives access tokens the type `at+jwt` so a resource server can tell them from OpenID Connect ID tokens, which are JWTs from the same issuer with the same keys. Without the check, an ID token issued to some other client, for login, would be accepted as an access token.
4. **The key comes from the key ID**, fetched from the authorization server's key set; the next section covers rotation.
5. **Expiry and subject are required.** go-jose checks `exp` only when it is present, and RFC 9068 requires it, so a token with no expiry, which would be valid forever, is refused here.
6. **Issuer and audience must match**, with a small leeway for clock skew between servers. The audience check is the one rule from the start of the page.
7. **The user becomes `TokenInfo.UserID`.** The SDK uses it to tie an MCP session to the user who opened it, so a token from another user can't take over a session whose ID it has learned.

Every refusal wraps `auth.ErrInvalidToken`, which the SDK answers with 401, and its message names the reason without echoing the token.
A failure that isn't the token's fault, such as the key set being unreachable, is returned unwrapped and becomes a 500, so a client doesn't throw away a good token because a server had trouble.

`TestVerifyRefuses` sends each of those tokens and checks the reason it was refused:

```go include=mcpauth/verify_test.go tag=refuse-test
```

### Keys that rotate

An authorization server rotates its signing key from time to time, publishing the new one in its key set under a new key ID.
The verifier caches the key set and fetches it again when it sees a key ID it doesn't know:

```go include=mcpauth/verify.go tag=key
```

The fast path, a known key, takes a read lock only, so verifications don't queue behind each other.
A new key ID takes the fetch lock, checks the cache again in case another request fetched while it waited, and fetches.
Fetches are spaced at least 30 seconds apart, because anyone can send a token with a made-up key ID: without the limit, a stream of such tokens would become a stream of requests to the authorization server.
`TestKeyRotation` checks both sides: a token signed with a rotated key verifies after one fetch, and fifty tokens with made-up key IDs add no fetches at all.

### Why verify locally

A JWT can be checked with a public key the server already holds, so verifying costs no network call per request.
The price is revocation: a signed token stays valid until it expires, even if the user revokes access a second after it was issued.
The demo's tokens live five minutes, short enough that a revoked user loses access quickly, and the client gets a new token when the old one runs out.
The alternative is token introspection ([RFC 7662](https://www.rfc-editor.org/rfc/rfc7662.html)), where the server asks the authorization server about each token: revocation takes effect at once, at the cost of a call per request, usually cached for a short time.
Some identity providers issue opaque tokens that only introspection can read.
For an MCP server whose tools change things, short-lived JWTs plus a way to revoke the user's refresh tokens is the common choice; when a revocation must take effect before the token expires, introspect.

## Scopes per tool, and a challenge the client can use

`RequireBearerToken` checks one set of scopes for the whole endpoint.
An MCP server has one endpoint, so with the SDK alone every tool needs the same grant: a client that only wants to search must still ask for permission to file issues.
Per-tool scopes need a check that knows which tool a request calls, so it goes inside the SDK's middleware, after the token is verified:

```go include=mcpauth/server.go tag=scopes
```

```go include=mcpauth/scopes.go tag=tool-scopes
```

The middleware reads the JSON-RPC request, and for a `tools/call` it checks the token's scopes against the tool's.
A tool missing from the map is refused, so a tool added later without a scope can't be called by everyone by default.

The second thing it adds is the response.
The MCP specification says a server refusing a call for lack of scope should answer 403 with `error="insufficient_scope"`, every scope the operation needs, and the metadata address.
That is what lets a client step up: ask the user for the extra scope, get a new token, and retry.
The SDK's own middleware answers 403 without `error="insufficient_scope"`, and the SDK's client only steps up when it sees it.
`TestStepUp` shows the result.
A server starts by requiring `issues:read` and then, as after a deploy, `issues:write` as well; with the SDK's middleware alone the client authorizes once and then fails, and with `ToolScopes` it authorizes a second time and the call succeeds:

```go include=mcpauth/flow_test.go tag=step-up-test
```

When it steps up, the SDK's client asks for the union of the scopes it already had and the ones challenged, so the new token can still do what the old one could.
The scopes here are flat; a server whose scopes form a hierarchy, where a broader scope implies narrower ones, has to account for that when it decides a token is enough, which the specification requires.

The tool handlers themselves never see a token: by the time a call reaches them, the middleware has checked who sent it and what they may do.
That keeps authorization out of the code the model's arguments flow into, and out of the model's reach entirely: the model asks for a tool, and the harness and the server decide whether the user it acts for may use it.

## The client: what the SDK does and what it leaves to you

The client's side is mostly the SDK's `AuthorizationCodeHandler`, attached to the transport.
On a 401, or a 403 with `insufficient_scope`, the transport calls it, and it:

- parses `WWW-Authenticate` and fetches the Protected Resource Metadata, trying the address in the header first, then the path-specific well-known address, then the root, and checks that `resource` matches the URL it called;
- finds the authorization server's metadata, trying the RFC 8414 address before OpenID Connect discovery, and checks that its `issuer` matches the URL it was fetched from;
- refuses an authorization server whose metadata doesn't list `code_challenge_methods_supported`, since the specification forbids proceeding without PKCE;
- chooses how to identify the client, in the specification's order: a pre-registered client, then a Client ID Metadata Document, then Dynamic Client Registration;
- builds the authorization URL with an S256 PKCE challenge, a random `state`, the scopes, and `resource`;
- checks `state` and `iss` on the response, and redeems the code with the verifier and `resource` again.

Configuring it takes the client's identity, its redirect address, and one function:

```go include=mcpauth/client.go tag=client-config
```

The function is the part only an application can do: getting the user to the authorization URL and the code back.
For a program on the user's machine, that means listening on a loopback address and opening the browser:

```go include=mcpauth/client.go tag=loopback
```

It takes the first redirect only, and it doesn't pass on an error's description.
The specification forbids a client from acting on or displaying an error response before checking which authorization server sent it, and the issuer check happens after this function returns.

### Client ID Metadata Documents

A client and an MCP server usually have no prior relationship, so the client needs a way to identify itself to an authorization server it has never seen.
The specification used to point to Dynamic Client Registration, where each client registers itself with each authorization server and gets back an ID; it is now deprecated, kept for servers that support nothing newer.
Its replacement, a Client ID Metadata Document, makes the client's ID an HTTPS URL on the client's own site, pointing to a JSON document that describes it:

```go include=mcpauth/client.go tag=client-metadata
```

When an authorization request arrives with a URL as its `client_id`, the authorization server fetches the document, checks that its `client_id` is that URL exactly, and checks the request's redirect address against `redirect_uris`.
Nothing is registered in advance, the same ID works with every authorization server, and the client's site vouches for it, which is visible in the demo's log as the authorization server's fetch of `/client.json` during each authorization.

One weakness remains, and the specification says so: a document can list a loopback redirect address, and nothing stops another program on the user's machine from listening there.
An authorization server should show the redirect address's host on its consent page and warn when it is loopback-only.

### Two attacks the client refuses

A mix-up attack tricks a client that talks to two authorization servers into sending a code from the honest one to the attacker's.
The defense, [RFC 9207](https://www.rfc-editor.org/rfc/rfc9207.html), has the authorization server put its issuer in the redirect, and the client compare it with the issuer it recorded before sending the user there.
`TestMixUp` changes the `iss` in the response, and the SDK refuses to redeem the code.

`TestImpersonatedMetadata` is the second, described above: metadata that names another server is discarded before any authorization starts.
Its log shows what the SDK does next: after discarding the metadata, it falls back to an older version of the specification and looks for an authorization server at the MCP server's own address.
That fallback can only produce a token for the server the client is calling, so the victim's tokens stay safe, but it means a client keeps trying after the metadata check fails.

## Never pass a token on

Step 4 of the demo is the attack the audience rule exists for: the token the client got for the first MCP server, sent to the second.

```go include=mcpauth/demo.go tag=replay
```

The second server trusts the same authorization server, and the token's signature, issuer, and expiry are all valid.
Its audience is the first server, so the second refuses it: `401 invalid token: issued for another resource`.

The specification makes this a rule for servers as well as a check: an MCP server must not accept tokens meant for others, and must not pass on the token it received.
An MCP server that calls an upstream API, a ticketing system behind an issue tool, say, is a client of that API, and it uses a token issued for that API, obtained through its own grant or by [token exchange](/glossary/#token-exchange), as [Agent identity](/production/agent-identity/) describes.

## Why this way and not another

- **API keys in an environment variable.** That is what the specification says for stdio, and it is right for a server on the user's own machine. For a remote server with many users, a static key identifies no user, carries no scopes, never expires, and can't be revoked for one person without breaking everyone.
- **One token for all of an agent's servers.** Simpler for the client, and it hands every server the power to act at every other one. Audience binding costs one authorization per server and contains each compromise to one server.
- **Scopes for the whole endpoint.** What the SDK supports alone. It makes every client ask for everything up front, and it means a read-only user's token can file issues.
- **Letting the model decide.** The model chooses tools; it doesn't hold tokens or decide what a user may do. Authorization happens in middleware, before a tool handler runs, on a token the model never sees, so no prompt can talk its way past it.
- **Writing the client flow by hand.** Possible, and instructive, but the SDK already does discovery, PKCE, the issuer check, and the scope union, and tracks the specification as it changes. Read what it checks, as this page does, and test the parts it leaves to you.
- **Dynamic Client Registration.** Deprecated in the current specification. A Client ID Metadata Document needs no registration endpoint and works with every authorization server.

## With a real identity provider

The example's authorization server is a test double that approves every request.
In production the authorization server is your identity provider, and the server's configuration becomes its values: `Issuer` is the provider's issuer identifier, and `JWKSURL` comes from its metadata.
Before trusting the verifier with a provider, check what it puts in a token, by decoding one:

- **The audience.** Some providers honor the `resource` parameter and write it into `aud`; others ignore it and use an audience configured per API. Either way, the MCP server must find its own canonical URI there, or the audience check will refuse every token.
- **The type.** Not every provider sets `typ` to `at+jwt`. If yours doesn't, keep the protection the check gives by another means, such as a dedicated audience no ID token will carry, and change the check rather than delete it.
- **The signing algorithm.** Pin the one the provider uses; RS256 is common.
- **Opaque tokens.** If the provider's access tokens aren't JWTs, use its introspection endpoint instead of local verification.
- **Support for the MCP flow.** The client needs the provider to publish RFC 8414 or OpenID Connect metadata with `code_challenge_methods_supported`, and, for Client ID Metadata Documents, `client_id_metadata_document_supported`. Without it, register the client in advance.

The code here was tested against its own authorization server only.
Run the same tests against your provider's test tenant before relying on it.

## Mistakes to avoid

- **Leaving an expected value empty.** JWT libraries commonly skip checks with no expected value. Refuse to verify with no issuer or audience configured.
- **Letting the token choose its algorithm.** Pin the algorithms you accept.
- **Accepting any JWT from the issuer.** ID tokens are JWTs from the same issuer. Check `typ`, and the audience.
- **Comparing the audience loosely.** Use the exact canonical URI. The SDK's `oauthex.MatchesResource` tolerates a trailing slash and nothing else, which is as loose as it should get.
- **Fetching keys on every unknown key ID.** Space the fetches, or anyone can make your server hammer the authorization server.
- **Logging the token.** Tokens in logs are tokens anyone with the logs can use. Log the subject, client, and token ID instead.
- **Forwarding the token upstream.** Get a token for the upstream API.
- **Asking for every scope at the first connection.** Start with the least, and step up.

## Try it

From the `code/mcpauth` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go run .
$ go test -race ./...
```

1. In `verify.go`, delete the check on `typ` and run `go test -run TestVerifyRefuses`. The "an ID token" case fails: the verifier now accepts a token issued for login.
2. In `server.go`, remove the line that wraps the handler in `ToolScopes`, and run `go test -run TestDemo`. Step 3 no longer steps up, because the read-only token can now file an issue.
3. Set `minRefetch` to zero and run `go test -run TestKeyRotation`: every made-up key ID now costs a request to the authorization server.

Undo your edits with `git restore .` in the `code` directory.
The specification is still changing; the version this page follows is [2026-07-28](https://modelcontextprotocol.io/specification/2026-07-28/basic/authorization).
