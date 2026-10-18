package main

import (
	"bytes"
	"errors"
	"fmt"
	"html"
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
// highlighted by chroma, excerpts included from the companion code, and
// code: links turned into links to the repository.
func newMarkdown(code *codeRoot) goldmark.Markdown {
	blocks := &codeBlocks{code: code}
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
			parser.WithASTTransformers(util.Prioritized(codeLinks{code}, 100)),
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

// codeBlocks renders fenced code blocks. A block whose info string has
// include= takes its text from the companion code:
//
//	```go include=ch01/agent/loop.go tag=run
//	```
//
// tag= picks the lines between "// tag::run[]" and "// end::run[]";
// lines=2..-1 picks a range of a whole file. title= names the listing
// in its caption; without it, the caption is the file's path. A block
// whose language is in blockLangs is a layout instead (blocks.go).
type codeBlocks struct {
	code *codeRoot
	md   goldmark.Markdown // the converter, for the Markdown inside blocks
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
	}
	code, err := highlight(lang, body.String())
	if err != nil {
		return ast.WalkStop, err
	}
	fmt.Fprintf(w, "<figure class=\"listing\" data-lang=\"%s\">", html.EscapeString(lang))
	switch {
	case caption != "" && href != "":
		fmt.Fprintf(w, "<figcaption><a href=\"%s\">%s</a></figcaption>",
			html.EscapeString(href), html.EscapeString(caption))
	case caption != "":
		fmt.Fprintf(w, "<figcaption>%s</figcaption>", html.EscapeString(caption))
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
		case "include", "tag", "lines", "title":
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

// highlight returns code as highlighted HTML, in a <pre>. A language
// chroma doesn't know is shown as plain text.
func highlight(lang, code string) (string, error) {
	lexer := lexers.Get(lang)
	if lang == "" || lexer == nil {
		lexer = lexers.Get("plaintext")
	}
	lexer = chroma.Coalesce(lexer)
	it, err := lexer.Tokenise(nil, code)
	if err != nil {
		return "", fmt.Errorf("highlight %s: %w", lang, err)
	}
	var buf bytes.Buffer
	if err := formatter.Format(&buf, styles.Fallback, it); err != nil {
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
