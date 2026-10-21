package main

import (
	"cmp"
	"encoding/xml"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"slices"
	"time"
)

// writeFeeds writes sitemap.xml, robots.txt, and feed.xml, an RSS feed
// of the guides, newest first. Every date comes from the pages, so two
// builds of the same content write the same files.
func writeFeeds(out string, s *Site) error {
	if err := writeSitemap(out, s); err != nil {
		return err
	}
	robots := "User-agent: *\nAllow: /\n\nSitemap: " + s.URL + "/sitemap.xml\n"
	if err := os.WriteFile(filepath.Join(out, "robots.txt"), []byte(robots), 0o644); err != nil {
		return fmt.Errorf("robots.txt: %w", err)
	}
	return writeRSS(out, s)
}

func writeSitemap(out string, s *Site) error {
	type url struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod,omitempty"`
	}
	type urlset struct {
		XMLName xml.Name `xml:"urlset"`
		NS      string   `xml:"xmlns,attr"`
		URLs    []url    `xml:"url"`
	}
	set := urlset{NS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	for _, p := range s.Pages {
		if p.URL == "/404.html" {
			continue
		}
		set.URLs = append(set.URLs, url{p.Abs(), p.Modified()})
	}
	return writeXML(filepath.Join(out, "sitemap.xml"), set)
}

func writeRSS(out string, s *Site) error {
	type item struct {
		Title       string `xml:"title"`
		Link        string `xml:"link"`
		GUID        string `xml:"guid"`
		PubDate     string `xml:"pubDate"`
		Description string `xml:"description"`
	}
	type atomLink struct {
		Href string `xml:"href,attr"`
		Rel  string `xml:"rel,attr"`
		Type string `xml:"type,attr"`
	}
	type channel struct {
		Title         string   `xml:"title"`
		Link          string   `xml:"link"`
		Description   string   `xml:"description"`
		Language      string   `xml:"language"`
		LastBuildDate string   `xml:"lastBuildDate,omitempty"`
		Self          atomLink `xml:"atom:link"`
		Items         []item   `xml:"item"`
	}
	type rss struct {
		XMLName xml.Name `xml:"rss"`
		Version string   `xml:"version,attr"`
		Atom    string   `xml:"xmlns:atom,attr"`
		Channel channel  `xml:"channel"`
	}
	var guides []*Page
	for _, p := range s.Pages {
		if p.Guide() {
			guides = append(guides, p)
		}
	}
	slices.SortFunc(guides, func(a, b *Page) int {
		return cmp.Or(cmp.Compare(b.Date, a.Date), cmp.Compare(a.URL, b.URL))
	})
	ch := channel{
		Title:       s.Name,
		Link:        s.URL + "/",
		Description: s.Tagline,
		Language:    "en",
		Self:        atomLink{s.URL + "/feed.xml", "self", "application/rss+xml"},
	}
	var latest string
	for _, p := range guides {
		ch.Items = append(ch.Items, item{p.Title, p.Abs(), p.Abs(), rfc822(p.Date), p.Description})
		latest = max(latest, p.Modified())
	}
	if latest != "" {
		ch.LastBuildDate = rfc822(latest)
	}
	return writeXML(filepath.Join(out, "feed.xml"),
		rss{Version: "2.0", Atom: "http://www.w3.org/2005/Atom", Channel: ch})
}

// rfc822 formats a 2006-01-02 date the way RSS wants it.
func rfc822(d string) string {
	t, err := time.Parse(time.DateOnly, d)
	if err != nil {
		return ""
	}
	return t.Format(time.RFC1123Z)
}

func writeXML(file string, v any) error {
	b, err := xml.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(file), err)
	}
	b = append([]byte(xml.Header), append(b, '\n')...)
	if err := os.WriteFile(file, b, 0o644); err != nil {
		return fmt.Errorf("%s: %w", filepath.Base(file), err)
	}
	return nil
}

// writeRedirects writes a redirect page at every URL that moved, so
// links and search results that predate a rename keep working on any
// static host: GitHub Pages serves files and nothing else. Each page
// sends the browser on at once, names the new URL as canonical, and
// asks search engines not to index it. A moved section gets a page at
// its old address and one for every page now in the new section; then
// each moved page gets its own, replacing the section's guess. It checks
// that every target exists in out and that no redirect lands on a real
// page.
func writeRedirects(out string) error {
	exists := func(url string) bool {
		_, err := os.Stat(filepath.Join(out, filepath.FromSlash(url), "index.html"))
		return err == nil
	}
	written := map[string]bool{}
	write := func(from, to string) error {
		dir := filepath.Join(out, filepath.FromSlash(from))
		if exists(from) && !written[from] {
			return fmt.Errorf("redirect %s to %s: a page already exists there", from, to)
		}
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("redirect %s: %w", from, err)
		}
		u := html.EscapeString(to)
		page := `<!doctype html><html lang="en"><head><meta charset="utf-8">` +
			`<title>Moved</title><meta name="robots" content="noindex">` +
			`<link rel="canonical" href="` + html.EscapeString(siteURL+to) + `">` +
			`<meta http-equiv="refresh" content="0; url=` + u + `">` +
			`<script>location.replace("` + u + `" + location.hash)</script></head>` +
			`<body><p>This page moved to <a href="` + u + `">` + u + `</a>.</p></body></html>` + "\n"
		if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte(page), 0o644); err != nil {
			return fmt.Errorf("redirect %s: %w", from, err)
		}
		written[from] = true
		return nil
	}
	for _, m := range moved {
		to := "/" + m.To + "/"
		if !exists(to) {
			return fmt.Errorf("redirect /%s/ to %s: no such section", m.From, to)
		}
		if err := write("/"+m.From+"/", to); err != nil {
			return err
		}
		entries, err := os.ReadDir(filepath.Join(out, m.To))
		if err != nil {
			return fmt.Errorf("redirect /%s/: %w", m.From, err)
		}
		for _, e := range entries {
			if page := to + e.Name() + "/"; e.IsDir() && exists(page) {
				if err := write("/"+m.From+"/"+e.Name()+"/", page); err != nil {
					return err
				}
			}
		}
	}
	for _, m := range movedPages {
		if !exists(m.To) {
			return fmt.Errorf("redirect %s to %s: no such page", m.From, m.To)
		}
		if err := write(m.From, m.To); err != nil {
			return err
		}
	}
	return nil
}
