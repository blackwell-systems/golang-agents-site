---
title: AI Agents in Go
description: Learn how AI agents work at the level of every request, reply, and tool call, by building one step by step with Go's standard library.
layout: home
---

## The loop, in four stages

An agent is a model inside a [harness](/glossary/#harness), the program around it.
On every turn, one pass through these four stages, the harness does the same four things:

<style>
.loopv-c {
  --lvc-ease: cubic-bezier(0.22, 1, 0.36, 1);
  margin: 2.25rem 0 2.75rem;
  padding: 0;
  max-width: none;
  display: grid;
  gap: 1.25rem;
}
.loopv-c * { box-sizing: border-box; }
/* The ring */
.loopv-c .lvc-ring { position: relative; justify-self: center; width: 100%; }
.loopv-c .lvc-ring svg { display: block; width: 100%; height: auto; overflow: visible; }
.loopv-c .lvc-track { fill: none; stroke: var(--line); stroke-width: 1.5; }
.loopv-c .lvc-arc { fill: none; stroke: var(--st, var(--amber)); stroke-width: 2; }
.loopv-c .lvc-head { fill: var(--amber); }
/* Strokes stay thin however large the ring is drawn. */
.loopv-c .lvc-track, .loopv-c .lvc-arc, .loopv-c .lvc-node circle, .loopv-c .lvc-halo { vector-effect: non-scaling-stroke; }
.loopv-c .lvc-node circle { fill: var(--page); stroke: var(--st); stroke-width: 2; }
.loopv-c .lvc-node text { fill: var(--st); font-family: var(--label); font-weight: 600; font-size: 16px; text-anchor: middle; }
.loopv-c .lvc-label { fill: var(--soft); font-family: var(--label); font-weight: 600; font-size: 15px; text-anchor: middle; }
.loopv-c .lvc-label.lvc-start { text-anchor: start; }
.loopv-c .lvc-label.lvc-end { text-anchor: end; }
.loopv-c .lvc-center { fill: var(--ivory); font-family: var(--display); font-size: 24px; text-anchor: middle; }
.loopv-c .lvc-sub { fill: var(--soft); font-family: var(--label); font-size: 13px; text-anchor: middle; letter-spacing: 0.06em; }
.loopv-c .lvc-node { cursor: pointer; }
.loopv-c .lvc-node.is-on circle { fill: var(--st); }
.loopv-c .lvc-node.is-on text { fill: var(--page); }
.loopv-c .lvc-label.is-on { fill: var(--st); }
.loopv-c .lvc-halo { fill: none; stroke: var(--lvc-st, var(--stage-1)); stroke-width: 2; transition: stroke 220ms ease-out; }
.loopv-c .lvc-box { fill: none; stroke: none; }
.loopv-c .lvc-marker {
  transform-box: fill-box;
  transform-origin: 50% 50%;
  transition: transform 600ms var(--lvc-ease);
}
/* The stages */
.loopv-c .lvc-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.25rem;
}
.loopv-c .lvc-list > li {
  position: relative;
  display: grid;
  grid-template-columns: 2.25rem minmax(0, 1fr);
  column-gap: 1rem;
  margin: 0;
  padding: 0.8rem 1rem 0.85rem 0.85rem;
  border: 1px solid transparent;
  border-radius: 6px;
  line-height: 1.5;
}
.loopv-c .lvc-list > li::marker { content: none; }
/* The rail between the numbers. */
.loopv-c .lvc-list > li::before {
  content: "";
  position: absolute;
  left: calc(0.85rem + 1.125rem - 0.5px);
  top: calc(0.8rem + 2.25rem + 0.3rem);
  bottom: calc(-0.8rem - 0.25rem + 0.3rem);
  width: 1px;
  background: var(--line);
}
.loopv-c .lvc-num {
  grid-row: 1 / span 2;
  display: grid;
  place-items: center;
  width: 2.25rem;
  height: 2.25rem;
  border: 2px solid var(--st);
  border-radius: 50%;
  background: var(--page);
  color: var(--st);
  font-family: var(--label);
  font-weight: 600;
  font-size: 0.95rem;
  line-height: 1;
}
.loopv-c .lvc-name {
  align-self: center;
  min-height: 2.25rem;
  display: flex;
  align-items: center;
  gap: 0.45rem;
  color: var(--ivory);
  font-family: var(--label);
  font-weight: 600;
  font-size: 1.08rem;
  text-decoration: none;
}
.loopv-c .lvc-name::after { content: ""; position: absolute; inset: 0; border-radius: 6px; }
.loopv-c .lvc-go { color: var(--st); font-weight: 500; }
.loopv-c .lvc-desc { grid-column: 2; margin: 0; color: var(--stone); font-size: 0.97rem; }
.loopv-c .lvc-list > li:hover,
.loopv-c .lvc-list > li.is-on { background: var(--panel); border-color: var(--line); }
.loopv-c .lvc-list > li:hover .lvc-num,
.loopv-c .lvc-list > li.is-on .lvc-num { background: var(--st); color: var(--page); }
.loopv-c .lvc-list > li:hover .lvc-t { text-decoration: underline; text-decoration-thickness: 1px; text-underline-offset: 0.2em; }
.loopv-c .lvc-list > li:hover .lvc-go,
.loopv-c .lvc-list > li.is-on .lvc-go { transform: translateX(3px); }
.loopv-c .lvc-go { display: inline-block; }
.loopv-c .lvc-name:focus-visible { outline: none; }
.loopv-c .lvc-name:focus-visible::after { outline: 3px solid var(--ivory); outline-offset: 2px; }
/* The caption: back to stage 1. */
.loopv-c figcaption {
  display: grid;
  grid-template-columns: 2.25rem minmax(0, 1fr);
  column-gap: 1rem;
  align-items: center;
  margin: 0.15rem 0 0;
  padding: 0 1rem 0 0.85rem;
  color: var(--soft);
  font-family: var(--label);
  font-size: 0.92rem;
  line-height: 1.45;
}
.loopv-c .lvc-back { display: block; width: 2.25rem; height: 2.25rem; }
.loopv-c .lvc-back path { fill: none; stroke: var(--stage-1); stroke-width: 1.5; stroke-linecap: round; stroke-linejoin: round; }
@media (prefers-reduced-motion: no-preference) {
  .loopv-c .lvc-node circle, .loopv-c .lvc-node text, .loopv-c .lvc-label { transition: fill 220ms ease-out; }
  .loopv-c .lvc-list > li, .loopv-c .lvc-num { transition: background-color 220ms ease-out, border-color 220ms ease-out, color 220ms ease-out; }
  .loopv-c .lvc-go { transition: transform 220ms var(--lvc-ease); }
}
@media (prefers-reduced-motion: reduce) {
  .loopv-c .lvc-marker { transition: none; }
}
/* Narrow screens: the ring, cropped to itself, above the list. */
@media (max-width: 47.99rem) {
  .loopv-c { gap: 0.5rem; }
  .loopv-c .lvc-ring { width: min(15rem, 66vw); aspect-ratio: 1; overflow: hidden; }
  .loopv-c .lvc-ring svg {
    position: absolute;
    left: -20%;
    top: -10%;
    width: 160%;
  }
  .loopv-c .lvc-label { display: none; }
  .loopv-c .lvc-sub { font-size: 14.5px; letter-spacing: 0.02em; }
  .loopv-c .lvc-center { font-size: 27px; }
  .loopv-c .lvc-list > li { padding-left: 0.6rem; padding-right: 0.75rem; column-gap: 0.85rem; }
  .loopv-c .lvc-list > li::before { left: calc(0.6rem + 1.125rem - 0.5px); }
  .loopv-c figcaption { padding-left: 0.6rem; column-gap: 0.85rem; }
}
@media (min-width: 48rem) {
  .loopv-c {
    /* As wide as the frame where the page has room, centered on the
       text column. */
    --lvc-w: max(100%, min(var(--frame), calc(100vw - 2 * var(--gutter) - 20px)));
    width: var(--lvc-w);
    margin-left: calc((100% - var(--lvc-w)) / 2);
    grid-template-columns: minmax(0, 1fr) minmax(18rem, min(29rem, 42%));
    column-gap: clamp(1.5rem, 3.5vw, 3.5rem);
    row-gap: 0.4rem;
    align-items: center;
  }
  .loopv-c .lvc-ring { grid-row: 1 / span 2; align-self: center; }
  .loopv-c .lvc-list { align-self: end; }
  .loopv-c figcaption { align-self: start; }
}
</style>
<figure class="loopv-c">
  <div class="lvc-ring">
    <svg viewBox="-10 20 480 365" aria-hidden="true" focusable="false">
      <defs><marker id="lvc-arrow-1" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0 10 5 0 10z" style="fill: var(--stage-1)"/></marker><marker id="lvc-arrow-2" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0 10 5 0 10z" style="fill: var(--stage-2)"/></marker><marker id="lvc-arrow-3" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0 10 5 0 10z" style="fill: var(--stage-3)"/></marker><marker id="lvc-arrow-4" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0 10 5 0 10z" style="fill: var(--stage-4)"/></marker></defs>
      <circle cx="200" cy="200" r="120" class="lvc-track"/>
      <path d="M232 84 A120 120 0 0 1 316 168" class="lvc-arc" style="--st: var(--stage-1)" marker-end="url(#lvc-arrow-1)"/>
      <path d="M316 232 A120 120 0 0 1 232 316" class="lvc-arc" style="--st: var(--stage-2)" marker-end="url(#lvc-arrow-2)"/>
      <path d="M168 316 A120 120 0 0 1 84 232" class="lvc-arc" style="--st: var(--stage-3)" marker-end="url(#lvc-arrow-3)"/>
      <path d="M84 168 A120 120 0 0 1 168 84" class="lvc-arc" style="--st: var(--stage-4)" marker-end="url(#lvc-arrow-4)"/>
      <g class="lvc-marker"><circle cx="200" cy="200" r="145" class="lvc-box"/><circle cx="200" cy="80" r="26" class="lvc-halo"/></g>
      <g class="lvc-node" style="--st: var(--stage-1)"><circle cx="200" cy="80" r="17"/><text x="200" y="86">1</text></g>
      <g class="lvc-node" style="--st: var(--stage-2)"><circle cx="320" cy="200" r="17"/><text x="320" y="206">2</text></g>
      <g class="lvc-node" style="--st: var(--stage-3)"><circle cx="200" cy="320" r="17"/><text x="200" y="326">3</text></g>
      <g class="lvc-node" style="--st: var(--stage-4)"><circle cx="80" cy="200" r="17"/><text x="80" y="206">4</text></g>
      <text x="200" y="44" class="lvc-label" data-i="0" style="--st: var(--stage-1)">Build the request</text>
      <text x="200" y="368" class="lvc-label" data-i="2" style="--st: var(--stage-3)">Run the tools</text>
      <text x="352" y="206" class="lvc-label lvc-start" data-i="1" style="--st: var(--stage-2)">Call the model</text>
      <text x="48" y="206" class="lvc-label lvc-end" data-i="3" style="--st: var(--stage-4)">Decide</text>
      <text x="200" y="196" class="lvc-center">one turn</text>
      <text x="200" y="220" class="lvc-sub">until an answer or a limit</text>
    </svg>
  </div>
  <ol class="lvc-list">
    <li style="--st: var(--stage-1)"><span class="lvc-num" aria-hidden="true">1</span><a class="lvc-name" href="/build/context-management/"><span class="lvc-t">Build the request</span><span class="lvc-go" aria-hidden="true">→</span></a><p class="lvc-desc">Decide what the model sees: instructions, the conversation so far, tool results, and the tools it may call.</p></li>
    <li style="--st: var(--stage-2)"><span class="lvc-num" aria-hidden="true">2</span><a class="lvc-name" href="/basics/calling-a-model/"><span class="lvc-t">Call the model</span><span class="lvc-go" aria-hidden="true">→</span></a><p class="lvc-desc">Send the request and read the reply, with a deadline and a count of what it cost.</p></li>
    <li style="--st: var(--stage-3)"><span class="lvc-num" aria-hidden="true">3</span><a class="lvc-name" href="/basics/tool-calling/"><span class="lvc-t">Run the tools</span><span class="lvc-go" aria-hidden="true">→</span></a><p class="lvc-desc">Carry out the calls the model asked for, and add each result to the conversation.</p></li>
    <li style="--st: var(--stage-4)"><span class="lvc-num" aria-hidden="true">4</span><a class="lvc-name" href="/production/stop-reasons/"><span class="lvc-t">Decide</span><span class="lvc-go" aria-hidden="true">→</span></a><p class="lvc-desc">Stop with an answer, stop at a limit, or go around again.</p></li>
  </ol>
  <figcaption><svg class="lvc-back" viewBox="0 0 36 36" aria-hidden="true" focusable="false"><path d="M25.5 11.5A10 10 0 1 0 28 18"/><path d="M27.5 6.5l-1.5 5.5-5.5-1"/></svg><span>Then back to stage 1, with everything that just happened in the conversation.</span></figcaption>
