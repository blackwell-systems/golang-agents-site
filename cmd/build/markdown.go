package main

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"

	"github.com/alecthomas/chroma/v2"
	chromahtml "github.com/alecthomas/chroma/v2/formatters/html"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// newMarkdown returns the Markdown converter: GitHub-flavored Markdown
// with heading IDs and {#id} attributes, curly quotes but no dashes
// (the prose rules ban em dashes, so "--" stays as typed), code blocks
// highlighted by chroma, excerpts included from the companion code,
// code: links turned into links to the repository, page: links
// resolved against pages, the site's pages, and the columns of figures
// in tables marked (tables.go).
func newMarkdown(code *codeRoot, pages *pageIndex) goldmark.Markdown {
	blocks := &codeBlocks{code: code, pages: pages}
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.DefinitionList,
			extension.NewTypographer(extension.WithTypographicSubstitutions(
				map[extension.TypographicPunctuation][]byte{
					extension.EnDash: nil,
					extension.EmDash: nil,
				})),
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
			parser.WithAttribute(),
			parser.WithASTTransformers(util.Prioritized(codeLinks{code}, 100),
				util.Prioritized(pageLinks{code, pages}, 100),
				util.Prioritized(numberColumns{}, 100)),
		),
		goldmark.WithRendererOptions(
			gmhtml.WithUnsafe(), // pages may hold HTML blocks, such as figures
			renderer.WithNodeRenderers(util.Prioritized(blocks, 100)),
		),
	)
	blocks.md = md // blocks hold Markdown of their own
	return md
}

// codeLinks rewrites links of the form code:ch01/agent/loop.go into
// links to that file in the companion repository, after checking that
// the file exists.
type codeLinks struct{ code *codeRoot }

func (t codeLinks) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		link, ok := n.(*ast.Link)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		rel, ok := strings.CutPrefix(string(link.Destination), "code:")
		if !ok {
			return ast.WalkContinue, nil
		}
		u, err := t.code.url(rel, 0, 0)
		if err != nil {
			t.code.fail(err)
			return ast.WalkContinue, nil
		}
		link.Destination = []byte(u)
		return ast.WalkContinue, nil
	})
}

// pageLinks resolves links of the form page:/production/approval/ (a
// #fragment may follow). A link to a published page becomes an ordinary
// link; a link to a draft becomes its text alone, so a page can name a
// guide that isn't out yet and gains the link when the guide is
// published. A URL that names no page is an error.
type pageLinks struct {
	code  *codeRoot // collects the errors
	pages *pageIndex
}

func (t pageLinks) Transform(doc *ast.Document, _ text.Reader, _ parser.Context) {
	var unlink []*ast.Link
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		link, ok := n.(*ast.Link)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		dest, ok := strings.CutPrefix(string(link.Destination), "page:")
		if !ok {
			return ast.WalkContinue, nil
		}
		url, _, _ := strings.Cut(dest, "#")
		_, live, err := t.pages.find(url)
		switch {
		case err != nil:
			t.code.fail(fmt.Errorf("page link: %w", err))
		case live:
			link.Destination = []byte(dest)
		default:
			unlink = append(unlink, link)
		}
		return ast.WalkContinue, nil
	})
	for _, link := range unlink {
		parent := link.Parent()
		for c := link.FirstChild(); c != nil; {
			next := c.NextSibling()
			parent.InsertBefore(parent, link, c)
			c = next
		}
		parent.RemoveChild(parent, link)
	}
}

// codeBlocks renders fenced code blocks. A block whose info string has
// include= takes its text from the companion code:
//
//	```go include=ch01/agent/loop.go tag=run
//	```
//
// tag= picks the lines between "// tag::run[]" and "// end::run[]";
// lines=2..-1 picks a range of a whole file. title= names the listing
// in its caption; without it, the caption is the file's path. Any
// block may take highlight=3-5,9, which marks those lines, counted
// from the listing's first. A block whose language is in blockLangs is
// a layout instead (blocks.go).
//
// In an included Go listing, the names that refer to declarations in
// the companion code show those declarations in a popover (godefs.go).
type codeBlocks struct {
	code     *codeRoot
	pages    *pageIndex        // the site's pages, for path blocks
	md       goldmark.Markdown // the converter, for the Markdown inside blocks
	listings int               // listings rendered, for unique ids
}

func (r *codeBlocks) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindFencedCodeBlock, r.render)
}

