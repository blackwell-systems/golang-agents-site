# golang-agents.com

The website for *Building AI Agents in Go: From the agent loop to production*, by Dayna Blackwell.

One static page: plain HTML and CSS, with one small inline script (the store link), no build step, no analytics, and no external requests. The fonts, images, and icons are all served from this repository.

| Path | What |
| --- | --- |
| `index.html` | The page |
| `style.css` | The cover's palette and type |
| `fonts/` | Cinzel, Inter, and Space Grotesk (WOFF2), with their SIL Open Font Licenses |
| `img/` | The cover at 360, 540, 720, and 1080 px wide, in AVIF and WebP, with a 720 px JPEG fallback |
| `sample/` | The free sample PDFs (`building-ai-agents-in-go-sample-light.pdf` and `-dark.pdf`), copied in before launch |
| `og.jpg` | The 1200 × 630 social card (Open Graph and Twitter) |
| `favicon.svg`, `favicon.ico`, `apple-touch-icon.png` | The icons: the cover's amber ring on night navy |
| `CNAME` | The custom domain for GitHub Pages |
| `.nojekyll` | Tells GitHub Pages to serve the files as they are, without Jekyll |

## Preview locally

```
python3 -m http.server 8000
```

Run it in this directory, then open http://localhost:8000/.

## Updating the images

The images come from the book's cover. When the cover changes, regenerate them from the book repository, which must sit next to this one (`../go-agents-book`):

```
cd ../go-agents-book
make site-assets
```

That runs `tools/site-assets.sh`, which rebuilds the cover if needed and writes `img/`, `og.jpg`, `favicon.ico`, and `apple-touch-icon.png` here. It needs ImageMagick 7, `avifenc`, and `cwebp`. The page, the CSS, the fonts, and `favicon.svg` are edited here directly.

## Publishing on GitHub Pages

The site is served from the root of the `main` branch.

1. Create the repository on GitHub. It must be public for GitHub Pages on a free plan. Then add it as a remote and push `main`.
2. In the repository, open **Settings > Pages**. Under **Build and deployment**, set **Source** to "Deploy from a branch", the branch to `main`, and the folder to `/ (root)`. Save.
3. Under **Custom domain**, enter `golang-agents.com` and save. The `CNAME` file already holds it.
4. At the domain registrar, add the DNS records below.
5. Once the DNS check passes and GitHub has issued the certificate (it can take up to an hour or so), tick **Enforce HTTPS** on the same page.

It's worth also verifying the domain for the GitHub account or organization (**Settings > Pages > Add a domain** in the account's settings). That stops anyone else from pointing a GitHub Pages site at it.

### DNS records

For the apex domain, `golang-agents.com`:

| Type | Name | Value |
| --- | --- | --- |
| A | `@` | `185.199.108.153` |
| A | `@` | `185.199.109.153` |
| A | `@` | `185.199.110.153` |
| A | `@` | `185.199.111.153` |
| AAAA | `@` | `2606:50c0:8000::153` |
| AAAA | `@` | `2606:50c0:8001::153` |
| AAAA | `@` | `2606:50c0:8002::153` |
| AAAA | `@` | `2606:50c0:8003::153` |

The AAAA records (IPv6) are optional but recommended.

For `www.golang-agents.com`:

| Type | Name | Value |
| --- | --- | --- |
| CNAME | `www` | `<owner>.github.io` |

Replace `<owner>` with the GitHub user or organization that owns this repository (for example `blackwell-systems.github.io`). With both in place, GitHub redirects `www` to the apex. Remove any other A, AAAA, or ALIAS records on the apex, such as a registrar's parking page.

These are GitHub's published addresses as of October 2026; check them against GitHub's "Managing a custom domain for your GitHub Pages site" page if anything fails.

## Before launch

- **Store link:** set `STORE_URL` in the inline script at the bottom of `index.html` to the Gumroad product URL. Every buy button (header, hero, buy box) has the class `buy-link` and picks it up. While it is empty they jump to `#buy`.
- **Sample:** copy `building-ai-agents-in-go-sample-light.pdf` and `building-ai-agents-in-go-sample-dark.pdf` into `sample/`. The sample holds the introduction, chapters 1 and 4, and chapter 15's section "One crash, step by step".
- **Price and updates:** $29, one price, permanent; buyers get every update of this edition free, by email through the store. There is no mailing list signup.
- **Author photo:** optional; the About section has none.

## Rules for changes

- This repository is public. Keep anything private out of it: no keys, no unpublished chapter text, no planning notes.
- Every figure on the page comes from the book's chapter text. The four results under "What the measurements showed" come from chapters 10, 9, 13, and 15.
- Never use the Go logo; "Go" is plain text. The footer keeps the non-affiliation line and the gopher credit.
- Keep the page free of external requests.
