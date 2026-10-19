package main

import (
	"encoding/json"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"
)

// The site's search runs in the browser, from a small index the build
// writes: each page's title, description, section, and headings, with
// their ids, so a result can link to the heading that matched. The
// script is static/search.js, and the search page (layout search) lists
// every page without JavaScript.

// searchEntry is one page in the index. The keys are short to keep the
// file small.
type searchEntry struct {
	URL      string     `json:"u"`
	Title    string     `json:"t"`
	Section  string     `json:"s,omitempty"`
	Desc     string     `json:"d"`
	Headings [][]string `json:"h,omitempty"` // [id, text] pairs, h2 and h3
}

var headingTag = regexp.MustCompile(`(?s)<h([23]) id="([^"]+)"[^>]*>(.*?)</h[23]>`)

// headings lists the h2 and h3 headings in body, leaving out one-letter
// headings such as the glossary's letters.
func headings(body string) [][]string {
	var out [][]string
	for _, m := range headingTag.FindAllStringSubmatch(body, -1) {
		text := strings.TrimSpace(html.UnescapeString(anyTag.ReplaceAllString(m[3], "")))
		if utf8.RuneCountInString(text) < 2 {
			continue
		}
		out = append(out, []string{html.UnescapeString(m[2]), text})
	}
	return out
}

// searchIndex builds the index from the published pages.
func searchIndex(s *Site) []searchEntry {
	var out []searchEntry
	for _, p := range s.Pages {
		if p.URL == "/404.html" || p.Layout == "search" {
			continue
		}
		e := searchEntry{URL: p.URL, Title: p.Title, Desc: p.Description,
			Headings: headings(string(p.Body))}
		if sec := s.section(p); sec != nil {
			e.Section = sec.Title
		}
		out = append(out, e)
	}
	return out
}

// writeSearchIndex writes search.json.
func writeSearchIndex(out string, s *Site) error {
	b, err := json.Marshal(searchIndex(s))
	if err != nil {
		return fmt.Errorf("search index: %w", err)
	}
	if err := os.WriteFile(filepath.Join(out, "search.json"), append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("search index: %w", err)
	}
	return nil
}
