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
	_, err := convert(newMarkdown(code, newPageIndex(nil, nil)), code, []byte("a — b"))
	if err == nil || !strings.Contains(err.Error(), "em dash") {
		t.Errorf("convert with an em dash: err = %v", err)
	}
}

func TestBlocks(t *testing.T) {
	code := &codeRoot{dir: t.TempDir()}
	md := newMarkdown(code, newPageIndex(nil, nil))
	result := "```result\nfigure: 79\nunit: of 80\ntask: a *first* request\nmodels: four\nruns: ten each\ndate: October 2026\nsource: Chapter 10\nlink: /findings/x/\n\nOne sentence decided it.\n```\n"
	fit := "```fit\nreach: many groups (chapter 9)\nleave: a small batch (chapters 8 and 9)\n```\n"
	tests := []struct {
		src  string
		want []string // substrings of the output
		bad  bool
	}{
		{result, []string{`<p class="figure">79 <span>of 80</span></p>`, "<dt>Task</dt><dd>a <em>first</em> request</dd>",
			"<dt>Date</dt><dd>October 2026</dd>", `<a href="/findings/x/">Read more</a>`}, false},
		{strings.Replace(result, "date: October 2026\n", "", 1), nil, true}, // no method date
		{strings.Replace(result, "One sentence decided it.\n", "", 1), nil, true},
		{fit, []string{`Reach for it</p><ul><li>many groups (chapter 9)</li></ul>`, `#block-reach"></use></svg>Reach for it`, "Leave it out"}, false},
		{"```fit\nreach: x (chapter 9)\n```\n", nil, true}, // no leave
	}
	for _, tt := range tests {
		out, err := convert(md, code, []byte(tt.src))
		if tt.bad {
			if err == nil {
				t.Errorf("convert(%q): no error", tt.src)
			}
			continue
		}
		if err != nil {
			t.Fatalf("convert(%q): %v", tt.src, err)
		}
		for _, w := range tt.want {
			if !strings.Contains(string(out), w) {
				t.Errorf("convert(%q) = %s\nwant it to contain %s", tt.src, out, w)
			}
		}
	}
}

