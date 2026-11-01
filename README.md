# Muddy's Music Cafe website

Website for Muddy's Music Cafe, a Second Life music venue (est. 2010). The
public site keeps the design from the proof of concept; the content (events,
DJs, hosts, partners, site text) now lives in a database and is managed through
a simple admin area, so changing content never needs a redeploy.

## Problem

The proof of concept was static HTML. Every event, DJ or host change meant
editing files and re-uploading the site. The owner's team needs to manage this
content themselves, with photos and posters, without touching code.

## Tech stack

Follows the same shape as the owner's other self-built apps (Stocktake,
SteadySum):

- **Backend**: Go (stdlib `net/http`), SQLite in WAL mode (`modernc.org/sqlite`, pure Go, no CGO)
- **Public pages**: the existing HTML/CSS/JS design, rendered server-side with `html/template` from the database (so search engines and link previews see real content)
- **Admin**: React + TypeScript (Vite) at `/admin`
- **Packaging**: one Docker image (multi-stage `node` -> `go build CGO_ENABLED=0` -> `alpine`); the Go binary `go:embed`s the built admin and the public templates/assets
- **Uploads**: photos and posters are validated, re-encoded and stored under the data directory (`<data-dir>/uploads`), served by the app
- **Access**: no in-app login. `/admin` and the write API are expected to sit behind an access-gating layer (Cloudflare Access, VPN, reverse proxy auth). Public pages need no login.
- **Config**: flags only: `-addr`, `-metrics-addr`, `-data-dir`

The repo contains nothing specific to where it is hosted. Deployment lives
elsewhere.

## Security and credentials

This repository is public. Rules for keeping it safe:

- **No secrets in Git, ever.** No passwords, API keys, tokens, private keys,
  `.env` files, database files or uploaded content. `.gitignore` blocks the
  common ones; GitHub secret scanning and push protection are enabled on the
  repo.
- **The app needs no secrets by default.** Access control is handled by the
  layer in front of it (see Access above), and the app never stores
  credentials or passwords. If a future feature needs a secret, it is read at
  runtime from an environment variable or a mounted secret file, never from
  the repo or the image, and documented here by name only (never by value).
- **No environment details.** Hostnames, IP addresses, network layout,
  deployment tooling and the access-gate configuration stay out of this repo.
- **Admin defense in depth.** The app should not rely solely on the outer
  gate: write endpoints must verify the identity assertion supplied by the
  access layer (and refuse requests without it), and the admin must be safe to
  run exposed only through that gate.
- **Uploads are untrusted.** Validate type by content (not filename), enforce
  size limits, re-encode images (stripping metadata), generate server-side
  filenames, and serve uploads with `X-Content-Type-Options: nosniff`.
- **Standard web hardening.** Parameterized SQL only, `html/template`
  auto-escaping, CSRF protection on state-changing admin requests, a strict
  Content-Security-Policy, and request size limits.
- **Dependencies** are pinned and kept current (Dependabot or Renovate);
  the container runs as a non-root user.
- Report vulnerabilities privately via GitHub's "Report a vulnerability"
  (see `SECURITY.md`).

## Package layout

```
server/
  cmd/server/            main
  internal/api/          JSON API for the admin (events, djs, hosts, partners, settings, uploads)
  internal/site/         public pages: html/template templates, static css/js/images, uploads serving
  internal/storage/      SQLite + numbered migrations
  internal/uploads/      image validation / resize / storage
  internal/webui/        embedded admin build
  internal/seed/         one-time import of the POC content
web/                     admin SPA (Vite)
poc/                     the original static proof of concept, kept for reference
```

## Data model (SQLite)

- `events`: title, summary, badge text, start/end, poster image, link + label, `featured`, `published`
- `staff_members` (`kind` = dj | host): name, SL name, bio paragraphs, photo, schedule lines, `sort_order`, `active`
- `partners`: name, `sort_order`
- `settings`: radio stream URL, SL group/Facebook/Flickr/YouTube links, hiring text, "on the road" block, club SLurl
- `images`: uploaded files (path, content type, size, dimensions)

Events past their end date drop off the main list automatically; nothing needs
deleting by hand.

## Admin (for the owner and managers)

Plain forms: add/edit/hide an event, upload a poster, add or retire a DJ/host,
reorder, edit partners, change the stream URL and links. Every save is visible
on the public site immediately.

### Usability requirements

The admin must be drop-dead easy to use. These are hard requirements, not
polish:

- **No markup of any kind in input fields.** No Markdown, HTML, or special
  syntax. Fields are plain text; the site applies all styling and formatting.
  Bios and descriptions are typed as ordinary paragraphs (a blank line starts a
  new paragraph, and the editor never asks the user to know that).
- **Plain-language labels and help text.** Say "Event name", not "title slug";
  no jargon, IDs, or technical error messages.
- **Pick, don't type, wherever possible.** Date and time pickers, photo
  upload by drag-and-drop or a big "Choose photo" button, toggles instead of
  status codes, up/down or drag handles to reorder.
- **One obvious action per screen.** Large buttons, clear "Save" and
  "Cancel", and a visible confirmation after saving ("Saved. It's now on the
  website.").
- **A live preview** of how an item will look on the site, where practical.
- **Hard to break things.** Confirm before deleting, prefer "hide" over
  "delete", validate and explain problems in plain words next to the field,
  and resize/optimize uploaded images automatically.
- **Works on a phone and a desktop**, with large touch targets and readable
  text sizes.

## Branding

All colors and fonts are CSS custom properties in the stylesheet (`--ink`,
`--amber`, `--blue`, `--red`, ...). The site is dark-only on purpose. Fonts:
Alfa Slab One (headings), Yellowtail (script accents), Inter (body). See
`poc/CLAUDE.md` for the original design decisions.

## Status / phase checklist

- [x] **Phase 0: Repo + docs.** POC imported under `poc/`, this README.
- [x] **Phase 1: Scaffold.** `server/`, `web/`, `Dockerfile`, `Makefile`, `/healthz`, `/metrics`.
- [x] **Phase 2: Storage.** SQLite schema + migrations, store packages with tests.
- [x] **Phase 3: Seed import.** `server -seed` loads the POC's 22 DJs, 28 hosts, 84 partners, events and settings into an empty database (refuses if content exists).
- [x] **Phase 4: Public pages.** `/`, `/djs`, `/hosts` rendered from the database; static assets and uploads served with caching; visual check against the POC done.
- [ ] **Phase 5: Admin API + uploads.** CRUD, image validation/resizing, tests.
- [ ] **Phase 6: Admin UI.** Events, DJs, hosts, partners, settings.
- [ ] **Phase 7: Containerize.** Verify the image builds and runs with a mounted data dir.
- [ ] **Phase 8: Deploy.** Done outside this repo, with `/admin` behind an access gate.
- [ ] **Phase 9: Handover.** Walk the owner through editing; backup/restore notes.

## Open items carried over from the POC

1. No SLurl for the club itself yet (becomes a setting; add a "Teleport to Muddy's" button once provided).
2. The radio stream is `http`; it will not play on an `https` site until Digistream provides an `https` URL (a setting).
3. Not built yet: "Muddy's in the Media" page, per-partner pages (~100 on the old site), Cloud Nine page.
