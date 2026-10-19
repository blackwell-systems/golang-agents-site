package main

import (
	"fmt"
	"html"
	"regexp"
	"slices"
	"strings"
)

// Reading paths are ordered routes through the site, one for each kind
// of reader, written as path blocks on the start page (content/start.md):
//
//	```path
//	id: newcomer
//	title: Learning how agents work
//	step: /loop/what-is-an-agent/
//	step: /run/ch01-scripted-loop/
//
//	Who the path suits, in Markdown.
//	```
//
// A step names a page by its URL. A step whose page is still a draft is
// listed as plain text and skipped by the links between steps, so a path
// fills in as its pages are published; a step that names no page fails
// the build.

// startPage is the source of the page that holds the reading paths.
const startPage = "start.md"

// readingPath is one path as its block declares it.
type readingPath struct {
	ID, Title string
	Steps     []string // page URLs, in order
	Text      string   // who the path suits, in Markdown
}

var (
	pathFence = regexp.MustCompile("(?s)```path\n(.*?)```")
	pathID    = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)
)

// parsePath reads one path block's body.
func parsePath(body string) (readingPath, error) {
	f, text, err := fields(body, "id", "title", "step")
	if err != nil {
		return readingPath{}, err
	}
	if len(f["id"]) != 1 || len(f["title"]) != 1 {
		return readingPath{}, fmt.Errorf("want one id: line and one title: line")
	}
	p := readingPath{ID: f["id"][0], Title: f["title"][0], Steps: f["step"], Text: text}
	if !pathID.MatchString(p.ID) {
		return p, fmt.Errorf("id %q: want lowercase words joined by hyphens", p.ID)
	}
	if len(p.Steps) < 2 {
		return p, fmt.Errorf("path %s: want at least two step: lines", p.ID)
	}
	for i, s := range p.Steps {
		if slices.Contains(p.Steps[:i], s) {
			return p, fmt.Errorf("path %s: step %s appears twice", p.ID, s)
		}
	}
	if p.Text == "" {
		return p, fmt.Errorf("path %s: no text after the fields", p.ID)
	}
	return p, nil
}

// readPaths reads the path blocks in src, the start page's Markdown.
func readPaths(src string) ([]readingPath, error) {
	var out []readingPath
	for _, m := range pathFence.FindAllStringSubmatch(src, -1) {
		p, err := parsePath(m[1])
		if err != nil {
			return nil, fmt.Errorf("%s: path block: %w", startPage, err)
		}
		if slices.ContainsFunc(out, func(q readingPath) bool { return q.ID == p.ID }) {
			return nil, fmt.Errorf("%s: two paths with id %s", startPage, p.ID)
		}
		out = append(out, p)
	}
	return out, nil
}

// pageIndex finds pages by URL: the pages this build publishes, and the
// drafts it holds back.
type pageIndex struct {
	built, held map[string]*Page
}

func newPageIndex(built, held []*Page) *pageIndex {
	ix := &pageIndex{built: map[string]*Page{}, held: map[string]*Page{}}
	for _, p := range built {
		ix.built[p.URL] = p
	}
	for _, p := range held {
		ix.held[p.URL] = p
	}
	return ix
}

// find returns the page at url, and whether this build publishes it.
// A URL that names no page, published or draft, is an error.
func (ix *pageIndex) find(url string) (*Page, bool, error) {
	if p, ok := ix.built[url]; ok {
		return p, true, nil
	}
	if p, ok := ix.held[url]; ok {
		return p, false, nil
	}
	return nil, false, fmt.Errorf("%s: no page, published or draft, has this URL", url)
}

// sitePaths reads the reading paths from the start page, if the build
// has one, and finds their pages. It reads the page's Markdown, so it
// runs before the pages are rendered.
func sitePaths(pages []*Page, ix *pageIndex) ([]*Path, error) {
	i := slices.IndexFunc(pages, func(p *Page) bool { return p.Source == startPage })
	if i < 0 {
		return nil, nil
	}
	rps, err := readPaths(string(pages[i].Body))
	if err != nil {
		return nil, err
	}
	var out []*Path
	for _, rp := range rps {
		p, err := ix.resolve(rp)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", startPage, err)
		}
		out = append(out, p)
	}
	return out, nil
}

