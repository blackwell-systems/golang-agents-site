package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// codeRoot is a local copy of the companion code's code/ directory,
// which excerpts are read from and code links are checked against.
type codeRoot struct {
	dir     string
	errs    []error              // errors from code: links, collected during a conversion
	counted map[string]codeStats // stats by directory, counted once
}

// openCode checks that dir looks like the companion code.
func openCode(dir string) (*codeRoot, error) {
	if _, err := os.Stat(filepath.Join(dir, "ch01", "agent", "loop.go")); err != nil {
		return nil, fmt.Errorf("companion code: %w (set -code to the code/ "+
			"directory of a clone of %s)", err, repoURL)
	}
	return &codeRoot{dir: dir}, nil
}

// url is the repository link for rel, a path under code/, after
// checking that it exists. first and last, when set, are the lines to
// highlight.
func (c *codeRoot) url(rel string, first, last int) (string, error) {
	rel = strings.TrimSuffix(rel, "/")
	if path.Clean(rel) != rel || strings.HasPrefix(rel, "../") || rel == ".." {
		return "", fmt.Errorf("code path %q: want a clean path under code/", rel)
	}
	fi, err := os.Stat(filepath.Join(c.dir, filepath.FromSlash(rel)))
	if err != nil {
		return "", fmt.Errorf("code path %q: %w", rel, err)
	}
	kind := "blob"
	if fi.IsDir() {
		kind = "tree"
	}
	u := fmt.Sprintf("%s/%s/%s/code/%s", repoURL, kind, codeRef, rel)
	if first > 0 {
		u += fmt.Sprintf("#L%d-L%d", first, last)
	}
	return u, nil
}

// excerpt is part of a file, ready to show.
type excerpt struct {
	text        string
	first, last int // the lines it came from, 1-based
}

var (
	tagLine  = regexp.MustCompile(`^\s*// (tag|end)::([a-z0-9-]+)\[\]\s*$`)
	callout  = regexp.MustCompile(`\s*// <\d+>$`)
	lineSpec = regexp.MustCompile(`^(\d+)\.\.(-?\d+)$`)
)

// excerpt reads rel and returns the lines between its tag markers, or,
// with no tag, the lines that spec names ("2..-1" is the second line to
// the last), or the whole file. As in the book, other tags' markers are
// dropped, the common indent is removed, and the numbered callouts
// ("// <1>") are taken off, since the page explains the code in prose.
func (c *codeRoot) excerpt(rel, tag, spec string) (excerpt, error) {
	b, err := os.ReadFile(filepath.Join(c.dir, filepath.FromSlash(rel)))
	if err != nil {
		return excerpt{}, fmt.Errorf("include: %w", err)
	}
	lines := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	lo, hi := 1, len(lines)
	switch {
	case tag != "":
		lo, hi = 0, 0
		for i, l := range lines {
			m := tagLine.FindStringSubmatch(l)
			if m == nil || m[2] != tag {
				continue
			}
			if m[1] == "tag" && lo == 0 {
				lo = i + 2
			} else if m[1] == "end" && lo != 0 {
				hi = i
				break
			}
		}
		if lo == 0 || hi < lo {
			return excerpt{}, fmt.Errorf("include %s: no tag %q", rel, tag)
		}
	case spec != "":
		m := lineSpec.FindStringSubmatch(spec)
		if m == nil {
			return excerpt{}, fmt.Errorf("include %s: lines %q: want 2..-1", rel, spec)
		}
		lo, _ = strconv.Atoi(m[1])
		hi, _ = strconv.Atoi(m[2])
		if hi < 0 {
			hi = len(lines) + 1 + hi
		}
		if lo < 1 || hi > len(lines) || hi < lo {
			return excerpt{}, fmt.Errorf("include %s: lines %q out of range", rel, spec)
		}
	}
	type numbered struct {
		n    int
		text string
	}
	var kept []numbered
	for n := lo; n <= hi; n++ {
		l := lines[n-1]
		if tagLine.MatchString(l) {
			continue
		}
		kept = append(kept, numbered{n, callout.ReplaceAllString(l, "")})
	}
	for len(kept) > 0 && strings.TrimSpace(kept[0].text) == "" {
		kept = kept[1:]
	}
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1].text) == "" {
		kept = kept[:len(kept)-1]
	}
	if len(kept) == 0 {
		return excerpt{}, fmt.Errorf("include %s: the excerpt is empty", rel)
	}
	indent := -1
	for _, k := range kept {
		if strings.TrimSpace(k.text) == "" {
			continue
		}
		n := len(k.text) - len(strings.TrimLeft(k.text, " \t"))
		if indent < 0 || n < indent {
			indent = n
		}
	}
	var sb strings.Builder
	for _, k := range kept {
		t := k.text
		if len(t) >= indent {
			t = t[indent:]
		}
		sb.WriteString(strings.TrimRight(t, " \t"))
		sb.WriteByte('\n')
	}
	ex := excerpt{text: sb.String(), first: kept[0].n, last: kept[len(kept)-1].n}
	if tag == "" && spec == "" {
		ex.first, ex.last = 0, 0 // a whole file links without a range
	}
	return ex, nil
}
