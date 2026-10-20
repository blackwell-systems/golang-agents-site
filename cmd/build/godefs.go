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
// companion code builds with, and third-party packages from the module
// cache, at the versions each companion module builds with (go list
// names both). A package go list can't find stands in as an empty
// package: the checker reports errors for the names used from it, which
// are ignored, and every other name still resolves.

// goDef is one declaration a listing may refer to.
type goDef struct {
	sig  string // the declaration, abbreviated when long
	doc  string // the first sentence of its doc comment, or ""
	rel  string // its file (or a package's directory), under code/, or "" outside it
	line int    // 0 for a package

	// Outside the companion code: the import path, the package's page
	// on pkg.go.dev, and the anchor there ("Client.Do"), "" for the
	// package itself.
	pkg, url, anchor string
}

// goLib is a package from outside the companion code: the standard
// library's, or a third-party module's.
type goLib struct {
	path string // its import path
	dir  string
	url  string // its page on pkg.go.dev, at the version the code uses
	main string // for a module's package, the companion module whose build uses it
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
	mods    []goModule                // longest path first
	parsed  map[string]*ast.File      // by file name
	units   map[string]*goUnit        // by directory, with "#test" or "#xtest" for test units
	byDir   map[string]*types.Package // the packages checked from source
	pkgs    map[string]*types.Package // the stand-ins, by import path
	defs    map[token.Pos]*goDef      // by the position of the declared name
	loading map[string]bool
	ctx     gobuild.Context // which files a package's build uses

	std      string                       // the standard library's source, GOROOT/src; "" leaves it out
	deps     map[string]map[string]*goLib // by companion module directory, then import path
	ext      map[string]*goLib            // the third-party packages loaded, by directory
	builtins map[string]*goDef            // the universe's names, from builtin/builtin.go
	others   map[types.Object]*goDef      // locals, packages, and the rest, made on demand
	objIDs   map[types.Object]int         // a number for each object a listing shows
}

// goModule is a module in the companion code.
type goModule struct{ path, dir string }

var moduleLine = regexp.MustCompile(`^module\s+(\S+)`)

