---
title: Icons and dividers
description: The site's icon set and dividers, each with its name, at the sizes the site uses, for review.
layout: page
draft: true
---

<style>
.ix-group { margin: var(--space-7) 0 var(--space-3); font-size: var(--fs-group); }
.prose .ix-grid {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: var(--space-3);
  grid-template-columns: repeat(auto-fill, minmax(11rem, 1fr));
}
.prose .ix-grid li { margin: 0; }
.ix-grid li {
  display: grid;
  gap: var(--space-3);
  padding: var(--space-4);
  background: var(--panel);
  border: 1px solid var(--line);
  border-radius: var(--radius);
}
.ix-sizes { display: flex; align-items: end; gap: var(--space-3); min-height: 48px; }
.ix-sizes .icon:nth-child(1) { width: 48px; height: 48px; --icon-sw: 0.75; }
.ix-sizes .icon:nth-child(2) { width: 24px; height: 24px; }
.ix-sizes .icon:nth-child(3) { width: 18px; height: 18px; }
.ix-sizes .icon:nth-child(4) { width: 24px; height: 24px; color: var(--soft); }
.ix-name { font-family: var(--mono); font-size: var(--fs-xs); color: var(--ivory); }
.ix-use { font-size: var(--fs-xs); line-height: var(--lh-snug); color: var(--soft); }
.ix-rules { display: grid; gap: var(--space-5); }
.ix-rules p { margin: 0; font-family: var(--mono); font-size: var(--fs-xs); color: var(--soft); }
</style>

Every icon at 48, 24, and 18 pixels in the accent color, then at 24 pixels with soft strokes and accent details.
The 48 pixel copy draws its strokes at 0.75, so its lines are as fine as the 24 pixel copy's.
Each name is the symbol's ID in `static/img/icons.svg`, and the line under it says where the site uses it.
This page is a draft, so only a build with `-drafts` has it.

<h2 class="ix-group">The loop’s four stages</h2>
<ul class="ix-grid">
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-build"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-build"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-build"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-build"></use></svg></div><span class="ix-name">stage-build</span><span class="ix-use">Build the request. Not placed yet</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-call"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-call"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-call"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-call"></use></svg></div><span class="ix-name">stage-call</span><span class="ix-use">Call the model. Not placed yet</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-tools"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-tools"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-tools"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-tools"></use></svg></div><span class="ix-name">stage-tools</span><span class="ix-use">Run the tools. Not placed yet</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-decide"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-decide"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-decide"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#stage-decide"></use></svg></div><span class="ix-name">stage-decide</span><span class="ix-use">Decide. Not placed yet</span></li>
</ul>

<h2 class="ix-group">The sections</h2>
<ul class="ix-grid">
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-basics"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-basics"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-basics"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-basics"></use></svg></div><span class="ix-name">section-basics</span><span class="ix-use">Basics: the home page’s section cards, the section page, and its guides’ eyebrows</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-build"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-build"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-build"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-build"></use></svg></div><span class="ix-name">section-build</span><span class="ix-use">Build: the home page’s section cards, the section page, and its guides’ eyebrows</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-production"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-production"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-production"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-production"></use></svg></div><span class="ix-name">section-production</span><span class="ix-use">Production: the home page’s section cards, the section page, and its guides’ eyebrows</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-ecosystem"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-ecosystem"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-ecosystem"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-ecosystem"></use></svg></div><span class="ix-name">section-ecosystem</span><span class="ix-use">Go ecosystem: the home page’s section cards, the section page, and its guides’ eyebrows</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-examples"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-examples"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-examples"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-examples"></use></svg></div><span class="ix-name">section-examples</span><span class="ix-use">Examples: the home page’s section cards, the section page, and its guides’ eyebrows</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-research"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-research"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-research"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#section-research"></use></svg></div><span class="ix-name">section-research</span><span class="ix-use">Research: the home page’s section cards, the section page, and its guides’ eyebrows</span></li>
</ul>

<h2 class="ix-group">Content blocks</h2>
<ul class="ix-grid">
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-definition"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-definition"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-definition"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-definition"></use></svg></div><span class="ix-name">block-definition</span><span class="ix-use">A definition block’s label</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-result"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-result"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-result"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-result"></use></svg></div><span class="ix-name">block-result</span><span class="ix-use">A result block’s label</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-fit"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-fit"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-fit"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-fit"></use></svg></div><span class="ix-name">block-fit</span><span class="ix-use">The “When to use it” heading</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-reach"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-reach"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-reach"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-reach"></use></svg></div><span class="ix-name">block-reach</span><span class="ix-use">A fit block’s “Reach for it”</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-leave"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-leave"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-leave"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-leave"></use></svg></div><span class="ix-name">block-leave</span><span class="ix-use">A fit block’s “Leave it out”, in the soft color</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-try"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-try"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-try"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-try"></use></svg></div><span class="ix-name">block-try</span><span class="ix-use">The “Try it” heading</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-watch"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-watch"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-watch"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-watch"></use></svg></div><span class="ix-name">block-watch</span><span class="ix-use">The “Mistakes to avoid” and “What to watch” headings</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-production"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-production"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-production"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#block-production"></use></svg></div><span class="ix-name">block-production</span><span class="ix-use">In production. Not placed yet</span></li>
</ul>

<h2 class="ix-group">Utility</h2>
<ul class="ix-grid">
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#external"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#external"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#external"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#external"></use></svg></div><span class="ix-name">external</span><span class="ix-use">A link off the site. Not placed yet</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#copy"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#copy"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#copy"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#copy"></use></svg></div><span class="ix-name">copy</span><span class="ix-use">The copy button on code listings</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#source"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#source"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#source"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#source"></use></svg></div><span class="ix-name">source</span><span class="ix-use">Source code, a repository. Not placed yet</span></li>
<li><div class="ix-sizes"><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#arrow"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#arrow"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#arrow"></use></svg><svg class="icon" aria-hidden="true"><use href="/img/icons.svg#arrow"></use></svg></div><span class="ix-name">arrow</span><span class="ix-use">The pager’s Previous and Next, flipped for Previous</span></li>
</ul>

<h2 class="ix-group">Dividers</h2>
<div class="ix-rules">
<div class="rule" aria-hidden="true"><svg class="rule-mark"><use href="/img/icons.svg#rule"></use></svg></div>
<p>rule: between the home page’s major sections</p>
<div class="rule rule-small" aria-hidden="true"><svg class="rule-mark"><use href="/img/icons.svg#rule-small"></use></svg></div>
<p>rule-small: the mark across the footer’s top edge, on every page</p>
</div>
