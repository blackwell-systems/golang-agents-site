package main

import (
	"html"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// writeFiles writes files, by path under dir.
func writeFiles(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// refCode is a small companion module: an agent package, and a main
// package that uses it.
var refCode = map[string]string{
	"go.mod": "module example.com/code\n\ngo 1.27\n",
	"ch01/agent/agent.go": `// Package agent is a test package.
package agent

import (
	"context"
	"errors"
)

// ErrMaxTurns means the run reached its turn limit. More text.
var ErrMaxTurns = errors.New("agent: max turns reached")

// Version is the version.
const Version = "1"

// Model produces replies.
type Model interface {
	// Generate returns the next reply.
	Generate(ctx context.Context, req Request) (Reply, error)
}

// Request is what the model gets.
type Request struct {
	Messages []string // the conversation
}

// Reply is the model's answer.
type Reply struct{ Text string }

// Agent runs a model.
type Agent struct{ M Model }

// Run runs the agent.
func (a *Agent) Run(ctx context.Context) (Reply, error) {
	return a.M.Generate(ctx, Request{})
}

// New returns an agent.
func New(m Model) *Agent { return &Agent{M: m} }

// Map is generic.
func Map[T any](xs []T) []T { return xs }
`,
	"ch01/main.go": `package main

import (
	"context"
	"fmt"

	"example.com/code/ch01/agent"
)

// tag::run[]
func run(ctx context.Context, m agent.Model) error {
	a := agent.New(m)
	reply, err := a.Run(ctx)
	if err == agent.ErrMaxTurns {
		return err
	}
	req := agent.Request{Messages: []string{reply.Text}}
	_ = len(req.Messages)
	fmt.Println(agent.Version, helper(), agent.Map([]int{1}))
	var helper = 2 // shadows the function below
	_ = helper
	return nil
}
// end::run[]

// tag::helper[]
// helper helps.
func helper() string { return limit() }

func limit() string { return "x" }
// end::helper[]
`,
}

func TestRefs(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, refCode)
	c := &codeRoot{dir: dir}
	tests := []struct {
		file, tag string
		want      []string // the names decorated, in order
	}{
		// Names from the agent package, through selectors; a method
		// through a value of a companion type; the package-level
		// helper, but not the local variable that shadows it later;
		// never context, fmt, len, a field, or a local.
		{"ch01/main.go", "run", []string{"Model", "New", "Run", "ErrMaxTurns",
			"Request", "Version", "helper", "Map"}},
		// limit is declared in this listing, so only helper's own body
		// would show it; it's left alone.
		{"ch01/main.go", "helper", nil},
		// A whole file declares everything it uses.
		{"ch01/agent/agent.go", "", nil},
	}
	for _, tt := range tests {
		ex, err := c.excerpt(tt.file, tt.tag, "")
		if err != nil {
			t.Fatal(err)
		}
		refs, err := c.refs(tt.file, ex)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, r := range refs {
			got = append(got, ex.text[r.off:r.end])
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("refs(%s, %q) = %q; want %q", tt.file, tt.tag, got, tt.want)
		}
	}
}

func TestRefDefs(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, refCode)
	c := &codeRoot{dir: dir}
	ex, err := c.excerpt("ch01/main.go", "run", "")
	if err != nil {
		t.Fatal(err)
	}
	refs, err := c.refs("ch01/main.go", ex)
	if err != nil {
		t.Fatal(err)
	}
	defs := map[string]*goDef{}
	for _, r := range refs {
		defs[ex.text[r.off:r.end]] = r.def
	}
	tests := []struct {
		name, sig, doc string
		rel            string
		line           int
	}{
		{"ErrMaxTurns", `var ErrMaxTurns = errors.New("agent: max turns reached")`,
			"ErrMaxTurns means the run reached its turn limit.", "ch01/agent/agent.go", 10},
		{"Version", `const Version = "1"`, "Version is the version.", "ch01/agent/agent.go", 13},
		{"Model", "type Model interface {\n\t// Generate returns the next reply.\n" +
			"\tGenerate(ctx context.Context, req Request) (Reply, error)\n}",
			"Model produces replies.", "ch01/agent/agent.go", 16},
		{"Request", "type Request struct {\n\tMessages []string // the conversation\n}",
			"Request is what the model gets.", "ch01/agent/agent.go", 22},
		{"Run", "func (a *Agent) Run(ctx context.Context) (Reply, error)",
			"Run runs the agent.", "ch01/agent/agent.go", 33},
		{"New", "func New(m Model) *Agent", "New returns an agent.", "ch01/agent/agent.go", 38},
		{"Map", "func Map[T any](xs []T) []T", "Map is generic.", "ch01/agent/agent.go", 41},
		{"helper", "func helper() string", "helper helps.", "ch01/main.go", 28},
	}
	for _, tt := range tests {
		d := defs[tt.name]
		if d == nil {
			t.Errorf("%s: not found", tt.name)
			continue
		}
		if d.sig != tt.sig || d.doc != tt.doc || d.rel != tt.rel || d.line != tt.line {
			t.Errorf("%s = %q, %q, %s:%d; want %q, %q, %s:%d", tt.name,
				d.sig, d.doc, d.rel, d.line, tt.sig, tt.doc, tt.rel, tt.line)
		}
	}
}

