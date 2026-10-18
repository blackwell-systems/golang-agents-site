package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExcerpt(t *testing.T) {
	dir := t.TempDir()
	src := "package x\n\n// tag::run[]\n\n// Run runs.\nfunc Run() {\n\t// tag::inner[]\n\tgo f() // <1>\n\t// end::inner[]\n}\n\n// end::run[]\n"
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	c := &codeRoot{dir: dir}
	tests := []struct {
		tag, lines  string
		want        string
		first, last int
	}{
		{"run", "", "// Run runs.\nfunc Run() {\n\tgo f()\n}\n", 5, 10},
		{"inner", "", "go f()\n", 8, 8},
		{"", "1..1", "package x\n", 1, 1},
		{"", "", "package x\n\n\n// Run runs.\nfunc Run() {\n\tgo f()\n}\n", 0, 0}, // tag lines dropped
	}
	for _, tt := range tests {
		ex, err := c.excerpt("x.go", tt.tag, tt.lines)
		if err != nil {
			t.Fatalf("excerpt(%q, %q): %v", tt.tag, tt.lines, err)
		}
		if ex.text != tt.want || ex.first != tt.first || ex.last != tt.last {
			t.Errorf("excerpt(%q, %q) = %q, %d-%d; want %q, %d-%d",
				tt.tag, tt.lines, ex.text, ex.first, ex.last, tt.want, tt.first, tt.last)
		}
	}
	if _, err := c.excerpt("x.go", "missing", ""); err == nil {
		t.Error("excerpt of a missing tag: no error")
	}
}

func TestParseInfo(t *testing.T) {
	lang, attrs, err := parseInfo(`go include=ch01/a.go tag=run title="A title, with spaces"`)
	if err != nil {
		t.Fatal(err)
	}
	if lang != "go" || attrs["include"] != "ch01/a.go" || attrs["tag"] != "run" ||
		attrs["title"] != "A title, with spaces" {
		t.Errorf("parseInfo = %q, %v", lang, attrs)
	}
	if _, _, err := parseInfo("go colour=red"); err == nil {
		t.Error("parseInfo with an unknown attribute: no error")
	}
}

func TestFrontMatter(t *testing.T) {
	m, body, err := frontMatter([]byte("---\ntitle: \"A: b\"\nweight: 3\n---\nBody\n"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Title != "A: b" || m.Weight != 3 || string(body) != "Body\n" {
		t.Errorf("frontMatter = %+v, %q", m, body)
	}
	if _, _, err := frontMatter([]byte("---\ntitel: x\n---\n")); err == nil {
		t.Error("frontMatter with a misspelled key: no error")
	}
}

func TestText(t *testing.T) {
	tests := []struct{ got, want string }{
		{chapterText("1"), "chapter 1"},
		{chapterText("4, 2"), "chapters 4 and 2"},
		{chapterText("1, 4, 5"), "chapters 1, 4, and 5"},
		{smartQuotes(`the model's "first" move`), "the model’s “first” move"},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("got %q, want %q", tt.got, tt.want)
		}
	}
}

func TestConvertRejectsEmDash(t *testing.T) {
	code := &codeRoot{dir: t.TempDir()}
	_, err := convert(newMarkdown(code), code, []byte("a — b"))
	if err == nil || !strings.Contains(err.Error(), "em dash") {
		t.Errorf("convert with an em dash: err = %v", err)
	}
}
