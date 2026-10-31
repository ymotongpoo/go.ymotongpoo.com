#!/bin/sh
# Regenerates the site's own files and deploys public/ as the production
# deployment of the Cloudflare Pages project go-ymotongpoo-com.
set -eu
cd "$(dirname "$0")"
go run .
wrangler pages deploy public --project-name go-ymotongpoo-com --branch main
