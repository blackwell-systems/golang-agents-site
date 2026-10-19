package main

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Site is what every template sees about the whole site.
type Site struct {
	URL, Name, Tagline, Author, Repo, Store string
	CSS                                     string // the stylesheet's path, with a version
	Pages                                   []*Page
	Sections                                []*Section
	Guides                                  []*Page // every section's guides, in reading order
	Held                                    []*Page // drafts this build holds back
	Paths                                   []*Path // the reading paths on the start page
}

// Section is a section with its own page and the pages in it.
type Section struct {
	Dir, Title, Nav, Art string
	Index                *Page   // the section's own page, content/<dir>/_index.md
	Pages                []*Page // the rest, by weight
	Held                 int     // drafts in the section, held back from this build
}

// newSite groups pages into sections and versions the stylesheet, so a
// new style.css is never served from a stale cache. Guides runs through
// the sections in nav order, so the last guide of one section leads to
// the first of the next.
func newSite(pages, held []*Page, paths []*Path, static string) (*Site, error) {
	css, err := os.ReadFile(filepath.Join(static, "style.css"))
	if err != nil {
		return nil, fmt.Errorf("stylesheet: %w", err)
	}
	sum := sha256.Sum256(css)
	s := &Site{
		URL: siteURL, Name: siteName, Tagline: siteTag, Author: author,
		Repo: repoURL, Store: storeURL,
		CSS:   "/style.css?v=" + hex.EncodeToString(sum[:4]),
		Pages: pages, Held: held, Paths: paths,
	}
	for _, sec := range sections {
		sv := &Section{Dir: sec.Dir, Title: sec.Title, Nav: cmp.Or(sec.Nav, sec.Title), Art: sec.Art}
		for _, p := range pages {
			switch {
			case p.Section != sec.Dir:
			case p.Index():
				sv.Index = p
			default:
				sv.Pages = append(sv.Pages, p)
			}
		}
		if sv.Index == nil {
			return nil, fmt.Errorf("section %s has no _index.md", sec.Dir)
		}
		for _, p := range held {
			if p.Section == sec.Dir && !p.Index() {
				sv.Held++
			}
		}
		s.Sections = append(s.Sections, sv)
		s.Guides = append(s.Guides, sv.Pages...)
	}
	return s, nil
}

// Others are the published pages outside the sections, other than the
// home page, the 404 page, and the search page: the start page, the
// glossary, the book, and the like, for the search page's full list.
func (s *Site) Others() []*Page {
	var out []*Page
	for _, p := range s.Pages {
		if p.Section == "" && p.URL != "/" && p.URL != "/404.html" && p.Layout != "search" {
			out = append(out, p)
		}
	}
	return out
}

// FirstLive is the first published step of path, or nil.
func (s *Site) FirstLive(path *Path) *Page {
	for _, st := range path.Steps {
		if st.Live {
			return st.Page
		}
	}
	return nil
}

// section returns the section p belongs to, or nil.
func (s *Site) section(p *Page) *Section {
	for _, sec := range s.Sections {
		if sec.Dir == p.Section {
			return sec
		}
	}
	return nil
}

// view is the data one page's template gets.
type view struct {
	Site       *Site
	Page       *Page
	Section    *Section // the page's section, or nil
	Prev, Next *Page    // neighbors in the reading order, for guides
	// PrevIn and NextIn name the neighbor's section when it isn't
	// this page's, so a link that leaves the section says so.
	PrevIn, NextIn string
	Places         []PathPlace // where the page stands on the reading paths
	Nav            []navItem
	JSONLD         template.JS
}

type navItem struct {
	Title, URL string
	Current    bool
}

// templateFuncs are the helpers the templates use.
var templateFuncs = template.FuncMap{
	"date": func(s string) string {
		t, err := time.Parse(time.DateOnly, s)
		if err != nil {
			return s
		}
		return t.Format("January 2, 2006")
	},
	"chapters": chapterText,
	"picture":  picture,
}

// chapterText turns "1, 4" into "chapters 1 and 4".
func chapterText(s string) string {
	var cs []string
	for c := range strings.SplitSeq(s, ",") {
		if c = strings.TrimSpace(c); c != "" {
			cs = append(cs, c)
		}
	}
	switch len(cs) {
	case 0:
		return ""
	case 1:
		return "chapter " + cs[0]
	case 2:
		return "chapters " + cs[0] + " and " + cs[1]
	}
	return "chapters " + strings.Join(cs[:len(cs)-1], ", ") + ", and " + cs[len(cs)-1]
}

// loadTemplates parses base.html once for each layout, together with
// that layout's file, which defines "main".
func loadTemplates(dir string) (map[string]*template.Template, error) {
	base, err := template.New("base.html").Funcs(templateFuncs).
		ParseFiles(filepath.Join(dir, "base.html"))
	if err != nil {
		return nil, fmt.Errorf("templates: %w", err)
	}
	out := map[string]*template.Template{}
	for _, layout := range []string{"home", "section", "article", "book", "glossary", "page", "work", "search"} {
		t, err := template.Must(base.Clone()).ParseFiles(filepath.Join(dir, layout+".html"))
		if err != nil {
			return nil, fmt.Errorf("templates: %w", err)
		}
		out[layout] = t
	}
	return out, nil
}

