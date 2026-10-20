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
	"maps"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// Listings of Go from the companion code show what every name refers
// to, as an editor does: each identifier gets a popover with its
// declaration, the first sentence of its doc comment, and where it's
// declared (the line in the companion code, or the standard library's
// page on pkg.go.dev). That covers the companion code's declarations,
// the standard library's, the builtins, packages, struct fields, and the
// listing's own variables and parameters, with their types. Every use of
// the same name is marked, so the page can highlight them together.
// go/types resolves the names.
//
// The index loads the standard library from the Go toolchain the
// companion code builds with (go list names it). Third-party modules
// aren't loaded: each stands in as an empty package. The checker reports
// errors for the names used from them, which are ignored; every other
// name still resolves, since it doesn't depend on those packages'
// contents.

// goDef is one declaration a listing may refer to.
type goDef struct {
	sig  string // the declaration, abbreviated when long
	doc  string // the first sentence of its doc comment, or ""
	rel  string // its file (or a package's directory), under code/, or "" outside it
	line int    // 0 for a package

	// For the standard library: the import path, and the anchor on
	// pkg.go.dev ("Client.Do"), "" for the package itself.
	pkg, anchor string
}

// goUnit is a type-checked set of files: a package, or a package with
// its test files.
type goUnit struct {
	pkg   *types.Package
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
	ctx     gobuild.Context // which files a package's build uses

	std      string                  // the standard library's source, GOROOT/src; "" leaves it out
	builtins map[string]*goDef       // the universe's names, from builtin/builtin.go
	others   map[types.Object]*goDef // locals, packages, and the rest, made on demand
	objIDs   map[types.Object]int    // a number for each object a listing shows
}

// goModule is a module in the companion code.
type goModule struct{ path, dir string }

var moduleLine = regexp.MustCompile(`^module\s+(\S+)`)

// newGoIndex finds the modules under root.
func newGoIndex(root string) (*goIndex, error) {
	x := &goIndex{root: root, fset: token.NewFileSet(),
		parsed: map[string]*ast.File{}, units: map[string]*goUnit{},
		pkgs: map[string]*types.Package{}, defs: map[token.Pos]*goDef{},
		loading: map[string]bool{}, ctx: gobuild.Default,
		builtins: map[string]*goDef{}, others: map[types.Object]*goDef{},
		objIDs: map[types.Object]int{}}
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

// newStdGoIndex is newGoIndex with the standard library: the source
// of the toolchain that builds the code under root, read with that
// toolchain's build tags, so its release's packages and experiments
// are there.
func newStdGoIndex(root string) (*goIndex, error) {
	x, err := newGoIndex(root)
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("go", "list", "-f",
		"{{context.GOROOT}}\n{{join context.ToolTags \",\"}}\n{{join context.ReleaseTags \",\"}}", "unsafe")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("find the companion code's Go toolchain: %w", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 3 {
		return nil, fmt.Errorf("find the companion code's Go toolchain: go list said %q", out)
	}
	x.ctx.GOROOT = lines[0]
	x.ctx.ToolTags = strings.Split(lines[1], ",")
	x.ctx.ReleaseTags = strings.Split(lines[2], ",")
	x.ctx.CgoEnabled = false // the pure Go files, which say more than cgo's
	x.std = filepath.Join(lines[0], "src")

	f, err := x.parse(filepath.Join(x.std, "builtin", "builtin.go"))
	if err != nil {
		return nil, err
	}
	for _, decl := range f.Decls {
		var ids []*ast.Ident
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			ids = append(ids, decl.Name)
		case *ast.GenDecl:
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					ids = append(ids, spec.Name)
				case *ast.ValueSpec:
					ids = append(ids, spec.Names...)
				}
			}
		}
		for _, id := range ids {
			if d := x.defs[id.Pos()]; d != nil {
				x.builtins[id.Name] = d
			}
		}
	}
	return x, nil
}

// dirOf returns the directory of the package with the given import
// path: a companion package, or when the index has it, a standard one. It
// returns "" for any other package.
func (x *goIndex) dirOf(importPath string) string {
	for _, m := range x.mods {
		if importPath == m.path {
			return m.dir
		}
		if rest, ok := strings.CutPrefix(importPath, m.path+"/"); ok {
			return filepath.Join(m.dir, filepath.FromSlash(rest))
		}
	}
	if first, _, _ := strings.Cut(importPath, "/"); x.std != "" && !strings.Contains(first, ".") {
		dir := filepath.Join(x.std, filepath.FromSlash(importPath))
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
	}
	return ""
}

