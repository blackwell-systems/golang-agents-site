package main

import (
	"bufio"
	"bytes"
	"fmt"
	"go/ast"
	gobuild "go/build"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"html"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Listings of Go from the companion code show what each name refers to:
// an identifier that names a function, method, type, variable, or
// constant declared at the top level of a package in the companion code
// gets a popover with its declaration, the first sentence of its doc
// comment, and the file and line it's declared on. go/types resolves
// the names, so local variables, fields, builtins, and the standard
// library are never mistaken for them.
//
// The standard library and third-party modules aren't loaded: each
// import outside the companion code stands in as an empty package. The
// checker reports errors for the names used from them, which are
// ignored; names from the companion code still resolve, since they
// don't depend on those packages' contents.

// goDef is one declaration a listing may refer to.
type goDef struct {
	sig  string // the declaration, abbreviated when long
	doc  string // the first sentence of its doc comment, or ""
	rel  string // its file, under code/
	line int
}

// goUnit is a type-checked set of files: a package, or a package with
// its test files.
type goUnit struct {
	info  *types.Info
	files map[string]*ast.File // by file name
}

// goIndex loads and type-checks the companion code's packages on
// demand, and keeps every declaration it has seen.
type goIndex struct {
	root    string // the code/ directory
	fset    *token.FileSet
	mods    []goModule           // longest path first
	parsed  map[string]*ast.File // by file name
	units   map[string]*goUnit   // by directory, with "#test" or "#xtest" for test units
	pkgs    map[string]*types.Package
	defs    map[token.Pos]*goDef // by the position of the declared name
	loading map[string]bool
}

// goModule is a module in the companion code.
type goModule struct{ path, dir string }

var moduleLine = regexp.MustCompile(`^module\s+(\S+)`)

// newGoIndex finds the modules under root.
func newGoIndex(root string) (*goIndex, error) {
	x := &goIndex{root: root, fset: token.NewFileSet(),
		parsed: map[string]*ast.File{}, units: map[string]*goUnit{},
		pkgs: map[string]*types.Package{}, defs: map[token.Pos]*goDef{},
		loading: map[string]bool{}}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && p != root && (d.Name() == "testdata" || d.Name() == "vendor" ||
			strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if d.Name() != "go.mod" {
			return nil
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if m := moduleLine.FindStringSubmatch(sc.Text()); m != nil {
				x.mods = append(x.mods, goModule{m[1], filepath.Dir(p)})
				break
			}
		}
		return sc.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("companion modules: %w", err)
	}
	slices.SortFunc(x.mods, func(a, b goModule) int { return len(b.path) - len(a.path) })
	return x, nil
}

// dirOf returns the directory of the companion package with the given
// import path, or "" for a package outside the companion code.
func (x *goIndex) dirOf(importPath string) string {
	for _, m := range x.mods {
		if importPath == m.path {
			return m.dir
		}
		if rest, ok := strings.CutPrefix(importPath, m.path+"/"); ok {
			return filepath.Join(m.dir, filepath.FromSlash(rest))
		}
	}
	return ""
}

// Import implements types.Importer. Companion packages are checked
// from source; any other package is an empty stand-in.
func (x *goIndex) Import(importPath string) (*types.Package, error) {
	if p, ok := x.pkgs[importPath]; ok {
		return p, nil
	}
	if dir := x.dirOf(importPath); dir != "" {
		if _, err := x.unit(dir, ""); err != nil {
			return nil, err
		}
		if p, ok := x.pkgs[importPath]; ok {
			return p, nil
		}
		return nil, fmt.Errorf("no package %s", importPath)
	}
	name := path.Base(importPath)
	if len(name) > 1 && name[0] == 'v' && strings.Trim(name[1:], "0123456789") == "" {
		name = path.Base(path.Dir(importPath)) // encoding/json/v2 is json
	}
	name = strings.ReplaceAll(name, "-", "")
	p := types.NewPackage(importPath, name)
	p.MarkComplete()
	x.pkgs[importPath] = p
	return p, nil
}

// importPath is the import path of the package in dir.
func (x *goIndex) importPath(dir string) string {
	for _, m := range x.mods {
		if rel, err := filepath.Rel(m.dir, dir); err == nil && !strings.HasPrefix(rel, "..") {
			if rel == "." {
				return m.path
			}
			return m.path + "/" + filepath.ToSlash(rel)
		}
	}
	return filepath.ToSlash(dir)
}

// unit type-checks the package in dir. kind "" is the package itself;
// "test" adds the test files in the package; "xtest" is the external
// test package.
func (x *goIndex) unit(dir, kind string) (*goUnit, error) {
	key := dir
	if kind != "" {
		key += "#" + kind
	}
	if u, ok := x.units[key]; ok {
		return u, nil
	}
	if x.loading[key] {
		return nil, fmt.Errorf("import cycle through %s", dir)
	}
	x.loading[key] = true
	defer delete(x.loading, key)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("go package: %w", err)
	}
	var pkgFiles, testFiles, xtestFiles []*ast.File
	var pkgName string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		if ok, err := gobuild.Default.MatchFile(dir, name); err != nil || !ok {
			continue
		}
		f, err := x.parse(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
		switch {
		case !strings.HasSuffix(name, "_test.go"):
			pkgFiles = append(pkgFiles, f)
			pkgName = f.Name.Name
		case strings.HasSuffix(f.Name.Name, "_test"):
			xtestFiles = append(xtestFiles, f)
		default:
			testFiles = append(testFiles, f)
		}
	}
	files, ipath := pkgFiles, x.importPath(dir)
	switch kind {
	case "test":
		files = append(slices.Clip(pkgFiles), testFiles...)
	case "xtest":
		files, ipath = xtestFiles, ipath+"_test"
	}
	if kind == "xtest" && len(pkgFiles) > 0 {
		if _, err := x.unit(dir, ""); err != nil { // the package it tests
			return nil, err
		}
	}
	info := &types.Info{
		Defs: map[*ast.Ident]types.Object{},
		Uses: map[*ast.Ident]types.Object{},
	}
	conf := types.Config{Importer: x, FakeImportC: true, Error: func(error) {}}
	p, _ := conf.Check(ipath, x.fset, files, info) // errors are expected; see above
	if kind == "" && p != nil && pkgName != "" {
		x.pkgs[ipath] = p
	}
	u := &goUnit{info: info, files: map[string]*ast.File{}}
	for _, f := range files {
		u.files[x.fset.File(f.Pos()).Name()] = f
	}
	x.units[key] = u
	return u, nil
}

