package main

import (
	"errors"
	"fmt"
	"html"
	"io/fs"
	"maps"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

var (
	linkAttr = regexp.MustCompile(`\s(?:href|src)="([^"]*)"`)
	srcset   = regexp.MustCompile(`\ssrcset="([^"]*)"`)
	idAttr   = regexp.MustCompile(`\sid="([^"]*)"`)
)

// checkLinks checks every internal link and asset reference in the
// HTML under out: the file must exist, and a #fragment must name an id
// in it. Links into the companion repository's code/ tree must name a
// file in the local copy. It returns how many links it checked.
func checkLinks(out string, code *codeRoot) (int, error) {
	ids := map[string]map[string]bool{} // file under out -> its ids
	pages := map[string]string{}        // file under out -> its HTML
	err := filepath.WalkDir(out, func(file string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(file) != ".html" {
			return err
		}
		b, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(out, file)
		rel = filepath.ToSlash(rel)
		pages[rel] = string(b)
		ids[rel] = map[string]bool{}
		for _, m := range idAttr.FindAllStringSubmatch(string(b), -1) {
			ids[rel][html.UnescapeString(m[1])] = true
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("check links: %w", err)
	}
	var errs []error
	n := 0
	for _, rel := range slices.Sorted(maps.Keys(pages)) {
		doc := pages[rel]
		var refs []string
		for _, m := range linkAttr.FindAllStringSubmatch(doc, -1) {
			refs = append(refs, html.UnescapeString(m[1]))
		}
		for _, m := range srcset.FindAllStringSubmatch(doc, -1) {
			for c := range strings.SplitSeq(html.UnescapeString(m[1]), ",") {
				if f := strings.Fields(c); len(f) > 0 {
					refs = append(refs, f[0])
				}
			}
		}
		for _, ref := range refs {
			ok, err := checkRef(out, rel, ref, ids, code)
			if ok {
				n++
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", rel, err))
			}
		}
	}
	if err := errors.Join(errs...); err != nil {
		return n, fmt.Errorf("broken links:\n%w", err)
	}
	return n, nil
}

// checkRef checks one reference from the page at rel. It reports
// whether the reference was one it checks.
func checkRef(out, rel, ref string, ids map[string]map[string]bool, code *codeRoot) (bool, error) {
	u, err := url.Parse(ref)
	if err != nil {
		return true, fmt.Errorf("link %q: %w", ref, err)
	}
	if u.Scheme != "" || u.Host != "" {
		prefix := repoURL + "/"
		if !strings.HasPrefix(ref, prefix) {
			return false, nil // external; not checked offline
		}
		rest := strings.TrimPrefix(u.Path, strings.TrimPrefix(prefix, "https://github.com"))
		for _, kind := range []string{"blob/", "tree/"} {
			if p, ok := strings.CutPrefix(rest, kind+codeRef+"/code/"); ok {
				_, err := code.url(p, 0, 0)
				return true, err
			}
		}
		return false, nil
	}
	if u.Scheme == "mailto" {
		return false, nil
	}
	target := rel
	if u.Path != "" {
		p := u.Path
		if !strings.HasPrefix(p, "/") {
			p = path.Join(path.Dir("/"+rel), p)
		}
		target = strings.TrimPrefix(p, "/")
		if target == "" || strings.HasSuffix(p, "/") {
			target = path.Join(target, "index.html")
		}
		fi, err := os.Stat(filepath.Join(out, filepath.FromSlash(target)))
		if err != nil {
			return true, fmt.Errorf("link %q: no file %s", ref, target)
		}
		if fi.IsDir() {
			return true, fmt.Errorf("link %q: %s is a directory; end the link with /", ref, target)
		}
	}
	if u.Fragment != "" {
		pageIDs, ok := ids[target]
		if !ok {
			return true, fmt.Errorf("link %q: a fragment on a file that isn't a page", ref)
		}
		if !pageIDs[u.Fragment] {
			return true, fmt.Errorf("link %q: no id %q in %s", ref, u.Fragment, target)
		}
	}
	return true, nil
}
