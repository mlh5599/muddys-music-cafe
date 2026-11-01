# Muddy's Music Cafe website

Website for Muddy's Music Cafe, a Second Life music venue (est. 2010). The
public site keeps the original design; the content (events, DJs, hosts,
partners, site text) lives in a database and is managed through a simple admin
area, so changing content never needs a redeploy.

## Features

- Public pages (home, DJs, hosts) rendered server-side from the database, so search engines and link previews see real content
- Admin area for events, DJs, hosts, partners and site details, with photo and poster uploads
- Events drop off the site automatically once they end
- Single Docker image; all state in one data directory (SQLite + uploads)

## Quick start

```sh
make build
./bin/muddys -data-dir=./data -seed                          # one-time starter content
./bin/muddys -addr=127.0.0.1:8080 -data-dir=./data -dev-no-auth
```

Open `http://127.0.0.1:8080/` for the site and `/admin/` for the admin
(`-dev-no-auth` skips sign-in and only works on a loopback address). Or with
Docker:

```sh
docker build -t muddys:local .
docker run --rm -p 8080:8080 -v "$PWD/data:/data" muddys:local -seed -data-dir=/data
docker run --rm -p 8080:8080 -v "$PWD/data:/data" muddys:local
```

## Documentation

- [Architecture](docs/architecture.md): stack, hostnames, configuration flags, package layout, data model, branding
- [Admin usability requirements](docs/admin-usability.md): what the admin must be like to use
- [Security and credentials](docs/security.md): rules for this public repo and how the admin is protected
- [Development](docs/development.md): build, test, run, container, backup and restore
- [CI/CD and deployment](docs/deployment.md): release PR approval, image publishing, push deploys, migrations and rollback
- [Status and roadmap](docs/roadmap.md): what is done and what is left

Open work is tracked in [GitHub issues](https://github.com/mlh5599/muddys-music-cafe/issues).

## Security

The admin is meant to sit behind an access-gating layer and also verifies the
gate's signed identity itself. Never commit secrets. See
[docs/security.md](docs/security.md); report vulnerabilities privately per
[SECURITY.md](SECURITY.md).

The repo contains nothing specific to where it is hosted. Deployment lives
elsewhere.
