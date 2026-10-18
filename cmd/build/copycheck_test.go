package main

import (
	"strings"
	"testing"
)

func TestCheckCopy(t *testing.T) {
	book := "The harness runs the tools the model asks for and decides when the work is done."
	runs := bookRuns{}
	w := copyWords(book)
	for i := 0; i+copyRun <= len(w); i++ {
		runs[strings.Join(w[i:i+copyRun], " ")] = true
	}
	copied := &Page{Source: "a.md", Meta: Meta{Title: "A", Description: "A page."},
		Body: "It said: the harness runs the tools the model asks for and decides when, more or less."}
	if err := checkCopy(runs, []*Page{copied}); err == nil {
		t.Error("checkCopy missed a copied run")
	}
	own := &Page{Source: "b.md", Meta: Meta{Title: "B", Description: "A page."},
		Body: "The program around the model runs its tools and decides when to stop.\n\n```go\nthe harness runs the tools the model asks for and decides when the work\n```\n"}
	if err := checkCopy(runs, []*Page{own}); err != nil {
		t.Errorf("checkCopy flagged original text or a code block: %v", err)
	}
}

func TestCheckCopyReadsBlocks(t *testing.T) {
	book := "The harness runs the tools the model asks for and decides when the work is done."
	runs := bookRuns{}
	w := copyWords(book)
	for i := 0; i+copyRun <= len(w); i++ {
		runs[strings.Join(w[i:i+copyRun], " ")] = true
	}
	p := &Page{Source: "c.md", Meta: Meta{Title: "C", Description: "A page."},
		Body: "```fit\nreach: the harness runs the tools the model asks for and decides when (chapter 1)\nleave: x (chapter 1)\n```\n"}
	if err := checkCopy(runs, []*Page{p}); err == nil {
		t.Error("checkCopy missed a copied run inside a fit block")
	}
}
