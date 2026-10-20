package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// themes are the site's reading themes, the same five as the book's.
var themes = []string{"light", "dark", "tokyo-night", "gruvbox-dark", "catppuccin-latte"}

// diagramDir holds the diagrams, NAME-THEME.svg for each theme; build
// sets it from the static directory.
var diagramDir = filepath.Join("static", "img", "diagrams")

var (
	viewBox  = regexp.MustCompile(`viewBox="[-\d.]+ [-\d.]+ ([\d.]+) ([\d.]+)"`)
	bookWord = regexp.MustCompile(`(?i)\bbook\b|\bchapters? \d`)
)

// diagram renders a figure in the reader's theme:
//
//	```diagram
//	name: growing
//	alt: What the diagram shows, for a screen reader.
//	```
//	The caption, in Markdown.
//
// Every theme's version goes in the page and CSS shows the one that
// matches the theme, so the theme menu switches them without a reload.
// Each image gets its width and height from the SVG, so nothing shifts
// as it loads, and hidden images aren't downloaded.
func (r *codeBlocks) diagram(body string) (string, error) {
	f, caption, err := fields(body, "name", "alt")
	if err != nil {
		return "", err
	}
	for _, k := range []string{"name", "alt"} {
		if len(f[k]) != 1 {
			return "", fmt.Errorf("want one %s: line", k)
		}
	}
	name, alt := f["name"][0], f["alt"][0]
	if caption == "" {
		return "", fmt.Errorf("diagram %s: no caption after the fields", name)
	}
	if bookWord.MatchString(alt + " " + caption) {
		return "", fmt.Errorf("diagram %s: the caption or alt text mentions the book or a chapter", name)
	}
	var b strings.Builder
	b.WriteString(`<figure class="diagram">`)
	for _, theme := range themes {
		file := name + "-" + theme + ".svg"
		svg, err := os.ReadFile(filepath.Join(diagramDir, file))
		if err != nil {
			return "", fmt.Errorf("diagram %s: %w", name, err)
		}
		m := viewBox.FindSubmatch(svg)
		if m == nil {
			return "", fmt.Errorf("diagram %s: no viewBox in %s", name, file)
		}
		fmt.Fprintf(&b, `<img class="diagram-theme diagram-%s" src="/img/diagrams/%s" alt="%s" width="%s" height="%s" loading="lazy" decoding="async">`,
			theme, file, html.EscapeString(alt), m[1], m[2])
	}
	c, err := r.inline(caption)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "<figcaption>%s</figcaption></figure>\n", c)
	return b.String(), nil
}

// definition sets a term's definition apart from the text around it:
//
//	```definition
//	term: agent
//
//	An **agent** is ..., in Markdown.
//	```
func (r *codeBlocks) definition(body string) (string, error) {
	f, text, err := fields(body, "term")
	if err != nil {
		return "", err
	}
	if len(f["term"]) != 1 || text == "" {
		return "", fmt.Errorf("definition: want one term: line and the definition after it")
	}
	t, err := r.inline(text)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`<aside class="definition" aria-label="Definition of %s"><p class="definition-label">Definition</p>`+
		`<p class="definition-text">%s</p></aside>`+"\n", html.EscapeString(f["term"][0]), t), nil
}
