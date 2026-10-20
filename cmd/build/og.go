package main

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"
)

// A shareCard is a page's own social share image (Open Graph and
// Twitter card), 1200 x 630. The build describes each card here; the
// separate tool tools/og renders them into static/og/, so rendering
// never slows a build. A page whose card file is missing falls back to
// the generic image, ogImage.
type shareCard struct {
	File   string `json:"file"`             // under static/, such as "og/basics/agent-loop.jpg"
	Layout string `json:"layout"`           // home, book, section, or page
	Label  string `json:"label,omitempty"`  // the section and group, in small capitals
	Title  string `json:"title"`            // the large text
	Sub    string `json:"sub,omitempty"`    // a smaller line under it
	Figure string `json:"figure,omitempty"` // a finding's headline number, shown big
	Unit   string `json:"unit,omitempty"`   // what the figure counts
	Art    string `json:"art,omitempty"`    // the plate's image, under static/
	Alt    string `json:"alt"`              // og:image:alt
}

// The share cards' size and format. They are JPEG because most carry
// a painted plate, which as PNG is several times the size.
const (
	shareW, shareH = 1200, 630
	shareExt       = ".jpg"
	shareDir       = "og"
)

// sectionNouns name what a section's pages are, for its card's label.
var sectionNouns = map[string]string{"findings": "findings", "run": "examples"}

// shareCards describes the card of every page in pages that has one.
// The 404 page has none.
func shareCards(pages []*Page) map[*Page]*shareCard {
	count := map[string]int{}
	for _, p := range pages {
		if p.Section != "" && !p.Index() {
			count[p.Section]++
		}
	}
	cards := map[*Page]*shareCard{}
	for _, p := range pages {
		if p.URL == "/404.html" {
			continue
		}
		c := &shareCard{File: shareFile(p.URL), Layout: "page"}
		sec := sectionOf(p.Section)
		art := p.Art
		switch {
		case p.URL == "/":
			c.Layout = "home"
			c.Title, c.Sub = siteName, smartQuotes(siteTag)+"."
			art = "intro"
		case p.Layout == "book":
			c.Layout = "book"
			c.Label = "The book · " + author
			c.Title, c.Sub = splitTitle(p.Title)
		case p.Index():
			c.Layout = "section"
			c.Label = fmt.Sprintf("Section · %d %s", count[p.Section], cmp.Or(sectionNouns[p.Section], "guides"))
			c.Title = p.Title
			c.Sub = summary(p.Description)
		default:
			c.Label = sec.Title
			if p.Group != "" && p.Section != "" {
				c.Label += " · " + p.Group
			}
			c.Title, c.Sub = splitTitle(p.Title)
			c.Figure, c.Unit = leadFigure(string(p.Body))
			if p.Section == "" && c.Sub == "" {
				c.Sub = summary(p.Description)
			}
		}
		if art == "" {
			art = sec.Art
		}
		switch {
		case p.Layout == "book":
			c.Art = "img/cover-720.jpg"
		case art != "":
			a := arts[art]
			c.Art = fmt.Sprintf("img/art/%s-%d.%s", art, a.Widths[len(a.Widths)-1], a.Ext)
		}
		c.Alt = shareAlt(c, art)
		cards[p] = c
	}
	return cards
}

// sectionOf is the section with directory dir, or the zero section.
func sectionOf(dir string) section {
	for _, s := range sections {
		if s.Dir == dir && dir != "" {
			return s
		}
	}
	return section{}
}

// shareFile is the card's path under static/ for the page at url:
// og/home.jpg for the home page, og/basics.jpg for a section's page,
// og/basics/agent-loop.jpg for a guide.
func shareFile(url string) string {
	name := strings.Trim(url, "/")
	if name == "" {
		name = "home"
	}
	return shareDir + "/" + name + shareExt
}

// splitTitle splits a title at its first colon into the large text and
// the line under it, so "Errors in a Go agent: wrapping, ..." shows its
// subject big. The line under it starts with a capital unless its first
// word is code, such as iter.Seq.
func splitTitle(t string) (title, sub string) {
	head, tail, ok := strings.Cut(t, ": ")
	if !ok {
		return t, ""
	}
	first, _, _ := strings.Cut(tail, " ")
	if r, n := utf8.DecodeRuneInString(tail); unicode.IsLower(r) && !strings.ContainsAny(first, "./()") {
		tail = string(unicode.ToUpper(r)) + tail[n:]
	}
	return head, tail
}

