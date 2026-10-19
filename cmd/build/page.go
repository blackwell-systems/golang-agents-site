package main

import (
	"bufio"
	"bytes"
	"cmp"
	"fmt"
	"html"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yuin/goldmark"
)

// Page is one page of the site.
type Page struct {
	Source  string // the Markdown file, relative to content/
	URL     string // the site path, such as "/loop/agent-loop/"
	Section string // the section's directory, or "" outside sections
	Meta
	Body     template.HTML
	BookHTML template.HTML // BookLine, rendered
	TOC      []TOCItem     // the h2 headings, for guides
}

// Meta is a page's front matter: "key: value" lines between two "---"
// lines at the top of the file.
type Meta struct {
	Title       string // the page's title, used in <title> and the h1
	Short       string // a shorter title for nav and lists; defaults to Title
	Description string // one or two sentences for search results and lists
	Card        string // one line for a section page's card; defaults to Description
	Layout      string // home, section, article, book, glossary, page, work, or search
	Chapters    string // the book chapters the page draws on, such as "1, 4"
	Date        string // first published, as 2006-01-02
	Updated     string // last changed in substance, as 2006-01-02
	Weight      int    // the order within its section, lowest first
	Kind        string // a label shown above the title, such as "Example"
	Stage       string // the loop stage the page works on: 1 to 4, or "all"
	BookLine    string // the closing line about the book, in Markdown
	Work        bool   // close with a line pointing at /work-with-me/
	Draft       bool   // kept out of the build: no page, sitemap, feed, or nav entry
	Art         string // an illustration from cmd/build/art.go, such as "ch04"
}

// Index reports whether p is a section's own page.
func (p *Page) Index() bool { return p.Layout == "section" }

// Abs is the page's full URL.
func (p *Page) Abs() string { return siteURL + p.URL }

// Name is the page's short title.
func (p *Page) Name() string { return cmp.Or(p.Short, p.Title) }

// Blurb is the line a section page's card shows.
func (p *Page) Blurb() string { return cmp.Or(p.Card, p.Description) }

// Modified is the date the page last changed.
func (p *Page) Modified() string { return cmp.Or(p.Updated, p.Date) }

// Guide reports whether p is a guide: a dated article that belongs in
// the feed and carries Article metadata.
func (p *Page) Guide() bool { return p.Layout == "article" }

// loadPages reads every page under dir. It returns the pages to build,
// which are every page if drafts is set and otherwise the pages that
// aren't drafts, and the drafts it holds back, which are never built
// but which reading paths and page: links may name. PLAN.md and
// README.md at the top of dir describe the content and are not pages.
func loadPages(dir string, drafts bool) (pages, held []*Page, err error) {
	err = filepath.WalkDir(dir, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(file) != ".md" {
			return nil
		}
		rel, err := filepath.Rel(dir, file)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "PLAN.md" || rel == "README.md" {
			return nil
		}
		p, err := loadPage(file, rel)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		if p.Draft && !drafts {
			held = append(held, p) // on disk for later; not built
			return nil
		}
		pages = append(pages, p)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("load pages: %w", err)
	}
	byOrder := func(a, b *Page) int {
		return cmp.Or(cmp.Compare(a.Section, b.Section),
			cmp.Compare(a.Weight, b.Weight), cmp.Compare(a.URL, b.URL))
	}
	slices.SortFunc(pages, byOrder)
	slices.SortFunc(held, byOrder)
	return pages, held, nil
}

// loadPage reads one page and works out its URL and section from rel,
// its path under content/.
func loadPage(file, rel string) (*Page, error) {
	src, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	meta, body, err := frontMatter(src)
	if err != nil {
		return nil, err
	}
	p := &Page{Source: rel, Meta: meta, Body: template.HTML(body)}
	p.Title, p.Short = smartQuotes(p.Title), smartQuotes(p.Short)
	p.Description, p.Card = smartQuotes(p.Description), smartQuotes(p.Card)
	dir, name := path.Split(strings.TrimSuffix(rel, ".md"))
	switch {
	case rel == "index.md":
		p.URL = "/"
	case rel == "404.md":
		p.URL = "/404.html"
	case name == "_index":
		p.URL = "/" + dir
	default:
		p.URL = "/" + dir + name + "/"
	}
	if first, _, ok := strings.Cut(rel, "/"); ok {
		p.Section = first
		if !slices.ContainsFunc(sections, func(s section) bool { return s.Dir == first }) {
			return nil, fmt.Errorf("directory %s is not a section", first)
		}
	}
	if p.Layout == "" {
		p.Layout = "article"
		if name == "_index" {
			p.Layout = "section"
		}
	}
	return p, p.check()
}

// check reports front matter that is missing or malformed.
func (p *Page) check() error {
	if p.Title == "" || p.Description == "" {
		return fmt.Errorf("title and description are required")
	}
	switch p.Layout {
	case "home", "section", "article", "book", "glossary", "page", "work", "search":
	default:
		return fmt.Errorf("unknown layout %q", p.Layout)
	}
	for _, d := range []string{p.Date, p.Updated} {
		if d == "" {
			continue
		}
		if _, err := time.Parse(time.DateOnly, d); err != nil {
			return fmt.Errorf("date %q: want 2006-01-02", d)
		}
	}
	switch p.Stage {
	case "", "all", "1", "2", "3", "4":
	default:
		return fmt.Errorf("stage %q: want 1 to 4, or all", p.Stage)
	}
	// A card holds about two lines at its narrowest.
	if n := utf8.RuneCountInString(p.Card); n > 120 {
		return fmt.Errorf("card is %d characters; keep it to 120", n)
	}
	if p.Guide() && p.Date == "" {
		return fmt.Errorf("an article needs a date")
	}
	if _, ok := arts[p.Art]; p.Art != "" && !ok {
		return fmt.Errorf("art %q: not in arts (cmd/build/art.go)", p.Art)
	}
	return nil
}

