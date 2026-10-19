package main

import (
	"strings"
	"unicode"

	"github.com/yuin/goldmark/ast"
	east "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// numberColumns marks the columns of a table that hold figures, such as
// pass rates, costs, and counts, with the class "num", which style.css
// sets right-aligned in tabular digits so the figures line up. A column
// counts as figures when every cell under its header starts with a
// number and is short; one cell of prose leaves the column as text.
type numberColumns struct{}

func (numberColumns) Transform(doc *ast.Document, reader text.Reader, _ parser.Context) {
	src := reader.Source()
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		table, ok := n.(*east.Table)
		if !entering || !ok {
			return ast.WalkContinue, nil
		}
		var rows [][]ast.Node
		for r := table.FirstChild(); r != nil; r = r.NextSibling() {
			var cells []ast.Node
			for c := r.FirstChild(); c != nil; c = c.NextSibling() {
				cells = append(cells, c)
			}
			rows = append(rows, cells)
		}
		if len(rows) < 2 {
			return ast.WalkSkipChildren, nil
		}
		for col := range rows[0] {
			figures, seen := true, 0
			for _, row := range rows[1:] {
				if col >= len(row) {
					continue
				}
				s := cellText(row[col], src)
				if s == "" {
					continue
				}
				seen++
				if !isFigure(s) {
					figures = false
					break
				}
			}
			if !figures || seen == 0 {
				continue
			}
			for _, row := range rows {
				if col < len(row) {
					row[col].SetAttributeString("class", []byte("num"))
				}
			}
		}
		return ast.WalkSkipChildren, nil
	})
}

// cellText returns the text of a table cell, code spans included.
func cellText(cell ast.Node, src []byte) string {
	var b strings.Builder
	ast.Walk(cell, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := n.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(src))
		case *ast.String:
			b.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
	return strings.TrimSpace(b.String())
}

// isFigure reports whether s reads as a figure: a number, perhaps with a
// sign or a currency symbol in front, followed by at most a few words,
// as in "70% (53 to 85%)", "$0.0165", "+3 points", or "0 of 7".
func isFigure(s string) bool {
	t := strings.TrimLeft(s, "$+-−~≈")
	if t == "" || !unicode.IsDigit([]rune(t)[0]) {
		return false
	}
	return len(strings.Fields(s)) <= 6
}
