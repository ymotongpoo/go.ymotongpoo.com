// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownCanonicalHeaders(t *testing.T) {
	t.Chdir(t.TempDir())
	for _, name := range []string{"index", "chapter", "ja/index", "ja/chapter"} {
		for _, ext := range []string{".html", ".md"} {
			if err := write("guide/example/"+name+ext, "content"); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := write("guide/example/source-only.md", "no HTML version"); err != nil {
		t.Fatal(err)
	}
	m := module{Path: "example", Docs: true}
	d, err := inspectDocs(m)
	if err != nil {
		t.Fatal(err)
	}
	h := headers([]view{{module: m, docs: d}})
	for _, name := range []string{"index", "chapter", "ja/index", "ja/chapter"} {
		page := name
		if filepath.Base(name) == "index" {
			page = strings.TrimSuffix(name, "index")
		}
		for _, prefix := range []string{"/example/guide/", "/guide/example/"} {
			want := prefix + name + ".md\n  Link: <https://" + host + "/example/guide/" + page + ">; rel=\"canonical\"\n"
			if !strings.Contains(h, want) {
				t.Errorf("missing canonical rule %q", want)
			}
		}
	}
	if strings.Contains(h, "source-only.md") {
		t.Fatal("canonical added without an HTML version")
	}
}

func TestPublishedMarkdownCanonicalTargets(t *testing.T) {
	var views []view
	for _, m := range modules {
		d, err := inspectDocs(m)
		if err != nil {
			t.Fatal(err)
		}
		views = append(views, view{m, d})
		for _, md := range d.Markdown {
			page := strings.TrimSuffix(md, ".md")
			if filepath.Base(md) == "index.md" {
				page = strings.TrimSuffix(md, "index.md")
			}
			body, err := os.ReadFile(filepath.Join(outDir, "guide", m.Path, strings.TrimSuffix(md, ".md")+".html"))
			if err != nil {
				t.Fatal(err)
			}
			want := `rel="canonical" href="https://` + host + "/" + m.Path + "/guide/" + page + `"`
			if !strings.Contains(string(body), want) {
				t.Errorf("%s: HTML canonical differs from the header target %s", md, want)
			}
		}
	}
	rules := 0
	for _, line := range strings.Split(headers(views), "\n") {
		if strings.HasPrefix(line, "/") {
			rules++
		}
	}
	if rules > 100 {
		t.Fatalf("%d rules exceed the Pages limit of 100", rules)
	}
}

func TestMovedPagesRedirect(t *testing.T) {
	v := view{module: module{Path: "pbt", Docs: true, Moved: []move{{From: "10-reference", To: "11-reference"}}},
		docs: docs{Langs: []lang{{Code: "ja"}}}}
	got := redirects([]view{v})
	for _, want := range []string{
		"/pbt/guide/10-reference", "/pbt/guide/11-reference",
		"/pbt/guide/ja/10-reference.md", "/pbt/guide/ja/11-reference.md",
		"/pbt/guide/10-reference.html",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in\n%s", want, got)
		}
	}
	if strings.Index(got, "/pbt/guide/10-reference ") > strings.Index(got, "/pbt/guide/* ") {
		t.Errorf("the moved page must come before the splat:\n%s", got)
	}
}
