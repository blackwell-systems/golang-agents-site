// Command og renders golang-agents.com's share cards: one 1200 x 630
// image per page, for Open Graph and Twitter, into static/og/.
//
// The site's build describes each card (go run ./cmd/build -og-spec);
// this tool fills card.html with each description, has headless Chrome
// take a screenshot of it, and writes the screenshot as a JPEG. A card
// whose inputs (its description, the template, the fonts, its art) are
// unchanged since the last run, as recorded in manifest.json, is
// skipped. Cards for pages that no longer exist are removed.
//
// Run it from the repository root:
//
//	go run ./tools/og
//
// See README.md in this directory for the flags and requirements.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
)

// version is part of every card's hash: change it to render every card
// again after a change this tool can't see, such as to its own code.
const version = "og 1"

// card is one card's description, as cmd/build writes it (shareCard in
// cmd/build/og.go).
type card struct {
	File   string `json:"file"`
	Layout string `json:"layout"`
	Label  string `json:"label,omitempty"`
	Title  string `json:"title"`
	Sub    string `json:"sub,omitempty"`
	Figure string `json:"figure,omitempty"`
	Unit   string `json:"unit,omitempty"`
	Art    string `json:"art,omitempty"`
	Alt    string `json:"alt"`
}

// sizes are the font sizes, in px, the fitting in card.html tries for a
// card's text, each as "largest smallest".
type sizes struct {
	Title, Sub, Figure string
}

// sizesFor is the range of sizes for c. A short title takes the
// largest size that fits; a long one shrinks until it fits in three
// lines.
func sizesFor(c card) sizes {
	switch {
	case c.Layout == "home":
		return sizes{"104 64", "40 28", "0 0"}
	case c.Layout == "book":
		return sizes{"66 40", "34 24", "0 0"}
	case c.Layout == "section":
		return sizes{"132 64", "38 26", "0 0"}
	case c.Figure != "":
		return sizes{"52 30", "28 20", "176 72"}
	case c.Art == "":
		return sizes{"104 34", "34 22", "0 0"}
	default:
		return sizes{"84 34", "34 22", "0 0"}
	}
}

// manifestEntry records what a card was rendered from: spec is the hash
// of its description alone, which the build compares to tell a stale
// card; hash covers every input.
type manifestEntry struct {
	Spec string `json:"spec"`
	Hash string `json:"hash"`
}

type config struct {
	root, chrome, spec, only, keep string
	force                          bool
	jobs                           int
}

func main() {
	var cfg config
	flag.StringVar(&cfg.root, "root", ".", "the site repository")
	flag.StringVar(&cfg.chrome, "chrome", os.Getenv("OG_CHROME"),
		"headless Chrome or chrome-headless-shell; default $OG_CHROME, else a search of the usual places")
	flag.StringVar(&cfg.spec, "spec", "",
		"the cards' descriptions; default: run go run ./cmd/build -og-spec")
	flag.StringVar(&cfg.only, "only", "", "render only cards whose file contains this text")
	flag.StringVar(&cfg.keep, "keep", "", "also write each card's HTML into this directory, for debugging")
	flag.BoolVar(&cfg.force, "force", false, "render every card, changed or not")
	flag.IntVar(&cfg.jobs, "j", min(4, max(1, runtime.NumCPU()/2)), "cards rendered at once")
	flag.Parse()
	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "og:", err)
		os.Exit(1)
	}
}

