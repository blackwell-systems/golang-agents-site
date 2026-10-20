---
title: "Commands and files: os/exec and os.Root for an agent's tools"
short: Commands and files
description: How an AI agent's tools in Go run commands and touch files. exec.CommandContext and what cancelling does, WaitDelay for pipes a child holds open, killing a whole process group on Unix, combined output with a size cap, a non-zero exit as a result the model reads, an environment built from scratch so keys never reach test code, os.Root for file tools that can't leave the workspace, and what none of it protects against.
card: Run go test so it stops when told, keeps your keys, and reports what failed; keep file tools inside one directory.
group: Go in depth
chapters: 4, 7, 14
stage: 3
date: 2026-10-20
weight: 92
---

A coding agent's most useful tools reach outside the Go process.
One runs `go test` on the code the model just changed; others read, search, and edit files in a workspace.
These tools are where a model's choices become real processes and real writes, so their limits have to hold no matter what the model asks for.

Two parts of the standard library carry most of that weight.
`os/exec` starts a command, collects what it prints, and stops it when the run is cancelled or a deadline passes.
`os.Root` opens one directory and refuses any path that would lead out of it.
This page covers both, with code from the [companion repository](https://github.com/blackwell-systems/go-agents-book-code), where it is tested.
Some of the pieces are recent:

| Feature | Since |
| --- | --- |
| `exec.CommandContext` | Go 1.7 |
| `exec.Cmd.Cancel`, `exec.Cmd.WaitDelay`, `exec.ErrWaitDelay` | Go 1.20 |
| `os.Root`, `os.OpenRoot`, `os.OpenInRoot` | Go 1.24 |
| `root.ReadFile`, `root.WriteFile`, `root.MkdirAll`, and nine more `os.Root` methods | Go 1.25 |
| `errors.AsType` | Go 1.26 |

Go 1.26 and 1.27 added no methods to `os.Root`.
The companion code needs Go {{code go-version}}.

## Running a command from a tool

`exec.CommandContext` builds a command tied to a context.
If the context ends before the command does, `os/exec` calls the command's `Cancel` function, which `CommandContext` sets to kill the process; on Unix that is `SIGKILL`, which the process can't catch.
The context to pass is the one the tool was given, derived from the run's, so Ctrl-C, a shutdown, and the tool's own deadline all reach the process the same way.
The [context page](/basics/context/#tools-that-start-processes) covers that chain; here is the runner behind the agent's `run_tests` tool:

```go include=ch17/agent/gotest.go lines=56..74 highlight=6,9-10,16
```

`run_tests` gives it two minutes, as a deadline with its own cause, `ErrTestsTimedOut`, so a timeout can be told apart from a cancelled run.
`go test` gets a `-timeout` of three quarters of that, 90 seconds, so a hung test is usually stopped by `go test` itself, which names the test and prints its goroutines.

`WaitDelay` handles a problem that has nothing to do with the command's own exit.
When `Stdout` or `Stderr` is a Go writer, such as a `bytes.Buffer`, `os/exec` connects the child to a pipe and copies from it in a goroutine, and `Run` doesn't return until that copy reaches the end of the pipe.
Any process the command started inherits the pipe, so it can keep the pipe open after the command itself is dead.
A short program shows the difference: a shell that started `sleep 30` in the background was killed after half a second.
With no `WaitDelay`, `Run` returned after 30 seconds, when `sleep` finally exited.
With a `WaitDelay` of one second, `Run` returned after 1.5 seconds, by closing the pipe; `sleep` was still running.
`WaitDelay` gets your goroutine back, but it does nothing about that leftover process, which is the next section's subject.

`Cancel` can be replaced, as long as you set it before the command starts.
A custom one might send `SIGTERM` to let the process exit on its own; if the process is still running when `WaitDelay` expires, `os/exec` kills it anyway.
Sometimes killing is the wrong first step altogether.
The agent's [MCP](/build/mcp/) client starts a server process that it talks to over standard input and output, and the protocol asks a client to close the server's input before anything harsher:

```go include=ch17/mcp/client.go lines=514..521 highlight=3-5
```

So it uses plain `exec.Command` and arranges the stop itself, with `context.AfterFunc`, as the context page shows.

## Stopping everything it started

`go test` is a parent: it runs the compiler, then a test binary, and test code can start processes of its own.
The default `Cancel` kills only the process `exec` started.
In a check with a test that sleeps for 40 seconds, killing the `go` command after four seconds left the test binary running, and `Run` waited out its two-second `WaitDelay` because the test binary still held the output pipe.

```definition
term: process group

A **process group** is a set of processes that the Unix kernel can signal together; a child joins its parent's group unless it is put in a new one, and a kill sent to a negative process ID reaches every member of the group with that ID.
```

The triage agent has a hardened test runner, which it uses when its [defenses](/production/prompt-injection/) are on. That runner starts the command in a group of its own and makes cancelling kill the group:

```go include=ch17/safety/group_unix.go tag=group highlight=8-11
```

`Setpgid: true` puts the child in a new group whose ID is the child's own process ID, so `-cmd.Process.Pid` names the whole group.
Every process `go test` starts joins that group unless it asks to leave.
With this `Cancel`, the same check ended at four seconds with nothing left running.
`killGroup` covers the other way a run ends: the `go` command exits normally while a child of the test is still alive.

Here is where both are used, along with the output handling the next section describes:

```go include=ch17/safety/sandbox.go tag=run highlight=22,25
```

Three limits are worth stating plainly:

- **It is Unix only.** `group_unix.go` has the `unix` build constraint; on other systems, `group_other.go` provides two functions that do nothing, and only the `go` command is killed. On Windows the equivalent is a Job Object, which the companion code doesn't use.
- **A process can leave the group.** One that starts a new session with `setsid` is no longer in the group, so the kill misses it. An operating system sandbox catches that: bubblewrap, used on Linux at the next rung, puts the run in a process namespace of its own and ends it with its parent.
- **The plain runner doesn't do it.** `GoTest`, above, uses the default `Cancel` and a five-second `WaitDelay`. The triage tools use it too when the defenses are off, so there a test's children can outlive a timeout.

## Output the model can read

What a command prints becomes a tool result, which goes into the conversation and is sent again with every later request.
So the output has to arrive in a readable order, be bounded in size, and say when it was cut.

**One writer for both streams.**
The runner above assigns the same `*capped` value to `Stdout` and `Stderr`.
When the two are the same writer and can be compared with `==`, `os/exec` gives the child a single pipe for both, so error lines and normal output stay in the order the process wrote them, and only one goroutine calls `Write` at a time.
`cmd.CombinedOutput` does the same with a `bytes.Buffer`, with no limit, so a test that prints in a loop can fill memory.

**A cap applied while reading.**
`capped` keeps the first bytes it is given and drops the rest:

```go include=ch17/safety/sandbox.go lines=151..173 highlight=13-17
```

`Write` reports the whole write as taken even when it keeps none of it.
A short count would end the copy from the pipe with an error, `os/exec` would close its end of the pipe, and the command would fail on its next write instead of finishing.
This cap, 1 MiB by default, protects the harness's memory.
When it is reached, the run's error says so, with `ErrOutputCapped`.

**A smaller cap for the model, with a note.**
`run_tests` sends the model far less: its report keeps the output of failing tests, build errors, and each package's result line, drops the output of passing tests, timings, and the temporary directory's path, and then cuts the text to 8 KiB:

```go include=ch17/agent/gotest.go lines=201..210
```

A cut result that says how much is missing tells the model the output was long, so it doesn't treat a partial result as the whole story.
The file tools follow the same rule: `code_read` shows at most 200 lines and ends with the line number to ask for next, as [designing tools](/build/tool-design/#results-the-model-can-use) describes.

**A non-zero exit is a result.**
`go test` exits with status 1 when a test fails, and a failing test is exactly what the model is there to see.
Here is the rest of `GoTest`, after `Run`:

```go include=ch17/agent/gotest.go lines=75..91 highlight=8-11
```

The order of the checks matters:

1. **The context first.** A command killed by `Cancel` returns an `*exec.ExitError` too, whose message is just "signal: killed". Checked any later, a timeout would look like a test failure.
2. **Then the exit status.** `errors.AsType[*exec.ExitError]` tells "the command ran and exited non-zero" apart from "the command couldn't run", such as a missing `go` binary, for which `errors.Is(err, exec.ErrNotFound)` is true. If you need the status itself, `ExitCode()` on the `*exec.ExitError` returns it.
3. **A failure with nothing on standard output.** That means `go test` refused to start, for example over a broken `go.mod`; its standard error becomes the error text.
4. **Everything else is the report**, returned with a nil error, whatever the exit status was.

The hardened runner makes the same choice in fewer lines:

```go include=ch17/safety/sandbox.go lines=122..139
```

It adds one case: `exec.ErrWaitDelay` is what `Run` returns when the command exited successfully but something it started kept the output open until `WaitDelay` expired.
`killGroup` has already ended that process, so the run counts as finished.

The errors these functions do return still don't stop the agent.
A tool's error goes back to the model as the tool's result, marked as an error, unless it wraps `ErrToolFatal`; the [errors page](/basics/errors/#errors-the-model-reads) shows that split, and [why the agent stopped](/production/stop-reasons/) lists what does end a run.
A test run that timed out is something the model can act on, by running fewer tests or fixing a hang.

## The environment it runs in

`cmd.Env` decides what the child's environment holds:

- **`nil`** gives the child a copy of your whole environment, `OPENROUTER_API_KEY` and any other secret in it included.
- **A non-nil slice** is the child's entire environment. An empty, non-nil slice really is empty: `env` run that way prints nothing.
- **A repeated key** takes its last value, so appending `CGO_ENABLED=1` after `CGO_ENABLED=0` turns cgo on.

The code `go test` runs is code the model wrote or changed, so it gets an environment built from nothing:

```go include=ch17/agent/gotest.go tag=test-env
```

The first six entries keep `go test` offline and independent of your own Go setup:

| Setting | What it stops |
| --- | --- |
| `GOWORK=off` | A `go.work` file in a parent directory pulling other modules into the build |
| `GOPROXY=off` | Downloading modules; a module not already in the cache is an error |
| `GOFLAGS=` | Flags you set for all your `go` commands, such as `-mod=mod`, applying to this one |
| `GOENV=off` | Settings saved with `go env -w` in your Go configuration file |
| `GOTOOLCHAIN=local` | Downloading a newer Go when `go.mod` asks for one; the installed `go` reports "go.mod requires go >= …" instead |
| `CGO_ENABLED=0` | Calling a C compiler, unless the run asked for `-race` |

The loop after them passes through only the variables the `go` command needs to find itself and its caches, each copied only if it is set.
The `run_tests` tests include a case named "no API key", which sets `OPENROUTER_API_KEY` and runs a test that fails if it can see the key.

The hardened runner goes further and inherits nothing but `PATH`:

```go include=ch17/safety/sandbox.go tag=env
```

`HOME`, the build cache, the module cache, and the temporary directory all point into a private scratch directory made for the run.
A program that finds your files through `$HOME`, such as `git` reading its configuration, sees an empty home instead.
A program that names `/Users/you/.ssh` directly still reaches it; stopping that is the operating system sandbox's job.

The same rule applies to every child the harness starts.
The MCP client above copies `cmd.Env` into a new slice, because a nil `Env` would hand the server your whole environment.
The triage program also [removes its key variables](code:ch17/secrets.go) from its own environment at startup, so a command started anywhere with a nil `Env` can't inherit them.
On macOS that still leaves a gap: a process can read the starting environment of another process run by the same user, which is why the [prompt injection guide](/production/prompt-injection/#a-sandboxing-ladder-for-the-code-it-runs) keeps keys in files only you can read.

## Files inside one directory: os.Root

A file tool takes a path from the model.
Joining that path to the workspace with `filepath.Join` and calling `os.Open` trusts it completely: `../../.ssh/id_ed25519` leads out, and so does a symbolic link inside the workspace that points elsewhere.
Checking the text of the path catches the first and can't see the second.

```definition
term: os.Root

An **`os.Root`** is an open directory; its methods take paths relative to it, and refuse with an error any path that would reach something outside it, whether by `..`, an absolute path, or a symbolic link.
```

`os.OpenRoot(dir)` opens the directory, on most systems as a file descriptor, so the root keeps referring to the same directory even if it is renamed while open.
The methods mirror the `os` functions.
Go 1.24 has `Open`, `OpenFile`, `Create`, `Stat`, `Lstat`, `Mkdir`, `Remove`, `OpenRoot` for a subdirectory, `FS`, `Name`, and `Close`.
Go 1.25 added `ReadFile`, `WriteFile`, `MkdirAll`, `RemoveAll`, `Rename`, `Link`, `Symlink`, `Readlink`, `Chmod`, `Chown`, `Lchown`, and `Chtimes`.
`os.OpenInRoot(dir, name)` opens one file in one call.
A `Root` is safe to use from several goroutines.

A test program on macOS opened a workspace directory, `ws`, as a root, put `secret.txt` beside it, and tried these names:

| Name given to `root.Open` | Result |
| --- | --- |
| `store/a.go` | Opened |
| `store/../store/a.go` | Opened: `..` that stays inside is allowed |
| `../secret.txt`, `store/../../secret.txt` | Refused |
| `/etc/passwd`, or any absolute path, even one inside the workspace | Refused |
| A link to `../secret.txt` | Refused |
| A link with an absolute target, even a target inside the workspace | Refused |
| A link to `store/a.go` | Opened |
| A link to `../ws/store/a.go`, which leaves the workspace and comes back | Refused |
| A name that doesn't exist | An error that matches `fs.ErrNotExist` |

Writes behave the same: `root.Create("../new.txt")` and `root.WriteFile` through an escaping link both failed, and nothing appeared outside.
Three details from the same run matter when you write a tool:

- **The error is an `*fs.PathError`** whose message ends in "path escapes from parent". The error inside it is unexported in package `os`, so there is no sentinel to check with `errors.Is`; check `fs.ErrNotExist` for a missing file and refuse the call on any other error.
- **`root.Lstat` on an escaping link succeeds,** because it describes the link itself, not what the link points to.
- **`root.FS()` is stricter than the root.** It returns an `fs.FS`, which accepts only names that pass `fs.ValidPath`, so `store/../store/a.go` fails there with `fs.ErrInvalid` even though `root.Open` accepts it.

The companion code first uses a root to write the project files that `run_tests` copies into a temporary directory:

```go include=ch17/agent/memfs.go tag=write-dir
```

`TestWriteDirStaysInDir` gives it a file named `../escape.go` and checks that the write fails.

The triage agent opens its workspace once, when its tools are set up, and keeps the root for the whole run:

```go include=ch17/triage/env.go lines=89..100 highlight=4
```

Every file tool goes through it: `code_read` and `code_patch` with `root.ReadFile`, `root.WriteFile`, and `root.Stat`, and `code_list` and `code_search` walk `root.FS()` with `fs.WalkDir`.
A text check on the path runs first, because its refusal can tell the model what a correct path looks like:

```go include=ch17/triage/codetools.go lines=23..32
```

The root is the second check, and the one that catches links.
`code_read` turns any read error, whether a missing file or a refusal, into a message with a next step:

```go include=ch17/triage/codetools.go lines=97..106 highlight=6-9
```

The approval gate, which hashes each file a call would change so that a person's yes covers exactly what they saw, reads through a root too, and uses `root.Lstat` before opening anything:

```go include=ch17/safety/approve.go lines=351..381 highlight=10,16,21
```

Checking the type first means a named pipe planted in the workspace can't make the gate wait forever on a read, and `io.LimitReader` bounds the read.

A root allows links that stay inside it, and that leaves one gap a write tool may care about.
A link from `store/a` to `../auth` stays inside the workspace, so `os.Root` follows it, and a write to `store/a/auth.go` changes a file in `auth/`.
If some paths in the workspace are off limits, check them separately: the [protected-path check](/production/prompt-injection/#five-layers-strongest-first) refuses any write whose path has a link or a `..` anywhere along it.

## What these don't cover

`os.Root` limits the harness's own file calls, nothing more.
The command `run_tests` starts is a separate program, running as your user:

- It can open any file you can, by its absolute path, and connect to any address your machine can reach. An empty environment and a private `HOME` don't change that.
- `os.Root` guards paths, not files. Its documentation says it doesn't stop crossing into other file systems, Linux bind mounts, `/proc` special files, or Unix device files. And a hard link is the same file under a second name: in a test, a hard link inside the root to a file outside it let `root.ReadFile` read that file and `root.WriteFile` change it. Test code runs as you, so it can make such a link.
- A process group can be left, as above.

Everything on this page stops accidents: a test that hangs, a test that prints forever, a model that guesses a path with `..` in it, a key sitting in the environment.
None of it stops code written to do harm.
When the agent runs code shaped by text from strangers, such as a crash report, it needs an operating system sandbox, a check on everything that leaves the run, and a person's approval for the actions that matter; the [prompt injection guide](/production/prompt-injection/) lays out that ladder and the [approval guide](/production/approval/) the gate.

## Mistakes to avoid

- **A nil `cmd.Env`.** The child inherits everything, keys included. `append(os.Environ(), ...)` has the same problem.
- **No `WaitDelay` when the output goes to a Go writer.** A grandchild that keeps the pipe open keeps `Run` waiting with it.
- **Killing only the parent.** On Unix, start the command in its own process group and kill the group, on cancel and after a normal exit.
- **Reading a killed command's error as its result.** "signal: killed" is an `*exec.ExitError`; check the context before the exit status.
- **A non-zero exit as a Go error that ends the run.** For a test runner, a failing test is the result the model needs.
- **`CombinedOutput` on a command you don't control.** It keeps everything; use a writer with a cap, and tell the model when its output was cut.
- **`filepath.Join(workspace, path)` with a path from the model.** Text checks miss links; open the workspace with `os.OpenRoot` and use its methods.
- **Expecting `os.Root` to confine the programs you run.** It only governs calls made through it.

## Try it

From the `code` directory of a clone of the [companion repository](https://github.com/blackwell-systems/go-agents-book-code); none of these needs an API key:

```console
$ go test ./ch17/agent -run 'TestRunTests$|TestWriteDirStaysInDir|TestReportOrderAndPaths' -v
$ go test ./ch17/triage -run TestCodeTools -v
$ go test ./ch17/safety -run TestRunner -v
```

The first runs `run_tests` against failing, passing, and broken projects and the "no API key" case, and checks that `WriteDir` refuses `../escape.go`.
The second includes a `code_read` of `../../etc/passwd`.
The third, which runs only on Unix, covers the hardened runner: the environment, the group kill on timeout, the output cap, and a child left holding the output.
`go test ./ch17/agent -run TestRunTestsLimits -v` checks the timeouts and the 8 KiB cut; it takes about a minute and a half, because it waits for `go test`'s own 90-second timeout to stop a hanging test.

1. In `testEnv`, add `"OPENROUTER_API_KEY"` to the list of variables passed through, and run `TestRunTests` again: the "no API key" case fails with "key visible".
2. In `TestRunner.Run`, comment out `group(cmd)` and run `TestRunnerKillsGroup` and `TestRunnerLinger`: both report that the child outlived the run.
3. In the triage workspace that `TestCodeTools` sets up, make a symbolic link that points outside it, such as to `/etc/hosts`, and call `code_read` on the link's name: the text check lets it through, and the root refuses it.
