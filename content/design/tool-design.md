---
title: Designing tools a model uses well
short: Designing tools
description: The caller of an agent's tools reads a name, a description, and a schema, and never your code. This page covers task-shaped tools, short results, schemas generated from Go structs, strict argument checks, useful errors, and a cheap test of tool choice.
chapters: 7, 4, 3
stage: 3
date: 2026-10-18
work: true
weight: 10
art: ch07
---

Everything a model knows about one of your tools fits in three fields: what the tool is called, what its description says, and the JSON schema of its arguments.
From those alone it decides whether to call the tool, what to pass, and what the answer means.
It has no source to open and nobody to ask, and it will sometimes pick a tool for a job you never had in mind.
Treat that model as the only user of a small API, and design for it the way you would for any API user who works from the docs.

Chapter 7 of the book builds fourteen tools for its crash-triage agent.
Below are the rules it follows, with code from [`ch07`](code:ch07) and the results its recorded runs produced.

## What the model reads when it chooses

Your tool definitions don't reach the model through a side channel.
The provider renders the messages and the definitions together into a single stream of text, in the format that model was trained on, called its [chat template](/glossary/#chat-template).
In Qwen3's template, which chapter 3 of the book renders in Go, each definition becomes a line of JSON inside the system message, and after the list comes a short passage in English explaining that a call is a JSON object written between `<tool_call>` tags.

So a tool call is text the model writes, and the provider parses that text back into the `tool_calls` field you receive.
Picking a tool is the same act as writing any other reply: given the request, the system prompt, and every name and description in the list, the model samples a likely continuation, and sometimes that continuation is a call.
Unless the provider restricts decoding to the declared tools, as some strict modes do, nothing in that process even guarantees the name is real.
That is why names and descriptions carry so much weight: they are all the model has to tell one tool from another.
When no tool fits what the model wants to do, a name that looks closer can win on a one-line description, which is what the smoke test at the end of this page found.

## Design around tasks

Start from what the agent needs to find out, and shape each tool as an answer to one kind of question.
Asking whether the snooze endpoint ever crashed is a typical question for this agent, so `crash_search` accepts a query and gives back only the lines that match.
A tool that dumped every report would also contain the answer, buried in thousands of tokens the model then carries for the rest of the run.

**Group names by prefix.**
The agent's tools fall into three families: `crash_list`, `crash_read`, `crash_search` for reports, `code_list`, `code_read`, `code_search` for source, and `issue_search`, `issue_file`, `issue_comment` for the tracker.
Once the list reaches fourteen entries, the prefix is what lets the model see the families.

**Write descriptions for a new teammate.**
A good description covers what the tool is for, what comes back, and the case where another tool is the better choice.
The description of `code_read` mentions its 200-line limit and how to request a line range.
The description of `issue_file` tells the model to search before filing: a duplicate issue is the one error this tool has no way to take back.

**Avoid a single tool that takes a verb argument**, like `crash(action: "list" | "read" | "search", ...)`.
A combined tool forces two decisions per call, the tool and then the action, and its schema mixes the arguments of all the actions, most of which are irrelevant to any given call.
Three small tools with three small schemas give the model an easier choice and give your checker less to validate.

**Make mistakes hard.**
The tool resolves every path inside the workspace and refuses anything outside it, so asking for `../../etc/passwd` returns an error.
When a field accepts only a handful of values, declare them as an enumeration in the schema so the model sees the full list.

## Results the model can use

Whatever a tool returns becomes part of the conversation, and the whole conversation goes back to the model on each later turn.
A long result is therefore paid for again and again.

- **Short by default, more on request.** By default `crash_read` prints only the stack frames from the service itself, one per line, and drops the runtime and standard-library frames; passing `full: true` returns the whole trace.
- **Names and line numbers, not opaque IDs.** A location such as `api/reminders.go:144` can go straight into a `code_read` call, with no lookup step in between.
- **A hard cap, and a message when it cuts.** `code_read` stops at 200 lines, then tells the model how many lines remain and what to send to see them. A truncated result that says so leaves nothing to guess.
- **Summarize what is large.** One catalog task is a deadlock whose goroutine dump weighs 85 KB. `crash_goroutines` merges goroutines that share a stack, and the summary comes to 7 KB: 46 goroutines in 8 groups, with the two that matter easy to spot.
- **References instead of contents.** `crash_list` gives one line per report; the model then opens only the reports it cares about.

## Schemas from Go types

A schema written by hand and a struct written by hand will disagree sooner or later.
Chapter 7 avoids that by deriving the schema from the struct.
Each tool declares its arguments as a Go struct, and a `desc` tag carries the text the model will read for each field:

```go include=ch07/triage/codetools.go tag=read-args
```

A field whose `json` tag includes `omitzero` is optional for the model.
`NewTool` wraps a function over such a struct and produces a tool:

```go include=ch07/agent/registry.go tag=new-tool
```

The generic type parameter `A` has to reach the `reflect` package somehow, and `reflect.TypeFor[A]()` (Go 1.22) does that without needing a value, which lets the generator read each field and its tags.
Generation happens a single time, at construction.
If the struct uses something JSON Schema can't describe here, a map or an interface field for example, `NewTool` panics during startup: that is a bug in the program, and it should fail before any model call.
At run time, each call's arguments are validated against the schema first, and only then decoded into the struct that `run` receives; tool code works with typed values and never parses JSON.

Properties come out in the order the fields are declared, and the object gets `additionalProperties: false`.
A tool that takes no arguments still gets empty `properties` and `required`, because in chapter 7's runs OpenAI's API refused the no-argument schema without them.

## Check every call strictly

Decoding with `encoding/json/v2` can't tell you whether a field was sent: an absent field and a field set to its zero value look identical afterward.
So the tool layer runs its own checks before decoding.
It confirms that required fields are present, that enumerated strings hold a listed value, and that no unknown fields were sent, and it explains JSON that fails to parse in terms the model can use.
Each error message points to the offending field with a path like `"groups[0].status"`, then shows the argument shape the tool wants.

The parse-error message exists because of one live run.
Three calls in a row, the default model produced `{"package": "./...", "run": , "race": true}`.
It had intended `null` for an optional field, and the streamed reply lost the `null`.
The decoder at that point answered with "the arguments must be a JSON object".
That was accurate and told the model nothing it could fix, so it sent the identical call again.
Two changes fixed it.
The error now reports where the JSON broke, and the schema lets optional fields be omitted, so `null` never needs to be sent.

## Errors the model can act on

Whatever your error message says, the model reads it and decides its next call from it.
In chapter 4's measurements, three runs per model on its bug-fix task, a tool that failed with "the file is locked" sent three of four models into retries lasting a median of 18 to 20 turns.
Rewording the error to say the change could not be made, and what to do instead, made every model stop after its first try.
[An error that said "locked"](/findings/error-wording/) has the details.
Good tool errors cover three things: the cause, whether a retry has any chance, and the call to make instead.

## Flags that say what a tool does

Some tools only read, some compute a result without leaving anything behind, and some change state outside the run.
Chapter 7 captures this with two fields on `Tool`: [read-only](/glossary/#read-only-tool), and [idempotent](/glossary/#idempotent), which means a second identical call does no extra harm.
Both default to false, and a tool with neither flag set is assumed to have side effects; a new tool starts out treated as risky, and someone has to mark it safe.
Later chapters act on the flags.
Only read-only calls run in parallel, only idempotent tools are retried, and approval gates sit in front of anything with side effects.

## Does the model pick the right tool?

A description that sends the model to the wrong tool passes every test and every build.
Chapter 7 adds a smoke test for it.
It has fifteen short requests, each tagged with the tool that ought to handle it, and sends every request three times to each of four models, using the agent's real system prompt and tool set; the recorded runs are from October 2026.
It then repeats the whole run with thirty extra tools that look plausible but aren't part of the agent.

Out of a possible 45, the models scored between 30.5 and 36.
Adding the extra tools changed scores by one or two points, which three runs can't tell apart from chance.
One answer moved for real.
Asked to comment on issue 101, `z-ai/glm-5.3-flash` and `deepseek/deepseek-v4.1-flash` usually began with `issue_search` to find the issue; when the extras were present, both picked `issue_lookup` (whose description was just "Look up an issue.") on every one of their six tries.
The models wanted a tool that fetches one issue by number, and the agent didn't have one.
Chapter 8 adds `issue_read`.

Not every lost point was a wrong choice.
Some came from the system prompt overriding the request: given a request to file an issue, the models looked in the tracker before filing, which is what the prompt instructs, so the test gives those first steps half a point.
The score counts only the first tool a model asks for, and some requests list an acceptable second choice.
A test this small finds big problems, like a vague description or a pair of overlapping tool names, and is too coarse to show a small improvement.

## When to use it

These are the measured reasons to spend time on tool design, rather than a list of when to skip it.

```fit
reach: Error messages. Told a file was "locked", three of four models used a median of 18 to 20 turns; told it was read-only, every model finished in four or five (chapter 4).
reach: A cheap tool-choice test. With thirty decoy tools added, two models chose a look-alike name in all six tries, which exposed a missing tool (chapter 7).
leave: Piling on tools without a test. The thirty extras moved scores by a point or two, within the noise, but the one systematic change pointed at a gap you only see by testing (chapter 7).
leave: Tool descriptions that assume trust. A description is prompt text; one sentence in another server's tool decided 79 of 80 first moves (chapter 10).
```

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), with no key set:

```console
$ go run ./ch07/evals/toolselect
$ go run ./ch07/evals/toolselect -distractors
$ go run ./ch03 template
```

The first two replay the recorded smoke-test runs, print each model's score out of 45, and list every answer that lost points; with `-distractors`, the `issue_lookup` picks show up in the list.
The third prints the start of chapter 1's bug fix, one tool included, as Qwen3's chat template lays it out for the model.
The tools live in [`ch07/triage`](code:ch07/triage), and the schema generator in [`ch07/agent/schema.go`](code:ch07/agent/schema.go).