func run(cfg config) error {
	dir := filepath.Join(cfg.root, "tools", "og")
	static := filepath.Join(cfg.root, "static")
	raws, err := loadSpec(cfg)
	if err != nil {
		return err
	}
	tmplSrc, err := os.ReadFile(filepath.Join(dir, "card.html"))
	if err != nil {
		return err
	}
	tmpl, err := template.New("card").Parse(string(tmplSrc))
	if err != nil {
		return fmt.Errorf("card.html: %w", err)
	}
	fonts, fontSum, err := loadFonts(filepath.Join(static, "fonts"))
	if err != nil {
		return err
	}
	manifestFile := filepath.Join(dir, "manifest.json")
	manifest := map[string]manifestEntry{}
	if b, err := os.ReadFile(manifestFile); err == nil {
		if err := json.Unmarshal(b, &manifest); err != nil {
			return fmt.Errorf("%s: %w", manifestFile, err)
		}
	}

	type job struct {
		card       card
		spec, hash string
	}
	var jobs []job
	want := map[string]bool{}
	for _, raw := range raws {
		var c card
		if err := json.Unmarshal(raw, &c); err != nil {
			return fmt.Errorf("spec: %w", err)
		}
		want[c.File] = true
		var compact bytes.Buffer
		if err := json.Compact(&compact, raw); err != nil {
			return fmt.Errorf("spec: %w", err)
		}
		spec := sum(compact.Bytes())
		h := sha256.New()
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00", version, sum(tmplSrc), fontSum, spec)
		if c.Art != "" {
			art, err := os.ReadFile(filepath.Join(static, filepath.FromSlash(c.Art)))
			if err != nil {
				return fmt.Errorf("%s: art: %w", c.File, err)
			}
			h.Write(art)
		}
		hash := hex.EncodeToString(h.Sum(nil))
		if cfg.only != "" && !strings.Contains(c.File, cfg.only) {
			continue
		}
		_, statErr := os.Stat(filepath.Join(static, filepath.FromSlash(c.File)))
		if !cfg.force && statErr == nil && manifest[c.File].Hash == hash {
			continue
		}
		jobs = append(jobs, job{c, spec, hash})
	}

	chrome, err := findChrome(cfg.chrome)
	if len(jobs) > 0 && err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "og-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	var (
		mu   sync.Mutex
		errs []error
		wg   sync.WaitGroup
		sem  = make(chan struct{}, max(1, cfg.jobs))
	)
	for i, j := range jobs {
		wg.Go(func() {
			sem <- struct{}{}
			defer func() { <-sem }()
			err := render(renderArgs{
				card: j.card, tmpl: tmpl, fonts: fonts, chrome: chrome,
				static: static, work: filepath.Join(tmp, fmt.Sprint(i)), keep: cfg.keep,
			})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				errs = append(errs, fmt.Errorf("%s: %w", j.card.File, err))
				return
			}
			manifest[j.card.File] = manifestEntry{Spec: j.spec, Hash: j.hash}
			fmt.Println("og: wrote", j.card.File)
		})
	}
	wg.Wait()

	removed := 0
	if cfg.only == "" {
		removed, err = prune(static, want, manifest)
		if err != nil {
			errs = append(errs, err)
		}
	}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(manifestFile, append(b, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("og: %d cards, %d rendered, %d removed\n", len(raws), len(jobs)-len(errs), removed)
	return errors.Join(errs...)
}

// loadSpec reads the cards' descriptions, from -spec or from the build.
func loadSpec(cfg config) ([]json.RawMessage, error) {
	var b []byte
	var err error
	if cfg.spec != "" {
		b, err = os.ReadFile(cfg.spec)
	} else {
		cmd := exec.Command("go", "run", "./cmd/build", "-og-spec", "-")
		cmd.Dir = cfg.root
		cmd.Stderr = os.Stderr
		b, err = cmd.Output()
	}
	if err != nil {
		return nil, fmt.Errorf("card descriptions: %w", err)
	}
	var raws []json.RawMessage
	if err := json.Unmarshal(b, &raws); err != nil {
		return nil, fmt.Errorf("card descriptions: %w", err)
	}
	return raws, nil
}

// fontFaces are the site's fonts the cards use, from static/fonts/.
var fontFaces = []struct{ family, weight, file string }{
	{"Cinzel", "600", "cinzel-latin-600-normal.woff2"},
	{"Inter", "100 900", "inter-latin-wght-normal.woff2"},
	{"Space Grotesk", "500", "space-grotesk-latin-500-normal.woff2"},
	{"Space Grotesk", "600", "space-grotesk-latin-600-normal.woff2"},
}

// loadFonts returns @font-face rules with each font inline, since a
// page opened from a file may not load fonts from other files, and a
// hash of the fonts.
func loadFonts(dir string) (template.CSS, string, error) {
	var css strings.Builder
	h := sha256.New()
	for _, f := range fontFaces {
		b, err := os.ReadFile(filepath.Join(dir, f.file))
		if err != nil {
			return "", "", fmt.Errorf("font: %w", err)
		}
		h.Write(b)
		fmt.Fprintf(&css, "@font-face { font-family: %q; font-weight: %s; font-display: block; "+
			"src: url(data:font/woff2;base64,%s) format(\"woff2\"); }\n",
			f.family, f.weight, base64.StdEncoding.EncodeToString(b))
	}
	return template.CSS(css.String()), hex.EncodeToString(h.Sum(nil)), nil
}

type renderArgs struct {
	card                       card
	tmpl                       *template.Template
	fonts                      template.CSS
	chrome, static, work, keep string
}

// render writes one card: its HTML, a screenshot of it, and the JPEG.
func render(a renderArgs) error {
	if err := os.MkdirAll(a.work, 0o755); err != nil {
		return err
	}
	var art template.URL
	if a.card.Art != "" {
		abs, err := filepath.Abs(filepath.Join(a.static, filepath.FromSlash(a.card.Art)))
		if err != nil {
			return err
		}
		art = template.URL("file://" + filepath.ToSlash(abs))
	}
	var page bytes.Buffer
	err := a.tmpl.Execute(&page, map[string]any{
		"Card": a.card, "Size": sizesFor(a.card), "Fonts": a.fonts, "Art": art,
	})
	if err != nil {
		return fmt.Errorf("template: %w", err)
	}
	htmlFile := filepath.Join(a.work, "card.html")
	if err := os.WriteFile(htmlFile, page.Bytes(), 0o644); err != nil {
		return err
	}
	if a.keep != "" {
		name := strings.ReplaceAll(strings.TrimSuffix(a.card.File, filepath.Ext(a.card.File)), "/", "_") + ".html"
		if err := os.MkdirAll(a.keep, 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(a.keep, name), page.Bytes(), 0o644); err != nil {
			return err
		}
	}
	shot := filepath.Join(a.work, "card.png")
	abs, err := filepath.Abs(htmlFile)
	if err != nil {
		return err
	}
	cmd := exec.Command(a.chrome,
		"--headless", "--disable-gpu", "--hide-scrollbars", "--mute-audio",
		"--force-device-scale-factor=1", "--window-size=1200,630",
		"--virtual-time-budget=15000", "--run-all-compositor-stages-before-draw",
		"--user-data-dir="+filepath.Join(a.work, "profile"),
		"--screenshot="+shot, "file://"+filepath.ToSlash(abs))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chrome: %w\n%s", err, out)
	}
	if err := checkShot(shot); err != nil {
		return err
	}
	dst := filepath.Join(a.static, filepath.FromSlash(a.card.File))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	// JPEG because the plates are paintings: as PNG a card is about
	// 700 KB; at this quality about 100 to 140 KB, with the text sharp.
	cmd = exec.Command("magick", shot, "-strip", "-sampling-factor", "4:2:0",
		"-quality", "82", "-interlace", "JPEG", dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("magick: %w\n%s", err, out)
	}
	return nil
}

