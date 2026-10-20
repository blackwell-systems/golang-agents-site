package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// iconSprite is the path of the icon sprite, static/img/icons.svg, with
// a version that loadIcons sets from its contents, so a changed sprite
// is never served from a stale cache. The link check fails the build if
// an icon names a symbol the sprite doesn't have.
var iconSprite = "/img/icons.svg"

// loadIcons checks and versions the icon sprite under static. A
// browser draws nothing at all from a sprite that isn't well-formed XML
// (two hyphens in a comment are enough), so that fails the build.
func loadIcons(static string) error {
	b, err := os.ReadFile(filepath.Join(static, "img", "icons.svg"))
	if err != nil {
		return fmt.Errorf("icons: %w", err)
	}
	d := xml.NewDecoder(bytes.NewReader(b))
	for {
		_, err := d.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("icons: static/img/icons.svg: %w", err)
		}
	}
	sum := sha256.Sum256(b)
	iconSprite = "/img/icons.svg?v=" + hex.EncodeToString(sum[:4])
	return nil
}

// icon draws the sprite's symbol name. Every icon on the site sits
// beside words that say the same thing, so it is hidden from screen
// readers. Its size and colors come from the stylesheet (.icon).
func icon(name string) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<svg class="icon icon-%s" aria-hidden="true" focusable="false"><use href="%s#%s"></use></svg>`,
		html.EscapeString(name), iconSprite, html.EscapeString(name)))
}

// headingIcons are the h2s, by id, that open with an icon: the
// sections many guides share, which the book sets apart the same way.
var headingIcons = map[string]string{
	"try-it":            "block-try",   // the closing exercises
	"when-to-use-it":    "block-fit",   // above a fit block
	"mistakes-to-avoid": "block-watch", // the book's "Watch out"
	"what-to-watch":     "block-watch",
	// what a production service adds to an example: the book's "In production"
	"what-a-real-service-adds": "block-production",
}

// markHeadings puts each heading's icon at the start of its h2 in body.
func markHeadings(body template.HTML) template.HTML {
	s := string(body)
	for id, name := range headingIcons {
		open := `<h2 id="` + id + `">`
		s = strings.ReplaceAll(s, open, `<h2 id="`+id+`" class="has-icon">`+string(icon(name)))
	}
	return template.HTML(s)
}
