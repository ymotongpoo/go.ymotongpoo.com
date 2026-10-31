# go.ymotongpoo.com

This orphan branch holds the static site served at `https://go.ymotongpoo.com`
by Cloudflare Pages. It has no shared history with `main`.

The site exists only to serve `go-import` meta tags, which let `go get`
resolve `go.ymotongpoo.com/pbt` (and its subpackages) to the repository on
GitHub. See `go help importpath` for the protocol.

## Files

- `_redirects`: rewrites every path under `/pbt` to `/pbt/index.html` with
  status 200, so `/pbt/fsm?go-get=1` and `/pbt?go-get=1` return the same tags.
- `pbt/index.html`: the `go-import` and `go-source` meta tags for the module,
  plus a redirect to pkg.go.dev for human visitors.
- `index.html`: a plain index page for the root. It carries no `go-import` tag
  on purpose, so `go.ymotongpoo.com` itself is never mistaken for a module.

## Deploying

Either connect this repository to a Cloudflare Pages project with the
production branch set to `pages`, no build command, and `/` as the output
directory, or deploy directly from a checkout of this branch:

```sh
wrangler pages deploy . --project-name go-ymotongpoo-com --branch pages
```

If the project is connected through Git, disable preview deployments for
other branches; `main` holds Go source and must not be published as a site.

## Verifying

```sh
curl -sL 'https://go.ymotongpoo.com/pbt?go-get=1' | grep go-import
curl -sL 'https://go.ymotongpoo.com/pbt/fsm?go-get=1' | grep go-import
```