// newGoIndex finds the modules under root.
func newGoIndex(root string) (*goIndex, error) {
	x := &goIndex{root: root, fset: token.NewFileSet(),
		parsed: map[string]*ast.File{}, units: map[string]*goUnit{},
		byDir: map[string]*types.Package{}, pkgs: map[string]*types.Package{},
		defs: map[token.Pos]*goDef{}, deps: map[string]map[string]*goLib{},
		ext:     map[string]*goLib{},
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

// depDir returns the directory of a third-party package that the
// package in srcDir imports, at the version its companion module builds
// with, or "" when there's none.
func (x *goIndex) depDir(srcDir, importPath string) string {
	main := ""
	if l := x.ext[srcDir]; l != nil {
		main = l.main
	} else {
		for _, m := range x.mods {
			if r, err := filepath.Rel(m.dir, srcDir); err == nil && !strings.HasPrefix(r, "..") &&
				len(m.dir) > len(main) {
				main = m.dir
			}
		}
	}
	if main == "" {
		return ""
	}
	deps, ok := x.deps[main]
	if !ok {
		deps = x.listDeps(main)
		x.deps[main] = deps
	}
	l := deps[importPath]
	if l == nil {
		return ""
	}
	if x.ext[l.dir] == nil {
		x.ext[l.dir] = l
	}
	return l.dir
}

// listDeps asks go list for the third-party packages that the companion
// module in dir builds with, tests included, without going to the
// network: they're in the module cache wherever the code builds. A
// failure leaves the packages as stand-ins, so it isn't an error.
func (x *goIndex) listDeps(dir string) map[string]*goLib {
	cmd := exec.Command("go", "list", "-e", "-deps", "-test", "-f",
		"{{if and (not .Standard) (not .ForTest) .Module}}{{.ImportPath}}\t{{.Dir}}\t"+
			"{{.Module.Path}}\t{{.Module.Version}}{{end}}", "./...")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOPROXY=off", "GOFLAGS=-mod=mod")
	out, _ := cmd.Output()
	deps := map[string]*goLib{}
	for line := range strings.Lines(string(out)) {
		f := strings.Split(strings.TrimSpace(line), "\t")
		if len(f) != 4 || f[1] == "" || f[3] == "" {
			continue // the companion code's own packages have no version
		}
		pkg, d, mod, ver := f[0], f[1], f[2], f[3]
		deps[pkg] = &goLib{path: pkg, dir: d, main: dir,
			url: "https://pkg.go.dev/" + mod + "@" + ver + strings.TrimPrefix(pkg, mod)}
	}
	return deps
}

// lib describes the package in dir if it's from outside the companion
// code, and returns nil otherwise.
func (x *goIndex) lib(dir string) *goLib {
	if l := x.ext[dir]; l != nil {
		return l
	}
	if p, ok := x.stdPath(dir); ok {
		return &goLib{path: p, dir: dir, url: "https://pkg.go.dev/" + p}
	}
	return nil
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

// Import implements types.Importer.
func (x *goIndex) Import(importPath string) (*types.Package, error) {
	return x.ImportFrom(importPath, "", 0)
}

// ImportFrom implements types.ImporterFrom. Companion packages, and the
// standard library and the third-party packages srcDir's module builds
// with when the index has them, are checked from source; any other
// package is an empty stand-in.
func (x *goIndex) ImportFrom(importPath, srcDir string, _ types.ImportMode) (*types.Package, error) {
	if importPath == "unsafe" && x.std != "" {
		return types.Unsafe, nil
	}
	dir := x.dirOf(importPath)
	if dir == "" && x.std != "" {
		dir = x.depDir(srcDir, importPath)
	}
	if dir != "" {
		if p, ok := x.byDir[dir]; ok {
			return p, nil
		}
		if _, err := x.unit(dir, ""); err != nil {
			return nil, err
		}
		if p, ok := x.byDir[dir]; ok {
			return p, nil
		}
		return nil, fmt.Errorf("no package %s", importPath)
	}
	if p, ok := x.pkgs[importPath]; ok {
		return p, nil
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
	if l := x.lib(dir); l != nil {
		return l.path
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
	lib := x.lib(dir) != nil
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") ||
			lib && strings.HasSuffix(name, "_test.go") {
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
		x.byDir[dir] = p
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
	lib := x.lib(filepath.Dir(name))
	rel, err := filepath.Rel(x.root, name)
	if lib == nil && (err != nil || strings.HasPrefix(rel, "..")) {
		return
	}
	add := func(id *ast.Ident, anchor, sig string, docs ...*ast.CommentGroup) *goDef {
		if id == nil || id.Name == "_" {
			return nil
		}
		d := &goDef{sig: sig}
		if lib != nil {
			d.pkg, d.url, d.anchor = lib.path, lib.url, anchor
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

	name string         // the name itself
	qual string         // the name before it in a selector ("m" in m.Generate), or ""
	pkg  *types.Package // the package a package name names, or nil
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
		ref := goRef{off: at, end: at + len(id.Name), name: id.Name}
		ref.def, ref.local = x.def(obj, u.pkg)
		if at > 0 && ex.text[at-1] == '.' {
			j := at - 1
			for j > 0 && isIdentByte(ex.text[j-1]) {
				j--
			}
			ref.qual = ex.text[j : at-1]
		}
		if pn, ok := obj.(*types.PkgName); ok {
			ref.pkg = pn.Imported()
		}
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
	if l := x.lib(dir); l != nil {
		d.pkg, d.url = l.path, l.url
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
			if err := c.writeDef(&c.pops, id, ref.def); err != nil {
				c.fail(err)
			}
		}
		c.noteName(ref)
		open := fmt.Sprintf(`<span class="ref" data-def="%s" data-obj="o%d"`, id, ref.obj)
		if ref.local || ref.def.rel == "" || ref.def.line == 0 {
			open += ` data-notab`
		}
		marks[i] = mark{ref.off, ref.end, open + `>`, `</span>`}
	}
	return decorate(code, marks)
}

// takePops returns the popovers the page uses, in one hidden element,
// and starts the next page.
func (c *codeRoot) takePops() string {
	c.names, c.pkgNames = nil, nil
	if c.pops.Len() == 0 {
		return ""
	}
	s := `<div class="ref-defs" hidden>` + c.pops.String() + `</div>`
	c.pops.Reset()
	c.popIDs = nil
	return s
}

// proseName is what a name in the page's listings refers to, for the
// inline code in its prose (linkProse): every object the name is used
// for, by object number.
type proseName map[int]*goDef

// noteName records a name in a listing under the keys inline code may
// use for it: the name ("byName"), and with the name before it in a
// selector ("m.Generate").
func (c *codeRoot) noteName(ref goRef) {
	if c.names == nil {
		c.names = map[string]proseName{}
		c.pkgNames = map[string]*types.Package{}
	}
	if ref.pkg != nil {
		c.pkgNames[ref.pkg.Name()] = ref.pkg
	}
	keys := []string{ref.name}
	if ref.qual != "" {
		keys = append(keys, ref.qual+"."+ref.name)
	}
	for _, k := range keys {
		if c.names[k] == nil {
			c.names[k] = proseName{}
		}
		c.names[k][ref.obj] = ref.def
	}
}

// pick is the one thing a name in the prose means: the object it names
// on the page, or among several, the first one when they're all
// declared alike (byName, a local in two functions), or else the only
// type among them (Message, a type and a field). ok is false when the
// name stays ambiguous.
func (n proseName) pick() (def *goDef, obj int, ok bool) {
	objs := slices.Sorted(maps.Keys(n))
	sigs := map[string]bool{}
	for _, o := range objs {
		sigs[n[o].sig] = true
	}
	if len(sigs) == 1 {
		return n[objs[0]], objs[0], true
	}
	for _, o := range objs {
		if strings.HasPrefix(n[o].sig, "type ") {
			if def != nil {
				return nil, 0, false
			}
			def, obj = n[o], o
		}
	}
	return def, obj, def != nil
}

var (
	// inlineCode is inline code in a page's prose, with no markup inside.
	inlineCode = regexp.MustCompile(`<code>([^<]+)</code>`)
	// proseSkip is where inline code keeps no popover: inside a listing,
	// a link (which already goes somewhere), or a heading.
	proseSkip = regexp.MustCompile(`(?s)<pre[ >].*?</pre>|<a[ >].*?</a>|<h[1-6][ >].*?</h[1-6]>`)
	// codeName is inline code that names something: m.Generate,
	// *Message, ctx.Err(), errors.Is(err, ErrBudget).
	codeName = regexp.MustCompile(`^[*&]?([A-Za-z_]\w*(?:\.[A-Za-z_]\w*)*)(?:\(.*\))?$`)
)

// linkProse gives the inline code in page, a page's HTML, the popover of
// the name it shows, when the page's listings use that name for one
// thing only, so prose and code explain each other. A selector names
// its last two parts ("reply.Message.ToolCalls" is Message.ToolCalls),
// and a package-qualified name ("context.Cause") resolves in a package
// the page's listings import even when they don't use that name, as
// does a package-qualified field or method ("exec.Cmd.WaitDelay"). The
// builtins are left alone: nil and error read better as plain code.
func (c *codeRoot) linkProse(page string) string {
	if len(c.names) == 0 {
		return page
	}
	var b strings.Builder
	last := 0
	for _, loc := range append(proseSkip.FindAllStringIndex(page, -1), []int{len(page), len(page)}) {
		b.WriteString(inlineCode.ReplaceAllStringFunc(page[last:loc[0]], c.proseRef))
		b.WriteString(page[loc[0]:loc[1]])
		last = loc[1]
	}
	return b.String()
}

// proseRef is the inline code span s, with a popover when it names
// something the page shows.
func (c *codeRoot) proseRef(s string) string {
	text := html.UnescapeString(inlineCode.FindStringSubmatch(s)[1])
	m := codeName.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return s
	}
	parts := strings.Split(m[1], ".")
	key := parts[len(parts)-1]
	if len(parts) > 1 {
		key = parts[len(parts)-2] + "." + key
	}
	var def *goDef
	var obj int
	if n := c.names[key]; n != nil {
		var ok bool
		if def, obj, ok = n.pick(); !ok {
			return s
		}
	} else if p := c.pkgNames[parts[0]]; p != nil && len(parts) <= 3 && c.gox != nil {
		o := p.Scope().Lookup(parts[1])
		if len(parts) == 3 && o != nil { // a field or method: exec.Cmd.WaitDelay
			if _, isType := o.(*types.TypeName); !isType {
				return s
			}
			o, _, _ = types.LookupFieldOrMethod(o.Type(), true, p, parts[2])
		}
		if o == nil || !o.Exported() {
			return s
		}
		def, _ = c.gox.def(o, nil)
		if obj = c.gox.objIDs[o]; obj == 0 {
			obj = len(c.gox.objIDs) + 1
			c.gox.objIDs[o] = obj
		}
	} else {
		return s
	}
	if def == nil || def.pkg == "builtin" {
		return s
	}
	id, ok := c.popIDs[def]
	if !ok {
		if c.popIDs == nil {
			c.popIDs = map[*goDef]string{}
		}
		id = fmt.Sprintf("ref-%d", len(c.popIDs)+1)
		c.popIDs[def] = id
		if err := c.writeDef(&c.pops, id, def); err != nil {
			c.fail(err)
		}
	}
	return fmt.Sprintf(`<code class="ref" data-def="%s" data-obj="o%d" data-notab>`, id, obj) +
		s[len("<code>"):]
}

func isIdentByte(b byte) bool {
	return b == '_' || '0' <= b && b <= '9' || 'a' <= b && b <= 'z' || 'A' <= b && b <= 'Z'
}

// writeDef writes the popover for one declaration.
func (c *codeRoot) writeDef(w *strings.Builder, id string, d *goDef) error {
	sig, err := highlight("go", d.sig+"\n")
	if err != nil {
		return err
	}
	// Where it's declared: the line in the companion code, or the
	// standard library's documentation.
	var href, at, where string
	switch {
	case d.pkg != "":
		href, at = d.url, d.pkg
		if d.anchor != "" {
			href += "#" + d.anchor
			at += "." + d.anchor
		}
		where = "Documented at pkg.go.dev, " + at
	case d.rel != "":
		u, err := c.url(d.rel, 0, 0)
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