func TestInterfaceMethodDef(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, refCode)
	c := &codeRoot{dir: dir}
	ex, err := c.excerpt("ch01/agent/agent.go", "", "32..35") // Run
	if err != nil {
		t.Fatal(err)
	}
	refs, err := c.refs("ch01/agent/agent.go", ex)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range refs {
		got = append(got, ex.text[r.off:r.end]+": "+r.def.sig)
	}
	want := []string{
		"Agent: type Agent struct{ M Model }",
		"Reply: type Reply struct{ Text string }",
		"Generate: func (Model) Generate(ctx context.Context, req Request) (Reply, error)",
		"Request: type Request struct {\n\tMessages []string // the conversation\n}",
	}
	if !slices.Equal(got, want) {
		t.Errorf("refs = %q;\nwant %q", got, want)
	}
}

func TestAbbreviate(t *testing.T) {
	long := "type T struct {\n" + strings.Repeat("\tF int\n", 20) + "}"
	got := abbreviate(long)
	if n := strings.Count(got, "\n") + 1; n != sigLines {
		t.Errorf("abbreviate: %d lines; want %d", n, sigLines)
	}
	if !strings.HasSuffix(got, "\t// ...\n}") {
		t.Errorf("abbreviate = %q; want it to end with an elision and the brace", got)
	}
	if short := "type T int"; abbreviate(short) != short {
		t.Errorf("abbreviate(%q) changed it", short)
	}
}

func TestDecorate(t *testing.T) {
	tests := []struct {
		html  string
		marks []mark
		want  string
	}{
		{`<span class="n">foo</span>.<span class="n">Bar</span>`,
			[]mark{{4, 7, "[", "]"}},
			`<span class="n">foo</span>.<span class="n">[Bar]</span>`},
		// Entities count as the characters they stand for.
		{`a &lt; <b>Req</b>`, []mark{{4, 7, "[", "]"}}, `a &lt; <b>[Req]</b>`},
		// Several characters of UTF-8 count by their bytes.
		{`é <i>X</i>`, []mark{{3, 4, "[", "]"}}, `é <i>[X]</i>`},
		// A mark across a tag is closed and opened around it.
		{`ab<i>cd</i>`, []mark{{1, 3, "[", "]"}}, `a[b]<i>[c]d</i>`},
		{`<i>ab</i><i>cd</i>`, []mark{{0, 2, "[", "]"}, {2, 4, "(", ")"}},
			`<i>[ab]</i><i>(cd)</i>`},
	}
	for _, tt := range tests {
		if got := decorate(tt.html, tt.marks); got != tt.want {
			t.Errorf("decorate(%q) = %q; want %q", tt.html, got, tt.want)
		}
	}
}

var anyTagRE = regexp.MustCompile(`<[^>]+>`)

