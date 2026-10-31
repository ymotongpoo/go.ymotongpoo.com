# go.ymotongpoo.com

The static site served at <https://go.ymotongpoo.com> by Cloudflare Pages:
the vanity import paths of the Go modules that Yoshi Yamaguchi publishes,
and their documentation.

A request for `go.ymotongpoo.com/<module>?go-get=1`, or for any path below
it, answers with the `go-import` and `go-source` meta tags that let the go
command resolve the module to its repository on GitHub (see `go help
importpath`). A module's documentation is served at
`/<module>/guide/`, with a Markdown version of every page and an `llms.txt`
index for AI agents next to it.

## Layout

- `modules.go` lists the modules: import path, repository, a summary, a
  description and extra links. This is the only file to edit for a new
  module.
- `main.go` and `templates/` generate, into `public/`, the module pages
  (`goimport/<module>/index.html`, with a copy at `<module>/index.html`),
  `index.html`, `404.html`, the domain's `llms.txt`, and the `_redirects`
  and `_headers` rules of Cloudflare Pages.
- `public/` is the deployed directory and is committed, so that the
  repository holds the whole site. `public/guide/<module>/` holds a
  module's documentation, rendered by that module's own repository and
  copied here; the generator does not touch it, but reads it to write the
  rules (language directories, `llms.txt`, `llms-full.txt`).
- `deploy.sh` regenerates and deploys.

## Adding a module

1. Add an entry to `modules.go`.
2. If the module publishes documentation, render it in its repository and
   copy the output to `public/guide/<module>/`, with an `index.html` at the
   top; language versions go in subdirectories with their own `index.html`.
3. Run `go run .`, review `git diff public`, commit, and run `./deploy.sh`.

For pbt the documentation step is

```sh
(cd ../pbt/site && go run . -version vX.Y.Z -analytics G-EC4QPYV95Y)
rm -rf public/guide/pbt && cp -R ../pbt/site/public public/guide/pbt
```

## Deploying

The Cloudflare Pages project is `go-ymotongpoo-com`, a direct-upload
project whose production branch is `main`, with the custom domain
`go.ymotongpoo.com`. `./deploy.sh` runs the generator and

```sh
wrangler pages deploy public --project-name go-ymotongpoo-com --branch main
```

`wrangler pages dev public` serves the site locally with the same rules
and reports the rules it rejects.

## How the rules work

`_redirects` rules apply in order, the first match wins, and they take
precedence over static assets. For each module the generator writes the
exact paths first (`/<module>/guide` → `/<module>/guide/` with 301, the
`llms.txt` files, and `/<module>` → `/goimport/<module>/` with 200), then
the splats: `/<module>/guide/*` → `/guide/<module>/:splat` and the
catch-all `/<module>/*` → `/goimport/<module>/`, which is what makes
`/<module>/<subpackage>?go-get=1` answer with the module's tags.

Two pitfalls shape the layout. The rule validator drops a rule whose
destination names an `index.html` or `.html` file as an infinite loop, so
destinations are clean URLs or non-HTML files, and the asset server
answers an `.html` destination with a 308 to the clean URL anyway. And a
rule cannot rewrite `/<module>/*` to a file under `/<module>`, so the
module page is stored under `/goimport/` and the documentation under
`/guide/`, outside the pattern. Without `404.html`, Pages would serve the
root `index.html` with status 200 for every unknown path.

`_headers` rules match the request path, and a request that matches
several rules gets the headers of all of them. The Markdown pages and the
`llms.txt` files are served as `text/markdown`; every documentation page
carries a `Link` header with `rel="describedby"` pointing at the module's
`llms.txt`, and a `Content-Signal` of `search=yes, ai-input=yes`. The
training signal is left unstated on purpose.

## License

Apache License 2.0; see [LICENSE.md](LICENSE.md).
