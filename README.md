# go.ymotongpoo.com

This orphan branch holds the static site served at `https://go.ymotongpoo.com`
by Cloudflare Pages. It has no shared history with `main`.

The site serves the `go-import` meta tags, which let `go get` resolve
`go.ymotongpoo.com/pbt` (and its subpackages) to the repository on GitHub
(see `go help importpath` for the protocol), and the user guide of pbt at
`/pbt/guide/`.

## Files

- `_redirects`: serves the guide's files from `/guide/pbt` at `/pbt/guide`,
  and rewrites every other path under `/pbt` to the module page with status
  200, so `/pbt/fsm?go-get=1` and `/pbt?go-get=1` return the same tags.
  Rules take precedence over static assets and the first match wins, so the
  guide's rules come first. Destinations must be clean URLs (`/goimport/pbt/`,
  not `/goimport/pbt/index.html`): wrangler drops a rule whose destination
  names an `index.html` or `.html` file as an infinite loop, and the asset
  server answers such a destination with a 308 to the clean URL.
- `goimport/pbt/index.html`: the module page with the `go-import` and
  `go-source` meta tags, and links to the guide, pkg.go.dev and GitHub for
  human visitors. It lives outside `/pbt` because a rule cannot rewrite
  `/pbt/*` to a file under `/pbt`. `pbt/index.html` is a copy, kept as the
  static fallback for `/pbt/`; change both together.
- `guide/pbt/`: the rendered guide. Do not edit it here: the sources are
  `docs/guide` on `main`, and `cd site && go run . -version vX.Y.Z
  -analytics G-EC4QPYV95Y` renders them into `site/public`, which is copied
  to `guide/pbt`. The `-analytics` flag is the Google Analytics measurement
  ID of go.ymotongpoo.com; without it the pages carry no tag. Pages are served
  at clean URLs (`/pbt/guide/01-generators`), and the links inside them
  are relative and extensionless.
- `404.html`: served with status 404 for unknown paths. Without it, Pages
  would serve the root `index.html` with status 200 for every unknown path.
- `index.html`: a plain index page for the root. It carries no `go-import` tag
  on purpose, so `go.ymotongpoo.com` itself is never mistaken for a module.

## Deploying

Either connect this repository to a Cloudflare Pages project with the
production branch set to `pages`, no build command, and `/` as the output
directory, or deploy a clean export of this branch (a worktree's `.git`
file would be uploaded as an asset):

```sh
rm -rf /tmp/pbt-deploy && mkdir /tmp/pbt-deploy && git archive pages | tar -x -C /tmp/pbt-deploy
wrangler pages deploy /tmp/pbt-deploy --project-name go-ymotongpoo-com --branch pages
```

`wrangler pages dev /tmp/pbt-deploy` serves the site locally with the same
redirect rules and reports the rules it rejects.

If the project is connected through Git, disable preview deployments for
other branches; `main` holds Go source and must not be published as a site.

## Verifying

```sh
curl -sL 'https://go.ymotongpoo.com/pbt?go-get=1' | grep go-import
curl -sL 'https://go.ymotongpoo.com/pbt/fsm?go-get=1' | grep go-import
curl -sI 'https://go.ymotongpoo.com/pbt/guide/' | head -1
curl -sI 'https://go.ymotongpoo.com/pbt/guide/ja/01-generators' | head -1
curl -sI 'https://go.ymotongpoo.com/pbt/guide/missing' | head -1   # 404
```