// parse parses a file once, and indexes its declarations.
func (x *goIndex) parse(name string) (*ast.File, error) {
	if f, ok := x.parsed[name]; ok {
		return f, nil
	}
	f, err := parser.ParseFile(x.fset, name, nil, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return nil, fmt.Errorf("go package: %w", err)
	}
	x.parsed[name] = f
	x.indexDecls(f)
	return f, nil
}

// indexDecls records the top-level declarations in f, with the methods
// of its interfaces.
func (x *goIndex) indexDecls(f *ast.File) {
	add := func(id *ast.Ident, sig string, docs ...*ast.CommentGroup) {
		if id == nil || id.Name == "_" {
			return
		}
		pos := x.fset.Position(id.Pos())
		rel, err := filepath.Rel(x.root, pos.Filename)
		if err != nil {
			return
		}
		d := &goDef{sig: sig, rel: filepath.ToSlash(rel), line: pos.Line}
		for _, g := range docs {
			if g != nil {
				d.doc = firstSentence(g.Text())
				break
			}
		}
		x.defs[id.Pos()] = d
	}
	for _, decl := range f.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			add(decl.Name, x.funcSig(decl), decl.Doc)
		case *ast.GenDecl:
			alone := len(decl.Specs) == 1
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					var groupDoc *ast.CommentGroup
					if alone {
						groupDoc = decl.Doc
					}
					add(spec.Name, x.typeSig(f, spec), spec.Doc, groupDoc, spec.Comment)
					if it, ok := spec.Type.(*ast.InterfaceType); ok {
						for _, m := range it.Methods.List {
							ft, ok := m.Type.(*ast.FuncType)
							if !ok || len(m.Names) == 0 {
								continue
							}
							sig := "func (" + spec.Name.Name + ") " + m.Names[0].Name +
								strings.TrimPrefix(x.print(ft), "func")
							add(m.Names[0], sig, m.Doc, m.Comment)
						}
					}
				case *ast.ValueSpec:
					var groupDoc *ast.CommentGroup
					if alone {
						groupDoc = decl.Doc
					}
					for i, name := range spec.Names {
						add(name, x.valueSig(decl.Tok, spec, i), spec.Doc, spec.Comment, groupDoc)
					}
				}
			}
		}
	}
}

// print formats a node without its comments.
func (x *goIndex) print(n any) string {
	var b bytes.Buffer
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}
	if err := cfg.Fprint(&b, x.fset, n); err != nil {
		return ""
	}
	return b.String()
}

// funcSig is a function's or method's declaration without its body.
func (x *goIndex) funcSig(d *ast.FuncDecl) string {
	head := *d
	head.Body, head.Doc = nil, nil
	return x.print(&head)
}

