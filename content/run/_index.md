---
title: Run it
description: Programs from the companion repository that run offline, from recorded model replies, with the commands to run them and the files to read.
---

Every example runs without an API key.
With no `OPENROUTER_API_KEY` set, a program replays model replies recorded from a real run, while its tools, including `go test`, run for real on your machine.
You need Go 1.27 or later; older `go` commands from 1.21 on download it when they read the module's `go.mod`.

```console
$ git clone https://github.com/blackwell-systems/go-agents-book-code
$ cd go-agents-book-code/code
```

Each example page starts from that directory.