</figure>
<script>
(function () {
  var root = document.querySelector(".loopv-c");
  if (!root) return;
  var marker = root.querySelector(".lvc-marker");
  var nodes = root.querySelectorAll(".lvc-node");
  // Labels are numbered, since their order in the markup isn't the stage order.
  var labels = Array.prototype.slice.call(root.querySelectorAll(".lvc-label"))
    .sort(function (a, b) { return a.dataset.i - b.dataset.i; });
  var items = root.querySelectorAll(".lvc-list > li");
  var list = root.querySelector(".lvc-list");
  var still = window.matchMedia("(prefers-reduced-motion: reduce)");
  var angle = 0, hovered = false, leaveTimer = 0, timers = [];
  // Move the marker to stage i (0 to 3) by the shorter way round.
  function move(i, ms) {
    var d = ((i * 90 - angle) % 360 + 540) % 360 - 180;
    if (d === -180) d = 180;
    angle += d;
    marker.style.transitionDuration = still.matches ? "0ms" : (ms || 380 + Math.abs(d) * 1.6) + "ms";
    marker.style.transform = "rotate(" + angle + "deg)";
  }
  function light(i) {
    // The loop's code, below, marks each line with its stage (stages=);
    // it comes after this script, so it's looked up here.
    var lines = document.querySelectorAll(".listing .line[data-stage]");
    for (var k = 0; k < 4; k++) {
      nodes[k].classList.toggle("is-on", k === i);
      labels[k].classList.toggle("is-on", k === i);
      items[k].classList.toggle("is-on", k === i);
    }
    if (i >= 0) root.style.setProperty("--lvc-st", "var(--stage-" + (i + 1) + ")");
    Array.prototype.forEach.call(lines, function (l) {
      l.classList.toggle("is-lit", l.dataset.stage === String(i + 1));
    });
  }
  function show(i) { stopTour(); clearTimeout(leaveTimer); light(i); move(i); }
  function rest() {
    clearTimeout(leaveTimer);
    leaveTimer = setTimeout(function () {
      if (hovered || root.contains(document.activeElement)) return;
      light(-1);
      move(0);
    }, 220);
  }
  function stopTour() { timers.forEach(clearTimeout); timers = []; }
  Array.prototype.forEach.call(items, function (li, i) {
    li.addEventListener("pointerenter", function () { hovered = true; show(i); });
    li.addEventListener("focusin", function () { show(i); });
  });
  // The ring's nodes answer a pointer too; the list holds the links.
  Array.prototype.forEach.call(nodes, function (g, i) {
    g.addEventListener("pointerenter", function () { hovered = true; show(i); });
    g.addEventListener("pointerleave", function () { hovered = false; rest(); });
    g.addEventListener("click", function () { items[i].querySelector("a").click(); });
  });
  list.addEventListener("pointerleave", function () { hovered = false; rest(); });
  // A stage's number in the loop's code answers a pointer as the ring
  // does. The code comes after this script, so wait for the page.
  document.addEventListener("DOMContentLoaded", function () {
    document.querySelectorAll(".listing .stage-badge").forEach(function (b) {
      var i = b.dataset.n - 1;
      b.addEventListener("pointerenter", function () { hovered = true; show(i); });
      b.addEventListener("pointerleave", function () { hovered = false; rest(); });
    });
  });
  root.addEventListener("focusout", rest);
  // One lap when the section first comes into view, then rest at stage 1.
  if (!still.matches && "IntersectionObserver" in window) {
    var seen = new IntersectionObserver(function (entries) {
      if (!entries[0].isIntersecting) return;
      seen.disconnect();
      if (hovered || root.contains(document.activeElement)) return;
      [1, 2, 3, 0].forEach(function (i, n) {
        timers.push(setTimeout(function () { light(i); move(i, 900); }, 500 + n * 1100));
      });
      timers.push(setTimeout(function () { light(-1); }, 500 + 4 * 1100 + 300));
    }, { threshold: 0.6 });
    seen.observe(root.querySelector(".lvc-ring"));
  }
})();
</script>