func (r *codeBlocks) render(w util.BufWriter, src []byte, n ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	block := n.(*ast.FencedCodeBlock)
	var info string
	if block.Info != nil {
		info = string(block.Info.Segment.Value(src))
	}
	lang, attrs, err := parseInfo(info)
	if err != nil {
		return ast.WalkStop, err
	}
	var body bytes.Buffer
	for i := range block.Lines().Len() {
		seg := block.Lines().At(i)
		body.Write(seg.Value(src))
	}
	if blockLangs[lang] {
		if len(attrs) > 0 {
			return ast.WalkStop, fmt.Errorf("%s block: takes no attributes", lang)
		}
		if err := r.renderBlock(w, lang, body.String()); err != nil {
			return ast.WalkStop, err
		}
		return ast.WalkSkipChildren, nil
	}
	caption, href := attrs["title"], ""
	var refs []goRef
	if file := attrs["include"]; file != "" {
		if body.Len() > 0 {
			return ast.WalkStop, fmt.Errorf("include %s: the block must be empty", file)
		}
		ex, err := r.code.excerpt(file, attrs["tag"], attrs["lines"])
		if err != nil {
			return ast.WalkStop, err
		}
		body.WriteString(ex.text)
		if caption == "" {
			caption = file
		}
		if href, err = r.code.url(file, ex.first, ex.last); err != nil {
			return ast.WalkStop, err
		}
		if lang == "go" && strings.HasSuffix(file, ".go") {
			if refs, err = r.code.refs(file, ex); err != nil {
				return ast.WalkStop, err
			}
		}
	}
	marked, err := lineRanges(attrs["highlight"], strings.Count(body.String(), "\n"))
	if err != nil {
		return ast.WalkStop, fmt.Errorf("code block %q: %w", info, err)
	}
	code, err := highlight(lang, body.String(), marked...)
	if err != nil {
		return ast.WalkStop, err
	}
	staged := attrs["stages"] != ""
	if staged {
		if code, err = markStages(code, attrs["stages"], strings.Count(body.String(), "\n")); err != nil {
			return ast.WalkStop, fmt.Errorf("code block %q: %w", info, err)
		}
	}
	r.listings++
	code = r.linkRefs(code, refs)
	hint := ""
	if len(refs) > 0 {
		// A listing whose names have popovers says so in its title bar;
		// the script fills in the hint only where popovers work
		// (base.html).
		hint = `<span class="listing-hint" aria-hidden="true"></span>`
	}
	class := "listing"
	if staged {
		class += " has-stages"
	}
	fmt.Fprintf(w, "<figure class=\"%s\" data-lang=\"%s\">", class, html.EscapeString(lang))
	// Every listing gets a title bar: its file or title, or else a
	// label for what kind of text it holds.
	switch {
	case caption != "" && href != "":
		fmt.Fprintf(w, "<figcaption><a href=\"%s\">%s</a>%s</figcaption>",
			html.EscapeString(href), html.EscapeString(caption), hint)
	case caption != "":
		fmt.Fprintf(w, "<figcaption>%s%s</figcaption>", html.EscapeString(caption), hint)
	default:
		fmt.Fprintf(w, "<figcaption>%s%s</figcaption>", html.EscapeString(barLabel(lang)), hint)
	}
	w.WriteString(code)
	w.WriteString("</figure>\n")
	return ast.WalkSkipChildren, nil
}

// parseInfo splits a fenced block's info string into its language and
// its key=value attributes; a value may be quoted to hold spaces.
func parseInfo(info string) (string, map[string]string, error) {
	lang, rest, _ := strings.Cut(strings.TrimSpace(info), " ")
	attrs := map[string]string{}
	for rest = strings.TrimSpace(rest); rest != ""; rest = strings.TrimSpace(rest) {
		key, after, ok := strings.Cut(rest, "=")
		if !ok || strings.ContainsAny(key, " \"") {
			return "", nil, fmt.Errorf("code block %q: want key=value", info)
		}
		var val string
		if strings.HasPrefix(after, `"`) {
			end := strings.Index(after[1:], `"`)
			if end < 0 {
				return "", nil, fmt.Errorf("code block %q: unclosed quote", info)
			}
			val, rest = after[1:end+1], after[end+2:]
		} else {
			val, rest, _ = strings.Cut(after, " ")
		}
		switch key {
		case "include", "tag", "lines", "title", "highlight", "stages":
			attrs[key] = val
		default:
			return "", nil, fmt.Errorf("code block %q: unknown attribute %q", info, key)
		}
	}
	return lang, attrs, nil
}

// formatter writes chroma's tokens as spans with short class names,
// which style.css colors from the palette's syntax tokens, so one
// highlighted block suits both the dark and the light theme.
var formatter = chromahtml.New(chromahtml.WithClasses(true))

