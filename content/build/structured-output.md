---
title: "Structured output: an agent's answer in a fixed shape"
short: Structured output
description: If a program reads an agent's answer, prose is a poor format, since a parser breaks the first time the wording changes. A final tool, whose arguments are the answer, gives you a typed result from any tool-calling model and a place to check the answer before you accept it.
card: Get a typed, checked answer from any tool-calling model by making the answer a final tool.
group: Tools and output
chapters: 7
stage: 4
date: 2026-10-18
weight: 40
art: ch07
---

A chatbot's last message is written for a human.
The last step of many agents is written for a program: the crash-triage agent has to say which reports share a cause, where the bug lives, which test proves it, and which tracker issue it belongs to.
Code that scrapes those facts out of prose works until the model words things differently, which it will.
The answer needs a shape the program can decode.

Below, the triage agent gets such a shape.
Here are the two available approaches, the one used by default in the companion code, and the part that counts for more than the format, which is verifying what the answer claims.

## Two ways to get structure

**A constrained reply.** [Structured outputs](/glossary/#structured-outputs) is a request option, `response_format`, that tells the API to make the reply conform to a JSON Schema.
Support varies by model and by provider.
On OpenRouter, leaving out `require_parameters` allows routing to a provider that doesn't honor the option.
Many providers also won't apply it in a turn that offers tools.
Reserve it for single calls that use no tools, for example condensing the early turns of a long conversation.

**A final tool.** Give the agent an extra tool, `submit_diagnosis` in this case, and make its arguments the answer.
Nothing about it is special: the schema comes from the same generator as every other tool's, the arguments go through the same decoder and checks, and a rejected call produces an ordinary tool error.
Any model that supports tool calling can use it.
The companion code takes this route by default.

## The answer as a Go type

The answer is declared as a Go struct, tagged the same way as the argument structs on the [tool design](/build/tool-design/) page, so its schema is generated:

```go include=ch07/triage/diagnosis.go tag=diagnosis
```

Fields that code will switch on, the status above all, are enumerations.
Each field's description explains it to the model, and the descriptions spell out the easy-to-miss points, such as a root cause that "may not be where it crashed".
A field the model has nothing for is meant to be left out of the JSON rather than sent as `null`.
There is one `Diagnosis` for each crash group, and the crash catalog's grader reads the same record.

## How the loop ends on it

Marking a tool `Final` changes the loop's stopping rule.
When a call to that tool succeeds, the run is over and the call's arguments are the run's result:

```go include=ch07/agent/final.go tag=final-finished
```

If the call is rejected, the model receives the error as a tool result and keeps going.
That gives the final tool the power to turn an answer down and lets the model correct it.
Once a final call succeeds, nothing else in that reply runs: later calls in the reply get "not run" as their result, so the answer matches the code exactly as it stood at the moment of acceptance.

A model that writes its answer as plain text instead of calling the tool gets a nudge: a user message telling it to call the tool once it is finished, or to carry on with the work.
If two nudges don't help, the run ends with `ErrNoFinalAnswer`, a sign that the program never received an answer it could use; callers test for it with [`errors.Is`](/basics/errors/).

## Checking what the answer claims

The schema makes sure the answer has the right fields and types.
Whether the content is correct is a separate matter, and a model sounds just as sure when it is wrong.
`submit_diagnosis` therefore verifies everything it can before it says yes:

- Each report belongs to one group and only one.
- A group marked duplicate points at an issue that is open, and a group marked fixed points at an issue that exists.
- For a fixed group, the reproducer test is run twice. Against the untouched original plus the tests the agent wrote, it has to fail; against the agent's changed code it has to pass.

The reproducer check is the one that tests the substance of the claim.
If the test already passes before the fix, it says nothing about the crash, no matter how convincing the fix is.
A test name that doesn't match any test gets a dedicated message, because that usually means a typo.
Every failed check comes back to the model as an error explaining the problem, and the run continues.

There are limits to what this can catch.
A test can fail for an unrelated reason, a fix can land in the wrong place, and a flaky test might get through by chance.
The crash catalog's grader narrows that gap with hidden tests the agent never sees.

```diagram
name: submit
alt: A decision flow: do the arguments decode against the generated schema; is every report in exactly one group, with real issues and duplicates; does each fixed group's test fail on the original code with the agent's tests added; does it pass on the agent's code. Every yes leads on, and the last yes is accepted, ending the run with Result.Final holding the answer. Every no becomes an error the model reads, and the run goes on.

What happens to a call to `submit_diagnosis` before its answer is accepted.
```

## One run, end to end

`go run ./ch07` replays a recorded run of `anthropic/claude-haiku-5.5` on one catalog task.
The task has four crash reports: two are crashes already in the tracker, and two come from a bug located away from the line that crashed.

The agent got the difficult part right.
The crash is a nil pointer dereference inside `api.owned`, but the defect is in `store.Get`: for a deleted reminder it returns neither a reminder nor an error.
Guarding against nil in `owned` would have ended the panic while leaving every other caller of `Get` open to the same bug.

Because the answer was typed, its error was easy to see too.
The catalog's grader checked it against the expected answer 16 ways, and 12 passed.
All four failures came from one mistake: the agent marked the two crashes already in the tracker as `fixed` when they should have been duplicates of the existing issues.
The first system prompt gave no instruction for a crash that matches a filed issue, and revising the prompt begins with that.
In a prose answer the mistake would have been one clause in a paragraph; in a struct it shows up as failed checks.
The run used 17 turns and cost $0.034.

## When to use which

```fit
reach: A final tool, when the agent already calls tools or has to run on many models. It works on any model that can call tools, and because the recorded run's answer was typed, the catalog's grader showed its one mistake as four failed checks out of 16.
reach: A constrained reply, for a single call with no tools. The companion code's summary call sends a JSON Schema as `response_format` with `require_parameters` set, so OpenRouter routes only to providers that apply it.
leave: Trusting the shape alone. A schema can't tell a passing reproducer from a failing one; `submit_diagnosis` runs the test on the original code and on the fix before it accepts the answer.
```

Whichever you choose, validate in your own code as well: decode strictly, enforce enumerations and required fields, and test every claim in the answer that a program is able to test.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), with no key set:

```console
$ go run ./ch07
```

This replays the run described above against a fresh copy of the service; every tool really executes, the reproducer check's `go test` runs included, and the grader's checks print at the end.
The final tool and its checks are in [`ch07/triage/diagnosis.go`](code:ch07/triage/diagnosis.go).
