package main

import (
	"cmp"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// writeRedirects writes _redirects, which Cloudflare reads from the
// assets directory: a permanent redirect from each moved page, then from
// each moved section, with
// and without its trailing slash, and from every page under it. It
// checks that each new section exists in out.
func writeRedirects(out string) error {
	var b strings.Builder
	b.WriteString("# Written by cmd/build from the moved lists in main.go.\n")
	for _, m := range movedPages {
		if _, err := os.Stat(filepath.Join(out, filepath.FromSlash(m.To), "index.html")); err != nil {
			return fmt.Errorf("redirect %s to %s: %w", m.From, m.To, err)
		}
		fmt.Fprintf(&b, "%s %s 301\n%s %s 301\n", strings.TrimSuffix(m.From, "/"), m.To, m.From, m.To)
	}
	for _, m := range moved {
		if _, err := os.Stat(filepath.Join(out, m.To, "index.html")); err != nil {
			return fmt.Errorf("redirect /%s/ to /%s/: %w", m.From, m.To, err)
		}
		fmt.Fprintf(&b, "/%[1]s /%[2]s/ 301\n/%[1]s/ /%[2]s/ 301\n/%[1]s/* /%[2]s/:splat 301\n",
			m.From, m.To)
	}
	if err := os.WriteFile(filepath.Join(out, "_redirects"), []byte(b.String()), 0o644); err != nil {
		return fmt.Errorf("_redirects: %w", err)
	}
	return nil
}
