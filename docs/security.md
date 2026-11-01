# Security and credentials

This repository is public. Rules for keeping it safe:

- **No secrets in Git, ever.** No passwords, API keys, tokens, private keys, `.env` files, database files or uploaded content. `.gitignore` blocks the common ones; GitHub secret scanning and push protection are enabled.
- **The app needs no secrets by default.** Access control is handled by the layer in front of it, and the app never stores credentials or passwords. If a future feature needs a secret, it is read at runtime from an environment variable or a mounted secret file, never from the repo or the image, and documented by name only.
- **No environment details.** Hostnames, IP addresses, network layout, deployment tooling and the access-gate configuration stay out of this repo.
- **Admin defense in depth.** Write endpoints verify the access gate's signed identity assertion (RS256 against the gate's published keys; issuer, audience and expiry checked) and refuse requests without it, and state-changing requests also need a CSRF header. The admin is safe to run exposed only through that gate, and stays locked until it is configured.
- **Uploads are untrusted.** Type is validated by content (not filename), size is limited, images are re-encoded (stripping metadata), filenames are generated server-side, and uploads are served with `X-Content-Type-Options: nosniff`.
- **Standard web hardening.** Parameterized SQL only, `html/template` auto-escaping, a strict Content-Security-Policy, server timeouts and request size limits.
- **Dependencies** are pinned and kept current; the container runs as a non-root user.

Report vulnerabilities privately, see [SECURITY.md](../SECURITY.md).
