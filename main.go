// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

// Command go.ymotongpoo.com writes the static site of go.ymotongpoo.com into
// public/: a page per module with the go-import and go-source meta tags
// that let the go command resolve go.ymotongpoo.com/<module> to its
// repository, the index page, the 404 page, the llms.txt index of the
// domain, and the _redirects and _headers rules of Cloudflare Pages.
//
// The documentation of a module lives under public/guide/<module>/ and is
// not written here: the module's own repository renders it and copies it
// in. This command looks at what is there (the language directories, the
// llms.txt files) and writes the rules that serve it at /<module>/guide/.
//
// Run `go run .` after editing modules.go or copying documentation in, then
// deploy public/ with wrangler; see README.md.
package main

import (
	"embed"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed templates/*.html
var templateFS embed.FS

const (
	host   = "go.ymotongpoo.com"
	outDir = "public"
)

type link struct{ Text, URL string }

// module is one Go module served under the domain.
type module struct {
	Path        string // the import path below the host, such as "pbt"
	Repo        string // the repository on GitHub
	Summary     string // one line, for the index pages
	Description string // a paragraph, for the module page
	Docs        bool   // whether public/guide/<Path> holds the documentation
	Links       []link // further links on the module page
}

// Import returns the full import path of the module.
func (m module) Import() string { return host + "/" + m.Path }

type lang struct{ Code, Label string }

var langLabels = map[string]string{"ja": "日本語版"}

// docs describes what the generator finds under public/guide/<module>.
type docs struct {
	Langs    []lang   // the language directories, each with an index.html
	LlmsTxt  bool     // guide/llms.txt exists
	LlmsFull bool     // guide/llms-full.txt exists
	FullIn   []string // the languages that have their own llms-full.txt
}

// view is what the templates receive for a module.
type view struct {
	module
	docs
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "go.ymotongpoo.com:", err)
		os.Exit(1)
	}
}

func run() error {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return err
	}
	var views []view
	for _, m := range modules {
		d, err := inspectDocs(m)
		if err != nil {
			return err
		}
		views = append(views, view{m, d})
	}
	for _, v := range views {
		var b strings.Builder
		if err := tmpl.ExecuteTemplate(&b, "module.html", v); err != nil {
			return err
		}
		// The page lives outside /<module>, where a rule can rewrite every
		// path of the module to it, and a copy under /<module> is the
		// static fallback.
		for _, dst := range []string{filepath.Join("goimport", v.Path, "index.html"), filepath.Join(v.Path, "index.html")} {
			if err := write(dst, b.String()); err != nil {
				return err
			}
		}
	}
	var b strings.Builder
	if err := tmpl.ExecuteTemplate(&b, "index.html", views); err != nil {
		return err
	}
	if err := write("index.html", b.String()); err != nil {
		return err
	}
	b.Reset()
	if err := tmpl.ExecuteTemplate(&b, "404.html", views); err != nil {
		return err
	}
	if err := write("404.html", b.String()); err != nil {
		return err
	}
	if err := write("llms.txt", llmsTxt(views)); err != nil {
		return err
	}
	if err := write("_redirects", redirects(views)); err != nil {
		return err
	}
	return write("_headers", headers(views))
}

func write(name, content string) error {
	dst := filepath.Join(outDir, name)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, []byte(content), 0o644)
}

func exists(parts ...string) bool {
	_, err := os.Stat(filepath.Join(parts...))
	return err == nil
}

// inspectDocs looks under public/guide/<module> for the language
// directories and the llms files.
func inspectDocs(m module) (docs, error) {
	var d docs
	if !m.Docs {
		return d, nil
	}
	dir := filepath.Join(outDir, "guide", m.Path)
	if !exists(dir, "index.html") {
		return d, fmt.Errorf("%s: Docs is set but %s has no index.html", m.Path, dir)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return d, err
	}
	for _, e := range entries {
		if !e.IsDir() || !exists(dir, e.Name(), "index.html") {
			continue
		}
		label := langLabels[e.Name()]
		if label == "" {
			label = e.Name()
		}
		d.Langs = append(d.Langs, lang{Code: e.Name(), Label: label})
		if exists(dir, e.Name(), "llms-full.txt") {
			d.FullIn = append(d.FullIn, e.Name())
		}
	}
	sort.Slice(d.Langs, func(i, j int) bool { return d.Langs[i].Code < d.Langs[j].Code })
	sort.Strings(d.FullIn)
	d.LlmsTxt = exists(dir, "llms.txt")
	d.LlmsFull = exists(dir, "llms-full.txt")
	return d, nil
}

// llmsTxt is the index of the domain for AI agents, in the format of
// https://llmstxt.org/. Each module with documentation has its own, more
// specific, index under its path.
func llmsTxt(views []view) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", host)
	fmt.Fprintf(&b, "> Go modules published by Yoshi Yamaguchi under the import path prefix %s, with their documentation. A module that has documentation has its own llms.txt under its path, which is the more specific index for the pages there.\n\n", host)
	b.WriteString("## Modules\n\n")
	for _, v := range views {
		if v.LlmsTxt {
			fmt.Fprintf(&b, "- [%s](https://%s/%s/llms.txt): %s\n", v.Import(), host, v.Path, v.Summary)
		} else {
			fmt.Fprintf(&b, "- [%s](https://pkg.go.dev/%s): %s\n", v.Import(), v.Import(), v.Summary)
		}
	}
	return b.String()
}

// redirects writes the _redirects file. Rules apply in order, the first
// match wins, and they take precedence over static assets, so the exact
// paths come first and the catch-all of each module last. A destination
// that names an index.html or .html file is rejected as a loop by the rule
// validator, so destinations are clean URLs or non-HTML files, and the
// documentation is stored under /guide/<module> rather than under
// /<module>, where the catch-all would shadow it.
func redirects(views []view) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s: vanity import paths for Go modules, plus their documentation.\n", host)
	b.WriteString("# Generated by `go run .`; edit modules.go instead. Rules apply in order, the\n")
	b.WriteString("# first match wins, and they take precedence over static assets: exact paths\n")
	b.WriteString("# first, then the splats. Destinations never name an .html file, which the\n")
	b.WriteString("# validator rejects as a loop. The module page is stored under /goimport,\n")
	b.WriteString("# outside the /<module> pattern that rewrites every path of the module to it,\n")
	b.WriteString("# so that /<module>/<subpackage>?go-get=1 answers with the same tags.\n")
	rule := func(from, to, status string) { fmt.Fprintf(&b, "%-28s %-30s %s\n", from, to, status) }
	for _, v := range views {
		if v.Docs {
			rule("/"+v.Path+"/guide", "/"+v.Path+"/guide/", "301")
			for _, l := range v.Langs {
				rule("/"+v.Path+"/guide/"+l.Code, "/"+v.Path+"/guide/"+l.Code+"/", "301")
			}
			if v.LlmsTxt {
				rule("/"+v.Path+"/llms.txt", "/guide/"+v.Path+"/llms.txt", "200")
			}
			if v.LlmsFull {
				rule("/"+v.Path+"/llms-full.txt", "/guide/"+v.Path+"/llms-full.txt", "200")
			}
		}
		rule("/"+v.Path, "/goimport/"+v.Path+"/", "200")
	}
	for _, v := range views {
		if v.Docs {
			rule("/"+v.Path+"/guide/*", "/guide/"+v.Path+"/:splat", "200")
		}
		rule("/"+v.Path+"/*", "/goimport/"+v.Path+"/", "200")
	}
	return b.String()
}

// headers writes the _headers file: the Markdown pages and the llms files
// are served as text/markdown, every documentation page points at the
// llms.txt that describes it, and Content-Signal allows search and use as
// AI input; the training signal is left unstated. Rules match the request
// path, and a request that matches several rules gets the headers of all.
func headers(views []view) string {
	const markdown = "  Content-Type: text/markdown; charset=utf-8\n"
	var b strings.Builder
	b.WriteString("# Responses for AI agents. Generated by `go run .`; see main.go. Rules match\n")
	b.WriteString("# the request path, and a request that matches several rules gets the\n")
	b.WriteString("# headers of all of them.\n")
	b.WriteString("/*.md\n" + markdown)
	b.WriteString("/llms.txt\n" + markdown)
	for _, v := range views {
		if !v.Docs {
			continue
		}
		if v.LlmsTxt {
			b.WriteString("/" + v.Path + "/llms.txt\n" + markdown)
			b.WriteString("/" + v.Path + "/guide/llms.txt\n" + markdown)
		}
		if v.LlmsFull {
			b.WriteString("/" + v.Path + "/llms-full.txt\n" + markdown)
			b.WriteString("/" + v.Path + "/guide/llms-full.txt\n" + markdown)
		}
		for _, code := range v.FullIn {
			b.WriteString("/" + v.Path + "/guide/" + code + "/llms-full.txt\n" + markdown)
		}
		b.WriteString("/" + v.Path + "/guide/*\n")
		if v.LlmsTxt {
			fmt.Fprintf(&b, "  Link: </%s/llms.txt>; rel=\"describedby\"; type=\"text/markdown\"\n", v.Path)
		}
		b.WriteString("  Content-Signal: search=yes, ai-input=yes\n")
	}
	return b.String()
}
