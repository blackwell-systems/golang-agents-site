package main

import (
	"html/template"
	"regexp"
	"strings"
)

var (
	letterStart = regexp.MustCompile(`<h2 id="letter-[a-z]"`)
	termStart   = regexp.MustCompile(`<h3 `)
)

// glossaryCards groups the glossary's rendered HTML for its layout:
// each letter's heading and terms in a section, and each term, its
// heading and the text under it, in a card the stylesheet sets in a
// grid. Text before the first letter is left as it is.
func glossaryCards(body template.HTML) template.HTML {
	s := string(body)
	starts := letterStart.FindAllStringIndex(s, -1)
	if len(starts) == 0 {
		return body
	}
	var b strings.Builder
	b.WriteString(s[:starts[0][0]])
	for i, st := range starts {
		end := len(s)
		if i+1 < len(starts) {
			end = starts[i+1][0]
		}
		sec := s[st[0]:end]
		terms := termStart.FindAllStringIndex(sec, -1)
		if len(terms) == 0 {
			b.WriteString(sec)
			continue
		}
		b.WriteString(`<section class="gl-letter">`)
		b.WriteString(sec[:terms[0][0]])
		b.WriteString(`<div class="gl-terms">`)
		for j, t := range terms {
			tend := len(sec)
			if j+1 < len(terms) {
				tend = terms[j+1][0]
			}
			b.WriteString(`<div class="gl-term">`)
			b.WriteString(strings.TrimSpace(sec[t[0]:tend]))
			b.WriteString("</div>\n")
		}
		b.WriteString("</div></section>\n")
	}
	return template.HTML(b.String())
}