// checkShot reports a screenshot of the wrong size, or one whose corner
// is magenta: the card's text didn't fit, or the fitting never ran.
func checkShot(file string) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return fmt.Errorf("screenshot: %w", err)
	}
	if b := img.Bounds(); b != image.Rect(0, 0, 1200, 630) {
		return fmt.Errorf("screenshot is %v, want 1200 x 630", b.Size())
	}
	c := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA)
	if c.R > 240 && c.G < 16 && c.B > 240 {
		return errors.New("the text doesn't fit at the smallest sizes in sizesFor, or the fitting didn't finish")
	}
	return nil
}

// prune removes card files under static/og/ that no page claims, and
// their manifest entries.
func prune(static string, want map[string]bool, manifest map[string]manifestEntry) (int, error) {
	n := 0
	root := filepath.Join(static, "og")
	err := filepath.WalkDir(root, func(file string, d fs.DirEntry, err error) error {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(static, file)
		if err != nil {
			return err
		}
		if rel = filepath.ToSlash(rel); !want[rel] {
			n++
			return os.Remove(file)
		}
		return nil
	})
	for f := range manifest {
		if !want[f] {
			delete(manifest, f)
		}
	}
	// Directories the removals emptied.
	var dirs []string
	_ = filepath.WalkDir(root, func(file string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() && file != root {
			dirs = append(dirs, file)
		}
		return nil
	})
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, d := range dirs {
		os.Remove(d) // fails, as it should, unless empty
	}
	return n, err
}

// findChrome returns path if set, or else the first headless Chrome it
// finds: Google Chrome or Chromium on the PATH or in /Applications, or
// a chrome-headless-shell that Puppeteer or Playwright downloaded.
func findChrome(path string) (string, error) {
	if path != "" {
		return path, nil
	}
	for _, name := range []string{"chrome-headless-shell", "google-chrome", "chromium", "chromium-browser"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
		"/Applications/Chromium.app/Contents/MacOS/Chromium",
	}
	for _, pattern := range []string{
		filepath.Join(home, ".cache/puppeteer/chrome-headless-shell/*/*/chrome-headless-shell"),
		filepath.Join(home, "Library/Caches/ms-playwright/chromium_headless_shell-*/*/chrome-headless-shell"),
		filepath.Join(home, ".cache/ms-playwright/chromium_headless_shell-*/*/chrome-headless-shell"),
	} {
		m, _ := filepath.Glob(pattern)
		slices.Reverse(m) // the newest version first, roughly
		candidates = append(candidates, m...)
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", errors.New("no headless Chrome found: set -chrome or $OG_CHROME (see tools/og/README.md)")
}

// sum is the hex SHA-256 of b.
func sum(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}