// writePage renders p into its file under out.
func writePage(out string, tmpl map[string]*template.Template, s *Site, p *Page) error {
	v := view{Site: s, Page: p, Section: s.section(p), Places: places(s.Paths, p)}
	if len(s.Paths) > 0 {
		v.Nav = append(v.Nav, navItem{"Start here", "/start/", p.URL == "/start/"})
	}
	for _, sec := range s.Sections {
		v.Nav = append(v.Nav, navItem{sec.Nav, "/" + sec.Dir + "/", sec.Dir == p.Section})
	}
	v.Nav = append(v.Nav,
		navItem{"Glossary", "/glossary/", p.URL == "/glossary/"},
		navItem{"The book", "/book/", p.URL == "/book/"},
		navItem{"Work with me", "/work-with-me/", p.URL == "/work-with-me/"})
	if i := slices.Index(s.Guides, p); i >= 0 {
		if i > 0 {
			v.Prev = s.Guides[i-1]
			if v.Prev.Section != p.Section {
				v.PrevIn = s.section(v.Prev).Title
			}
		}
		if i+1 < len(s.Guides) {
			v.Next = s.Guides[i+1]
			if v.Next.Section != p.Section {
				v.NextIn = s.section(v.Next).Title
			}
		}
	}
	ld, err := jsonLD(s, p)
	if err != nil {
		return fmt.Errorf("%s: %w", p.Source, err)
	}
	v.JSONLD = ld
	file := filepath.Join(out, filepath.FromSlash(p.URL))
	if strings.HasSuffix(p.URL, "/") {
		file = filepath.Join(file, "index.html")
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	f, err := os.Create(file)
	if err != nil {
		return err
	}
	if err := tmpl[p.Layout].ExecuteTemplate(f, "base", v); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", p.Source, err)
	}
	return f.Close()
}

// jsonLD is the page's structured data: Article for guides, Book for
// the book's page, WebSite for the home page, and a plain WebPage for
// the rest.
func jsonLD(s *Site, p *Page) (template.JS, error) {
	person := map[string]any{"@type": "Person", "name": s.Author}
	var d map[string]any
	switch {
	case p.Layout == "book":
		d = map[string]any{
			"@type":               "Book",
			"name":                "Building AI Agents in Go",
			"alternativeHeadline": "From the agent loop to production",
			"author":              person,
			"inLanguage":          "en",
			"bookFormat":          "https://schema.org/EBook",
			"url":                 p.Abs(),
			"image":               s.URL + ogImage,
			"offers": map[string]any{
				"@type":         "Offer",
				"price":         "29.00",
				"priceCurrency": "USD",
				"url":           s.Store,
				"availability":  "https://schema.org/InStock",
			},
		}
	case p.Guide():
		d = map[string]any{
			"@type":            "Article",
			"headline":         p.Title,
			"description":      p.Description,
			"author":           person,
			"datePublished":    p.Date,
			"dateModified":     p.Modified(),
			"mainEntityOfPage": p.Abs(),
			"image":            s.URL + ogImage,
			"inLanguage":       "en",
			"isPartOf":         map[string]any{"@type": "WebSite", "name": s.Name, "url": s.URL + "/"},
		}
	case p.Layout == "work":
		d = map[string]any{
			"@type":       "ProfessionalService",
			"name":        "Blackwell Systems",
			"description": p.Description,
			"url":         p.Abs(),
			"founder":     person,
			"areaServed":  "Worldwide",
			"sameAs":      []string{"https://blackwell-systems.com/"},
			"contactPoint": map[string]any{
				"@type":       "ContactPoint",
				"contactType": "sales",
				"email":       "dayna@blackwell-systems.com",
				"url":         "https://blackwell-systems.com/contact/",
			},
		}
	case p.URL == "/":
		d = map[string]any{
			"@type":       "WebSite",
			"name":        s.Name,
			"description": p.Description,
			"url":         s.URL + "/",
			"inLanguage":  "en",
		}
	default:
		d = map[string]any{
			"@type":       "WebPage",
			"name":        p.Title,
			"description": p.Description,
			"url":         p.Abs(),
			"isPartOf":    map[string]any{"@type": "WebSite", "name": s.Name, "url": s.URL + "/"},
		}
	}
	d["@context"] = "https://schema.org"
	b, err := json.MarshalIndent(d, "", "  ") // escapes <, >, and & for a <script>
	if err != nil {
		return "", fmt.Errorf("json-ld: %w", err)
	}
	return template.JS(b), nil
}

// copyStatic copies the static files into out. Dotfiles are skipped,
// except .assetsignore, which wrangler reads from the assets directory.
func copyStatic(static, out string) error {
	err := filepath.WalkDir(static, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(static, file)
		if err != nil {
			return err
		}
		name := d.Name()
		if rel != "." && strings.HasPrefix(name, ".") && name != ".assetsignore" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		dst := filepath.Join(out, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		return copyFile(file, dst)
	})
	if err != nil {
		return fmt.Errorf("copy static files: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}