// typeSig is a type's declaration, with the comments inside it (a
// struct's field comments), cut to sigLines lines.
func (x *goIndex) typeSig(f *ast.File, spec *ast.TypeSpec) string {
	var inner []*ast.CommentGroup
	for _, g := range f.Comments {
		if g.Pos() <= spec.Name.End() || g.End() > spec.Type.End() {
			continue
		}
		var kept ast.CommentGroup
		for _, c := range g.List {
			if !tagLine.MatchString(c.Text) && !calloutOnly.MatchString(c.Text) {
				kept.List = append(kept.List, c)
			}
		}
		if len(kept.List) > 0 {
			inner = append(inner, &kept)
		}
	}
	bare := *spec
	bare.Doc, bare.Comment = nil, nil
	sig := "type " + x.print(&printer.CommentedNode{Node: &bare, Comments: inner})
	return abbreviate(sig)
}

// sigLines is the most lines a popover shows of a declaration.
const sigLines = 12

// abbreviate cuts a long declaration to sigLines lines, keeping its
// closing brace.
func abbreviate(sig string) string {
	lines := strings.Split(sig, "\n")
	if len(lines) <= sigLines {
		return sig
	}
	kept := append(lines[:sigLines-2:sigLines-2], "\t// ...")
	if last := lines[len(lines)-1]; strings.TrimSpace(last) == "}" {
		kept = append(kept, last)
	}
	return strings.Join(kept, "\n")
}

// valueSig is the declaration of the i'th name in a var or const spec:
// its type when the spec gives one, its value when the value is short.
func (x *goIndex) valueSig(tok token.Token, spec *ast.ValueSpec, i int) string {
	s := tok.String() + " " + spec.Names[i].Name
	if spec.Type != nil {
		s += " " + x.print(spec.Type)
	}
	if len(spec.Values) == len(spec.Names) {
		v := x.print(spec.Values[i])
		switch first, _, _ := strings.Cut(v, "\n"); {
		case first == v && len(v) <= 72:
			s += " = " + v
		case len(first) <= 60:
			s += " = " + first + " ..."
		case spec.Type == nil:
			s += " = ..."
		}
	}
	return s
}

// calloutOnly is a comment that holds only a callout number.
var calloutOnly = regexp.MustCompile(`^// <\d+>$`)

var calloutText = regexp.MustCompile(`(?m)[ \t]*<\d+>$`)

var tagText = regexp.MustCompile(`(?m)^(tag|end)::[a-z0-9-]+\[\]\n`)

// firstSentence is the doc comment's first sentence, as go doc shows
// it in a package listing, leaving out the book's tag markers.
func firstSentence(text string) string {
	text = tagText.ReplaceAllString(text, "")
	text = calloutText.ReplaceAllString(text, "")
	return new(doc.Package).Synopsis(text)
}

// goRef is a name in a listing that refers to a declaration.
type goRef struct {
	off, end int // byte offsets in the listing's text
	def      *goDef
}

// refs finds the names in ex, an excerpt of rel, that refer to
// declarations in the companion code, leaving out uses of a
// declaration the excerpt itself shows.
func (x *goIndex) refs(rel string, ex excerpt) ([]goRef, error) {
	name := filepath.Join(x.root, filepath.FromSlash(rel))
	dir, base := filepath.Split(name)
	dir = filepath.Clean(dir)
	kind := ""
	if strings.HasSuffix(base, "_test.go") {
		f, err := x.parse(name)
		if err != nil {
			return nil, err
		}
		kind = "test"
		if strings.HasSuffix(f.Name.Name, "_test") {
			kind = "xtest"
		}
	}
	u, err := x.unit(dir, kind)
	if err != nil {
		return nil, err
	}
	f := u.files[name]
	if f == nil {
		return nil, nil // excluded by a build constraint
	}
	// Where each source line starts in the excerpt.
	starts := map[int]int{}
	off := 0
	for _, n := range ex.src {
		starts[n] = off
		off += strings.IndexByte(ex.text[off:], '\n') + 1
	}
	var out []goRef
	for id, obj := range u.info.Uses {
		pos := x.fset.Position(id.Pos())
		if pos.Filename != name {
			continue
		}
		start, ok := starts[pos.Line]
		if !ok {
			continue
		}
		switch o := obj.(type) {
		case *types.Func:
			obj = o.Origin()
		case *types.Var:
			obj = o.Origin()
		}
		def := x.defs[obj.Pos()]
		if def == nil {
			continue
		}
		if def.rel == rel && ex.shows(def.line) {
			continue // declared in the listing itself
		}
		col := pos.Column - 1 - ex.indent
		at := start + col
		if col < 0 || at+len(id.Name) > len(ex.text) || ex.text[at:at+len(id.Name)] != id.Name {
			continue
		}
		out = append(out, goRef{at, at + len(id.Name), def})
	}
	slices.SortFunc(out, func(a, b goRef) int { return a.off - b.off })
	return out, nil
}

