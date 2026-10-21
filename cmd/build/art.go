package main

import (
	"errors"
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// An artwork is one of the book's illustrations, written at web sizes
// into static/img/art/ by tools/art.sh. The openers are 4:5 plates on
// their own navy ground; the badge is the Gopher on a transparent one.
type artwork struct {
	Alt    string
	Widths []int  // the widths written, in px
	Ratio  [2]int // width and height, for the height of each width
	Ext    string // the fallback format: jpg, or png for the badge
}

// The widths tools/art.sh writes.
var (
	plateWidths = []int{480, 720, 1080}
	badgeWidths = []int{240, 480}
	cardWidths  = []int{480, 960}
)

// plate is an opener, with its alt text.
func plate(alt string) artwork {
	return artwork{Alt: alt, Widths: plateWidths, Ratio: [2]int{1122, 1402}, Ext: "jpg"}
}

// card is a 3:2 picture at the top of a card, with its alt text. The
// cards are the site's own art, from art/ in this repository.
func card(alt string) artwork {
	return artwork{Alt: alt, Widths: cardWidths, Ratio: [2]int{3, 2}, Ext: "jpg"}
}

// arts are the illustrations by name, with alt text that describes each
// picture for a reader who can't see it. The cast: the Gopher, the Go
// mascot, who builds and runs the harness; and the Ghost, a tall stone
// android, the model in the loop.
var arts = map[string]artwork{
	"intro":                 plate("The Gopher and the Ghost, a tall stone android, lean over a map on a stone block at the foot of a monument; the Ghost points to the first stop on a lit route."),
	"part1":                 plate("The Gopher reaches up for the last curved piece of a toy train track that the kneeling Ghost, a stone android, holds out, to close a loop where four small cars wait, each carrying one tool."),
	"part2":                 plate("In a workshop under a lamp, the Gopher on a stool holds up a wrench, while the Ghost, a stone android, sits on the floor with a wrench from the pegboard, a finger to its head, studying a blank crash report pinned among the tools."),
	"part3":                 plate("High on a terrace under great stone rings, the Gopher holds up a lantern beside the seated Ghost, a stone android, with the monument's lit workshop windows falling away below."),
	"ch01":                  plate("The Gopher as a conductor in a cap, one hand on a tall lever, watches the Ghost, a stone android, ride a loop-the-loop in a car far too small for it, past four stations: a box, a cylinder, a cone, and an arch."),
	"ch02":                  plate("The Gopher talks into a tin can whose string arcs up to the Ghost, a stone android, small on a high terrace with the other can to its head, while a long blank receipt curls out of the Gopher's can across the floor."),
	"ch03":                  plate("The kneeling Ghost, a stone android, holds open a door in its chest while the Gopher, on a stepstool, looks in through a magnifying glass at a tiny conveyor of blank tiles running toward a tray where two dice tumble."),
	"ch04":                  plate("The Gopher holds up a small screwdriver to the kneeling Ghost, a stone android, whose long fingers reach to take its first tool, beside an open toolbox and a table clock opened to show its gears."),
	"ch05":                  plate("The Gopher bolts a guardrail onto a loop track beside a stone post with a red stop button and a bead lap counter, a piggy bank nearby, while the Ghost, a stone android, kneels and watches the empty ride car."),
	"ch06":                  plate("The Ghost, a stone android, types at a small iron teleprinter while a long blank paper strip streams out across the floor, and the Gopher walks beside it reading as it comes, scissors in hand."),
	"ch07":                  plate("On a stool beside a tall cabinet of tagged drawers, the Gopher ties a blank tag onto a drawer while the kneeling Ghost, a stone android, reaches toward it and waits."),
	"ch08":                  plate("In front of a tall shelf of scrolls, the Gopher, an armful of long scrolls in hand, slides one short tied scroll into a small backpack that the kneeling Ghost, a stone android, holds open."),
	"ch09":                  plate("Six copies of the Ghost, a stone android, work at desks in a row of stone alcoves, each on a folder marked with a different shape, while the Gopher holds up a stopwatch beside one shared jar of coins."),
	"ch10":                  plate("A solid stone wall separates two rooms; the Gopher slides an open toolbox into a window through the wall, and on the other side the Ghost, a stone android, reaches in and lifts out a wrench."),
	"ch11":                  plate("The Gopher fits an adapter on the plug at the end of the Ghost's gathered cables, toward one of three differently shaped sockets in a stone wall, while the Ghost, a stone android, kneels and holds the cables."),
	"ch12":                  plate("In a stone control room, the Gopher in a hard hat sits by a breaker switch watching a trace of amber bars on a large screen, while the Ghost, a stone android, works at a bench beside it, one of its cables frayed."),
	"ch13":                  plate("Five identical copies of the Ghost, a stone android, kneel on stone plinths with the same folder, their eyes slightly different in brightness, while the Gopher judges with a clipboard in front of a chalkboard of five overlapping error bars."),
	"ch14":                  plate("The Ghost, a stone android, kneels in a walled sandbox digging with a small shovel, ignoring a crash report with a folded note tucked into it outside the wall, while the Gopher rests a hand on a green approval button without pressing it."),
	"ch15":                  plate("The Gopher, lantern in hand, plants flags marked with a circle, a triangle, and a square on the landings of a long stair, holding a book stamped with the same shapes, while the Ghost, a stone android, climbs below with an armful of folders and an unplugged cord lies on the bottom step."),
	"ch16":                  plate("In a lantern-lit market arcade, the Gopher studies a boxed kit beside open trays of neatly packed parts, while the Ghost, a stone android, turns over a well-made wrench from one of the kits."),
	"ch17":                  plate("Under great stone rings, the Ghost, a stone android, sorts crash reports into folders marked with simple shapes along a long stone table, while the Gopher checks a small tray set aside, labeled Ask a human."),
	"not-found":             card("On a stone ledge that ends in open air, the Gopher frowns at a map by lantern light while the Ghost, a stone android, leans out from a pillar and points the way back."),
	"path-learning":         card("At the foot of a wide stone stair, the Gopher holds up a lantern and points up a row of small lanterns toward the great stone rings, while the Ghost, a stone android, kneels beside it."),
	"path-shipping":         card("On a loading dock under a work lamp, the Gopher in an amber hard hat ticks a line on its clipboard while the Ghost, a stone android, sets a sealed crate onto a cart on iron rails."),
	"path-leading":          card("Under a hanging lamp, the Gopher on a stool moves a token along a dotted route on a map of the monument, while the Ghost, a stone android, leans in and points to the same route."),
	"section-research":      plate("At a stone workbench by lantern light, the Gopher sets small stone blocks on one pan of a brass balance while the kneeling Ghost, a stone android, tallies the weighings on a slate."),
	"getting-set-up":        plate("By lantern light, the Gopher stands in an open wooden chest of supplies and lifts out a coil of rope while the kneeling Ghost, a stone android, holds the lid, with tool cases ready on the stones."),
	"first-model-call":      plate("Under a stone arch by lantern light, the Gopher pulls the rope of a great bronze bell while, far off on a high bridge, the Ghost, a stone android, raises a hand in answer."),
	"rag":                   plate("Before a towering wall of stone drawers, the Gopher on a rolling ladder pulls out stone tablets and drops them into a basket the kneeling Ghost, a stone android, holds up, by lantern light."),
	"memory":                plate("On a ladder before a wall of arched stone pigeonholes, the Gopher files a note into one by lantern light while the kneeling Ghost, a stone android, steadies the ladder."),
	"patterns-by-name":      plate("By lantern light, the Gopher, clipboard in hand, points a long stick at four carved stone panels (a ring, a fork, three parallel arrows, and a looping arrow) while the crouching Ghost, a stone android, studies them."),
	"glossary":              plate("On a stone lectern by lantern light, the Gopher, clipboard in hand, reads a great open book while the kneeling Ghost, a stone android, turns a page."),
	"go-context":            plate("The kneeling Ghost, a stone android, holds up a branching rope hung with lit lanterns while the Gopher, on a stool, snips one branch with shears, and the lanterns hanging below the cut go dark."),
	"go-errors":             plate("By lantern light, the Gopher lifts a small stone block out of a set of nested stone boxes, each inside the next, while the Ghost, a stone android, holds the outer lid open."),
	"go-json":               plate("Beside an iron printing press by lantern light, the Gopher holds up two freshly printed sheets with the same block layout while the Ghost, a stone android, carries a neat stack of blank sheets."),
	"go-http":               plate("Beside an hourglass and a lantern, the Gopher speaks into the mouth of a long brass speaking tube that climbs the stone terraces to where the Ghost, a stone android, waits at its far end."),
	"go-goroutines":         plate("On a stone viaduct, the Gopher works a lever that sends small carts of stone down parallel rails, and the rails merge into one track where the kneeling Ghost, a stone android, gathers each load into a basket by lantern light."),
	"go-iterators":          plate("The Gopher turns the crank of a stone roller conveyor that brings one block at a time to the kneeling Ghost, a stone android, who lifts each block off by lantern light."),
	"choosing-a-model":      plate("By lantern light, the Gopher tries a chisel on a rough stone block while the kneeling Ghost, a stone android, holds up two other chisels of different sizes, with more laid out on a cloth beside them."),
	"commands-and-files":    plate("Inside a low walled stone courtyard with one gate, the Gopher tends a brass clockwork machine, while outside the wall the kneeling Ghost, a stone android, rests a hand on an iron stop lever whose rod runs through the wall to the machine."),
	"common-errors":         plate("On a broken flagstone path, the Gopher plants a small red flag beside a gap, with two more flags already marking gaps ahead, while the stooping Ghost, a stone android, holds a lantern low over the path."),
	"messages-and-roles":    plate("A long scroll unrolls across a stone table, its sections stamped in order with simple seals: a circle, a diamond, a triangle, a square. The Gopher holds a heavy stone seal over the next blank section while the Ghost, a stone android, follows the earlier sections with one finger, by lantern light."),
	"rag-or-agentic-search": plate("Two ways to fetch stone tablets: on the left a chute pours a heap of them into a basket at once; on the right the Ghost, a stone android, lantern raised, picks tablets from a tall shelf one at a time. Between them the Gopher holds a clipboard and a small sand timer, watching both."),
	"more-providers":        plate("Along a row of stone doors, each with a differently shaped keyhole, the Gopher turns a key in the round lock of the first door, a ring of brass keys in its other hand, while the kneeling Ghost, a stone android, holds a lantern to the next keyhole."),
	"why-go":                plate("On a stone step by lantern light, the Gopher lifts a wrench from a plain, well-made iron toolbox with neat fitted compartments while the kneeling Ghost, a stone android, looks over the tidy tools inside."),
	"measured-mistakes":     plate("Before shelves of broken things under glass domes (a snapped gear, a frayed rope, a cracked lantern, a bent key, a split tablet), the Gopher, clipboard in hand, studies one through a magnifying glass while the stooping Ghost, a stone android, holds a lantern close."),
	"launch-checklist":      plate("Beside a great iron engine lined with round gauges, the Gopher, clipboard under one arm and a pencil behind its ear, taps one gauge to check it while the kneeling Ghost, a stone android, waits with a hand on the start lever, by lantern light."),
	"first-agent":           plate("By lantern light, the Gopher sets a stone block into a small ring of blocks on a workbench while the Ghost, a stone android, leans in close to watch, mallet and chisel at hand."),
	"section-examples":      plate("On a stone ledge by lantern light, the Gopher turns the crank of a working model of gears and a looping track while the leaning Ghost, a stone android, watches a little cart ride the rails."),
	"section-ecosystem":     plate("On a round stone platform high above a city of bridges and towers, the Gopher studies a map and points down one stone causeway while the kneeling Ghost, a stone android, sets a lantern down beside it."),
	"section-production":    plate("At night beside a great barred stone door, the Ghost, a stone android, stands guard with one hand on the heavy bar while the Gopher keeps a tally of plain marks at a stone desk by lantern light."),
	"section-build":         plate("On a wooden scaffold high on the monument, the kneeling Ghost, a stone android, holds a great curved stone segment of a ring steady while the Gopher, on the top plank, taps it into place with a mallet beside a hanging lantern."),
	"section-basics":        plate("At the foot of the monument's great stairs, the kneeling Ghost, a stone android, holds a lantern low while the Gopher, a clipboard under one arm, steps onto the first stair and looks up the long climb toward the lit rings."),
	"path-complete":         card("On a terrace at the top of the stairs, the kneeling Ghost, a stone android, holds the Gopher on its open palm, the Gopher's clipboard fully ticked, beside a lantern, with lit towers and stairs far below."),
	"gopher": {
		Alt:    "The Gopher, the site's mascot, with its clipboard.",
		Widths: badgeWidths, Ratio: [2]int{1, 1}, Ext: "png",
	},
}

// artSizes are the sizes attribute for each place art appears, so the
// browser picks the smallest file that is sharp at that size.
var artSizes = map[string]string{
	"hero":  "(min-width: 48rem) 15rem, 8rem",
	"guide": "(min-width: 60rem) 10rem, 6rem",
	"badge": "12rem",
	"card":  "(min-width: 48rem) 20rem, 100vw",
	"wide":  "(min-width: 48rem) 40rem, 100vw",
}

// artToken places art in a page's own HTML: {{art path-learning card}}
// becomes the picture, sized for that place.
var artToken = regexp.MustCompile(`\{\{art ([a-z0-9-]+) ([a-z]+)\}\}`)

// bodyArt is the art that tokens placed, for the build to check that
// its files exist.
var bodyArt []string

// fillArt replaces every art token in src with its picture.
func fillArt(src []byte) ([]byte, error) {
	var errs []error
	out := artToken.ReplaceAllFunc(src, func(m []byte) []byte {
		sub := artToken.FindSubmatch(m)
		pic, err := picture(string(sub[1]), string(sub[2]), false)
		if err != nil {
			errs = append(errs, err)
			return m
		}
		bodyArt = append(bodyArt, string(sub[1]))
		return []byte(pic)
	})
	return out, errors.Join(errs...)
}

// checkArt reports a page's art that isn't known, or whose files are
// missing from static/img/art/.
func checkArt(static string, names ...string) error {
	for _, name := range names {
		a, ok := arts[name]
		if !ok {
			return fmt.Errorf("art %q: not in arts (cmd/build/art.go)", name)
		}
		for _, w := range a.Widths {
			for _, ext := range []string{"avif", "webp", a.Ext} {
				f := filepath.Join(static, "img", "art", fmt.Sprintf("%s-%d.%s", name, w, ext))
				if _, err := os.Stat(f); err != nil {
					return fmt.Errorf("art %q: %w (run tools/art.sh)", name, err)
				}
			}
		}
	}
	return nil
}

// picture is the <picture> element for art name in place, one of the
// keys of artSizes. Every image has its width and height, so the page
// doesn't shift when it loads, and decodes off the main thread; lazy
// defers it until it nears the viewport, for art below the fold.
func picture(name, place string, lazy bool) (template.HTML, error) {
	a, ok := arts[name]
	if !ok {
		return "", fmt.Errorf("art %q: unknown", name)
	}
	sizes, ok := artSizes[place]
	if !ok {
		return "", fmt.Errorf("art place %q: unknown", place)
	}
	srcset := func(ext string) string {
		var parts []string
		for _, w := range a.Widths {
			parts = append(parts, fmt.Sprintf("/img/art/%s-%d.%s %dw", name, w, ext, w))
		}
		return strings.Join(parts, ", ")
	}
	w := a.Widths[0]
	h := (w*a.Ratio[1] + a.Ratio[0]/2) / a.Ratio[0] // rounded, as the resize rounds
	loading := "eager"
	if lazy {
		loading = "lazy"
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<picture class="art art-%s">`, place)
	fmt.Fprintf(&b, `<source type="image/avif" srcset="%s" sizes="%s">`, srcset("avif"), sizes)
	fmt.Fprintf(&b, `<source type="image/webp" srcset="%s" sizes="%s">`, srcset("webp"), sizes)
	fmt.Fprintf(&b, `<img src="/img/art/%s-%d.%s" srcset="%s" sizes="%s" width="%d" height="%d" alt="%s" loading="%s" decoding="async">`,
		name, w, a.Ext, srcset(a.Ext), sizes, w, h, html.EscapeString(a.Alt), loading)
	b.WriteString(`</picture>`)
	return template.HTML(b.String()), nil
}
