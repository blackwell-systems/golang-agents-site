---
title: "910 crash scenarios, no duplicate issues: a crash sweep of an agent's loop"
short: 910 crashes, no duplicates
description: Chapter 15 crashed the agent's run at every point a crash could land, restarted it, and counted duplicate issues. Saving once per turn duplicated work at 8 of its 18 crash points; idempotency keys removed every duplicate, and attempt markers without keys did too, at the price of 669 questions to a person.
card: The agent crashed at every point and restarted: what caused duplicates, and what removed them.
chapters: 15
stage: 3
date: 2026-10-18
weight: 60
art: ch15
---

We crashed an agent's run at every point where it could crash, restarted it each time, and counted duplicate issues.
With a saved checkpoint file and a tracker that recognizes a repeated request by its idempotency key, none of the 910 restarts filed anything twice.

```result
figure: 910
unit: crashes, 0 duplicates
task: the agent's own loop, crashed at each of its 38 crash points and resumed, then crashed again at every point each restart reaches
models: a scripted model, against a tracker that honors idempotency keys
runs: 910 crash-and-resume scenarios
date: deterministic; `go test ./ch15/durable/` repeats it
source: Chapter 15

On the agent’s own loop, with a file checkpoint and a tracker that honors idempotency keys, 910 crash-and-restart scenarios made no duplicate issues or comments, lost nothing, and asked nobody.
```

Once an agent's tools act on the world (filing issues, sending mail, deploying), a crash in the middle of a run stops being a private matter.
The restarted process can go wrong in two directions.
It can forget what it already did, start from the top, and pay for every model call again.
Or it can do something twice.
Suppose the issue tracker created an issue and the process died before writing down the reply.
The new process finds a tool call with no result, makes the call, and the crash now has two issues.

The fix is to make the run survive crashes, and then check that work by crashing it in every place it can be crashed.

## The window no checkpoint can close

Follow one tool call from start to finish: the request goes out, the other side acts on it, the reply comes back, and the run writes the reply to its [checkpoint](/production/durable-runs/), a file written to a temporary name and then renamed into place, so it is always whole.
If the process dies after the second step and before the fourth, the checkpoint shows nothing.
You can't fix that by saving sooner, since the reply you'd want to save hasn't arrived yet.