// refs finds the names in ex, an excerpt of the Go file rel, that refer
// to declarations in the companion code.
func (c *codeRoot) refs(rel string, ex excerpt) ([]goRef, error) {
	if c.gox == nil {
		x, err := newGoIndex(c.dir)
		if err != nil {
			return nil, err
		}
		c.gox = x
	}
	r, err := c.gox.refs(rel, ex)
	if err != nil {
		return nil, fmt.Errorf("include %s: %w", rel, err)
	}
	return r, nil
}

// linkRefs wraps each name in refs in code, a highlighted listing, in
// a span the page's script gives a popover, and returns the popovers'
// contents: one hidden element per declaration, which the span names in
// data-def. The popovers sit outside the <pre>, so copying the code
// copies only the code.
func (r *codeBlocks) linkRefs(code string, refs []goRef) (string, string) {
	if len(refs) == 0 {
		return code, ""
	}
	ids := map[*goDef]string{}
	var defs strings.Builder
	defs.WriteString(`<div class="ref-defs" hidden>`)
	marks := make([]mark, len(refs))
	for i, ref := range refs {
		id, ok := ids[ref.def]
		if !ok {
			id = fmt.Sprintf("ref-%d-%d", r.listings, len(ids)+1)
			ids[ref.def] = id
			if err := r.writeDef(&defs, id, ref.def); err != nil {
				r.code.fail(err)
			}
		}
		marks[i] = mark{ref.off, ref.end, `<span class="ref" data-def="` + id + `">`, `</span>`}
	}
	defs.WriteString(`</div>`)
	return decorate(code, marks), defs.String()
}

// writeDef writes the popover for one declaration.
func (r *codeBlocks) writeDef(w *strings.Builder, id string, d *goDef) error {
	sig, err := highlight("go", d.sig+"\n")
	if err != nil {
		return err
	}
	u, err := r.code.url(d.rel, 0, 0)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, `<div class="ref-def" id="%s">%s`, id, sig)
	if d.doc != "" {
		fmt.Fprintf(w, `<p class="ref-doc">%s</p>`, html.EscapeString(d.doc))
	}
	fmt.Fprintf(w, `<p class="ref-at"><a href="%s#L%d">%s:%d</a></p>`,
		html.EscapeString(u), d.line, html.EscapeString(d.rel), d.line)
	// What a screen reader announces: the same, as plain text, since
	// the highlighted declaration reads as a run of separate tokens.
	desc := strings.Join(strings.Fields(d.sig), " ")
	if d.doc != "" {
		desc = d.doc + " " + desc
	}
	fmt.Fprintf(w, `<p class="ref-text" id="%s-text">%s. In %s, line %d.</p></div>`,
		id, html.EscapeString(desc), html.EscapeString(d.rel), d.line)
	return nil
}

// mark is markup to put around the text from byte off to end.
type mark struct {
	off, end    int
	open, close string
}

// decorate inserts marks, sorted and not overlapping, into h, HTML
// whose text (with its tags removed and its entities decoded) the
// marks' offsets count. A mark that would cross a tag is closed before
// the tag and opened again after it, so the result stays well formed.
func decorate(h string, marks []mark) string {
	var b strings.Builder
	text, next, open := 0, 0, false
	for i := 0; i < len(h); {
		if h[i] != '<' && !open && next < len(marks) && text == marks[next].off {
			b.WriteString(marks[next].open)
			open = true
		}
		switch h[i] {
		case '<':
			j := strings.IndexByte(h[i:], '>')
			if j < 0 {
				j = len(h) - i - 1
			}
			if open {
				b.WriteString(marks[next].close)
			}
			b.WriteString(h[i : i+j+1])
			if open {
				b.WriteString(marks[next].open)
			}
			i += j + 1
			continue
		case '&':
			j := strings.IndexByte(h[i:], ';')
			if j > 0 && j < 10 {
				b.WriteString(h[i : i+j+1])
				text += len(html.UnescapeString(h[i : i+j+1]))
				i += j + 1
			} else {
				b.WriteByte(h[i])
				text++
				i++
			}
		default:
			b.WriteByte(h[i])
			text++
			i++
		}
		if open && text >= marks[next].end {
			b.WriteString(marks[next].close)
			open = false
			next++
		}
	}
	return b.String()
}
