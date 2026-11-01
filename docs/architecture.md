# Architecture

## Stack

- **Backend**: Go (stdlib `net/http`), SQLite in WAL mode (`modernc.org/sqlite`, pure Go, no CGO)
- **Public pages**: the original HTML/CSS/JS design, rendered server-side with `html/template` from the database, so search engines and link previews see real content
- **Admin**: React + TypeScript (Vite) at `/admin`
- **Packaging**: one Docker image (multi-stage `node` -> `go build CGO_ENABLED=0` -> `alpine`); the Go binary `go:embed`s the built admin, the public templates/assets and the timezone database
- **Uploads**: photos and posters are validated, re-encoded and stored under `<data-dir>/uploads`, served by the app at `/uploads/`
- **Access**: no in-app login. `/admin` and the write API sit behind an access-gating layer (Cloudflare Access), and the app verifies the signed identity assertion itself. Public pages need no login.

The repo contains nothing specific to where it is hosted. Deployment lives elsewhere.

## Hostnames

With `-public-host` and `-admin-host` set (they must be set together):

- the admin app and API (`/admin`, `/api/admin`) are served only on the admin hostname, plus `/uploads/` so the admin can show pictures
- the public site is served only on the public hostname and any `-public-host-alias` (e.g. `www`)
- `/healthz` is served on both; any other hostname gets 404

Unset, every route is served on every hostname (local development).

## Configuration

Flags only, no environment variables and no secrets:

| Flag | Purpose |
|---|---|
| `-addr` | listen address (default `:8080`) |
| `-metrics-addr` | internal-only `/metrics` listener (default `:9090`) |
| `-data-dir` | SQLite database and uploads (default `./data`) |
| `-public-host`, `-public-host-alias`, `-admin-host` | hostname split (see above) |
| `-site-url` | public origin, makes link-preview image URLs absolute |
| `-access-team-domain`, `-access-aud` | access gate identifiers; with both set the admin API is enabled, otherwise it stays locked |
| `-seed` | import the starter content into an empty database, then exit |
| `-dev-no-auth` | development only: skips admin sign-in; refuses non-loopback addresses |

## Package layout

```
server/
  cmd/server/            main
  internal/admin/        admin JSON API (events, djs, hosts, partners, settings, uploads) and auth
  internal/api/          routing, host split, middleware, metrics
  internal/site/         public pages: html/template templates, static css/js/images, uploads serving
  internal/storage/      SQLite + numbered migrations
  internal/uploads/      image validation / resize / storage
  internal/webui/        embedded admin build
  internal/seed/         one-time import of the starter content
web/                     admin SPA (Vite)
poc/                     the original static proof of concept, kept for reference
```

## Data model (SQLite)

- `events`: title, summary, badge text, start/end, poster image, link + label, `featured`, `published`
- `staff_members` (`kind` = dj | host): name, SL name, bio paragraphs, photo, schedule lines, `sort_order`, `active`
- `partners`: name, `sort_order`
- `settings`: radio stream URL, SL group/Facebook/Flickr/YouTube links, hiring text, "on the road" block, club SLurl
- `images`: uploaded files (path, content type, size, dimensions)

Events past their end date drop off the main list automatically; nothing needs deleting by hand.

## Branding

All colors and fonts are CSS custom properties in the stylesheet (`--ink`, `--amber`, `--blue`, `--red`, ...). The site is dark-only on purpose. Fonts: Alfa Slab One (headings), Yellowtail (script accents), Inter (body). See `poc/CLAUDE.md` for the original design decisions.