On its own, a single process can offer one of two promises.
[At most once](/glossary/#at-most-once) means it never sends a request a second time, so an action can be lost.
[At least once](/glossary/#at-least-once) means it keeps sending until it hears back, so an action can happen twice.
The behavior people call exactly once is really the second promise with a cooperative receiver: the sender may resend, and the receiver spots a request it has already handled.

Two mechanisms handle this:

- **An [idempotency key](/glossary/#idempotency-key)** goes out with every request that changes something. A receiver that keeps track of keys treats a second request with a known key as a repeat: it hands back its first answer and changes nothing.
- **An [attempt marker](/glossary/#attempt-marker)** is written to the checkpoint just before such a request goes out. A restarted run that finds a marker with no result can't tell whether the call took effect, so it settles the call on a resolution ladder: run it again if a repeat is harmless (the tool only reads, is idempotent, or sent a key the other side still holds), otherwise ask the other side whether it happened, and only when neither works, ask a person.

A key is only useful if a retry carries exactly the key the first attempt carried, even across a restart, and if no two different calls ever share one.
Joining the run's ID with the tool call's ID gives both properties, because the call ID went into the checkpoint with the model's reply before the call was made.

<div class="hood">

```go include=ch15/agent/once.go tag=key
```

Each ID is prefixed by its length.
Without the lengths, run `a:b` with call `c` and run `a` with call `b:c` would collide.

</div>

## How the sweep works

Killing a process at one chosen moment proves that moment is safe and says nothing about the others.
The sweep is exhaustive instead.
It first runs a short scripted triage job without interruption and notes every place a crash could happen:

- the model's reply has arrived
- a marker has been saved
- the tracker has accepted a request
- a tool has returned
- a result has been saved
- a save is half done: the temporary file exists and hasn't been renamed
- the rename has happened

For each of those points it starts over, crashes there, restarts, and lets the run finish.
A second pass crashes the restart too, once at each point the restart reaches.
The tracker stands in for the outside world, so its records outlive every crash, as a real service's database would.

The scripted job makes four tool calls.
It files two issues, comments on the first, and runs one search; only the search leaves the tracker unchanged.

## Five designs, compared

The first sweep runs five versions of a durable loop, from the simplest to the one this article settles on.
These five are simplified copies of the loop, built for the comparison, so their counts (such as 1,014 in the last row's `twice` column) differ from the 910 above, which comes from the agent's real loop, covered in the next section:

```text include=ch15/durable/testdata/sweep.txt lines=14..19 title="ch15/durable/testdata/sweep.txt: all scenarios"
```

Reading the columns: `once` is the number of crash points a design has, with one scenario for each.
`twice` is the extra scenarios that crash the restart as well.
`with dups` is how many scenarios finished with an extra issue or comment.
`missing` counts issues or comments the script made that the tracker never got, `asks` counts the times a person was consulted, and `calls+` counts repeated model calls.

- **Save once per turn:** duplicates at 8 of 18 crash points. Each one is a crash that landed after the tracker took a request and before the turn's save.
- **Save as soon as the model replies:** the window shrinks to a single call, and duplicates still show up at 9 of 34 crash points.
- **Keys, no markers:** zero duplicates.
- **Markers, no keys:** zero duplicates as well, because any crash between a marker and its result turns into a question for a person. That came to 669 questions across 1,039 scenarios, roughly two for every three crashes.
- **Keys and markers:** zero duplicates and zero questions.

None of the five lost anything.
In every scenario, each issue and comment the script makes ended up in the tracker.

## The agent's own loop

The five designs are simplified copies of the loop, built for the comparison.
The second sweep crashes the real loop: the agent resuming from a file checkpoint, once against a tracker that honors keys and once against one that ignores them.
That loop has 38 crash points, its own save points among them.

Against the keyed tracker, all 910 scenarios ended with no duplicate, no missing effect, and no question.
Against the keyless one, the 892 scenarios also had no duplicate and lost nothing, and they asked a person 624 times.
The counts differ because each scenario also crashes again at every point its restart reaches, and a restart that asks a person takes a different path from one that doesn't.

The sweep also covers two harder situations.
In one, a tool fails after the tracker has already accepted its request, as happens when an [MCP](/build/mcp/) server dies mid-call.
In the other, a tool waits behind an [approval gate](/production/approval/), and the crash lands while the person is deciding.
Both came through with no duplicate and no lost effect, and with keys on, nobody had to answer anything.

## What it can't show

The sweep leaves some things untested:

- **A panic isn't a power cut.** The test crashes by panicking, and the operating system's file cache survives a panic. The sweep also turns off file syncs so that it finishes in seconds. It checks the order of the saves, and leaves open whether they reach the disk.
- **The tracker is assumed durable.** It never crashes in the test.
- **The scripted person never answers wrong.** The keyless run's zero duplicates depend on that, and a real person can make a mistake.
- **The model is [scripted](/production/testing-offline/).** Asking it again gives the same reply. A real model asked again may choose different tool calls, so saving once per turn would fare worse in practice than in the table.

## What to do with it

- **Save the model's reply before you run any of its tools.** Each call then has a fixed ID before it goes out, and a restarted run can see which calls already have answers.
- **Send an idempotency key with every call that changes something**, made from the run ID and the call ID. Pick services that honor keys, and add key handling to the services you own.
- **When a service won't take a key, write a marker before the call.** On restart, a marker without a result means the outcome is unknown; work down the ladder above, and ask a person only as the last step.
- **Never reuse a run ID.** Providers happen to make call IDs unique, but nothing promises it, and a replayed run repeats the same call IDs as the run it came from. Call IDs keep keys apart within a run; only the run ID keeps them apart across runs.
- **Crash on purpose at every point** to test the order of your saves, including the points you didn't expect trouble from.

Durable workflow engines such as Temporal run into the same wall.
Temporal asks for its activities, the steps that reach outside the program, to be idempotent: an activity can do its work and die before reporting it, and then it runs a second time.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code):

```console
$ go test ./ch15/durable/ -run TestCrashSweep -v
$ go test ./ch15/durable/ -run TestLoopCrashSweep -v
```

`TestCrashSweep` runs the five designs and compares their counts with `ch15/durable/testdata/sweep.txt`.
`TestLoopCrashSweep` crashes the agent's own loop and prints its counts in the test log.
Neither needs a network connection, since the model is scripted.