// summary is a short line from a page's description, for a card that
// has no other line under its title: the first sentence when that is
// short, or else its opening, up to its first colon, comma, or full
// stop, such as "How AI agents work, from the ground up".
func summary(d string) string {
	if i := strings.Index(d, ". "); i > 0 && utf8.RuneCountInString(d[:i]) <= 110 {
		return d[:i+1]
	}
	if i := strings.IndexAny(d, ".!?"); i == len(d)-1 && utf8.RuneCountInString(d) <= 110 {
		return d
	}
	if head, _, ok := strings.Cut(d, ":"); ok {
		return head
	}
	if i := strings.IndexAny(d, ",."); i > 0 {
		return d[:i]
	}
	return d
}

// leadFigure is the figure and unit of the result block a page opens
// with, if it opens with one: the number a findings page leads with.
func leadFigure(body string) (figure, unit string) {
	rest := strings.TrimLeft(body, " \t\n")
	rest, ok := strings.CutPrefix(rest, "```result\n")
	if !ok {
		return "", ""
	}
	block, _, _ := strings.Cut(rest, "\n```")
	f, _, err := fields(block, append([]string{"figure", "unit", "source", "link", "more"}, methodKeys...)...)
	if err != nil || len(f["figure"]) == 0 {
		return "", "" // the render reports a malformed block
	}
	plain := func(s string) string { return smartQuotes(strings.NewReplacer("`", "", "*", "").Replace(s)) }
	figure = plain(f["figure"][0])
	if u := f["unit"]; len(u) > 0 {
		unit = plain(u[0])
	}
	return figure, unit
}

// shareAlt describes the card for a reader who can't see it, within
// the 420 characters X allows: what it says, and the illustration
// beside it when that fits.
func shareAlt(c *shareCard, art string) string {
	text := c.Title
	if c.Figure != "" {
		text = strings.TrimSpace(c.Figure+" "+c.Unit) + ", over the title " + text
	}
	if c.Sub != "" {
		text += ": " + strings.TrimSuffix(c.Sub, ".")
	}
	if c.Layout == "home" {
		text += "."
	} else {
		text += ", on a card from " + siteName
		if c.Label != "" {
			text += ", " + strings.ReplaceAll(c.Label, " · ", ", ")
		}
		text += "."
	}
	var more string
	switch {
	case c.Layout == "book":
		more = " Beside it, the book’s cover."
	case art != "":
		more = " Beside it, an illustration: " + arts[art].Alt
	}
	if utf8.RuneCountInString(text+more) <= 420 {
		text += more
	}
	return text
}

// ShareImage is the full URL of the page's share image: its own card,
// or the generic image.
func (p *Page) ShareImage() string {
	if p.Share != nil {
		return siteURL + "/" + p.Share.File
	}
	return siteURL + ogImage
}

// specHash identifies what a card shows. The renderer records it in its
// manifest, so the build can tell when a card no longer matches its
// page.
func (c *shareCard) specHash() string {
	b, _ := json.Marshal(c) // a struct of strings always encodes
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// writeShareSpec writes every card's description, as JSON, for the
// renderer: go run ./cmd/build -og-spec FILE.
func writeShareSpec(file string, cards map[*Page]*shareCard, pages []*Page) error {
	var list []*shareCard
	for _, p := range pages {
		if c, ok := cards[p]; ok {
			list = append(list, c)
		}
	}
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return fmt.Errorf("share spec: %w", err)
	}
	if file == "-" {
		_, err = os.Stdout.Write(append(b, '\n'))
		return err
	}
	if err := os.WriteFile(file, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("share spec: %w", err)
	}
	return nil
}

// shareManifest is tools/og/manifest.json: for each card file, the
// specHash it was rendered from.
type shareManifest map[string]struct {
	Spec string `json:"spec"`
}

// attachShareCards gives each page its card when the card's file is in
// static/og/, and reports how many pages fall back to the generic image
// and how many cards are out of date, so a build after a title change
// says to rerun the renderer.
func attachShareCards(static, manifest string, cards map[*Page]*shareCard, pages []*Page) error {
	var m shareManifest
	if b, err := os.ReadFile(manifest); err == nil {
		if err := json.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("%s: %w", manifest, err)
		}
	}
	var missing, stale int
	for _, p := range pages {
		c, ok := cards[p]
		if !ok {
			continue
		}
		if _, err := os.Stat(filepath.Join(static, filepath.FromSlash(c.File))); err != nil {
			missing++
			continue
		}
		if m[c.File].Spec != c.specHash() {
			stale++
		}
		p.Share = c
	}
	if missing > 0 || stale > 0 {
		var parts [][]byte
		if missing > 0 {
			parts = append(parts, fmt.Appendf(nil, "%d pages have no share card and use the generic image", missing))
		}
		if stale > 0 {
			parts = append(parts, fmt.Appendf(nil, "%d share cards are out of date", stale))
		}
		fmt.Fprintf(os.Stderr, "build: %s (run go run ./tools/og)\n", bytes.Join(parts, []byte("; ")))
	}
	return nil
}