func TestCounts(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":                 "module x\n\ngo 1.27.0\n",
		"ch01/a.go":              "package a\n\nfunc A() {}\n",
		"ch01/a_test.go":         "package a\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\nfunc TestB(t *testing.T) {}\nfunc helper(t *testing.T) {}\n",
		"ch01/testdata/fix.go":   "package fixture\n",
		"ch02/b/b.go":            "package b\n",
		"ch02/b/.hidden/skip.go": "package skip\n",
	}
	for name, src := range files {
		f := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c := &codeRoot{dir: dir}
	got, err := c.fillCounts([]byte("{{code files}} files, {{code lines ch01}} lines, {{code tests}} tests, {{code packages}} packages, Go {{code go-version}}"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "3 files, 10 lines, 2 tests, 2 packages, Go 1.27"; string(got) != want {
		t.Errorf("fillCounts = %q, want %q", got, want)
	}
	for _, bad := range []string{"{{code bytes}}", "{{code files nowhere}}", "{{ code files }}"} {
		if _, err := c.fillCounts([]byte(bad)); err == nil {
			t.Errorf("fillCounts(%q): no error", bad)
		}
	}
	if s := thousands(1644); s != "1,644" {
		t.Errorf("thousands(1644) = %q", s)
	}
}

func TestRedirects(t *testing.T) {
	out := t.TempDir()
	if err := writeRedirects(out); err == nil {
		t.Error("writeRedirects with no new sections built: no error")
	}
	for _, m := range movedPages {
		if err := os.MkdirAll(filepath.Join(out, filepath.FromSlash(m.To)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, filepath.FromSlash(m.To), "index.html"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, m := range moved {
		if err := os.MkdirAll(filepath.Join(out, m.To), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(out, m.To, "index.html"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeRedirects(out); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(out, "_redirects"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(string(b), "/loop/why-go/ ") > strings.Index(string(b), "/loop/* ") {
		t.Errorf("a moved page's redirect comes after its old section's:\n%s", b)
	}
	for _, line := range []string{"/foundations /basics/ 301\n", "/loop/* /basics/:splat 301\n", "/providers/ /ecosystem/ 301\n", "/loop/why-go/ /ecosystem/why-go/ 301\n"} {
		if !strings.Contains(string(b), line) {
			t.Errorf("_redirects lacks %q:\n%s", line, b)
		}
	}
}

func TestPicture(t *testing.T) {
	got, err := picture("ch04", "guide", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<source type="image/avif" srcset="/img/art/ch04-480.avif 480w, /img/art/ch04-720.avif 720w, /img/art/ch04-1080.avif 1080w"`,
		`width="480" height="600"`, `loading="eager"`, `decoding="async"`, `alt="The Gopher holds up`,
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("picture(ch04) = %s\nwant it to contain %s", got, want)
		}
	}
	if got, _ := picture("gopher", "badge", true); !strings.Contains(string(got), `width="240" height="240"`) ||
		!strings.Contains(string(got), `loading="lazy"`) || !strings.Contains(string(got), "gopher-240.png") {
		t.Errorf("picture(gopher) = %s", got)
	}
	if _, err := picture("ch99", "guide", false); err == nil {
		t.Error("picture of an unknown name: no error")
	}
	if err := checkArt(t.TempDir(), "ch04"); err == nil {
		t.Error("checkArt with no files: no error")
	}
	for name, a := range arts {
		if strings.Contains(a.Alt, "art:") || a.Alt == "" {
			t.Errorf("arts[%q]: alt %q", name, a.Alt)
		}
	}
}

func TestPaths(t *testing.T) {
	a := &Page{URL: "/a/", Title: "A", Description: "d"}
	b := &Page{URL: "/b/", Title: "B", Description: "d", Draft: true}
	c := &Page{URL: "/c/", Title: "C", Description: "d"}
	ix := newPageIndex([]*Page{a, c}, []*Page{b})
	src := "```path\nid: new\ntitle: New\nstep: /a/\nstep: /b/\nstep: /c/\n\nFor someone new.\n```\n"
	rps, err := readPaths(src)
	if err != nil {
		t.Fatal(err)
	}
	p, err := ix.resolve(rps[0])
	if err != nil {
		t.Fatal(err)
	}
	if p.Live() != 2 {
		t.Errorf("Live = %d, want 2", p.Live())
	}
	pl := places([]*Path{p}, a)
	if len(pl) != 1 || pl[0].N != 1 || pl[0].Prev != nil || pl[0].Next != c {
		t.Errorf("places(a) = %+v; want step 1, next c (the draft skipped)", pl)
	}
	if pl := places([]*Path{p}, c); pl[0].Prev != a || pl[0].Next != nil || pl[0].N != 3 {
		t.Errorf("places(c) = %+v", pl[0])
	}
	for _, bad := range []string{
		"```path\nid: x\ntitle: X\nstep: /a/\n\nOne step.\n```\n",            // too few steps
		"```path\nid: x\ntitle: X\nstep: /a/\nstep: /a/\n\nTwice.\n```\n",    // a repeat
		"```path\nid: X Y\ntitle: X\nstep: /a/\nstep: /c/\n\nBad id.\n```\n", // id
		"```path\nid: x\ntitle: X\nstep: /a/\nstep: /c/\n```\n",              // no text
	} {
		if _, err := readPaths(bad); err == nil {
			t.Errorf("readPaths(%q): no error", bad)
		}
	}
	rp, _ := parsePath("id: x\ntitle: X\nstep: /a/\nstep: /nowhere/\n\nText.")
	if _, err := ix.resolve(rp); err == nil {
		t.Error("resolve with an unknown step: no error")
	}

	code := &codeRoot{dir: t.TempDir()}
	out, err := convert(newMarkdown(code, ix), code, []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<h2 id="new">New</h2>`, `<a href="/a/">`, `<li class="pending"><span class="step-title">B</span>`} {
		if !strings.Contains(string(out), want) {
			t.Errorf("path block = %s\nwant it to contain %s", out, want)
		}
	}
}

func TestPageLinks(t *testing.T) {
	ix := newPageIndex([]*Page{{URL: "/a/"}}, []*Page{{URL: "/b/"}})
	code := &codeRoot{dir: t.TempDir()}
	md := newMarkdown(code, ix)
	out, err := convert(md, code, []byte("See [the a page](page:/a/#x) and [the *b* page](page:/b/)."))
	if err != nil {
		t.Fatal(err)
	}
	if want := `<p>See <a href="/a/#x">the a page</a> and the <em>b</em> page.</p>`; strings.TrimSpace(string(out)) != want {
		t.Errorf("page links = %s, want %s", out, want)
	}
	if _, err := convert(md, code, []byte("[x](page:/nowhere/)")); err == nil {
		t.Error("a page link to no page: no error")
	}
}

func TestHeadings(t *testing.T) {
	body := `<h2 id="letter-a" class="letter">A</h2><h3 id="agent">Agent <code>x</code></h3><h2 id="b">Tools &amp; calls</h2><h4 id="c">No</h4>`
	got := headings(body)
	if len(got) != 2 || got[0][0] != "agent" || got[0][1] != "Agent x" || got[1][1] != "Tools & calls" {
		t.Errorf("headings = %q", got)
	}
}

func TestCheckLinksSprite(t *testing.T) {
	out := t.TempDir()
	if err := os.MkdirAll(filepath.Join(out, "img"), 0o755); err != nil {
		t.Fatal(err)
	}
	sprite := `<svg xmlns="http://www.w3.org/2000/svg"><symbol id="arrow" viewBox="0 0 24 24"/></svg>`
	if err := os.WriteFile(filepath.Join(out, "img", "icons.svg"), []byte(sprite), 0o644); err != nil {
		t.Fatal(err)
	}
	code := &codeRoot{dir: t.TempDir()}
	for _, tt := range []struct {
		page string
		bad  bool
	}{
		{`<svg><use href="/img/icons.svg?v=1#arrow"></use></svg>`, false},
		{`<svg><use href="/img/icons.svg#nowhere"></use></svg>`, true},
	} {
		if err := os.WriteFile(filepath.Join(out, "index.html"), []byte(tt.page), 0o644); err != nil {
			t.Fatal(err)
		}
		_, err := checkLinks(out, code)
		if (err != nil) != tt.bad {
			t.Errorf("checkLinks(%s): error %v, want an error: %v", tt.page, err, tt.bad)
		}
	}
}

func TestLoadIcons(t *testing.T) {
	for _, tt := range []struct {
		sprite string
		bad    bool
	}{
		{`<svg xmlns="http://www.w3.org/2000/svg"><!-- fine --><symbol id="a"/></svg>`, false},
		{`<svg xmlns="http://www.w3.org/2000/svg"><!-- uses --icon-sw --><symbol id="a"/></svg>`, true},
	} {
		static := t.TempDir()
		if err := os.MkdirAll(filepath.Join(static, "img"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(static, "img", "icons.svg"), []byte(tt.sprite), 0o644); err != nil {
			t.Fatal(err)
		}
		err := loadIcons(static)
		if (err != nil) != tt.bad {
			t.Errorf("loadIcons(%s): error %v, want an error: %v", tt.sprite, err, tt.bad)
		}
	}
	iconSprite = "/img/icons.svg"
}