// frontMatter splits src into its front matter and its Markdown body.
// Unknown keys are an error, so a misspelled key can't go unnoticed.
func frontMatter(src []byte) (Meta, []byte, error) {
	var m Meta
	rest, ok := bytes.CutPrefix(src, []byte("---\n"))
	if !ok {
		return m, nil, fmt.Errorf("no front matter")
	}
	head, body, ok := bytes.Cut(rest, []byte("\n---\n"))
	if !ok {
		return m, nil, fmt.Errorf("front matter has no closing ---")
	}
	sc := bufio.NewScanner(bytes.NewReader(head))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			return m, nil, fmt.Errorf("front matter line %q: want key: value", line)
		}
		val = strings.TrimSpace(val)
		if uq, err := strconv.Unquote(val); err == nil {
			val = uq
		}
		if err := m.set(strings.TrimSpace(key), val); err != nil {
			return m, nil, err
		}
	}
	return m, body, nil
}

func (m *Meta) set(key, val string) error {
	switch key {
	case "title":
		m.Title = val
	case "short":
		m.Short = val
	case "description":
		m.Description = val
	case "card":
		m.Card = val
	case "layout":
		m.Layout = val
	case "chapters":
		m.Chapters = val
	case "date":
		m.Date = val
	case "updated":
		m.Updated = val
	case "kind":
		m.Kind = val
	case "stage":
		m.Stage = val
	case "book":
		m.BookLine = val
	case "art":
		m.Art = val
	case "draft":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("draft %q: %w", val, err)
		}
		m.Draft = b
	case "work":
		b, err := strconv.ParseBool(val)
		if err != nil {
			return fmt.Errorf("work %q: %w", val, err)
		}
		m.Work = b
	case "weight":
		n, err := strconv.Atoi(val)
		if err != nil {
			return fmt.Errorf("weight %q: %w", val, err)
		}
		m.Weight = n
	default:
		return fmt.Errorf("unknown front matter key %q", key)
	}
	return nil
}

// render converts the page's Markdown body, and its closing book line,
// to HTML.
func (p *Page) render(md goldmark.Markdown, code *codeRoot) error {
	body, err := convert(md, code, []byte(p.Body))
	if err != nil {
		return fmt.Errorf("%s: %w", p.Source, err)
	}
	p.Body = body
	if p.Guide() {
		p.TOC = contents(body)
	}
	if p.BookLine != "" {
		line, err := convert(md, code, []byte(p.BookLine))
		if err != nil {
			return fmt.Errorf("%s: book line: %w", p.Source, err)
		}
		p.BookHTML = line
	}
	return nil
}

// convert renders Markdown, and returns any error the code excerpts or
// code links reported along the way.
func convert(md goldmark.Markdown, code *codeRoot, src []byte) (template.HTML, error) {
	if bytes.ContainsRune(src, '—') || bytes.Contains(src, []byte("&mdash;")) {
		return "", fmt.Errorf("an em dash: the prose rules ban them")
	}
	src, err := code.fillCounts(src)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := md.Convert(src, &buf); err != nil {
		return "", err
	}
	if err := code.takeErr(); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

// stageNames are the four stages of the agent loop, in order.
var stageNames = [4]string{"Build the request", "Call the model", "Run the tools", "Decide"}

// StageItem is one stage in a page's map of the loop.
type StageItem struct {
	N    int
	Name string
	On   bool // the page works on this stage
}

// Stages is the loop map for a page that names a stage, or nil.
func (p *Page) Stages() []StageItem {
	if p.Stage == "" {
		return nil
	}
	var out []StageItem
	for i, name := range stageNames {
		on := p.Stage == "all" || p.Stage == strconv.Itoa(i+1)
		out = append(out, StageItem{i + 1, name, on})
	}
	return out
}

// TOCItem is one h2 of a page, for its "On this page" list.
type TOCItem struct {
	ID, Text string
}

var (
	h2Tag  = regexp.MustCompile(`<h2 id="([^"]+)"[^>]*>(.*?)</h2>`)
	anyTag = regexp.MustCompile(`<[^>]+>`)
)

// contents lists the h2 headings in body.
func contents(body template.HTML) []TOCItem {
	var out []TOCItem
	for _, m := range h2Tag.FindAllStringSubmatch(string(body), -1) {
		text := html.UnescapeString(anyTag.ReplaceAllString(m[2], ""))
		out = append(out, TOCItem{ID: html.UnescapeString(m[1]), Text: text})
	}
	return out
}

// smartQuotes curls the quotes in front matter text, which goldmark's
// typographer never sees: an apostrophe becomes ’, and double quotes
// alternate between “ and ”.
func smartQuotes(s string) string {
	var b strings.Builder
	open := true
	for _, r := range s {
		switch r {
		case '\'':
			b.WriteRune('’')
		case '"':
			if open {
				b.WriteRune('“')
			} else {
				b.WriteRune('”')
			}
			open = !open
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
