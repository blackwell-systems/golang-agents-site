package main

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"slices"
	"strings"

	"github.com/yuin/goldmark/util"
)

// Blocks are fenced code blocks whose language names a layout instead
// of a language. Their text is prose, in Markdown, so the copy check
// reads it (proseFence in copycheck.go).
//
//	```result     one measured result, with its method
//	```fit        when to reach for a design, and when to leave it out
//	```catalog    the table of runnable examples, with computed counts

// blockLangs are the languages that render as blocks.
var blockLangs = map[string]bool{"result": true, "fit": true, "catalog": true}

// renderBlock writes the block named lang, from body.
func (r *codeBlocks) renderBlock(w util.BufWriter, lang, body string) error {
	var out string
	var err error
	switch lang {
	case "result":
		out, err = r.result(body)
	case "fit":
		out, err = r.fit(body)
	case "catalog":
		out, err = r.catalog(body)
	}
	if err != nil {
		return fmt.Errorf("%s block: %w", lang, err)
	}
	w.WriteString(out)
	return nil
}

// fields reads "key: value" lines until the first blank line, and
// returns them with the rest of body, the block's text. A key may
// repeat; its values are kept in order.
func fields(body string, keys ...string) (map[string][]string, string, error) {
	f := map[string][]string{}
	head, rest, _ := strings.Cut(body, "\n\n")
	for line := range strings.SplitSeq(strings.TrimSpace(head), "\n") {
		key, val, ok := strings.Cut(line, ":")
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if !ok || val == "" || !slices.Contains(keys, key) {
			return nil, "", fmt.Errorf("line %q: want one of %s, then a value",
				line, strings.Join(keys, ", "))
		}
		f[key] = append(f[key], val)
	}
	return f, strings.TrimSpace(rest), nil
}

// inline renders a line of Markdown without its paragraph tags.
func (r *codeBlocks) inline(src string) (string, error) {
	var buf bytes.Buffer
	if err := r.md.Convert([]byte(src), &buf); err != nil {
		return "", err
	}
	s := strings.TrimSpace(buf.String())
	s = strings.TrimPrefix(s, "<p>")
	s = strings.TrimSuffix(s, "</p>")
	return s, nil
}

// methodKeys are the parts of a result's method, in the order shown.
// Every result states all four, so no number appears without how it was
// measured.
var methodKeys = []string{"task", "models", "runs", "date"}

// result renders a measured result:
//
//	```result
//	figure: 79
//	unit: of 80 replies
//	task: the agent's first request, ...
//	models: four, ...
//	runs: ten per condition
//	date: October 2026
//	source: Chapter 10
//	link: /findings/tool-description-poisoning/
//	more: The finding
//
//	What the number means, in Markdown.
//	```
//
// unit, link, and more are optional; the others are required. link may
// be a code: link, as in Markdown.
func (r *codeBlocks) result(body string) (string, error) {
	keys := append([]string{"figure", "unit", "source", "link", "more"}, methodKeys...)
	f, text, err := fields(body, keys...)
	if err != nil {
		return "", err
	}
	for _, k := range append([]string{"figure", "source"}, methodKeys...) {
		if len(f[k]) != 1 {
			return "", fmt.Errorf("want one %s: line", k)
		}
	}
	if text == "" {
		return "", fmt.Errorf("no text after the fields")
	}
	var b strings.Builder
	b.WriteString(`<div class="result">`)
	fmt.Fprintf(&b, `<p class="figure">%s`, html.EscapeString(f["figure"][0]))
	if u := f["unit"]; len(u) > 0 {
		fmt.Fprintf(&b, ` <span>%s</span>`, html.EscapeString(u[0]))
	}
	b.WriteString(`</p>`)
	t, err := r.inline(text)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, `<p>%s</p><dl class="method">`, t)
	for _, k := range methodKeys {
		v, err := r.inline(f[k][0])
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&b, `<div><dt>%s</dt><dd>%s</dd></div>`, strings.ToUpper(k[:1])+k[1:], v)
	}
	b.WriteString(`</dl><p class="result-source">`)
	b.WriteString(html.EscapeString(f["source"][0]))
	if l := f["link"]; len(l) > 0 {
		more := "Read more"
		if m := f["more"]; len(m) > 0 {
			more = m[0]
		}
		href := l[0]
		if rel, ok := strings.CutPrefix(href, "code:"); ok {
			if href, err = r.code.url(rel, 0, 0); err != nil {
				return "", err
			}
		}
		fmt.Fprintf(&b, `<span aria-hidden="true"> · </span><a href="%s">%s</a>`,
			html.EscapeString(href), html.EscapeString(more))
	}
	b.WriteString("</p></div>\n")
	return b.String(), nil
}