Each page on this site works on one of these stages or on the whole loop, and says which.
When an agent misbehaves, the stage tells you where to look: one that never stops has a Decide problem, and one that forgets what it read has a Build-the-request problem.

## The whole loop, in code

The loop talks to the model through one small interface:

```go include=ch01/agent/types.go tag=model
```

Anything with a `Generate` method is a `Model`: a scripted list of replies for tests, or a client for a real model's API.
The loop calls `m.Generate` once per turn and never needs to know which one it has.
Here is the loop as the [companion code](https://github.com/blackwell-systems/go-agents-book-code) writes it, each line marked with its stage's number and color from the ring above:

```go include=ch01/agent/loop.go tag=run stages=7-8:1,10-12:4,13:1,14-19:2,20-22:4,23-25:3,27:4
```

Its first line calls `index`, which splits the tools in two: `defs`, the definitions that go to the model in every request, and `byName`, which the loop uses to find the Go code for the tool a call names:

```go include=ch01/agent/loop.go lines=49..63
```

The other names are plain types and helpers: a `Message` is one entry in the conversation, a `Request` is the messages and tool definitions for one call, `Limits` holds settings such as `MaxTurns`, a `Result` holds the conversation a run ends with, `runTool` runs one call and turns its output (or its error) into a message for the model, and `ErrMaxTurns` is the error a run returns when it hits the turn limit.

The loop sends the whole conversation on every call, because the model keeps nothing between calls.
It appends the model's reply to the conversation before running its tools, so each tool result is paired with a tool call the conversation already holds.
It stops on an answer, at the turn limit, or when `ctx` is cancelled.
[The agent loop in Go](/basics/agent-loop/) explains each line.

Run it with no model at all, from a scripted list of replies:

```console
$ git clone https://github.com/blackwell-systems/go-agents-book-code
$ cd go-agents-book-code/code
$ go run ./ch01
```

The output replays a real model's fix of a failing test, turn by turn.
Then [`go run ./ch04`](/run/ch04-first-agent/) runs the same loop against a real model's [recorded replies](/production/testing-offline/), with a real `go test` behind its `run_tests` tool.
Both need Go {{code go-version}} or later and no API key.
The repository holds {{code files}} Go files and {{code tests}} test functions, and none of them calls a model; [Examples](/run/) lists every program in the repository.

<!-- paths -->

## Common questions

Each links to the guide that answers it, with Go code you can run.

<ul class="questions"><li><a href="/basics/getting-set-up/">What do I need to get started?</a><span>Go 1.27, the companion code, and examples that run with no API key.</span></li><li><a href="/basics/what-is-an-agent/">What is an AI agent, and how is it different from a chatbot?</a><span>A program where the model picks each next step and your Go code carries it out.</span></li><li><a href="/basics/first-model-call/">How do I make my first LLM API call?</a><span>An API key, one request with curl, and every part of the reply, including its cost.</span></li><li><a href="/basics/messages-and-roles/">What are messages, roles, and the system prompt?</a><span>The four roles in a conversation, and how to write instructions for an agent.</span></li><li><a href="/basics/tool-calling/">How does tool calling work?</a><span>The JSON a tool call is made of, and how a Go client sends and reads it.</span></li><li><a href="/build/first-agent/">How do I build my first agent in Go?</a><span>A complete agent in a few short files of standard-library Go, start to finish.</span></li><li><a href="/basics/choosing-a-model/">Which model should I use for an agent?</a><span>Tool support, cost per finished task, reasoning models, and local models.</span></li><li><a href="/ecosystem/go-agent-frameworks/">Should I use a Go agent framework?</a><span>ADK, Eino, Genkit, LangChainGo, and trpc-agent-go, set against the parts of a harness.</span></li></ul>