// PathStep is one step of a reading path.
type PathStep struct {
	Page *Page
	Live bool // published in this build; a draft is listed as plain text
}

// Path is a reading path with its pages found.
type Path struct {
	ID, Title string
	Lead      string // the first sentence of its text, which says who it suits
	Steps     []PathStep
}

// URL is the path's place on the start page.
func (p *Path) URL() string { return "/start/#" + p.ID }

// Live counts the steps this build publishes.
func (p *Path) Live() int {
	n := 0
	for _, s := range p.Steps {
		if s.Live {
			n++
		}
	}
	return n
}

// resolve finds every step's page.
func (ix *pageIndex) resolve(rp readingPath) (*Path, error) {
	lead, _, _ := strings.Cut(rp.Text, ". ")
	p := &Path{ID: rp.ID, Title: rp.Title, Lead: strings.TrimSuffix(lead, ".") + "."}
	for _, url := range rp.Steps {
		pg, live, err := ix.find(url)
		if err != nil {
			return nil, fmt.Errorf("path %s: step %w", rp.ID, err)
		}
		p.Steps = append(p.Steps, PathStep{pg, live})
	}
	return p, nil
}

// PathPlace is where a page stands on one reading path: its step
// number, and the nearest published steps before and after it.
type PathPlace struct {
	Path       *Path
	N          int // 1-based, counting every step, drafts included
	Prev, Next *Page
}

// places lists where p stands on each path that includes it.
func places(paths []*Path, p *Page) []PathPlace {
	var out []PathPlace
	for _, path := range paths {
		i := slices.IndexFunc(path.Steps, func(s PathStep) bool { return s.Page == p })
		if i < 0 {
			continue
		}
		pl := PathPlace{Path: path, N: i + 1}
		for j := i - 1; j >= 0 && pl.Prev == nil; j-- {
			if path.Steps[j].Live {
				pl.Prev = path.Steps[j].Page
			}
		}
		for j := i + 1; j < len(path.Steps) && pl.Next == nil; j++ {
			if path.Steps[j].Live {
				pl.Next = path.Steps[j].Page
			}
		}
		out = append(out, pl)
	}
	return out
}

// pathHTML renders a path block on the start page: its title, its text,
// and its steps, each a link to its page or, for a draft, its title as
// plain text.
func (r *codeBlocks) path(body string) (string, error) {
	rp, err := parsePath(body)
	if err != nil {
		return "", err
	}
	p, err := r.pages.resolve(rp)
	if err != nil {
		return "", err
	}
	text, err := r.inline(rp.Text)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<section class="reading-path" aria-labelledby="%s">`, p.ID)
	fmt.Fprintf(&b, `<h2 id="%s">%s</h2><p>%s</p>`, p.ID, html.EscapeString(p.Title), text)
	fmt.Fprintf(&b, `<p class="path-count">%d steps; %s</p><ol class="path-steps">`,
		len(p.Steps), liveCount(p))
	for _, s := range p.Steps {
		if s.Live {
			fmt.Fprintf(&b, `<li><a href="%s"><span class="step-title">%s</span>`+
				`<span class="step-desc">%s</span></a></li>`,
				html.EscapeString(s.Page.URL), html.EscapeString(s.Page.Name()), html.EscapeString(s.Page.Description))
			continue
		}
		// A draft's description hasn't passed the copy check, so only
		// its title is shown.
		fmt.Fprintf(&b, `<li class="pending"><span class="step-title">%s</span>`+
			`<span class="step-tag">Being reviewed</span></li>`, html.EscapeString(s.Page.Name()))
	}
	b.WriteString("</ol></section>\n")
	return b.String(), nil
}

// liveCount says how many of a path's steps are published.
func liveCount(p *Path) string {
	switch n := p.Live(); n {
	case len(p.Steps):
		return "all published"
	default:
		return fmt.Sprintf("%d published so far, the rest being reviewed", n)
	}
}
