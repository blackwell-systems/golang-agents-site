package main

import (
	"bufio"
	"bytes"
	"cmp"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// codeStats counts the Go in a directory of the companion code, so the
// site never states a count someone typed and forgot to update.
type codeStats struct {
	files     int // .go files, tests included
	lines     int // lines in those files
	testFiles int // _test.go files
	tests     int // top-level Test functions
	packages  int // directories holding .go files
}

// testFunc matches a test function's declaration.
var testFunc = regexp.MustCompile(`^func Test[A-Z0-9_]\w*\(\w+ \*testing\.T\)`)

// stats counts the Go under rel, a directory under code/ ("." for all
// of it). As the go command does, it skips testdata directories, whose
// Go files are fixtures, not code: the agent's practice projects live
// there.
func (c *codeRoot) stats(rel string) (codeStats, error) {
	if st, ok := c.counted[rel]; ok {
		return st, nil
	}
	var st codeStats
	dirs := map[string]bool{}
	root := filepath.Join(c.dir, filepath.FromSlash(rel))
	err := filepath.WalkDir(root, func(file string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if file != root && (name == "testdata" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(name) != ".go" {
			return nil
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		st.files++
		dirs[filepath.Dir(file)] = true
		st.lines += bytes.Count(b, []byte("\n"))
		if !strings.HasSuffix(name, "_test.go") {
			return nil
		}
		st.testFiles++
		sc := bufio.NewScanner(bytes.NewReader(b))
		sc.Buffer(nil, 1<<20)
		for sc.Scan() {
			if testFunc.Match(sc.Bytes()) {
				st.tests++
			}
		}
		return sc.Err()
	})
	if err != nil {
		return st, fmt.Errorf("count %s: %w", rel, err)
	}
	st.packages = len(dirs)
	if c.counted == nil {
		c.counted = map[string]codeStats{}
	}
	c.counted[rel] = st
	return st, nil
}

// goVersion is the go line of the companion code's go.mod.
func (c *codeRoot) goVersion() (string, error) {
	b, err := os.ReadFile(filepath.Join(c.dir, "go.mod"))
	if err != nil {
		return "", err
	}
	for line := range strings.SplitSeq(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "go "); ok {
			v = strings.TrimSpace(v)
			// "1.27.0" reads as 1.27 in prose.
			if parts := strings.Split(v, "."); len(parts) == 3 && parts[2] == "0" {
				v = parts[0] + "." + parts[1]
			}
			return v, nil
		}
	}
	return "", fmt.Errorf("go.mod has no go line")
}

// countToken is a computed value in a page's prose:
// {{code files}}, {{code lines ch04}}, {{code go-version}}.
var countToken = regexp.MustCompile(`\{\{code ([a-z-]+)(?: ([A-Za-z0-9_./-]+))?\}\}`)

// fillCounts replaces every count token in src with its value.
func (c *codeRoot) fillCounts(src []byte) ([]byte, error) {
	var errs []error
	out := countToken.ReplaceAllFunc(src, func(m []byte) []byte {
		sub := countToken.FindSubmatch(m)
		name, dir := string(sub[1]), cmp.Or(string(sub[2]), ".")
		if name == "go-version" {
			v, err := c.goVersion()
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", m, err))
			}
			return []byte(v)
		}
		st, err := c.stats(dir)
		if err != nil {
			errs = append(errs, err)
			return m
		}
		var n int
		switch name {
		case "files":
			n = st.files
		case "lines":
			n = st.lines
		case "tests":
			n = st.tests
		case "test-files":
			n = st.testFiles
		case "packages":
			n = st.packages
		default:
			errs = append(errs, fmt.Errorf("%s: unknown count %q", m, name))
			return m
		}
		return []byte(thousands(n))
	})
	if len(errs) > 0 {
		return nil, errs[0]
	}
	if bytes.Contains(out, []byte("{{")) {
		return nil, fmt.Errorf("a {{ that isn't a count token; want {{code files}} or similar")
	}
	return out, nil
}

// thousands formats n with commas: 1644 as "1,644".
func thousands(n int) string {
	s := strconv.Itoa(n)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}
