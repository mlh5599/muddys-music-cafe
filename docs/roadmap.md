# Status

- [x] Repo and docs; proof of concept kept under `poc/`
- [x] Scaffold: server, admin, Dockerfile, Makefile, `/healthz`, `/metrics`
- [x] Storage: SQLite schema, migrations, store packages with tests
- [x] Seed import of the starter content
- [x] Public pages rendered from the database
- [x] Admin API and uploads, behind the access gate
- [x] Admin UI
- [x] Containerized
- [x] Deployed, with the admin behind an access gate
- [ ] Handover: walk the owners through editing

Open work is tracked in [GitHub issues](https://github.com/mlh5599/muddys-music-cafe/issues).

## Not built yet

"Muddy's in the Media" page, per-partner pages (~100 on the old site), Cloud Nine page.

## Known gaps

- Orphaned image files remain on disk after a picture is replaced or deleted
- The seed import is not transactional