// cites matches a chapter citation, which every reason in a fit block
// carries: the book result behind it.
var cites = regexp.MustCompile(`\b[Cc]hapters? \d+`)

// fit renders when to use a design and when not to:
//
//	```fit
//	reach: a reason, citing its chapter (chapter 9)
//	leave: a reason, citing its chapter (chapter 9)
//	```
//
// Each reason must cite a chapter, so the build fails on one that
// doesn't say where it was measured.
func (r *codeBlocks) fit(body string) (string, error) {
	f, rest, err := fields(body, "reach", "leave")
	if err != nil {
		return "", err
	}
	if rest != "" {
		return "", fmt.Errorf("text after the reasons: %q", rest)
	}
	if len(f["reach"]) == 0 || len(f["leave"]) == 0 {
		return "", fmt.Errorf("want at least one reach: and one leave: line")
	}
	var b strings.Builder
	b.WriteString(`<div class="fit">`)
	for _, col := range []struct{ key, title string }{
		{"reach", "Reach for it"}, {"leave", "Leave it out"},
	} {
		fmt.Fprintf(&b, `<div class="fit-%s"><p class="fit-title">%s</p><ul>`, col.key, col.title)
		for _, reason := range f[col.key] {
			if !cites.MatchString(reason) {
				return "", fmt.Errorf("%s: %q cites no chapter", col.key, reason)
			}
			t, err := r.inline(reason)
			if err != nil {
				return "", err
			}
			fmt.Fprintf(&b, "<li>%s</li>", t)
		}
		b.WriteString(`</ul></div>`)
	}
	b.WriteString("</div>\n")
	return b.String(), nil
}

// catalog renders the examples table. Each line is
//
//	dir | what it shows | command | guide
//
// where dir is a directory under code/, the command is how to run it
// from code/ (in backticks), and the guide, which may be empty, is a
// page on this site. The counts come from the code itself.
func (r *codeBlocks) catalog(body string) (string, error) {
	var b strings.Builder
	b.WriteString(`<section class="table-scroll" aria-label="The examples" tabindex="0"><table class="catalog"><thead><tr>` +
		`<th scope="col">Code</th><th scope="col">What it shows</th>` +
		`<th scope="col" class="num">Go files</th><th scope="col" class="num">Lines of Go</th>` +
		`<th scope="col" class="num">Tests</th><th scope="col">Run it</th><th scope="col">Guide</th>` +
		"</tr></thead><tbody>")
	for line := range strings.SplitSeq(strings.TrimSpace(body), "\n") {
		cells := strings.Split(line, "|")
		if len(cells) != 4 {
			return "", fmt.Errorf("line %q: want dir | what | command | guide", line)
		}
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		dir, what, cmd, guide := cells[0], cells[1], cells[2], cells[3]
		u, err := r.code.url(dir, 0, 0)
		if err != nil {
			return "", err
		}
		st, err := r.code.stats(dir)
		if err != nil {
			return "", err
		}
		w, err := r.inline(what)
		if err != nil {
			return "", err
		}
		c, err := r.inline(cmd)
		if err != nil {
			return "", err
		}
		g := `<span class="none">none yet</span>`
		if guide != "" {
			g = fmt.Sprintf(`<a href="%s">Guide</a>`, html.EscapeString(guide))
		}
		fmt.Fprintf(&b, `<tr><th scope="row"><a href="%s"><code>%s/</code></a></th><td>%s</td>`+
			`<td class="num">%s</td><td class="num">%s</td><td class="num">%s</td><td>%s</td><td>%s</td></tr>`,
			html.EscapeString(u), html.EscapeString(dir), w,
			thousands(st.files), thousands(st.lines), thousands(st.tests), c, g)
	}
	b.WriteString("</tbody></table></section>\n")
	return b.String(), nil
}
