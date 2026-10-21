#!/usr/bin/env bash
# Writes the book's opener art and the Gopher badge into static/img/art/
# at web sizes, from the book repository's images. The openers (4:5
# plates, 1122 x 1402) become AVIF, WebP, and JPEG at 480, 720, and
# 1080 px wide; the badge, which is transparent, becomes AVIF, WebP,
# and PNG at 240 and 480. The alt text and sizes the pages use are in
# cmd/build/art.go, which the build checks against these files.
# Needs ImageMagick 7, avifenc, and cwebp.
#
#   tools/art.sh [book-dir]      # default ../go-agents-book
set -euo pipefail

site=$(cd "$(dirname "$0")/.." && pwd)
book=$(cd "${1:-$site/../go-agents-book}" && pwd)
src=$book/book/images
out=$site/static/img/art
mkdir -p "$out"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

names="intro part1 part2 part3"
for n in $(seq -w 1 17); do names="$names ch$n"; done

for name in $names; do
	in=$src/openers/$name.jpg
	[ -f "$in" ] || { echo "art: no $in" >&2; exit 1; }
	for w in 480 720 1080; do
		png=$tmp/$name-$w.png
		magick "$in" -colorspace sRGB -filter Lanczos -resize "${w}x" -strip "$png"
		avifenc -q 55 -s 4 "$png" "$out/$name-$w.avif" >/dev/null
		cwebp -quiet -q 76 -m 6 "$png" -o "$out/$name-$w.webp"
		magick "$png" -quality 78 -sampling-factor 4:2:0 -interlace JPEG "$out/$name-$w.jpg"
	done
done

for w in 240 480; do
	png=$tmp/gopher-$w.png
	magick "$src/gopher-says.png" -filter Lanczos -resize "${w}x" -strip "$png"
	avifenc -q 60 -s 4 "$png" "$out/gopher-$w.avif" >/dev/null
	cwebp -quiet -q 80 -m 6 -alpha_q 90 "$png" -o "$out/gopher-$w.webp"
	magick "$png" -define png:compression-level=9 "$out/gopher-$w.png"
done

# The cards: the site's own 3:2 art, from art/ in this repository,
# including the 404 page's.
for name in path-learning path-shipping path-leading path-complete not-found; do
	in=$site/art/$name.png
	[ -f "$in" ] || { echo "art: no $in" >&2; exit 1; }
	for w in 480 960; do
		png=$tmp/$name-$w.png
		magick "$in" -alpha off -colorspace sRGB -filter Lanczos -resize "${w}x" -strip "$png"
		avifenc -q 55 -s 4 "$png" "$out/$name-$w.avif" >/dev/null
		cwebp -quiet -q 76 -m 6 "$png" -o "$out/$name-$w.webp"
		magick "$png" -quality 78 -sampling-factor 4:2:0 -interlace JPEG "$out/$name-$w.jpg"
	done
done

# The site's own 4:5 plates (sections, guides), from art/ in this
# repository: every art/*.png that isn't a card, at the openers'
# sizes. Their names and alt text are in cmd/build/art.go.
for in in "$site"/art/*.png; do
	name=$(basename "$in" .png)
	case $name in path-* | not-found) continue ;; esac
	for w in 480 720 1080; do
		png=$tmp/$name-$w.png
		magick "$in" -alpha off -colorspace sRGB -filter Lanczos -resize "${w}x" -strip "$png"
		avifenc -q 55 -s 4 "$png" "$out/$name-$w.avif" >/dev/null
		cwebp -quiet -q 76 -m 6 "$png" -o "$out/$name-$w.webp"
		magick "$png" -quality 78 -sampling-factor 4:2:0 -interlace JPEG "$out/$name-$w.jpg"
	done
done

echo "art: $(ls "$out" | wc -l | tr -d ' ') files in $out"
