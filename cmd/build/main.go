// Command build renders golang-agents.com: the Markdown pages under
// content/, through the templates under templates/, into dist/, along
// with the files under static/, a sitemap, robots.txt, and an RSS feed.
// It then checks every internal link in the output.
//
// Run it from the repository root:
//
//	go run ./cmd/build
//
// Code excerpts come from the companion code, which -code names: the
// code/ directory of a clone of go-agents-book-code.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	var cfg config
	flag.StringVar(&cfg.content, "content", "content", "the Markdown pages")
	flag.StringVar(&cfg.templates, "templates", "templates", "the HTML templates")
	flag.StringVar(&cfg.static, "static", "static", "files copied as they are")
	flag.StringVar(&cfg.out, "out", "dist", "the output directory, replaced on each build")
	flag.StringVar(&cfg.code, "code", "../go-agents-book/code",
		"the companion code's code/ directory, for excerpts")
	flag.StringVar(&cfg.book, "book", "../go-agents-book/book/chapters",
		"the book's chapters, for the copy check; empty skips the check")
	flag.BoolVar(&cfg.drafts, "drafts", false,
		"build draft pages too, for review; never deploy such a build")
	flag.Parse()
	if err := build(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "build:", err)
		os.Exit(1)
	}
}

// config holds the build's directories.
type config struct {
	content, templates, static, out, code, book string
	drafts                                      bool // build pages marked draft
}

// The site's fixed values.
const (
	siteURL  = "https://golang-agents.com"
	siteName = "AI Agents in Go"
	siteTag  = "Build AI agents with Go's standard library"
	author   = "Dayna Blackwell"
	ogImage  = "/og-v3.jpg"
	repoURL  = "https://github.com/blackwell-systems/go-agents-book-code"
	codeRef  = "v1.4" // the tag (or branch) code links point at: v1.4 added memory
	storeURL = "https://blackwellsystems.gumroad.com/l/go-agents"
)

// section is one part of the site, with a directory under content/.
// Nav is the shorter name the header uses, when Title is too long, and
// Art the illustration beside the section's introduction.
type section struct {
	Dir, Title, Nav, Art string
}

// sections are the site's sections, in the order the nav lists them.
var sections = []section{
	{"basics", "Basics", "", "part1"},
	{"build", "Build", "", "part2"},
	{"production", "Production", "", "part3"},
	{"ecosystem", "Go ecosystem", "Ecosystem", "ch11"},
	{"run", "Examples", "", "intro"},
	{"findings", "Research", "", "ch13"},
}

// moved maps a section's old directory to its new one. The build writes
// a permanent redirect from every URL under the old one, so links and
// search results that predate a rename keep working. Each entry points
// at a section's current directory, never at another old one.
var moved = []struct{ From, To string }{
	{"foundations", "basics"},
	{"loop", "basics"},
	{"patterns", "build"},
	{"design", "build"},
	{"examples", "run"},
	{"providers", "ecosystem"},
}

// movedPages are pages that left their section for another one. Their
// redirects come before the sections' in _redirects, since Cloudflare
// uses the first rule that matches.
var movedPages = []struct{ From, To string }{
	{"/loop/why-go/", "/ecosystem/why-go/"},
	{"/foundations/why-go/", "/ecosystem/why-go/"},
	{"/loop/streaming/", "/build/streaming/"},
	{"/foundations/streaming/", "/build/streaming/"},
}

func build(cfg config) error {
	diagramDir = filepath.Join(cfg.static, "img", "diagrams")
	code, err := openCode(cfg.code)
	if err != nil {
		return err
	}
	pages, held, err := loadPages(cfg.content, cfg.drafts)
	if err != nil {
		return err
	}
	if cfg.book == "" {
		fmt.Fprintln(os.Stderr, "build: copy check skipped (-book is empty)")
	} else {
		runs, err := loadBook(cfg.book)
		if err != nil {
			return err
		}
		if err := checkCopy(runs, pages); err != nil {
			return err
		}
	}
	ix := newPageIndex(pages, held)
	paths, err := sitePaths(pages, ix) // from the start page's Markdown, before it renders
	if err != nil {
		return err
	}
	md := newMarkdown(code, ix)
	for _, p := range pages {
		if err := p.render(md, code); err != nil {
			return err
		}
		if p.Layout == "glossary" {
			p.Body = glossaryCards(p.Body)
		}
	}
	site, err := newSite(pages, held, paths, cfg.static)
	if err != nil {
		return err
	}
	var used []string
	for _, p := range pages {
		if p.Art != "" {
			used = append(used, p.Art)
		}
	}
	for _, sec := range sections {
		used = append(used, sec.Art)
	}
	used = append(used, bodyArt...)
	if err := checkArt(cfg.static, used...); err != nil {
		return err
	}
	tmpl, err := loadTemplates(cfg.templates)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(cfg.out); err != nil {
		return fmt.Errorf("clear output: %w", err)
	}
	if err := copyStatic(cfg.static, cfg.out); err != nil {
		return err
	}
	for _, p := range pages {
		if err := writePage(cfg.out, tmpl, site, p); err != nil {
			return err
		}
	}
	if err := writeFeeds(cfg.out, site); err != nil {
		return err
	}
	if err := writeSearchIndex(cfg.out, site); err != nil {
		return err
	}
	if err := writeRedirects(cfg.out); err != nil {
		return err
	}
	n, err := checkLinks(cfg.out, code)
	if err != nil {
		return err
	}
	fmt.Printf("built %d pages into %s (%d drafts held back); %d internal links checked\n",
		len(pages), cfg.out, len(held), n)
	return nil
}
