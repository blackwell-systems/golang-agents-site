# Share cards

Every page except the 404 page has its own share image: a 1200 x 630 JPEG in `static/og/`, named after the page's path (`og/home.jpg`, `og/basics.jpg` for a section's page, `og/basics/agent-loop.jpg` for a guide). The build points each page's `og:image` and `twitter:image` at its card, with the card's size and alt text, and falls back to the generic `static/og-v3.jpg` for a page whose card file is missing.

The cards are rendered by this tool, never by the build, so a build stays fast. They are committed, like the art in `static/img/art/`.

## Regenerate

From the repository root:

```
go run ./tools/og
```

Run it after adding a page, or after changing a page's title, group, art, or opening `result` block. The build says when it is needed: it prints how many pages have no card and how many cards no longer match their page.

The tool asks the build for each card's description (`go run ./cmd/build -og-spec -`, which reads only the Markdown), fills `card.html` with it, has headless Chrome screenshot the page, and writes the JPEG with ImageMagick. `manifest.json` records a hash of each card's inputs (its description, `card.html`, the fonts, its art, and `version` in `main.go`), so an unchanged card is skipped; editing `card.html` renders every card again. Cards for pages that no longer exist are deleted.

Flags:

| Flag | What |
| --- | --- |
| `-force` | Render every card, changed or not |
| `-only text` | Render only cards whose file name contains `text`, such as `-only findings/` |
| `-keep dir` | Also write each card's HTML into `dir`, to open in a browser |
| `-chrome path` | The browser to use (or set `$OG_CHROME`) |
| `-j n` | Cards rendered at once (default: half the CPUs, at most 4) |
| `-spec file` | Read the descriptions from a file instead of running the build |

## Requirements

- ImageMagick 7 (`magick`).
- Headless Chrome. The tool looks for `chrome-headless-shell`, `google-chrome`, or `chromium` on the `PATH`, Google Chrome or Chromium in `/Applications`, and a `chrome-headless-shell` downloaded by Puppeteer or Playwright. To get one: `command npx @puppeteer/browsers install chrome-headless-shell@stable` (it lands in `~/.cache/puppeteer`, which is searched; in this user's shell `npx` is wrapped, hence `command`).

## What a card shows

`cmd/build/og.go` decides the content (`shareCards`); `card.html` and `sizesFor` in `main.go` decide the look.

- The site's name with its ring mark at the top, `golang-agents.com` at the foot.
- A label in small capitals: the section and the page's group (`Basics · Get oriented`); `Section · 17 guides` on a section's page; none outside the sections.
- The title, large. A title with a colon is split there: the part before it is the title, the part after it the line underneath. A section's page, and a page outside the sections with no colon, take that line from the start of their description.
- A findings page whose body opens with a `result` block shows its figure and unit big, above the title.
- The page's art (front matter `art:`, else its section's) as a plate filling the right side, behind a 4 px amber rule; the home page uses `intro`, the book page its cover, inset. A page with no art gets the ring mark, large, instead.

The text is fitted in the browser: the largest sizes within `sizesFor`'s ranges at which the title and the line under it each take at most three lines, no word runs past the column, and the label and figure each fit on one line. If nothing fits at the smallest sizes, the page paints its corner magenta, and the tool refuses the screenshot and names the card; shorten the title or lower the smallest size.

The design rules: the site's dark palette (the `:root` colors in `static/style.css`), Cinzel for the site's name, Space Grotesk for titles and labels, Inter for the line underneath; flat colors only, with no glow, gradients, or shadows.

## Fonts

The cards use the site's own WOFF2 files from `static/fonts/` (Cinzel, Inter, and Space Grotesk, all under the SIL Open Font License; the licenses are next to them). The tool inlines them into each temporary page as data URLs, since Chrome won't load fonts from other `file://` paths; nothing is converted or copied.