// TestListingRefs renders an included listing and checks that its text,
// as a browser's copy of the <pre> would see it, is still the code, and
// that every name links to a popover on the page.
func TestListingRefs(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, refCode)
	c := &codeRoot{dir: dir}
	md := newMarkdown(c, newPageIndex(nil, nil))
	out, err := convert(md, c, []byte("```go include=ch01/main.go tag=run\n```\n"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	pre := s[strings.Index(s, "<pre"):strings.Index(s, "</pre>")]
	ex, _ := c.excerpt("ch01/main.go", "run", "")
	if got := html.UnescapeString(anyTagRE.ReplaceAllString(pre, "")); got != ex.text {
		t.Errorf("listing text = %q; want the code, %q", got, ex.text)
	}
	for _, m := range regexp.MustCompile(`data-def="([^"]+)"`).FindAllStringSubmatch(s, -1) {
		if !strings.Contains(s, `id="`+m[1]+`"`) {
			t.Errorf("no popover %s", m[1])
		}
	}
	if !strings.Contains(s, `<div class="ref-defs" hidden>`) ||
		!strings.Contains(s, "ch01/agent/agent.go#L10\">ch01/agent/agent.go:10</a>") {
		t.Errorf("listing has no popovers, or a wrong location:\n%s", s)
	}
}

func TestValueSig(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{"go.mod": "module m\n", "p/p.go": `package p

const Short = "short"

const Long = "the first line of a long string, " +
	"and the second"

var Typed time.Duration = 3

var Huge = map[string]int{
	"a": 1,
	"b": 2,
	"c": 3, "d": 4, "e": 5, "f": 6, "g": 7, "h": 8, "i": 9, "j": 10, "k": 11,
}

const (
	A, B = 1, 2
)
`})
	x, err := newGoIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := x.parse(filepath.Join(dir, "p", "p.go")); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range x.defs {
		got = append(got, d.sig)
	}
	slices.Sort(got)
	want := []string{
		`const A = 1`,
		`const B = 2`,
		`const Long = "the first line of a long string, " + ...`,
		`const Short = "short"`,
		`var Huge = map[string]int{ ...`,
		`var Typed time.Duration = 3`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("sigs = %q;\nwant %q", got, want)
	}
}

func TestLineRanges(t *testing.T) {
	tests := []struct {
		spec string
		want [][2]int
		ok   bool
	}{
		{"", nil, true},
		{"3", [][2]int{{3, 3}}, true},
		{"3-5,9", [][2]int{{3, 5}, {9, 9}}, true},
		{"1-10", [][2]int{{1, 10}}, true},
		{"0", nil, false},
		{"5-3", nil, false},
		{"11", nil, false}, // past the end
		{"3-", nil, false},
		{"a", nil, false},
		{"3,,4", nil, false},
	}
	for _, tt := range tests {
		got, err := lineRanges(tt.spec, 10)
		if (err == nil) != tt.ok || !slices.Equal(got, tt.want) {
			t.Errorf("lineRanges(%q) = %v, %v; want %v, ok %v", tt.spec, got, err, tt.want, tt.ok)
		}
	}
}

// TestHighlightLines checks that highlight= marks the lines it names,
// counted in the excerpt, and only those.
func TestHighlightLines(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, refCode)
	c := &codeRoot{dir: dir}
	md := newMarkdown(c, newPageIndex(nil, nil))
	out, err := convert(md, c, []byte("```go include=ch01/main.go tag=run highlight=2-3,5\n```\n"))
	if err != nil {
		t.Fatal(err)
	}
	var marked []int
	for i, line := range strings.Split(string(out), `<span class="line`)[1:] {
		if strings.HasPrefix(line, ` hl"`) {
			marked = append(marked, i+1)
		}
	}
	if want := []int{2, 3, 5}; !slices.Equal(marked, want) {
		t.Errorf("marked lines %v; want %v", marked, want)
	}
	if _, err := convert(md, c, []byte("```go include=ch01/main.go tag=run highlight=40\n```\n")); err == nil {
		t.Error("highlight past the end: no error")
	}
}