// highlight returns code as highlighted HTML, in a <pre>, with the
// lines in marked (1-based, inclusive ranges) marked. A language chroma
// doesn't know is shown as plain text.
func highlight(lang, code string, marked ...[2]int) (string, error) {
	lexer := lexers.Get(lang)
	if lang == "" || lexer == nil {
		lexer = lexers.Get("plaintext")
	}
	lexer = chroma.Coalesce(lexer)
	it, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "", fmt.Errorf("highlight %s: %w", lang, err)
	}
	f := formatter
	if len(marked) > 0 {
		f = chromahtml.New(chromahtml.WithClasses(true), chromahtml.HighlightLines(marked))
	}
	var buf bytes.Buffer
	if err := f.Format(&buf, styles.Fallback, it); err != nil {
		return "", fmt.Errorf("highlight %s: %w", lang, err)
	}
	return buf.String(), nil
}

// fail records an error found while converting a page; goldmark's AST
// transformers have no way to return one.
func (c *codeRoot) fail(err error) { c.errs = append(c.errs, err) }

// takeErr returns the errors recorded since the last call, and clears
// them.
func (c *codeRoot) takeErr() error {
	err := errors.Join(c.errs...)
	c.errs = nil
	return err
}

// barLabel names a listing that has no file or title.
func barLabel(lang string) string {
	switch lang {
	case "console", "sh", "bash":
		return "terminal"
	case "text", "":
		return "output"
	case "json":
		return "JSON"
	case "go":
		return "Go"
	}
	return lang
}

// stageLine is where chroma starts a line of a listing.
var stageLine = regexp.MustCompile(`<span class="line( hl)?">`)

// markStages tags the lines of code, a highlighted listing, with the
// stage of the agent loop each belongs to, from a stages= value such as
// "7-8:1,10-12:4" ("all:3" marks every line): data-stage on every line in
// a range, and on the first
// line of each run of one stage, a badge: an empty element the
// stylesheet draws the stage's number in, with the stage's name beside
// it on hover. The badge holds no text, so copying the listing copies
// only the code, and the offsets the hover popovers count are unchanged.
// n is the number of lines.
func markStages(code, spec string, n int) (string, error) {
	stage := make([]int, n+1) // by line, 1-based; 0 for none
	for part := range strings.SplitSeq(spec, ",") {
		lines, num, ok := strings.Cut(part, ":")
		st, err := strconv.Atoi(num)
		if !ok || err != nil || st < 1 || st > 4 {
			return "", fmt.Errorf("stages %q: want lines and a stage from 1 to 4, such as 7-8:1,10-12:4", spec)
		}
		if lines == "all" {
			lines = fmt.Sprintf("1-%d", n)
		}
		ranges, err := lineRanges(lines, n)
		if err != nil {
			return "", fmt.Errorf("stages %q: %w", spec, err)
		}
		for _, r := range ranges {
			for l := r[0]; l <= r[1]; l++ {
				if stage[l] != 0 {
					return "", fmt.Errorf("stages %q: line %d is in two stages", spec, l)
				}
				stage[l] = st
			}
		}
	}
	line := 0
	return stageLine.ReplaceAllStringFunc(code, func(m string) string {
		line++
		if line > n || stage[line] == 0 {
			return m
		}
		open := strings.TrimSuffix(m, ">") + fmt.Sprintf(` data-stage="%d">`, stage[line])
		if stage[line-1] != stage[line] {
			open += fmt.Sprintf(`<span class="stage-badge" data-n="%d" data-name="%s" aria-hidden="true"></span>`,
				stage[line], stageNames[stage[line]-1])
		}
		return open
	}), nil
}

// lineRanges parses a highlight= value such as "3-5,9" into ranges of
// lines, checking each against n, the number of lines in the listing.
func lineRanges(spec string, n int) ([][2]int, error) {
	if spec == "" {
		return nil, nil
	}
	var out [][2]int
	for part := range strings.SplitSeq(spec, ",") {
		lo, hi, isRange := strings.Cut(part, "-")
		if !isRange {
			hi = lo
		}
		a, err1 := strconv.Atoi(lo)
		b, err2 := strconv.Atoi(hi)
		if err1 != nil || err2 != nil || a < 1 || b < a {
			return nil, fmt.Errorf("highlight %q: want lines such as 3-5,9", spec)
		}
		if b > n {
			return nil, fmt.Errorf("highlight %q: line %d is past the listing's %d lines", spec, b, n)
		}
		out = append(out, [2]int{a, b})
	}
	return out, nil
}
