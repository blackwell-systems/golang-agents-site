---
title: AI Agents in Go
description: Learn how AI agents work at the level of every request, reply, and tool call, by building one step by step with Go's standard library.
layout: home
---

## The loop, in four stages

An agent is a model inside a [harness](/glossary/#harness), the program around it.
On every turn, one pass through these four stages, the harness does the same four things:

<div class="loopv-current"><ol class="loop-stages"><li><strong>Build the request</strong>Decide what the model sees: instructions, the conversation so far, tool results, and the tools it may call.</li><li><strong>Call the model</strong>Send the request and read the reply, with a deadline and a count of what it cost.</li><li><strong>Run the tools</strong>Carry out the calls the model asked for, and add each result to the conversation.</li><li><strong>Decide</strong>Stop with an answer, stop at a limit, or go around again.</li></ol><p class="loop-back">Then back to stage 1, with everything that just happened in the conversation.</p></div>
<template id="loopv-a">
<div class="loopv-a">
<style>
.loopv-a {
  --lva-z: 2.5rem;      /* stage ring */
  --lva-r: 1.6rem;      /* loop corner radius, and the side gutter */
  --lva-g: 2rem;        /* gap between stacked cards */
  --lva-x: 3.6rem;      /* room for the exit below the loop (stacked) */
  --lva-ny: 2.05rem;    /* ring center from a card's top (stacked) */
  --lva-h: 12px;        /* arrowhead */
  --lva-t: 1.3rem;      /* the stop terminal */
  container: lva / inline-size;
  margin: 2.25rem 0 2.5rem;
}
.loopv-a .lva-row {
  position: relative;
  max-width: 34rem;
  padding: var(--lva-r) var(--lva-r) calc(var(--lva-r) + var(--lva-x)) calc(var(--lva-z) / 2 + 4px);
}
.loopv-a .lva-loop {
  position: absolute;
  top: 0;
  right: 0;
  bottom: var(--lva-x);
  left: calc(var(--lva-z) / 2 + 4px);
  border: 2px solid var(--amber);
  border-radius: var(--lva-r);
  pointer-events: none;
}
.loopv-a .lva-stages {
  --lva-across: 0;
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: var(--lva-g);
}
.loopv-a .lva-stage { position: relative; margin: 0; padding: 0; }
.loopv-a .lva-stage::marker { content: none; }
.loopv-a .lva-card {
  position: relative;
  height: 100%;
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  background: var(--panel);
  border: 1px solid var(--line);
  border-left: 2px solid var(--amber);
  border-radius: 4px;
  padding: 1.25rem 1.25rem 1.15rem calc(var(--lva-z) / 2 + 0.9rem);
  transition: border-color 0.5s ease-out;
}
.loopv-a .lva-num {
  position: absolute;
  z-index: 3;
  left: calc(-1px - var(--lva-z) / 2);
  top: calc(var(--lva-ny) - 1px - var(--lva-z) / 2);
  width: var(--lva-z);
  height: var(--lva-z);
  box-sizing: border-box;
  display: grid;
  place-items: center;
  border: 2px solid var(--amber);
  border-radius: 50%;
  background: var(--page);
  color: var(--amber);
  font-family: var(--label);
  font-weight: 600;
  font-size: 1.05rem;
  line-height: 1;
  pointer-events: none;
  transition: background-color 0.45s ease-out, color 0.45s ease-out;
}
.loopv-a .lva-name {
  margin: 0 0 0.5rem;
  font-family: var(--label);
  font-weight: 600;
  font-size: 1.25rem;
  line-height: 1.25;
  letter-spacing: -0.01em;
  color: var(--ivory);
}
.loopv-a .lva-link { color: inherit; text-decoration: none; }
.loopv-a .lva-link::after {
  content: "";
  position: absolute;
  inset: 0;
  z-index: 2;
  border-radius: 4px;
}
.loopv-a .lva-link:focus-visible { outline: none; }
.loopv-a .lva-link:focus-visible::after { outline: 3px solid var(--ivory); outline-offset: 4px; }
.loopv-a .lva-desc { margin: 0; font-size: 0.97rem; line-height: 1.55; color: var(--stone); }
.loopv-a .lva-more {
  margin-top: auto;
  padding-top: 1rem;
  font-family: var(--label);
  font-size: 0.86rem;
  font-weight: 500;
  color: var(--link);
}
.loopv-a .lva-more span { display: inline-block; transition: transform 0.3s ease-out; }
.loopv-a .lva-card:hover { border-color: var(--amber-dim); border-left-color: var(--amber); }
.loopv-a .lva-card:hover .lva-more { text-decoration: underline; text-underline-offset: 0.18em; }
.loopv-a .lva-card:hover .lva-more span { transform: translateX(3px); }
.loopv-a .lva-vh {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip-path: inset(50%);
  white-space: nowrap;
}
/* the moving turn: the stage it is in */
.loopv-a .is-on .lva-num { background: var(--amber); color: var(--amber-ink); }
.loopv-a .is-on .lva-card { border-color: var(--amber); }
/* arrowheads: one shape, turned to face the way the loop runs */
.loopv-a .lva-ah,
.loopv-a .lva-stage:not(:last-child)::after {
  content: "";
  position: absolute;
  width: var(--lva-h);
  height: var(--lva-h);
  background: var(--amber);
  clip-path: polygon(0 0, 100% 50%, 0 100%);
  pointer-events: none;
}
.loopv-a .lva-stage:not(:last-child)::after {
  left: calc(1px - var(--lva-h) / 2);
  top: calc(100% + var(--lva-g) / 2 - var(--lva-h) / 2);
  transform: rotate(90deg);
}
.loopv-a .lva-ah-b { bottom: calc(-1px - var(--lva-h) / 2); left: 50%; }
.loopv-a .lva-ah-r1, .loopv-a .lva-ah-r2 { right: calc(-1px - var(--lva-h) / 2); transform: rotate(-90deg); }
.loopv-a .lva-ah-r1 { top: 72%; }
.loopv-a .lva-ah-r2 { top: 24%; }
.loopv-a .lva-ah-t { top: calc(-1px - var(--lva-h) / 2); left: 55%; transform: rotate(180deg); }
.loopv-a .lva-ah-l, .loopv-a .lva-ah-b2 { display: none; }
.loopv-a .lva-next {
  position: absolute;
  right: -1px;
  top: 48%;
  transform: translate(50%, -50%) rotate(180deg);
  writing-mode: vertical-rl;
  padding: 0.6rem 0;
  background: var(--page);
  font-family: var(--label);
  font-weight: 500;
  font-size: 0.88rem;
  letter-spacing: 0.08em;
  line-height: 1;
  white-space: nowrap;
  color: var(--soft);
  transition: color 0.5s ease-out;
}
.loopv-a .lva-next.is-near { color: var(--amber); }
/* the way out: from Decide to a stop */
.loopv-a .lva-exit {
  position: absolute;
  left: 1px;
  top: 100%;
  width: 0;
  height: calc(var(--lva-r) + var(--lva-x) * 0.55);
  pointer-events: none;
}
.loopv-a .lva-stub {
  position: absolute;
  left: -1px;
  top: 0;
  width: 2px;
  height: calc(100% - var(--lva-t) / 2 - 4px - var(--lva-h) / 2);
  background: var(--amber);
}
.loopv-a .lva-ah-x {
  left: calc(-1 * var(--lva-h) / 2);
  top: calc(100% - var(--lva-t) / 2 - 4px - var(--lva-h));
  transform: rotate(90deg);
}
.loopv-a .lva-term {
  position: absolute;
  left: calc(-1 * var(--lva-t) / 2);
  top: calc(100% - var(--lva-t) / 2);
  width: var(--lva-t);
  height: var(--lva-t);
  box-sizing: border-box;
  border: 2px solid var(--amber);
  border-radius: 50%;
  background: var(--page);
  z-index: 3;
  transition: background-color 0.45s ease-out;
}
.loopv-a .lva-term::after {
  content: "";
  position: absolute;
  inset: 3px;
  border-radius: 50%;
  background: var(--amber);
}
.loopv-a .lva-stop {
  position: absolute;
  left: calc(var(--lva-t) / 2 + 0.7rem);
  top: calc(100% - 0.7em);
  font-family: var(--label);
  font-weight: 500;
  font-size: 0.92rem;
  line-height: 1.4;
  white-space: nowrap;
  color: var(--soft);
  transition: color 0.5s ease-out;
}
.loopv-a .lva-stop b { font-weight: 600; color: var(--ivory); }
.loopv-a .is-stop .lva-term { background: var(--amber); }
.loopv-a .is-stop .lva-stop { color: var(--ivory); }
/* the marker, one turn travelling the loop */
.loopv-a .lva-dot {
  position: absolute;
  left: 0;
  top: 0;
  z-index: 2;
  width: 22px;
  height: 22px;
  margin: -11px 0 0 -11px;
  box-sizing: border-box;
  border: 4px solid var(--page);
  border-radius: 50%;
  background: var(--amber);
  pointer-events: none;
  will-change: transform;
}
.loopv-a .lva-foot {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.5rem 1.5rem;
  max-width: 34rem;
  margin-top: 0.4rem;
}
.loopv-a .lva-cap {
  margin: 0;
  max-width: 34rem;
  font-family: var(--label);
  font-size: 0.95rem;
  line-height: 1.5;
  color: var(--soft);
}
.loopv-a .lva-toggle {
  flex: none;
  font: 500 0.8rem/1 var(--label);
  letter-spacing: 0.04em;
  color: var(--soft);
  background: transparent;
  border: 1px solid var(--line);
  border-radius: 999px;
  padding: 0.4rem 0.85rem;
  cursor: pointer;
  transition: color 0.2s ease-out, border-color 0.2s ease-out;
}
.loopv-a .lva-toggle:hover { color: var(--ivory); border-color: var(--amber-dim); }
.loopv-a .lva-toggle:focus-visible { outline: 3px solid var(--ivory); outline-offset: 3px; }
/* A row across, once there is room for four readable cards. */
@container lva (min-width: 50rem) {
  .loopv-a .lva-row {
    --lva-g: 1.6rem;
    --lva-e: 4.25rem;   /* room for the exit above the row */
    --lva-b: 2.6rem;    /* the return track below the row */
    --lva-nx: 2.4rem;   /* ring center from a card's left */
    max-width: none;
    padding: var(--lva-e) var(--lva-r) var(--lva-b);
  }
  .loopv-a .lva-loop { top: var(--lva-e); right: 0; bottom: 0; left: 0; }
  .loopv-a .lva-stages { --lva-across: 1; grid-template-columns: repeat(4, minmax(0, 1fr)); column-gap: var(--lva-g); }
  .loopv-a .lva-card {
    border-left: 1px solid var(--line);
    border-top: 2px solid var(--amber);
    padding: calc(var(--lva-z) / 2 + 0.95rem) 1.2rem 1.15rem;
  }
  .loopv-a .lva-card:hover { border-left-color: var(--amber-dim); border-top-color: var(--amber); }
  .loopv-a .is-on .lva-card { border-left-color: var(--amber); }
  .loopv-a .lva-num {
    left: calc(var(--lva-nx) - 1px - var(--lva-z) / 2);
    top: calc(-1px - var(--lva-z) / 2);
  }
  .loopv-a .lva-stage:not(:last-child)::after {
    left: calc(100% + var(--lva-g) / 2 - var(--lva-h) / 2);
    top: calc(1px - var(--lva-h) / 2);
    transform: none;
  }
  .loopv-a .lva-ah-t { display: none; }
  .loopv-a .lva-ah-l { display: block; left: calc(-1px - var(--lva-h) / 2); top: 50%; transform: rotate(-90deg); }
  .loopv-a .lva-ah-r1 { top: 50%; transform: rotate(90deg); }
  .loopv-a .lva-ah-r2 { display: none; }
  .loopv-a .lva-ah-b { left: 24%; transform: rotate(180deg); }
  .loopv-a .lva-ah-b2 { display: block; bottom: calc(-1px - var(--lva-h) / 2); left: 76%; transform: rotate(180deg); }
  .loopv-a .lva-next {
    right: auto;
    left: 50%;
    top: auto;
    bottom: -1px;
    transform: translate(-50%, 50%);
    writing-mode: horizontal-tb;
    padding: 0 0.9rem;
  }
  .loopv-a .lva-exit {
    left: var(--lva-nx);
    top: auto;
    bottom: calc(100% + var(--lva-z) / 2);
    height: calc(var(--lva-e) - var(--lva-z) / 2 - 0.9rem);
  }
  .loopv-a .lva-stub { top: auto; bottom: 0; }
  .loopv-a .lva-ah-x { top: calc(var(--lva-t) / 2 + 4px); transform: rotate(-90deg); }
  .loopv-a .lva-term { top: calc(-1 * var(--lva-t) / 2); }
  .loopv-a .lva-stop { left: auto; right: calc(var(--lva-t) / 2 + 0.8rem); top: -0.7em; }
  .loopv-a .lva-foot { max-width: none; padding: 0 var(--lva-r); margin-top: 0.75rem; }
  .loopv-a .lva-cap { max-width: none; }
}
@media (prefers-reduced-motion: reduce) {
  .loopv-a .lva-dot, .loopv-a .lva-toggle { display: none; }
  .loopv-a *, .loopv-a *::before, .loopv-a *::after { transition: none !important; }
}
</style>
<div class="lva-row">
<span class="lva-loop" aria-hidden="true"><span class="lva-ah lva-ah-t"></span><span class="lva-ah lva-ah-r1"></span><span class="lva-ah lva-ah-r2"></span><span class="lva-ah lva-ah-b"></span><span class="lva-ah lva-ah-b2"></span><span class="lva-ah lva-ah-l"></span><span class="lva-next">next turn</span></span>
<ol class="lva-stages">
<li class="lva-stage"><div class="lva-card"><span class="lva-num" aria-hidden="true">1</span><h3 class="lva-name"><a class="lva-link" href="/build/context-management/">Build the request<span class="lva-vh">: read the guide</span></a></h3><p class="lva-desc">Decide what the model sees: instructions, the conversation so far, tool results, and the tools it may call.</p><span class="lva-more" aria-hidden="true">Read the guide <span>→</span></span></div></li>
<li class="lva-stage"><div class="lva-card"><span class="lva-num" aria-hidden="true">2</span><h3 class="lva-name"><a class="lva-link" href="/basics/calling-a-model/">Call the model<span class="lva-vh">: read the guide</span></a></h3><p class="lva-desc">Send the request and read the reply, with a deadline and a count of what it cost.</p><span class="lva-more" aria-hidden="true">Read the guide <span>→</span></span></div></li>
<li class="lva-stage"><div class="lva-card"><span class="lva-num" aria-hidden="true">3</span><h3 class="lva-name"><a class="lva-link" href="/basics/tool-calling/">Run the tools<span class="lva-vh">: read the guide</span></a></h3><p class="lva-desc">Carry out the calls the model asked for, and add each result to the conversation.</p><span class="lva-more" aria-hidden="true">Read the guide <span>→</span></span></div></li>
<li class="lva-stage"><div class="lva-card"><span class="lva-num" aria-hidden="true">4</span><h3 class="lva-name"><a class="lva-link" href="/production/stop-reasons/">Decide<span class="lva-vh">: read the guide</span></a></h3><p class="lva-desc">Stop with an answer, stop at a limit, or go around again.</p><span class="lva-more" aria-hidden="true">Read the guide <span>→</span></span></div><span class="lva-exit" aria-hidden="true"><span class="lva-stub"></span><span class="lva-ah lva-ah-x"></span><span class="lva-term"></span><span class="lva-stop">answer or limit: <b>stop</b></span></span></li>
</ol>
</div>
<div class="lva-foot"><p class="lva-cap">Then back to stage 1, with everything that just happened in the conversation.</p></div>
<script>
(function () {
  var root = document.currentScript && document.currentScript.closest(".loopv-a");
  if (!root || !window.requestAnimationFrame || !window.matchMedia) return;
  var row = root.querySelector(".lva-row");
  var ol = root.querySelector(".lva-stages");
  var loop = root.querySelector(".lva-loop");
  var exit = root.querySelector(".lva-exit");
  var next = root.querySelector(".lva-next");
  var term = root.querySelector(".lva-term");
  var stages = [].slice.call(root.querySelectorAll(".lva-stage"));
  var nums = stages.map(function (s) { return s.querySelector(".lva-num"); });
  var NS = "http://www.w3.org/2000/svg";
  var svg = document.createElementNS(NS, "svg");
  svg.setAttribute("aria-hidden", "true");
  svg.setAttribute("focusable", "false");
  svg.style.cssText = "position:absolute;width:0;height:0;overflow:hidden";
  var path = document.createElementNS(NS, "path");
  svg.appendChild(path);
  root.appendChild(svg);
  var dot = document.createElement("span");
  dot.className = "lva-dot";
  dot.setAttribute("aria-hidden", "true");
  row.appendChild(dot);
  var btn = document.createElement("button");
  btn.type = "button";
  btn.className = "lva-toggle";
  btn.textContent = "Pause";
  root.querySelector(".lva-foot").appendChild(btn);
  // One cycle: three turns; the first two go around, the third stops.
  var steps = [];
  for (var turn = 1; turn <= 3; turn++) {
    for (var i = 0; i < 4; i++) {
      steps.push({ k: "at", i: i, d: i === 3 ? 1700 : 1350 });
      if (i < 3) steps.push({ k: "hop", a: i, b: i + 1, d: 1150 });
    }
    steps.push(turn < 3 ? { k: "wrap", d: 3600 } : { k: "exit", d: 1300 });
  }
  steps.push({ k: "stop", d: 2600 });
  var cycle = 0;
  steps.forEach(function (s) { s.t = cycle; cycle += s.d; });
  var g = null;
  function measure() {
    var o = row.getBoundingClientRect();
    var b = loop.getBoundingClientRect();
    var cs = getComputedStyle(loop);
    var r = parseFloat(cs.borderTopLeftRadius) - 1;
    var x0 = b.left - o.left + 1, y0 = b.top - o.top + 1;
    var x1 = b.right - o.left - 1, y1 = b.bottom - o.top - 1;
    var across = getComputedStyle(ol).getPropertyValue("--lva-across").trim() === "1";
    var c = nums.map(function (n) {
      var q = n.getBoundingClientRect();
      return [q.left + q.width / 2 - o.left, q.top + q.height / 2 - o.top];
    });
    var t = term.getBoundingClientRect();
    var n = next.getBoundingClientRect();
    var a = " " + r + " " + r + " 0 0 ";
    var d;
    if (across) {
      d = "M" + c[0][0] + " " + y0 + "H" + (x1 - r) + "A" + a + "1 " + x1 + " " + (y0 + r) +
        "V" + (y1 - r) + "A" + a + "1 " + (x1 - r) + " " + y1 + "H" + (x0 + r) +
        "A" + a + "1 " + x0 + " " + (y1 - r) + "V" + (y0 + r) + "A" + a + "1 " + (x0 + r) + " " + y0 + "Z";
    } else {
      d = "M" + x0 + " " + c[0][1] + "V" + (y1 - r) + "A" + a + "0 " + (x0 + r) + " " + y1 +
        "H" + (x1 - r) + "A" + a + "0 " + x1 + " " + (y1 - r) + "V" + (y0 + r) +
        "A" + a + "0 " + (x1 - r) + " " + y0 + "H" + (x0 + r) + "A" + a + "0 " + x0 + " " + (y0 + r) + "Z";
    }
    path.setAttribute("d", d);
    g = {
      len: path.getTotalLength(),
      stops: c.map(function (p) { return across ? p[0] - c[0][0] : p[1] - c[0][1]; }),
      term: [t.left + t.width / 2 - o.left, t.top + t.height / 2 - o.top],
      next: [n.left + n.width / 2 - o.left, n.top + n.height / 2 - o.top],
      reach: Math.max(n.width, n.height) / 2 + 24
    };
  }
  function ease(u) { return -(Math.cos(Math.PI * u) - 1) / 2; }
  function at(s) { var p = path.getPointAtLength(s); return [p.x, p.y]; }
  function render(clock) {
    if (!g) return;
    var t = clock % cycle, s = steps[0];
    for (var j = steps.length - 1; j >= 0; j--) if (steps[j].t <= t) { s = steps[j]; break; }
    var e = ease(Math.min(1, (t - s.t) / s.d));
    var p, on = -1, near = false, stop = false;
    if (s.k === "at") { p = at(g.stops[s.i]); on = s.i; }
    else if (s.k === "hop") p = at(g.stops[s.a] + (g.stops[s.b] - g.stops[s.a]) * e);
    else if (s.k === "wrap") {
      p = at(g.stops[3] + (g.len - g.stops[3]) * e);
      near = Math.hypot(p[0] - g.next[0], p[1] - g.next[1]) < g.reach;
    } else if (s.k === "exit") {
      var q = at(g.stops[3]);
      p = [q[0] + (g.term[0] - q[0]) * e, q[1] + (g.term[1] - q[1]) * e];
    } else { p = g.term; stop = true; }
    dot.style.transform = "translate(" + p[0] + "px," + p[1] + "px)";
    stages.forEach(function (st, k) { st.classList.toggle("is-on", k === on); });
    next.classList.toggle("is-near", near);
    exit.classList.toggle("is-stop", stop);
  }
  var reduce = matchMedia("(prefers-reduced-motion: reduce)");
  var clock = 0, last = 0, raf = 0, paused = false, seen = true;
  function tick(now) {
    if (last) clock += Math.min(now - last, 100);
    last = now;
    render(clock);
    raf = requestAnimationFrame(tick);
  }
  function sync() {
    var still = reduce.matches;
    dot.hidden = still;
    btn.hidden = still;
    if (still) {
      stages.forEach(function (st) { st.classList.remove("is-on"); });
      next.classList.remove("is-near");
      exit.classList.remove("is-stop");
    }
    var run = !still && !paused && seen;
    if (run && !raf) { last = 0; raf = requestAnimationFrame(tick); }
    else if (!run && raf) { cancelAnimationFrame(raf); raf = 0; }
  }
  btn.addEventListener("click", function () {
    paused = !paused;
    btn.textContent = paused ? "Play" : "Pause";
    sync();
  });
  if (reduce.addEventListener) reduce.addEventListener("change", sync);
  if (window.IntersectionObserver) {
    new IntersectionObserver(function (es) { seen = es[0].isIntersecting; sync(); }).observe(row);
  }
  function relayout() { measure(); if (!reduce.matches) render(clock); }
  if (window.ResizeObserver) new ResizeObserver(relayout).observe(row);
  else window.addEventListener("resize", relayout);
  relayout();
  sync();
})();
</script>
</div>
</template>
<template id="loopv-b">
<div class="loopv-b">
<style>
/* Variant B: the loop as one large ring, each stage beside its node.
   The stages are an ordered list on a 3 by 3 subgrid; the ring and
   "one turn" sit in its empty middle cell. */
.loopv-b {
  --ring: clamp(19rem, 26vw + 7rem, 32rem);
  --lvb-cycle: 18s;
  position: relative;
  margin: clamp(2.5rem, 5vw, 4rem) 0 clamp(2.75rem, 5vw, 4.25rem);
  display: grid;
  grid-template-columns: minmax(0, 1fr) var(--ring) minmax(0, 1fr);
  grid-template-rows: auto var(--ring) auto;
  --lvb-gap: clamp(1.25rem, 3vw, 2.75rem);
  column-gap: var(--lvb-gap);
  row-gap: clamp(0.75rem, 1.5vw, 1.1rem);
}
/* On wide screens the figure takes the page's full width (measured by
   the script below), so the ring stands in the middle of the page. */
@media (min-width: 64rem) {
  .loopv-b { max-width: none; margin-right: calc(-1 * var(--lvb-room, 0px)); }
}
/* The ring drawing. Decorative: the list carries the words. */
.loopv-b .loopv-b-ring {
  grid-area: 2 / 2;
  display: block;
  width: 100%;
  height: 100%;
  overflow: visible;
}
.loopv-b .lvb-track { fill: none; stroke: var(--line); stroke-width: 1.5; }
.loopv-b .lvb-arc { fill: none; stroke: var(--amber); stroke-width: 2; }
.loopv-b .lvb-head { fill: var(--amber); }
.loopv-b .lvb-node circle { fill: var(--page); stroke: var(--amber); stroke-width: 2; }
.loopv-b .lvb-node text {
  fill: var(--amber);
  font-family: var(--label);
  font-weight: 600;
  font-size: 18px;
  text-anchor: middle;
}
.loopv-b .lvb-tail { fill: none; stroke: var(--amber); stroke-width: 3; stroke-linecap: round; }
.loopv-b .lvb-dot { fill: var(--amber); }
.loopv-b .lvb-orbit { transform-box: view-box; transform-origin: 200px 200px; }
.loopv-b .lvb-comet { opacity: 0; }
/* One turn of the marker: it eases from node to node, rests inside
   each one while that node is lit, then moves on. */
@keyframes loopv-b-orbit {
  0% { transform: rotate(0deg); animation-timing-function: cubic-bezier(0.33, 0, 0.15, 1); }
  14%, 25% { transform: rotate(90deg); animation-timing-function: cubic-bezier(0.33, 0, 0.15, 1); }
  39%, 50% { transform: rotate(180deg); animation-timing-function: cubic-bezier(0.33, 0, 0.15, 1); }
  64%, 75% { transform: rotate(270deg); animation-timing-function: cubic-bezier(0.33, 0, 0.15, 1); }
  89%, 100% { transform: rotate(360deg); }
}
@keyframes loopv-b-comet {
  0% { opacity: 0; }
  10%, 46% { opacity: 1; }
  56%, 100% { opacity: 0; }
}
@keyframes loopv-b-lit {
  0%, 11% { fill: var(--page); }
  15%, 24% { fill: var(--amber); }
  29%, 100% { fill: var(--page); }
}
@keyframes loopv-b-lit-text {
  0%, 11% { fill: var(--amber); }
  15%, 24% { fill: var(--amber-ink); }
  29%, 100% { fill: var(--amber); }
}
@media (prefers-reduced-motion: no-preference) {
  .loopv-b .lvb-orbit { animation: loopv-b-orbit var(--lvb-cycle) infinite; }
  .loopv-b .lvb-comet { animation: loopv-b-comet calc(var(--lvb-cycle) / 4) ease-out infinite; }
  .loopv-b .lvb-node circle { animation: loopv-b-lit var(--lvb-cycle) ease-out infinite; }
  .loopv-b .lvb-node text { animation: loopv-b-lit-text var(--lvb-cycle) ease-out infinite; }
  .loopv-b .lvb-n1 :is(circle, text) { animation-delay: calc(var(--lvb-cycle) / -4); }
  .loopv-b .lvb-n3 :is(circle, text) { animation-delay: calc(var(--lvb-cycle) * -0.75); }
  .loopv-b .lvb-n4 :is(circle, text) { animation-delay: calc(var(--lvb-cycle) / -2); }
}
@keyframes loopv-b-lit-name {
  0%, 11% { text-decoration-color: var(--amber-dim); }
  15%, 24% { text-decoration-color: var(--amber); }
  29%, 100% { text-decoration-color: var(--amber-dim); }
}
@media (prefers-reduced-motion: no-preference) {
  /* Each stage's underline brightens while its node is lit. */
  .loopv-b .loopv-b-list > li .loopv-b-name { animation: loopv-b-lit-name var(--lvb-cycle) ease-out infinite; }
  .loopv-b .loopv-b-list > li:nth-child(1) .loopv-b-name { animation-delay: calc(var(--lvb-cycle) / -4); }
  .loopv-b .loopv-b-list > li:nth-child(3) .loopv-b-name { animation-delay: calc(var(--lvb-cycle) * -0.75); }
  .loopv-b .loopv-b-list > li:nth-child(4) .loopv-b-name { animation-delay: calc(var(--lvb-cycle) / -2); }
  .loopv-b .loopv-b-name:is(:hover, :focus-visible) { animation: none; }
}
@media (prefers-reduced-motion: reduce) {
  .loopv-b .lvb-orbit { display: none; }
  .loopv-b .loopv-b-name { transition: none; }
}
/* Pointing at a stage, or focusing its link, lights its node. */
.loopv-b:has(.loopv-b-list > li:nth-child(1):is(:hover, :focus-within)) .lvb-n1 circle,
.loopv-b:has(.loopv-b-list > li:nth-child(2):is(:hover, :focus-within)) .lvb-n2 circle,
.loopv-b:has(.loopv-b-list > li:nth-child(3):is(:hover, :focus-within)) .lvb-n3 circle,
.loopv-b:has(.loopv-b-list > li:nth-child(4):is(:hover, :focus-within)) .lvb-n4 circle { animation: none; fill: var(--amber); }
.loopv-b:has(.loopv-b-list > li:nth-child(1):is(:hover, :focus-within)) .lvb-n1 text,
.loopv-b:has(.loopv-b-list > li:nth-child(2):is(:hover, :focus-within)) .lvb-n2 text,
.loopv-b:has(.loopv-b-list > li:nth-child(3):is(:hover, :focus-within)) .lvb-n3 text,
.loopv-b:has(.loopv-b-list > li:nth-child(4):is(:hover, :focus-within)) .lvb-n4 text { animation: none; fill: var(--amber-ink); }
/* "one turn" in the middle of the ring, as real text. */
.loopv-b .loopv-b-center {
  grid-area: 2 / 2;
  place-self: center;
  margin: 0;
  max-width: 64%;
  text-align: center;
  line-height: 1.1;
}
.loopv-b .loopv-b-turn {
  display: block;
  font-family: var(--display);
  font-weight: 600;
  font-size: clamp(1.9rem, 1.1rem + 1.6vw, 2.75rem);
  letter-spacing: 0.02em;
  color: var(--ivory);
}
.loopv-b .loopv-b-until {
  display: block;
  margin-top: 0.75rem;
  font-family: var(--label);
  font-size: clamp(0.78rem, 0.7rem + 0.2vw, 0.9rem);
  line-height: 1.4;
  letter-spacing: 0.08em;
  color: var(--soft);
}
/* The four stages share the ring's grid, each beside its node. */
.loopv-b .loopv-b-list {
  grid-area: 1 / 1 / -1 / -1;
  display: grid;
  grid-template-columns: subgrid;
  grid-template-rows: subgrid;
  list-style: none;
  margin: 0;
  padding: 0;
  pointer-events: none;
}
.loopv-b .loopv-b-list > li {
  margin: 0;
  pointer-events: auto;
  font-size: clamp(0.95rem, 0.88rem + 0.15vw, 1.02rem);
  line-height: 1.55;
  color: var(--stone);
  text-wrap: pretty;
}
.loopv-b .loopv-b-list > li::marker { content: none; }
.loopv-b .loopv-b-list > li:nth-child(1) { grid-area: 1 / 2; align-self: end; justify-self: center; max-width: 25rem; text-align: center; }
.loopv-b .loopv-b-list > li:nth-child(2) { grid-area: 2 / 3; align-self: center; justify-self: start; max-width: 19rem; }
.loopv-b .loopv-b-list > li:nth-child(3) { grid-area: 3 / 2; align-self: start; justify-self: center; max-width: 25rem; text-align: center; }
.loopv-b .loopv-b-list > li:nth-child(4) { grid-area: 2 / 1; align-self: center; justify-self: end; max-width: 19rem; text-align: end; }
.loopv-b .loopv-b-list p { margin: 0; }
.loopv-b .loopv-b-list > li:nth-child(odd) p { text-wrap: balance; }
.loopv-b .loopv-b-name {
  display: inline-block;
  margin-bottom: 0.35rem;
  font-family: var(--label);
  font-weight: 600;
  font-size: clamp(1.15rem, 0.95rem + 0.45vw, 1.4rem);
  line-height: 1.3;
  letter-spacing: -0.005em;
  color: var(--ivory);
  text-decoration: underline;
  text-decoration-color: var(--amber-dim);
  text-decoration-thickness: 1px;
  text-underline-offset: 0.28em;
  transition: text-decoration-color 0.2s ease-out;
}
.loopv-b .loopv-b-name:hover { text-decoration-color: var(--amber); text-decoration-thickness: 2px; }
.loopv-b .loopv-b-name:focus-visible { outline: 2px solid var(--amber); outline-offset: 4px; border-radius: 2px; }
/* The way back, set beside the arc from stage 4 to stage 1. */
.loopv-b .loopv-b-back {
  grid-area: 2 / 1;
  align-self: start;
  justify-self: end;
  max-width: 14rem;
  margin: 0 calc(-1 * var(--lvb-gap) - var(--ring) * 0.15) 0 0;
  padding-top: calc(var(--ring) * 0.07);
  font-family: var(--label);
  font-size: 0.88rem;
  line-height: 1.45;
  text-align: end;
  text-wrap: pretty;
  color: var(--soft);
}
/* Beside the ring, the arc's own arrowhead shows the way back; the
   glyph is for the narrow layout's track. */
.loopv-b .loopv-b-back svg { display: none; overflow: visible; }
.loopv-b .loopv-b-back path { fill: none; stroke: var(--amber); stroke-width: 1.7; stroke-linecap: round; stroke-linejoin: round; }
@media (min-width: 48rem) and (max-width: 59.99rem) {
  .loopv-b .loopv-b-back {
    max-width: 12.5rem;
    margin-right: calc(-1 * var(--lvb-gap) - var(--ring) * 0.07);
    padding-top: calc(var(--ring) * 0.02);
    font-size: 0.82rem;
  }
}
/* Narrow screens: a smaller ring above the stages as a numbered track. */
@media (max-width: 47.99rem) {
  .loopv-b {
    --ring: min(18rem, 78vw);
    grid-template-columns: minmax(0, 1fr);
    grid-template-rows: auto;
    row-gap: 2rem;
    margin: 2rem 0 2.5rem;
  }
  .loopv-b .loopv-b-ring,
  .loopv-b .loopv-b-center { grid-area: 1 / 1; }
  .loopv-b .loopv-b-ring { width: var(--ring); height: var(--ring); justify-self: center; }
  .loopv-b .lvb-track { stroke-width: 2.2; }
  .loopv-b .lvb-arc { stroke-width: 3; }
  .loopv-b .lvb-node circle { stroke-width: 3; }
  .loopv-b .lvb-node text { font-size: 24px; }
  .loopv-b .lvb-tail { stroke-width: 4.5; }
  .loopv-b .loopv-b-turn { font-size: 1.65rem; }
  .loopv-b .loopv-b-until { font-size: 0.72rem; margin-top: 0.5rem; }
  .loopv-b .loopv-b-list {
    grid-area: 2 / 1;
    display: block;
    counter-reset: loopv-b;
    pointer-events: auto;
  }
  .loopv-b .loopv-b-list > li:nth-child(n) {
    max-width: none;
    padding: 0 0 1.5rem 3.25rem;
    text-align: start;
    position: relative;
    counter-increment: loopv-b;
  }
  .loopv-b .loopv-b-list > li::before {
    content: counter(loopv-b);
    position: absolute;
    left: 0;
    top: 0;
    width: 2.1rem;
    height: 2.1rem;
    display: grid;
    place-items: center;
    border: 2px solid var(--amber);
    border-radius: 50%;
    background: var(--page);
    color: var(--amber);
    font-family: var(--label);
    font-weight: 600;
    font-size: 1rem;
    line-height: 1;
  }
  .loopv-b .loopv-b-list > li::after {
    content: "";
    position: absolute;
    left: calc(1.05rem - 1px);
    top: 2.5rem;
    bottom: 0.4rem;
    border-left: 2px solid var(--amber);
  }
  .loopv-b .loopv-b-name { margin-bottom: 0.1rem; font-size: 1.15rem; line-height: 2.1rem; }
  .loopv-b .loopv-b-back {
    grid-area: 3 / 1;
    justify-self: stretch;
    max-width: none;
    margin: -2rem 0 0;
    padding: 0.1rem 0 0 3.25rem;
    text-align: start;
    position: relative;
  }
  .loopv-b .loopv-b-back svg {
    display: block;
    position: absolute;
    left: 0.35rem;
    top: 0.1rem;
    width: 1.35rem;
    height: 1.35rem;
    margin: 0;
  }
}
</style>
<svg class="loopv-b-ring" viewBox="0 0 400 400" aria-hidden="true" focusable="false">
  <defs><marker id="loopv-b-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0 10 5 0 10z" class="lvb-head"/></marker></defs>
  <circle cx="200" cy="200" r="170" class="lvb-track"/>
  <path d="M232.44 33.12 A170 170 0 0 1 366.88 167.56" class="lvb-arc" marker-end="url(#loopv-b-arrow)"/>
  <path d="M366.88 232.44 A170 170 0 0 1 232.44 366.88" class="lvb-arc" marker-end="url(#loopv-b-arrow)"/>
  <path d="M167.56 366.88 A170 170 0 0 1 33.12 232.44" class="lvb-arc" marker-end="url(#loopv-b-arrow)"/>
  <path d="M33.12 167.56 A170 170 0 0 1 167.56 33.12" class="lvb-arc" marker-end="url(#loopv-b-arrow)"/>
  <g class="lvb-orbit"><g class="lvb-comet"><path d="M152.37 36.81 A170 170 0 0 1 200 30" class="lvb-tail"/><circle cx="200" cy="30" r="6" class="lvb-dot"/></g></g>
  <g class="lvb-node lvb-n1"><circle cx="200" cy="30" r="20"/><text x="200" y="36.5">1</text></g>
  <g class="lvb-node lvb-n2"><circle cx="370" cy="200" r="20"/><text x="370" y="206.5">2</text></g>
  <g class="lvb-node lvb-n3"><circle cx="200" cy="370" r="20"/><text x="200" y="376.5">3</text></g>
  <g class="lvb-node lvb-n4"><circle cx="30" cy="200" r="20"/><text x="30" y="206.5">4</text></g>
</svg>
<p class="loopv-b-center"><span class="loopv-b-turn">one turn</span> <span class="loopv-b-until">until an answer or a limit</span></p>
<ol class="loopv-b-list">
  <li><a class="loopv-b-name" href="/build/context-management/">Build the request</a><p>Decide what the model sees: instructions, the conversation so far, tool results, and the tools it may call.</p></li>
  <li><a class="loopv-b-name" href="/basics/calling-a-model/">Call the model</a><p>Send the request and read the reply, with a deadline and a count of what it cost.</p></li>
  <li><a class="loopv-b-name" href="/basics/tool-calling/">Run the tools</a><p>Carry out the calls the model asked for, and add each result to the conversation.</p></li>
  <li><a class="loopv-b-name" href="/production/stop-reasons/">Decide</a><p>Stop with an answer, stop at a limit, or go around again.</p></li>
</ol>
<p class="loopv-b-back"><svg viewBox="0 0 20 20" aria-hidden="true" focusable="false"><path d="M15 18 C15 10 6 12 6 3.5"/><path d="M2.5 7 6 3.5 9.5 7"/></svg>Then back to stage 1, with everything that just happened in the conversation.</p>
<script>
// Let the figure use the page's full width where the text column is
// narrower than the page, so the ring stands in the middle.
(function () {
  var box = document.currentScript.parentNode;
  var col = box.parentNode;
  var wrap = box.closest(".wrap");
  if (!wrap || !window.ResizeObserver) return;
  function fit() {
    var cs = getComputedStyle(wrap);
    var inner = wrap.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight);
    var room = Math.max(0, Math.floor(inner - col.getBoundingClientRect().width));
    box.style.setProperty("--lvb-room", room + "px");
  }
  new ResizeObserver(fit).observe(wrap);
})();
</script>
</div>
</template>
<template id="loopv-c">
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
.loopv-c .lvc-arc { fill: none; stroke: var(--amber); stroke-width: 2; }
.loopv-c .lvc-head { fill: var(--amber); }
/* Strokes stay thin however large the ring is drawn. */
.loopv-c .lvc-track, .loopv-c .lvc-arc, .loopv-c .lvc-node circle, .loopv-c .lvc-halo { vector-effect: non-scaling-stroke; }
.loopv-c .lvc-node circle { fill: var(--page); stroke: var(--amber); stroke-width: 2; }
.loopv-c .lvc-node text { fill: var(--amber); font-family: var(--label); font-weight: 600; font-size: 16px; text-anchor: middle; }
.loopv-c .lvc-label { fill: var(--soft); font-family: var(--label); font-weight: 600; font-size: 15px; text-anchor: middle; }
.loopv-c .lvc-label.lvc-start { text-anchor: start; }
.loopv-c .lvc-label.lvc-end { text-anchor: end; }
.loopv-c .lvc-center { fill: var(--ivory); font-family: var(--display); font-size: 24px; text-anchor: middle; }
.loopv-c .lvc-sub { fill: var(--soft); font-family: var(--label); font-size: 13px; text-anchor: middle; letter-spacing: 0.06em; }
.loopv-c .lvc-node { cursor: pointer; }
.loopv-c .lvc-node.is-on circle { fill: var(--amber); }
.loopv-c .lvc-node.is-on text { fill: var(--amber-ink); }
.loopv-c .lvc-label.is-on { fill: var(--ivory); }
.loopv-c .lvc-halo { fill: none; stroke: var(--amber); stroke-width: 2; }
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
  border: 2px solid var(--amber);
  border-radius: 50%;
  background: var(--page);
  color: var(--amber);
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
.loopv-c .lvc-go { color: var(--amber); font-weight: 500; }
.loopv-c .lvc-desc { grid-column: 2; margin: 0; color: var(--stone); font-size: 0.97rem; }
.loopv-c .lvc-list > li:hover,
.loopv-c .lvc-list > li.is-on { background: var(--panel); border-color: var(--line); }
.loopv-c .lvc-list > li:hover .lvc-num,
.loopv-c .lvc-list > li.is-on .lvc-num { background: var(--amber); color: var(--amber-ink); }
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
.loopv-c .lvc-back path { fill: none; stroke: var(--amber); stroke-width: 1.5; stroke-linecap: round; stroke-linejoin: round; }
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
    /* Wider than the text column where the page has room. */
    width: max(100%, min(calc(80rem - 2 * var(--gutter)), calc(100vw - 2 * var(--gutter) - 20px)));
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
      <defs><marker id="lvc-arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M0 0 10 5 0 10z" class="lvc-head"/></marker></defs>
      <circle cx="200" cy="200" r="120" class="lvc-track"/>
      <path d="M232 84 A120 120 0 0 1 316 168" class="lvc-arc" marker-end="url(#lvc-arrow)"/>
      <path d="M316 232 A120 120 0 0 1 232 316" class="lvc-arc" marker-end="url(#lvc-arrow)"/>
      <path d="M168 316 A120 120 0 0 1 84 232" class="lvc-arc" marker-end="url(#lvc-arrow)"/>
      <path d="M84 168 A120 120 0 0 1 168 84" class="lvc-arc" marker-end="url(#lvc-arrow)"/>
      <g class="lvc-marker"><circle cx="200" cy="200" r="145" class="lvc-box"/><circle cx="200" cy="80" r="26" class="lvc-halo"/></g>
      <g class="lvc-node"><circle cx="200" cy="80" r="17"/><text x="200" y="86">1</text></g>
      <g class="lvc-node"><circle cx="320" cy="200" r="17"/><text x="320" y="206">2</text></g>
      <g class="lvc-node"><circle cx="200" cy="320" r="17"/><text x="200" y="326">3</text></g>
      <g class="lvc-node"><circle cx="80" cy="200" r="17"/><text x="80" y="206">4</text></g>
      <text x="200" y="44" class="lvc-label" data-i="0">Build the request</text>
      <text x="200" y="368" class="lvc-label" data-i="2">Run the tools</text>
      <text x="352" y="206" class="lvc-label lvc-start" data-i="1">Call the model</text>
      <text x="48" y="206" class="lvc-label lvc-end" data-i="3">Decide</text>
      <text x="200" y="196" class="lvc-center">one turn</text>
      <text x="200" y="220" class="lvc-sub">until an answer or a limit</text>
    </svg>
  </div>
  <ol class="lvc-list">
    <li><span class="lvc-num" aria-hidden="true">1</span><a class="lvc-name" href="/build/context-management/"><span class="lvc-t">Build the request</span><span class="lvc-go" aria-hidden="true">→</span></a><p class="lvc-desc">Decide what the model sees: instructions, the conversation so far, tool results, and the tools it may call.</p></li>
    <li><span class="lvc-num" aria-hidden="true">2</span><a class="lvc-name" href="/basics/calling-a-model/"><span class="lvc-t">Call the model</span><span class="lvc-go" aria-hidden="true">→</span></a><p class="lvc-desc">Send the request and read the reply, with a deadline and a count of what it cost.</p></li>
    <li><span class="lvc-num" aria-hidden="true">3</span><a class="lvc-name" href="/basics/tool-calling/"><span class="lvc-t">Run the tools</span><span class="lvc-go" aria-hidden="true">→</span></a><p class="lvc-desc">Carry out the calls the model asked for, and add each result to the conversation.</p></li>
    <li><span class="lvc-num" aria-hidden="true">4</span><a class="lvc-name" href="/production/stop-reasons/"><span class="lvc-t">Decide</span><span class="lvc-go" aria-hidden="true">→</span></a><p class="lvc-desc">Stop with an answer, stop at a limit, or go around again.</p></li>
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
    for (var k = 0; k < 4; k++) {
      nodes[k].classList.toggle("is-on", k === i);
      labels[k].classList.toggle("is-on", k === i);
      items[k].classList.toggle("is-on", k === i);
    }
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
</template>
<script>
(function () {
  // Preview of three designs for this section: ?loop=a, b, or c.
  var v = new URLSearchParams(location.search).get("loop");
  if (!/^[abc]$/.test(v || "")) return;
  var t = document.getElementById("loopv-" + v);
  var cur = document.querySelector(".loopv-current");
  if (!t || !cur) return;
  cur.replaceWith(document.importNode(t.content, true));
  if (v === "c") document.documentElement.classList.add("loopv-hero-off");
  var bar = document.createElement("nav");
  bar.className = "loopv-picker";
  bar.setAttribute("aria-label", "Loop section designs");
  ["current", "a", "b", "c"].forEach(function (k) {
    var a = document.createElement("a");
    a.href = k === "current" ? location.pathname : "?loop=" + k;
    a.textContent = k === "current" ? "Current" : k.toUpperCase();
    if (k === v) a.setAttribute("aria-current", "true");
    bar.appendChild(a);
  });
  document.body.appendChild(bar);
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
Here is the loop as the [companion code](https://github.com/blackwell-systems/go-agents-book-code) writes it:

```go include=ch01/agent/loop.go tag=run
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
