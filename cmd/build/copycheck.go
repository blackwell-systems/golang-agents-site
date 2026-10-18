package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// copyRun is the copy check's threshold: a published page may not share
// a run of this many consecutive words with the book's chapters. There
// is no free web edition of the book, so pages are written in their own
// words; a run this long is a copied sentence, not a shared phrase.
const copyRun = 10

// copyExempt are chapter files left out of the comparison: the author's
// bio and the copyright page, whose wording the site may reuse.
var copyExempt = map[string]bool{"about.adoc": true, "copyright.adoc": true}

var (
	wordRE      = regexp.MustCompile(`[a-z0-9_.'/-]+`)
	adocListing = regexp.MustCompile(`(?ms)^----\n.*?^----\n`)
	adocInclude = regexp.MustCompile(`(?m)^include::.*$`)
	mdFence     = regexp.MustCompile("(?s)```.*?```")
	mdTableRow  = regexp.MustCompile(`(?m)^\|.*$`)
	mdTag       = regexp.MustCompile(`<[^>]+>`)
	mdCodeSpan  = regexp.MustCompile("`[^`\n]*`")
	mdLinkDest  = regexp.MustCompile(`\]\([^)]*\)`)
)

// copyWords lowercases text, folds curly quotes, and splits it into
// words, keeping identifiers and numbers whole.
func copyWords(text string) []string {
	text = strings.NewReplacer("’", "'", "‘", "'", "“", `"`, "”", `"`).Replace(text)
	return wordRE.FindAllString(strings.ToLower(text), -1)
}

// bookRuns holds every run of copyRun words in the book's chapters.
type bookRuns map[string]bool

// loadBook reads the chapters in dir, leaving out code listings and
// includes, which the site may show as code.
func loadBook(dir string) (bookRuns, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.adoc"))
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("copy check: no chapters in %s (set -book to the "+
			"book's book/chapters directory, or -book \"\" to skip the check)", dir)
	}
	runs := bookRuns{}
	for _, f := range files {
		if copyExempt[filepath.Base(f)] {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("copy check: %w", err)
		}
		text := adocInclude.ReplaceAllString(adocListing.ReplaceAllString(string(b), " "), " ")
		w := copyWords(text)
		for i := 0; i+copyRun <= len(w); i++ {
			runs[strings.Join(w[i:i+copyRun], " ")] = true
		}
	}
	return runs, nil
}

// checkCopy reports every run of copyRun or more words that a page's
// title, description, or prose shares with the book. Code blocks, inline
// code (identifiers and program output), table rows (data), HTML tags,
// and link targets are left out.
func checkCopy(runs bookRuns, pages []*Page) error {
	var errs []error
	for _, p := range pages {
		src := p.Title + "\n" + p.Description + "\n" + string(p.Body)
		for _, re := range []*regexp.Regexp{mdFence, mdCodeSpan, mdTableRow, mdTag} {
			src = re.ReplaceAllString(src, " ")
		}
		src = mdLinkDest.ReplaceAllString(src, "]")
		w := copyWords(src)
		end := -1
		for i := 0; i+copyRun <= len(w); i++ {
			if !runs[strings.Join(w[i:i+copyRun], " ")] {
				continue
			}
			if i >= end { // a new run starts
				start := i
				end = i + copyRun
				for end < len(w) && runs[strings.Join(w[end-copyRun+1:end+1], " ")] {
					end++
				}
				errs = append(errs, fmt.Errorf("%s: %q", p.Source,
					strings.Join(w[start:end], " ")))
			}
		}
	}
	if err := errors.Join(errs...); err != nil {
		return fmt.Errorf("copy check: published pages share runs of %d or "+
			"more words with the book (threshold copyRun in cmd/build/copycheck.go):\n%w",
			copyRun, err)
	}
	return nil
}
