---
title: "OAuth and OpenID Connect for an agent service in Go: sign-in, sessions, and each user's own credentials"
short: OAuth and OIDC
description: How an agent service that people use from a browser signs them in with OpenID Connect and acts for them at other services, in tested Go. Authorization code with PKCE, state, and nonce; ID token checks go-oidc leaves to you; tokens kept server-side behind a session cookie (RFC 10017); each user's refresh tokens encrypted and bound to them; one refresh at a time under rotation (RFC 9700); revocation on disconnect (RFC 7009); and the settings that matter with Okta.
card: Sign people in, keep their tokens off the browser, and call other services with each user's own credentials.
group: Safety
stage: 3
level: advanced
date: 2026-11-03
weight: 72
beyond: true
art: oauth-and-oidc
---

An agent that people use from a browser has three jobs that OAuth and OpenID Connect split between them.
It has to know who is asking, which is signing in.
It has to keep knowing it on every request after, which is the session.
And it has to act at other services for that person, reading their calendar or filing issues in their tracker, with their rights and nobody else's, which is their connected accounts.

[OpenID Connect](/glossary/#openid-connect) handles the first; OAuth handles the third.
Each job has its own token, meant for a different party, and most mistakes in this area come from using one where another belongs.
This page builds the service in Go and runs it offline, against an identity provider and a calendar service, with tests for each check.
[MCP authorization](/production/mcp-authorization/) covers the other end: the server that receives a token and decides whether to accept it.

## Four credentials, four audiences

- **The [ID token](/glossary/#id-token)** is for the agent service. Its audience is the service's client ID, and it says who signed in, once. It is never sent to an API.
- **An access token** is for one API. The calendar's is issued for the calendar, by the calendar's authorization server, and the agent's tools send it there and nowhere else.
- **A [refresh token](/glossary/#refresh-token)** is for the authorization server that issued it. It renews access tokens while the user is away, for as long as the connection lasts, which makes it the credential most worth stealing.
- **The session cookie** is for the browser. It holds a random ID and nothing else, and the service looks everything up from it.

The browser holds only the last.
The other three stay on the server.

## The whole thing, in one run

The demo starts an identity provider that people sign in with, a calendar API with its own authorization server, and the agent service, all on loopback addresses, with stand-ins for two people's browsers.
It prints every request and never a token:

```text include=mcpauth/cmd/agentweb/testdata/agentweb.txt title="go run ./cmd/agentweb (in code/mcpauth)"
```

1. Alice signs in. Her browser goes to the identity provider and back, the agent service redeems the code itself, and the browser ends with a session cookie.
2. She asks for her next meeting. The agent's tool needs her calendar, which she hasn't connected.
3. She connects it. This is a second authorization, at the calendar's own authorization server, which asks for `offline_access` and so returns a refresh token. The service stores it encrypted.
4. She asks again, and the tool calls the calendar with her access token.
5. Ten minutes later her access token has expired, and two questions arrive at once. Both tool calls need a fresh token, and the service sends one refresh for both.
6. Bob signs in and asks. His agent can't reach alice's calendar, and it can't reach his own until he connects it.
7. A copy of alice's first refresh token, the one rotated away in step 5, turns up in the wrong hands. Without the service's client secret it is refused outright. With the secret, the calendar's authorization server sees a used token presented again and revokes everything descended from it, so alice's next question asks her to connect again.
8. Alice signs out, and a copy of her old session cookie gets nothing.

## Keep the tokens on the server

The service is a [backend for frontend](/glossary/#backend-for-frontend), the pattern [RFC 10017](https://www.rfc-editor.org/rfc/rfc10017.html) (OAuth 2.0 for Browser-Based Applications, August 2026) recommends first, in section 6.1.
The OAuth client is the server, a confidential client with a secret of its own, and every token lives there.
The browser gets an `HttpOnly` cookie that script can't read, and a page with a script injected into it can't take a token away, because the page never had one.
Such a script can still send requests as the user while the page is open, which no token storage prevents.
Keeping tokens off the browser takes away the part that outlives the page: a stolen refresh token works for weeks from anywhere.

```go include=mcpauth/agentweb/service.go tag=config
```

## Start the sign-in

```go include=mcpauth/agentweb/service.go tag=login
```

Three values go out with the browser, and each one stops a different attack:

- **`state`** ties the answer to this browser. Without it, an attacker can finish their own sign-in in your browser, by getting it to open a callback URL carrying the attacker's code, and you are using the agent as them.
- **`nonce`** ties the ID token to this sign-in. The identity provider copies it into the ID token, so an ID token from another sign-in can't be passed off as this one's.
- **The [PKCE](/glossary/#pkce) verifier** ([RFC 7636](https://www.rfc-editor.org/rfc/rfc7636.html)) ties the code to this server. Only its hash goes out, and the code is worth nothing without the verifier. [RFC 9700](https://www.rfc-editor.org/rfc/rfc9700.html), the OAuth security best current practice, recommends it for confidential clients too, since a client secret doesn't stop a code stolen on its way back from being redeemed by the client itself, in the attacker's session.

All three are kept server-side, under a random ID in a short-lived cookie:

```go include=mcpauth/agentweb/service.go tag=recall
```

The login cookie is `SameSite=Lax`, because it must come back on the identity provider's redirect, a navigation another site starts.

## Finish it

```go include=mcpauth/agentweb/service.go tag=callback
```

```diagram
name: agent-sign-in
alt: A sequence of three participants: the browser, the agent service, and the identity provider. The browser asks the agent service for /login and gets a login cookie and a redirect carrying state, nonce, and a PKCE challenge. The browser goes to the identity provider, where the user signs in, and comes back with a code, the state, and iss. The browser sends the callback with the login cookie; the agent service checks the state and iss, redeems the code at the identity provider with the verifier and its client secret, gets an ID token and an access token, checks the ID token and the nonce, and gives the browser a session cookie and nothing else.
tall: yes

Signing in, as in step 1 of the log. The code goes through the browser; the tokens never do.
```

1. **The answer must belong to this browser's sign-in**: the login cookie must name a sign-in in progress, the state must match, and the answer must come from the identity provider it was sent to. An `iss` in the answer must name that provider ([RFC 9207](https://www.rfc-editor.org/rfc/rfc9207.html)), and when the provider's metadata says it always sends one, an answer without one is refused. The sign-in is removed as it is read, so an answer works once.
2. **The code is redeemed with the verifier**, and with the client secret, in the request's `Authorization` header.
3. **The ID token is checked** by [go-oidc](https://github.com/coreos/go-oidc), the maintained OpenID Connect library for Go: its signature, with the algorithm pinned to ES256, its issuer, that its audience includes this client, and its expiry.
4. **The nonce is checked here**, because go-oidc doesn't: its verifier leaves nonce to the caller, and so does `azp`, the authorized party, which the next line checks when it is present.
5. **A new session starts**, and everything else from the sign-in is dropped. The service keeps who signed in and doesn't keep the tokens: it has no API to call with them.

The callback answers with a page rather than a redirect.
The session cookie is `SameSite=Strict`, which browsers don't send on a redirect that began at another site, so a redirect from the callback to `/` would arrive without it.
A page that sends the browser on is a navigation this site starts, and the cookie goes with it.

`TestSignInRefuses` sends six answers that must fail: one for a sign-in another browser started, a changed state, an answer with no `iss` from a server that always sends one, an answer naming another issuer, the attacker's code with the victim's state (PKCE stops it at the token endpoint), and a nonce changed on its way to the identity provider.

```go include=mcpauth/agentweb/agentweb_test.go tag=signin-test
```

## What the ID token is for

Its subject, `sub`, is who the user is to this identity provider.
Key everything on the issuer and the subject together, never on the email: addresses change, get reassigned, and at some providers can be set by the user.
The demo's subjects happen to be addresses, which keeps the log readable.

The ID token is not an access token.
Its audience is the agent service, and an API that accepted it would accept a token issued for someone else.
The identity provider here types it `JWT` and its access tokens `at+jwt`, and the [resource server's verifier](/production/mcp-authorization/#the-server-verify-every-token-yourself) refuses anything not typed as an access token.

## The session

```go include=mcpauth/agentweb/service.go tag=session
```

The cookie's name starts with `__Host-Http-`, the prefix RFC 10017 asks for: `__Host-` makes browsers refuse it unless it is `Secure`, has `Path=/`, and names no domain, so a subdomain can't set it, and `Http-` requires `HttpOnly`, so no script can set it or read it.
Every sign-in gets a new session ID, so an ID planted in the browser before sign-in isn't the one that becomes signed in.
A session ends after 30 minutes idle or 12 hours in all, and signing out deletes it on the server, so a copy of the cookie stops working too, which is the last line of step 8.

A POST must carry an `Origin` header naming the service.
Browsers send one on every POST, and a page on another site can't make it say this site, so a form on that site can't make a signed-in user's browser ask the agent to do something.
With `SameSite=Strict` as well, the cookie isn't sent on such a request in the first place.

## Connect the user's accounts

The calendar is a separate service with its own authorization server, where the agent service is registered as a client of its own.
Connecting is a second authorization code flow, started by a signed-in user:

```go include=mcpauth/agentweb/connect.go tag=connect
```

It asks for the scopes the tools need and `offline_access`, which is what gets a refresh token: the agent works for the user when they aren't there to sign in again.
It names the API with `resource` ([RFC 8707](https://www.rfc-editor.org/rfc/rfc8707.html)), so the access token is issued for the calendar API and no other.
The callback checks the answer exactly as the sign-in's does, against the calendar's issuer this time, and checks that the user signed in now is the one who started connecting.

## Store each credential for one user

```go include=mcpauth/agentweb/vault.go tag=vault
```

1. **Each row is sealed with AES-256-GCM, with its user and provider as additional data.** The ciphertext opens only under the same user and provider, so a row copied to bob's key, by a bug in a query or by someone with write access to the table, fails to open instead of giving bob alice's calendar.
2. **Opening checks the same binding**, and any change to the row fails it.

The key comes from a key management service, never from the database the rows live in, so a copy of the table alone is ciphertext.
In production the rows also carry a key ID, so the key can be rotated without rewriting every row at once.

```go include=mcpauth/agentweb/agentweb_test.go tag=vault-test
```

## Refresh once

The calendar's authorization server rotates refresh tokens: each works once, and the answer carries the next.
Presenting a used one means two parties hold copies, the client and a thief, and the server can't tell which is which, so it revokes the whole family, as RFC 9700 describes in section 4.14.2:

```go include=mcpauth/authserver/oidc_test.go tag=rotation-test
```

Rotation is good against theft and hard on concurrency.
An agent runs tools in parallel, and when the access token expires, every tool call that needs it finds it expired at once.
If each refreshes, all but the first present a used refresh token, and the provider revokes the connection: the service locks itself out with no attacker involved.

```go include=mcpauth/agentweb/connect.go tag=token-for
```

1. **Calls for one user and provider share a refresh**, through `singleflight`, and the refresh runs on a context that ignores the caller's cancellation. Once the provider has rotated the token, the new one must be stored, or the service holds only a used token and the next refresh revokes the connection. A caller giving up doesn't get to stop that halfway.
2. **The refresh token is redeemed** with the client secret. The service computes the new expiry with its own clock from `expires_in`.
3. **A refused refresh token means the connection is gone**, revoked by the user at the provider or by reuse detection. The service forgets it, and the tool tells the model the user must connect again, which is how step 7 ends.
4. **The new token is stored before it is used.**

`singleflight` coordinates calls in one process.
Several replicas of the service need the same guarantee across processes, a lock on the user and provider's row, for example, held while refreshing.
Some providers allow a grace period in which the previous refresh token still works, Okta's for one, which softens the race; it is not a substitute for not having it.

```go include=mcpauth/agentweb/agentweb_test.go tag=refresh-test
```

Step 7 shows the other reason the service is a confidential client: a refresh token alone is useless.
RFC 9700 binds a confidential client's refresh tokens to its client authentication, so a token copied out of a backup needs the client secret too, and the secret lives somewhere the backup doesn't.

## The model never sees a token

```go include=mcpauth/agentweb/connect.go tag=ask
```

The model picks a tool and its arguments.
Who the tool runs as comes from the session, and a tool gets its credential from `TokenFor`, by that user's name:

```go include=mcpauth/agentweb/tools.go tag=tool
```

No token passes through the model's context, so no prompt can get one printed, and an argument naming another user changes nothing: `TestToolRunsAsSessionUser` has the model ask for alice's calendar while bob is signed in, and the answer is that bob hasn't connected one.

## Disconnecting revokes

```go include=mcpauth/agentweb/connect.go tag=disconnect
```

Deleting the row isn't enough: every backup of the table holds the refresh token, and to the provider it is still good.
Revoking it at the provider's revocation endpoint ([RFC 7009](https://www.rfc-editor.org/rfc/rfc7009.html)) ends it, and the family descended from it, everywhere at once.
Access tokens already issued are JWTs that the provider doesn't track, so they run until they expire, five minutes here.

## With Okta

The example's identity provider and the calendar's authorization server are a test double.
With Okta as the identity provider, the configuration becomes Okta's values, and a few of its defaults matter:

- **The issuer.** A custom authorization server's issuer is `https://{yourOktaDomain}/oauth2/{authorizationServerId}`, `https://{yourOktaDomain}/oauth2/default` for the default one, with its discovery document at `/.well-known/openid-configuration` under that path. go-oidc compares the issuer in the document with the one you configured exactly.
- **Which authorization server.** For signing in, either works. Access tokens your own APIs will validate must come from a custom authorization server: Okta's documentation says access tokens from the org authorization server [aren't intended for validation by your own apps or resource servers](https://developer.okta.com/docs/concepts/auth-servers/).
- **The client.** Register the agent service as a web application, a confidential client, and turn on PKCE for it as well.
- **Refresh tokens.** Ask for `offline_access` and enable the refresh token grant for the app. Rotation is on by default only for single-page apps; [web apps get persistent refresh tokens](https://developer.okta.com/docs/guides/refresh-tokens/main/) unless you turn rotation on, with a grace period of 0 to 60 seconds, 30 by default. Reusing a rotated token past the grace period invalidates the newest one and the access tokens issued since.
- **The `iss` parameter.** Read `authorization_response_iss_parameter_supported` in the metadata; the service requires `iss` only when the provider says it sends one.
- **The user's key.** Okta's `sub` is a stable user ID; key on it with the issuer, not on the email.

The code here was tested against its own authorization servers only.
Run the same tests against a test tenant before relying on it.

## Why this way and not another

- **Tokens in the browser.** A single-page app as a public OAuth client is allowed, with a token store any injected script can read, and RFC 10017 describes the backend for frontend first, as the architecture that keeps tokens out of the browser altogether. An agent's tokens reach the user's other accounts, which raises the stakes.
- **Using the sign-in's access token for the calendar.** It is issued for the identity provider, and the calendar isn't. Each API gets a token issued for it.
- **Stateless sessions in a signed cookie.** No lookup per request, and no way to end one: signing out can't stop a copy of the cookie, and neither can an administrator.
- **Encryption without the binding.** It protects against someone reading the table, and not against a row put under the wrong user, which is the mistake the binding catches.
- **Refreshing wherever a token runs out.** Fine without rotation, and with it, a parallel agent revokes its own connections.
- **A key per user pasted into settings.** No expiry, no scopes, no revocation at the provider, and the user's whole account.

## Mistakes to avoid

- **Skipping the nonce check.** go-oidc's verifier doesn't do it.
- **Keying users by email.** Use the issuer and `sub`.
- **Letting a refresh be cancelled halfway.** Finish it and store the result.
- **Refreshing in parallel with rotating tokens.** Share one refresh per user and provider.
- **Deleting a credential without revoking it.**
- **Sending an ID token to an API.**
- **Ending a session only in the browser.** Delete it on the server.

## Try it

From the `code/mcpauth` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go run ./cmd/agentweb
$ go test -race ./...
```

1. In `agentweb/connect.go`, have `TokenFor` call `s.current` directly instead of through `s.flight.Do`, and run `go test ./agentweb -run TestOneRefreshAtATime`. The twenty tool calls refresh twenty times, and the provider revokes the connection.
2. Remove `context.WithoutCancel` from `TokenFor` and run `go test ./agentweb -run TestRefreshSurvivesCancel`. A caller that has given up now stops the refresh.
3. In `agentweb/service.go`, delete the nonce comparison and run `go test ./agentweb -run TestSignInRefuses`. The sign-in with the changed nonce goes through.
4. In `agentweb/vault.go`, pass `nil` for the additional data to both `Seal` and `Open`, and run `go test ./agentweb -run TestVault`. Alice's row, copied under bob, opens as bob's.

Undo your edits with `git restore .` in the `code` directory.
