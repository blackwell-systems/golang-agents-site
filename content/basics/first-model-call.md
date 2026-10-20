---
title: "Your first LLM API call: a key, one request, and the reply"
short: Your first model call
description: Before an agent, one call. Create an OpenRouter account and API key, send a single request with curl, and read every part of the reply that comes back, from the answer itself to what it cost.
card: An API key, one request with curl, and every part of the reply, including what it cost.
group: Talk to a model
chapters: 2
stage: 2
date: 2026-10-19
weight: 25
---

An agent makes many calls to a model, but each one is an ordinary HTTP request: some JSON goes out, some JSON comes back.
Making one by hand first, before any Go, shows exactly what your program will be sending and reading.

## Why OpenRouter

The examples on this site call models through [OpenRouter](https://openrouter.ai), a service that puts hundreds of models from different companies behind one API.
Its API uses the chat completions format that OpenAI introduced and most providers and local model servers now accept, so the code you write for it works elsewhere with a different address and key.
Switching models is a change to one string, which makes it easy to compare them.

Calling a provider directly, such as Anthropic or OpenAI, works the same way in principle; [Providers and SDKs](/ecosystem/providers-and-sdks/) covers what changes.

```diagram
name: route
alt: Your Go program sends a request to OpenRouter, which forwards it to one of several providers serving the model, trying another if the first fails, and returns the reply with its usage and cost

Your program sends one request to OpenRouter, which passes it to one of the providers that serve the model.
```

## An account and a key

1. Create an account at [openrouter.ai](https://openrouter.ai).
2. Add a few dollars of credit. Credit is prepaid, so when it runs out, requests fail with a 402 error instead of running up a bill. Five dollars covers every early example many times over.
3. Create an API key, the secret string that identifies your account on every request. Give the key its own credit limit, so a bug in your agent can't spend more than you set aside for it.
4. Put the key in your shell's environment:

```console
$ export OPENROUTER_API_KEY=sk-or-...
```

Keep the key out of your code and out of git.
A key that lands in a public repository is often found and used by someone else within minutes.

## One request

Here is a complete request, sent with `curl`.
It asks one question, with one instruction about how to answer:

```console
$ curl https://openrouter.ai/api/v1/chat/completions \
    -H "Authorization: Bearer $OPENROUTER_API_KEY" \
    -H "Content-Type: application/json" \
    -d '{
      "model": "anthropic/claude-haiku-5.5",
      "messages": [
        {"role": "system", "content": "You are a helpful assistant for Go programmers. Answer in a few sentences."},
        {"role": "user", "content": "What is a goroutine?"}
      ]
    }'
```

The body has two parts:

- **`model`** names the model, as the company that made it, a slash, and the model's name.
- **`messages`** is the conversation so far, in order. Each message has a **role** and some **content**. The `system` message holds your instructions; the `user` message holds the question. [Messages, roles, and the system prompt](/basics/messages-and-roles/) covers all four roles.

## The reply

This is the reply to exactly that request, recorded in October 2026 and kept in the companion code:

```json include=ch02/testdata/replies/chat-01.json lines=16..61 title="The response to the request above"
```

Most of it you can ignore at first.
The parts an agent reads:

- **`choices[0].message`** is the model's reply: a message with the role `assistant` and the answer in `content`. To continue the conversation, you append this message to your list and add the next `user` message after it.
- **`finish_reason`** says why the model stopped writing. `stop` means it finished on its own. `length` means it ran out of room and the answer is cut off; an agent has to check this on every reply, as [Common first errors](/basics/common-errors/) shows. Later, `tool_calls` will mean the model wants your program to run tools.
- **`usage`** counts the [tokens](/glossary/#token), the pieces of text the model reads and writes: 43 in, which covers both messages plus some formatting, and 122 out. **`cost`** is what this call cost in dollars: $0.0000653, or about 15,000 calls like it per dollar.
- **`model`** and **`provider`** say which model answered and which company's servers ran it. OpenRouter may send the same model to different hosts on different calls.

## The model remembers nothing

Send the same request again and the model has no idea it answered before.
Each request carries the whole conversation, and the model reads only what's in it.
To ask a follow-up question, your program sends the system message, the first question, the model's answer, and the new question, all together.

That one fact shapes everything an agent does: its requests grow with every turn, and so does what each turn costs.
[Tokens, cost, and context](/basics/tokens-and-cost/) measures how fast.

## The same call from Go

The chat program in the companion code makes exactly this call, and continues the conversation for as long as you type:

```console
$ go run ./ch02
```

With `OPENROUTER_API_KEY` set, it talks to the live model and prints the tokens and cost of each reply.
Without the key, it replays the recorded conversation above, so you can see the output first:

```text include=ch02/testdata/transcript.txt lines=1..9 title="go run ./ch02, with no key: the first question"
```

[Calling a model from Go](/basics/calling-a-model/) builds that client in under two hundred lines of standard-library Go.
