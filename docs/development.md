# Development

Requirements: Go (see `server/go.mod`), Node (see the `Dockerfile`), optionally Docker.

```sh
make build     # build the admin, then the server binary into bin/
make test      # go tests + admin type-check/build
make lint      # go vet + admin lint
make run       # build and run on :8080 with ./data
```

## Run locally

```sh
./bin/muddys -data-dir=./data -seed              # one-time: import the starter content
./bin/muddys -addr=127.0.0.1:8080 -data-dir=./data -dev-no-auth
```

`-dev-no-auth` skips the admin sign-in and only works on a loopback address. Then open `http://127.0.0.1:8080/` and `/admin/`. `-seed` refuses to run if content already exists.

## Container

```sh
docker build -t muddys:local .
docker run --rm -p 8080:8080 -v "$PWD/data:/data" muddys:local -seed -data-dir=/data
docker run --rm -p 8080:8080 -v "$PWD/data:/data" muddys:local
```

The image runs as uid/gid 1000 (`muddys`), so a mounted data directory must be writable by that user. All flags are documented in [architecture.md](architecture.md#configuration).

## Backup and restore

All state is in the data directory: `muddys.db` (SQLite, WAL) and `uploads/`. Back up the whole directory; for a consistent copy of the database, stop the app or use `sqlite3 muddys.db ".backup out.db"`. Restore by putting the directory back and starting the app.
