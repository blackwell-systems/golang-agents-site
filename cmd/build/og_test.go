package main

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitTitle(t *testing.T) {
	tests := []struct{ in, title, sub string }{
		{"The agent loop in Go", "The agent loop in Go", ""},
		{"Before you leave an agent running: a checklist", "Before you leave an agent running", "A checklist"},
		{"Iterators: iter.Seq and range-over-func", "Iterators", "iter.Seq and range-over-func"},
		{"910 crash scenarios: 0 duplicates", "910 crash scenarios", "0 duplicates"},
	}
	for _, tt := range tests {
		title, sub := splitTitle(tt.in)
		if title != tt.title || sub != tt.sub {
			t.Errorf("splitTitle(%q) = %q, %q; want %q, %q", tt.in, title, sub, tt.title, tt.sub)
		}
	}
}

func TestSummary(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Search every guide by title. And more.", "Search every guide by title."},
		{"How AI agents work, from the ground up: what an agent is, the loop at its center, calling a model from Go, and tool calling, in more depth.", "How AI agents work, from the ground up"},
		{"Why Go for agents, model APIs and official SDKs, how providers differ, and the libraries and Go agent frameworks you can adopt.", "Why Go for agents"},
		{"Short and whole.", "Short and whole."},
	}
	for _, tt := range tests {
		if got := summary(tt.in); got != tt.want {
			t.Errorf("summary(%q) = %q; want %q", tt.in, got, tt.want)
		}
	}
}

func TestLeadFigure(t *testing.T) {
	body := "\n```result\nfigure: 79\nunit: of 80 `replies`\nsource: Chapter 10\n\nText.\n```\n\nProse.\n"
	if f, u := leadFigure(body); f != "79" || u != "of 80 replies" {
		t.Errorf("leadFigure = %q, %q; want 79, of 80 replies", f, u)
	}
	if f, _ := leadFigure("Prose first.\n\n```result\nfigure: 1\n```\n"); f != "" {
		t.Errorf("leadFigure of a page that opens with prose = %q; want none", f)
	}
}

func TestShareFile(t *testing.T) {
	for url, want := range map[string]string{
		"/":                   "og/home.jpg",
		"/basics/":            "og/basics.jpg",
		"/basics/agent-loop/": "og/basics/agent-loop.jpg",
		"/glossary/":          "og/glossary.jpg",
	} {
		if got := shareFile(url); got != want {
			t.Errorf("shareFile(%q) = %q; want %q", url, got, want)
		}
	}
}

func TestShareAltLength(t *testing.T) {
	short := &shareCard{Layout: "page", Title: "The agent loop in Go"}
	if alt := shareAlt(short, "ch01"); !strings.Contains(alt, "illustration") {
		t.Errorf("short card's alt %q leaves out the illustration", alt)
	}
	long := &shareCard{Layout: "page", Title: strings.Repeat("x", 300)}
	if alt := shareAlt(long, "ch01"); utf8.RuneCountInString(alt) > 420 {
		t.Errorf("long card's alt is %d characters; want at most 420", utf8.RuneCountInString(alt))
	}
}
