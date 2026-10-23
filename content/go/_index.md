---
title: Go in depth
description: The Go an AI agent leans on, one page each, from context.Context and error handling to JSON v2, the HTTP client, streams, goroutines, iterators, and running commands, each tied to the agent code it comes from.
---

An agent is a Go program before it is anything else, and a few parts of the language and standard library carry most of the weight: a context that stops a run, errors that a model reads, a client that talks to an API, streams, goroutines, and iterators.
This section has one page for each, written around the agent code that uses it.
Each page explains the feature, shows the version the book's code uses, and ends with the mistakes that cost the most time.

It is for readers who know Go from the Tour and want the parts agents use.
You don't need to read it first: the guides in Basics and Build link here at the point they use a feature, and the pages stand on their own.
If goroutines, channels, or generics are new, finish the Tour first and come back.