// stdPath is the import path of dir if it holds a standard package.
func (x *goIndex) stdPath(dir string) (string, bool) {
	if x.std == "" {
		return "", false
	}
	rel, err := filepath.Rel(x.std, dir)
	if err != nil || rel == "." || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

// Import implements types.Importer. Companion packages, and the
// standard library when the index has it, are checked from source; any other
// package is an empty stand-in.
func (x *goIndex) Import(importPath string) (*types.Package, error) {
	if p, ok := x.pkgs[importPath]; ok {
		return p, nil
	}
	if importPath == "unsafe" && x.std != "" {
		return types.Unsafe, nil
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
	if p, ok := x.stdPath(dir); ok {
		return p
	}
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
	_, std := x.stdPath(dir)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") ||
			std && strings.HasSuffix(name, "_test.go") {
			continue
		}
		if ok, err := x.ctx.MatchFile(dir, name); err != nil || !ok {
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
	u := &goUnit{pkg: p, info: info, files: map[string]*ast.File{}}
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
// of its interfaces and the fields of its structs.
func (x *goIndex) indexDecls(f *ast.File) {
	name := x.fset.File(f.Pos()).Name()
	stdPkg, std := x.stdPath(filepath.Dir(name))
	rel, err := filepath.Rel(x.root, name)
	if !std && (err != nil || strings.HasPrefix(rel, "..")) {
		return
	}
	add := func(id *ast.Ident, anchor, sig string, docs ...*ast.CommentGroup) *goDef {
		if id == nil || id.Name == "_" {
			return nil
		}
		d := &goDef{sig: sig}
		if std {
			d.pkg, d.anchor = stdPkg, anchor
		} else {
			d.rel, d.line = filepath.ToSlash(rel), x.fset.Position(id.Pos()).Line
		}
		for _, g := range docs {
			if g != nil {
				d.doc = firstSentence(g.Text())
				break
			}
		}
		x.defs[id.Pos()] = d
		return d
	}
	for _, decl := range f.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			anchor := decl.Name.Name
			if decl.Recv != nil && len(decl.Recv.List) == 1 {
				anchor = recvName(decl.Recv.List[0].Type) + "." + anchor
			}
			add(decl.Name, anchor, x.funcSig(decl), decl.Doc)
		case *ast.GenDecl:
			alone := len(decl.Specs) == 1
			for _, spec := range decl.Specs {
				switch spec := spec.(type) {
				case *ast.TypeSpec:
					var groupDoc *ast.CommentGroup
					if alone {
						groupDoc = decl.Doc
					}
					tname := spec.Name.Name
					add(spec.Name, tname, x.typeSig(f, spec), spec.Doc, groupDoc, spec.Comment)
					switch t := spec.Type.(type) {
					case *ast.InterfaceType:
						for _, m := range t.Methods.List {
							ft, ok := m.Type.(*ast.FuncType)
							if !ok || len(m.Names) == 0 {
								continue
							}
							sig := "func (" + tname + ") " + m.Names[0].Name +
								strings.TrimPrefix(x.print(ft), "func")
							add(m.Names[0], tname+"."+m.Names[0].Name, sig, m.Doc, m.Comment)
						}
					case *ast.StructType:
						for _, fl := range t.Fields.List {
							typ := x.print(fl.Type)
							if fl.Tag != nil {
								typ += " " + fl.Tag.Value
							}
							for _, n := range fl.Names {
								add(n, tname+"."+n.Name, "field "+n.Name+" "+typ, fl.Doc, fl.Comment)
							}
						}
					}
				case *ast.ValueSpec:
					var groupDoc *ast.CommentGroup
					if alone {
						groupDoc = decl.Doc
					}
					for i, name := range spec.Names {
						add(name, name.Name, x.valueSig(decl.Tok, spec, i), spec.Doc, groupDoc, spec.Comment)
					}
				}
			}
		}
	}
}

// recvName is the type name in a method's receiver: T for *T or T[K].
func recvName(e ast.Expr) string {
	for {
		switch t := e.(type) {
		case *ast.StarExpr:
			e = t.X
		case *ast.IndexExpr:
			e = t.X
		case *ast.IndexListExpr:
			e = t.X
		case *ast.ParenExpr:
			e = t.X
		case *ast.Ident:
			return t.Name
		default:
			return ""
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
	obj      int  // which object the name is: the same number for every use
	local    bool // a name declared inside a function
}

// refs finds the names in ex, an excerpt of rel, and what each refers to.
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
	byOff := map[int]goRef{}
	visit := func(id *ast.Ident, obj types.Object) {
		if obj == nil || id.Name == "_" {
			return
		}
		pos := x.fset.Position(id.Pos())
		if pos.Filename != name {
			return
		}
		start, ok := starts[pos.Line]
		if !ok {
			return
		}
		switch o := obj.(type) {
		case *types.Func:
			obj = o.Origin()
		case *types.Var:
			obj = o.Origin()
		}
		col := pos.Column - 1 - ex.indent
		at := start + col
		if col < 0 || at+len(id.Name) > len(ex.text) || ex.text[at:at+len(id.Name)] != id.Name {
			return
		}
		ref := goRef{off: at, end: at + len(id.Name)}
		ref.def, ref.local = x.def(obj, u.pkg)
		n, ok := x.objIDs[obj]
		if !ok {
			n = len(x.objIDs) + 1
			x.objIDs[obj] = n
		}
		ref.obj = n
		byOff[at] = ref
	}
	// Uses come second, so an embedded field's name shows the type it
	// names rather than the field it declares.
	for id, obj := range u.info.Defs {
		visit(id, obj)
	}
	for id, obj := range u.info.Uses {
		visit(id, obj)
	}
	out := slices.Collect(maps.Values(byOff))
	slices.SortFunc(out, func(a, b goRef) int { return a.off - b.off })
	return out, nil
}

// def is what a listing shows for obj, used in pkg: its
// declaration when the index has one, and otherwise what go/types
// says it is. local reports a name declared inside a function.
func (x *goIndex) def(obj types.Object, pkg *types.Package) (def *goDef, local bool) {
	if d := x.defs[obj.Pos()]; d != nil && obj.Pos().IsValid() {
		return d, false
	}
	if obj.Parent() == types.Universe {
		if d := x.builtins[obj.Name()]; d != nil {
			return d, false
		}
	}
	local = obj.Parent() != nil && obj.Parent() != types.Universe &&
		obj.Pkg() != nil && obj.Parent() != obj.Pkg().Scope()
	if _, isPkg := obj.(*types.PkgName); isPkg {
		local = false
	}
	if d, ok := x.others[obj]; ok {
		return d, local
	}
	d := &goDef{}
	if pn, ok := obj.(*types.PkgName); ok {
		d = x.pkgDef(pn.Imported())
	} else {
		qual := func(p *types.Package) string {
			if p == pkg {
				return ""
			}
			return p.Name()
		}
		d.sig = types.ObjectString(obj, qual)
		if obj.Pos().IsValid() {
			pos := x.fset.Position(obj.Pos())
			if rel, err := filepath.Rel(x.root, pos.Filename); err == nil && !strings.HasPrefix(rel, "..") {
				d.rel, d.line = filepath.ToSlash(rel), pos.Line
			}
		}
	}
	x.others[obj] = d
	return d, local
}

// pkgDef describes an imported package: its name and path, and the
// first sentence of its package comment.
func (x *goIndex) pkgDef(p *types.Package) *goDef {
	d := &goDef{sig: fmt.Sprintf("package %s (%q)", p.Name(), p.Path())}
	dir := x.dirOf(p.Path())
	if dir == "" {
		return d
	}
	if std, ok := x.stdPath(dir); ok {
		d.pkg = std
	} else if rel, err := filepath.Rel(x.root, dir); err == nil {
		d.rel = filepath.ToSlash(rel)
	}
	var docs []string // by file name, doc.go first
	for fname, f := range x.parsed {
		if filepath.Dir(fname) == dir && f.Doc != nil && !strings.HasSuffix(fname, "_test.go") {
			if filepath.Base(fname) == "doc.go" {
				fname = ""
			}
			docs = append(docs, fname+"\x00"+f.Doc.Text())
		}
	}
	slices.Sort(docs)
	if len(docs) > 0 {
		_, text, _ := strings.Cut(docs[0], "\x00")
		d.doc = firstSentence(text)
	}
	return d
}

// refs finds the names in ex, an excerpt of the Go file rel, and what
// each refers to.
func (c *codeRoot) refs(rel string, ex excerpt) ([]goRef, error) {
	if c.gox == nil {
		x, err := newStdGoIndex(c.dir)
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
// a span the page's script gives a popover. The popovers' contents, one
// hidden element per declaration that the span names in data-def, are
// written once per page, after its body (convert): copying the code
// copies only the code, and a declaration that several listings use is
// sent once. data-obj names the object, the same for every use of it on
// the page. Only names the companion code declares at a package's top
// level (and their fields and methods) get a tab stop; data-notab marks
// the rest (locals, the standard library, builtins, packages), so that
// tabbing past a listing stays short.
func (r *codeBlocks) linkRefs(code string, refs []goRef) string {
	if len(refs) == 0 {
		return code
	}
	c := r.code
	if c.popIDs == nil {
		c.popIDs = map[*goDef]string{}
	}
	marks := make([]mark, len(refs))
	for i, ref := range refs {
		id, ok := c.popIDs[ref.def]
		if !ok {
			id = fmt.Sprintf("ref-%d", len(c.popIDs)+1)
			c.popIDs[ref.def] = id
			if err := r.writeDef(&c.pops, id, ref.def); err != nil {
				c.fail(err)
			}
		}
		open := fmt.Sprintf(`<span class="ref" data-def="%s" data-obj="o%d"`, id, ref.obj)
		if ref.local || ref.def.rel == "" || ref.def.line == 0 {
			open += ` data-notab`
		}
		marks[i] = mark{ref.off, ref.end, open + `>`, `</span>`}
	}
	return decorate(code, marks)
}

// takePops returns the popovers the page's listings use, in one hidden
// element, and starts the next page.
func (c *codeRoot) takePops() string {
	if c.pops.Len() == 0 {
		return ""
	}
	s := `<div class="ref-defs" hidden>` + c.pops.String() + `</div>`
	c.pops.Reset()
	c.popIDs = nil
	return s
}

// writeDef writes the popover for one declaration.
func (r *codeBlocks) writeDef(w *strings.Builder, id string, d *goDef) error {
	sig, err := highlight("go", d.sig+"\n")
	if err != nil {
		return err
	}
	// Where it's declared: the line in the companion code, or the
	// standard library's documentation.
	var href, at, where string
	switch {
	case d.pkg != "":
		href, at = "https://pkg.go.dev/"+d.pkg, d.pkg
		if d.anchor != "" {
			href += "#" + d.anchor
			at += "." + d.anchor
		}
		where = "Documented at pkg.go.dev, " + at
	case d.rel != "":
		u, err := r.code.url(d.rel, 0, 0)
		if err != nil {
			return err
		}
		href, at, where = u, d.rel, "In "+d.rel
		if d.line > 0 {
			href += fmt.Sprintf("#L%d", d.line)
			at += fmt.Sprintf(":%d", d.line)
			where += fmt.Sprintf(", line %d", d.line)
		}
	}
	fmt.Fprintf(w, `<div class="ref-def" id="%s">%s`, id, sig)
	if d.doc != "" {
		fmt.Fprintf(w, `<p class="ref-doc">%s</p>`, html.EscapeString(d.doc))
	}
	if href != "" {
		fmt.Fprintf(w, `<p class="ref-at"><a href="%s">%s</a></p>`,
			html.EscapeString(href), html.EscapeString(at))
	}
	// What a screen reader announces: the same, as plain text, since
	// the highlighted declaration reads as a run of separate tokens.
	desc := strings.Join(strings.Fields(d.sig), " ")
	if d.doc != "" {
		desc = d.doc + " " + desc
	}
	if where != "" {
		desc += ". " + where
	}
	fmt.Fprintf(w, `<p class="ref-text" id="%s-text">%s.</p></div>`, id, html.EscapeString(desc))
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
