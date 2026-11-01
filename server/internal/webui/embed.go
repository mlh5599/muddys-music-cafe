// Package webui embeds the built admin app so the Go binary can serve it
// directly in production. dist/ is written by `npm run build` in web/; a
// placeholder index.html is committed so a fresh clone can `go build` before
// the frontend has ever been built.
package webui

import "embed"

//go:embed all:dist
var DistFS embed.FS
