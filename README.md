# golang-agents.com

A code-first guide to building AI agents in Go, with a page for the book *Building AI Agents in Go* by Dayna Blackwell.

The pages are Markdown under `content/`, rendered by a small Go program (`cmd/build`) into `dist/`, which Cloudflare serves as static assets.

## Build and preview

The build reads code excerpts from the companion code, so it needs a copy of it: the book repository's `code/` directory next to this one (the default), or a clone of [go-agents-book-code](https://github.com/blackwell-systems/go-agents-book-code), passed with `-code`.

```
go run ./cmd/build                     # or: go run ./cmd/build -code ../go-agents-book-code/code
python3 -m http.server 8000 -d dist    # then open http://localhost:8000/
```

The build replaces `dist/`, copies `static/`, renders every page, writes `sitemap.xml`, `robots.txt`, and `feed.xml`, and then checks every internal link, asset reference, and `#fragment` in the output, and every link into the companion repository's `code/` tree. A broken link, a missing excerpt tag, an unknown front matter key, or an em dash in a page fails the build.

A page with `draft: true` is left out entirely: no HTML, no sitemap or feed entry, and no place in the nav or its section's list. Drafts stay on disk for review.

To review drafts, build them into a separate directory with `go run ./cmd/build -drafts -out /tmp/site-drafts` (add `-book ""` to skip the copy check, which several drafts don't pass yet). Never deploy a `-drafts` build.

The build also runs a copy check: no published page may share a run of 10 or more consecutive words with the book's chapters, since there is no free web edition of the book. It reads the chapters from `../go-agents-book/book/chapters` (set `-book` to another path; `-book ""` skips the check with a warning). Code blocks, inline code, table rows, HTML tags, and link targets are left out, and so are the author's bio and copyright page. The threshold is `copyRun` in `cmd/build/copycheck.go`.

Each page's social share image is its own card in `static/og/`, rendered separately by `go run ./tools/og` (see `tools/og/README.md`). The build warns when a page has no card or its card no longer matches the page.

The icons and dividers are symbols in one sprite, `static/img/icons.svg`: line work with 1.5 strokes on a 24 grid, its colors from the theme (`currentColor` and `--icon-accent`). A template draws one with `{{icon "name"}}`, and Go code with `icon("name")` (`cmd/build/icons.go`). The build fails if the sprite isn't well-formed XML, which leaves a browser drawing none of it, and the link check fails on a symbol the sprite doesn't have. `content/icons.md` is a draft that shows every icon and divider with where it's used: build with `-drafts` and open `/icons/`.

`go test ./...` tests the generator.

## Deploy

Push to `main`. The workflow in `.github/workflows/pages.yml` builds the site and publishes `dist/` to GitHub Pages, which serves golang-agents.com:

```console
go run ./cmd/build          # local build: runs the copy check against the book
git push                    # GitHub Actions builds again and publishes
```

The workflow checks out the public companion code at the tag `codeRef` names in `cmd/build/main.go`, downloads its modules so the hover popovers can read third-party packages, and builds with `-book ""`: the copy check needs the book's chapters, which aren't public, so it runs on local builds only. Always build locally before pushing.

Pages serves `dist/` as plain files: `/basics/agent-loop/` is `dist/basics/agent-loop/index.html`, and `404.html` answers unknown paths. Old URLs from renamed sections get small redirect pages, written by the build from the `moved` and `movedPages` lists in `cmd/build/main.go`.

## Layout

| Path | What |
| --- | --- |
| `content/` | The pages, in Markdown with front matter. `content/PLAN.md` is the content plan, not a page |
| `content/<section>/_index.md` | A section's own page; the other files in the directory are its guides |
| `templates/` | `base.html` (head, header, footer, and the reading-path links) and one file per layout: `home`, `section`, `article`, `book`, `glossary`, `page`, `work` (the Work with me page and its contact form), `search` |
| `static/` | Copied as is: `style.css`, `fonts/`, `img/`, `sample/`, the icons, `og/` (each page's share card), and `og-v3.jpg` (the generic share image) |
| `cmd/build/` | The generator: goldmark for Markdown, chroma for highlighting, `html/template` for layouts |
| `tools/og/` | Renders the share cards into `static/og/`; see its README |
| `dist/` | The built site (not committed) |

The sections are fixed in `cmd/build/main.go`: The loop (`/loop/`), Design (`/design/`), Production, Run it (`/run/`), Providers and frameworks (`/providers/`), and Findings. Four of them were renamed on 2026-10-18 from Foundations, Patterns, Examples, and Ecosystem; `moved` in the same file lists the old directories, and the build writes `dist/_redirects`, which Cloudflare reads to send every old URL to its new one with a 301.

## Writing a page

Front matter is `key: value` lines between `---` lines:

```
---
title: The agent loop in Go
short: The agent loop                  # optional; for nav and lists
description: One or two sentences for search results and lists.
chapters: 1, 4, 5                      # the book chapters it draws on
stage: all                             # the loop stage it works on: 1 to 4, or all
date: 2026-10-18                       # required for guides
updated: 2026-10-20                    # optional
weight: 20                             # order in its section
book: A custom closing line, in Markdown.   # optional
work: true                             # optional; adds the "Building this at work?" line
art: ch04                              # optional; the book's opener shown beside the title (cmd/build/art.go)
draft: true                            # optional; keeps the page out of the build
---
```

(The comments above are for this README; front matter lines take no comments after the value.)

Code comes from the companion code, never pasted. An empty fenced block with `include=` pulls a tagged excerpt (the same `// tag::name[]` markers the book uses), or a range of lines from any file, and its caption links to those lines on GitHub:

````
```go include=ch01/agent/loop.go tag=run
```

```text include=ch04/testdata/out/fix.txt lines=18..40 title="go run ./ch04, turns 3 and 4"
```
````

A link to `code:ch04/agent/gotest.go` becomes a link to that file in the repository, after the build checks that it exists. Glossary terms link as `/glossary/#term`.

A link to `page:/production/approval/` (a `#fragment` may follow) names another page on the site. If that page is published it becomes an ordinary link; if it is still a draft, the link text appears without a link, and the link appears by itself when the page is published. A URL that names no page, published or draft, fails the build. Use it for any link to a guide that may not be out yet; a plain link to a draft fails the link check.

### Results, fit lists, and counts

A measured number never appears without its method. A `result` block states one result with its task, models, runs, and date, all required, and the build fails without them:

````
```result
figure: 79
unit: of 80 replies
task: the crash-triage agent's first request, with a poisoned tool description
models: four (claude-haiku-5.5, gpt-5.6-luna, glm-5.3-flash, deepseek-v4.1-flash)
runs: ten per model and condition
date: October 2026
source: Chapter 10
link: /findings/tool-description-poisoning/
more: The finding

What the number means, in a sentence or two of Markdown.
```
````

`unit`, `link`, and `more` (the link's text) are optional. Wrap several in `<div class="result-cards">` for a row of cards.

Design and production pages say when to use what they describe with a `fit` block. Every reason has to cite the chapter whose result backs it, or the build fails:

````
```fit
reach: The batch has many groups that each need many turns (chapter 9).
leave: One agent can hold the batch with a context policy; fan-out cost 25 to 60 percent more there (chapter 9).
```
````

Counts about the code are computed from the local copy, never typed: `{{code files}}`, `{{code lines}}`, `{{code tests}}`, `{{code test-files}}`, and `{{code packages}}`, optionally followed by a directory under `code/` (`{{code tests ch04}}`), and `{{code go-version}}` from `code/go.mod`. Counts skip `testdata` directories, as the `go` command does. A `catalog` block renders the examples table on `/run/`, one line per directory (`dir | what it shows | command | guide`), with the counts filled in.

The copy check reads the prose inside all three blocks.

Code links point at the `v1.0` tag of the code repository (`codeRef` in `cmd/build/main.go`, the one setting for it), with line ranges computed from the local copy, so build from a copy that matches that tag. Move it to a new tag with each edition.

### Reading paths, the pager, and search

`content/start.md` (`/start/`) holds the reading paths, one `path` block each:

````
```path
id: newcomer
title: Learning how agents work
step: /loop/what-is-an-agent/
step: /run/ch01-scripted-loop/

Who the path suits, in Markdown. Its first sentence is shown on the home page.
```
````

Each `step` is a page's URL. A step whose page is a draft is listed without a link, as being reviewed, and skipped by the links between steps, so a path fills in as pages are published; a URL that names no page fails the build. Every page on a path shows, at its foot, its step number on each path it belongs to and links to the nearest published steps before and after it. The nav's "Start here" item appears when the start page has paths.

The Previous and Next links at the foot of a guide follow the reading order across the whole site: the sections in nav order, and within each the guides by weight, so the last guide of one section leads to the first of the next.

Section pages and the home page count the drafts held back, so a section that is still filling says how many guides are being reviewed.

Search is static. The build writes `search.json` (each page's title, section, description, and h2 and h3 headings with their ids), and `static/search.js` searches it on `/search/` (`content/search.md`, layout `search`), which lists every page by section for readers without JavaScript. The header's search icon links there.

## Images and fonts

The book's art comes from the book repository too. `tools/art.sh` writes each opener (`intro`, `part1` to `part3`, `ch01` to `ch17`) into `static/img/art/` as AVIF, WebP, and JPEG at 480, 720, and 1080 px wide, and the Gopher badge as AVIF, WebP, and PNG at 240 and 480:

```
tools/art.sh            # reads ../go-agents-book/book/images
```

`cmd/build/art.go` holds each picture's alt text and renders the `<picture>` elements, with width and height so nothing shifts as an image loads. A page names its picture with `art:` in its front matter, each section's is set in `cmd/build/main.go`, and the build fails if a name is unknown or one of its files is missing.


The cover images, the social card, and the icons come from the book's cover. When the cover changes, regenerate them from the book repository into `static/`:

```
cd ../go-agents-book
tools/site-assets.sh ../golang-agents-site/static
```

It needs ImageMagick 7, `avifenc`, and `cwebp`. The CSS, the fonts, and `favicon.svg` are edited here directly.

## Rules for changes

- This repository is public. Keep anything private out of it: no keys, no unpublished chapter text, no planning notes beyond `content/PLAN.md`.
- There is no free web edition of the book. Pages are original and short, answer one question, and show small excerpts from the public, Apache 2.0 companion code; the book has the full build, the tests, and the depth.
- Every figure comes from the book or the code, with its chapter cited. No new measurements.
- The book's prose rules apply: no em dashes, no hype, nothing disparaging about other languages or frameworks.
- The book appears in the nav, on `/book/`, and in one line at the end of each guide.
- Never use the Go logo; "Go" is plain text. The footer keeps the non-affiliation line and the gopher credit.
- Keep the site free of external requests: fonts, images, and scripts are served from here. The one exception is `/work-with-me/`, whose contact form loads Cloudflare Turnstile from `challenges.cloudflare.com` and posts to `https://blackwell-systems.com/api/contact` (the same endpoint as blackwell-systems.com/contact, which allows this origin).
- The site names no rates or prices for consulting; the book page's $29 is the only price.
- The theme follows the visitor's system setting (`prefers-color-scheme`); the header toggle overrides it and remembers the choice in the browser. The light theme is the light edition's palette.
