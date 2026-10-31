// SPDX-FileCopyrightText: 2026 Yoshi Yamaguchi <ymotongpoo@gmail.com>
// SPDX-License-Identifier: Apache-2.0

package main

// modules lists the Go modules served under go.ymotongpoo.com. Add an entry
// here, run `go run .` and deploy; the module page with its go-import tags,
// the index, the llms.txt index and the Cloudflare rules follow. A module
// that publishes documentation copies it into public/guide/<Path>/ and sets
// Docs to true.
var modules = []module{
	{
		Path:    "pbt",
		Repo:    "https://github.com/ymotongpoo/pbt",
		Summary: "property-based testing for Go with the feature set of PropEr",
		Description: "Property-based testing for Go with the feature set of PropEr: typed generators with automatic shrinking, " +
			"distributions you control and statistics to check them, stateful and parallel command testing, " +
			"state machines, targeted search and existential properties.",
		Docs: true,
		Links: []link{
			{Text: "Ports of every chapter of PropEr Testing, and practical examples", URL: "https://github.com/ymotongpoo/pbt/tree/main/examples"},
			{Text: "Changelog", URL: "https://github.com/ymotongpoo/pbt/blob/main/CHANGELOG.md"},
		},
	},
}
