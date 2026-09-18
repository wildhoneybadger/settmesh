# Settmesh

Self-hosted, end-to-end encrypted messaging server (Signal protocol), invite-only
closed community. See [CLAUDE.md](CLAUDE.md) for the full architecture.

This document covers deploying the **server** (Go + nginx, via Docker). The React
Native mobile client is not implemented yet (see [Mobile client](#mobile-client-android)).

## Requirements

- [Docker](https://docs.docker.com/get-docker/)
- [Docker Compose](https://docs.docker.com/compose/) (v2, `docker compose` command)
- `openssl`, if you generate your own test certificate locally
- A domain name pointing to the server and a TLS certificate for production
  (see [TLS certificates](#tls-certificates))

## Deployment architecture

Two containers, orchestrated by `docker-compose.yml` at the repository root:

- **`server`** — the Go server (authentication, key exchange, messages, encrypted
  files). **Never** exposed directly: only reachable from `nginx` over the internal
  Docker network.
- **`nginx`** — reverse proxy in front, terminates TLS and redirects all HTTP traffic
  to HTTPS. It's the only exposed entry point (ports 80/443).

Persistent data (SQLite database + encrypted files) lives in the named Docker volume
`settmesh-data`, independent of the containers' lifecycle.

## Environment variables

Set in a `.env` file at the repository root (copied from `.env.example`, never
committed) or directly in the environment at launch time.

| Variable            | Required | Default                | Description                                                                 |
|----------------------|:--------:|------------------------|-------------------------------------------------------------------------------|
| `ADMIN_TOKEN`         | **Yes**  | *(none)*                | Token required for the `/admin/*` routes (invite code generation). If unset, these routes refuse every request. Generate with `openssl rand -hex 32`. |
| `PORT`                | No       | `8080`                  | Internal listen port for the Go server (rarely needs changing).              |
| `DB_PATH`             | No       | `/data/settmesh.db`     | Path to the SQLite database (inside the container).                          |
| `FILES_DIR`           | No       | `/data/files`           | Storage directory for encrypted files (inside the container).                |
| `INVITE_TTL_HOURS`    | No       | `72`                    | Validity period of a generated invite code.                                  |
| `SESSION_TTL_HOURS`   | No       | `720` (30 days)         | Validity period of a session token after login.                              |
| `MESSAGE_TTL_HOURS`   | No       | `720` (30 days)         | Max retention for an undelivered text message.                               |
| `FILE_TTL_HOURS`      | No       | `168` (7 days)          | Max retention for an undelivered photo.                                      |

`DB_PATH` and `FILES_DIR` are already set in `docker-compose.yml` to point at the
persistent volume; only change them if you adapt the mounts yourself.

## TLS certificates

nginx mounts certificates from the `./certs` directory at the repository root — a
host directory kept **outside the containers**, never committed (`.gitignore`). It
must contain:

- `certs/fullchain.pem`
- `certs/privkey.pem`

**In production**, obtain a valid certificate (e.g. [Let's Encrypt](https://letsencrypt.org/)
via `certbot`) and drop both files there.

**Locally**, generate a self-signed certificate for testing:

```bash
openssl req -x509 -nodes -newkey rsa:2048 -days 365 \
  -keyout certs/privkey.pem -out certs/fullchain.pem \
  -subj "/CN=localhost" \
  -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
```

Browsers/clients will show a warning (untrusted certificate): that's expected, ignore
it for testing (`curl -k`).

## Build and run

```bash
git clone <repo-url> settmesh
cd settmesh

cp .env.example .env
# edit .env and set ADMIN_TOKEN (e.g. openssl rand -hex 32)

# drop your TLS certificates into ./certs (see above)

docker compose build
docker compose up -d
```

Check that the server responds:

```bash
curl -k https://your-domain/health
# -> ok
```

View logs:

```bash
docker compose logs -f
```

Stop the stack (data persists in the volume):

```bash
docker compose down
```

### Updating

```bash
git pull
docker compose build
docker compose up -d
```

### Backing up data

The SQLite database and encrypted files live in the `settmesh-data` volume. To
archive it:

```bash
docker run --rm \
  -v settmesh_settmesh-data:/data \
  -v "$(pwd)":/backup \
  alpine tar czf /backup/settmesh-backup.tar.gz -C /data .
```

## Generating a first admin invite code

Registration is closed by default: without a valid invite code, no account can be
created. Only the administrator (whoever holds `ADMIN_TOKEN`) can generate one:

```bash
curl -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  https://your-domain/admin/invite-codes
```

Response:

```json
{"code":"XXXX-XXXX-XXXX-XXXX","expires_at":"2026-09-17T20:28:49Z"}
```

The code is single-use (invalidated as soon as an account is created with it) and
valid for 72h by default (`INVITE_TTL_HOURS`). Share it with the person you're
inviting through a channel of your choice (the code alone can't impersonate an
account, but avoid posting it publicly while it's still valid).

## Mobile client (Android)

The React Native client is not implemented yet at this stage of the project — only
the folder structure is in place (`client/screens`, `client/components`,
`client/crypto`, `client/server-profiles`). This section will be filled in with APK
build instructions once the client is built (`libsignal` bindings, screens,
multi-server-profile support).

## License

Code released under the [MIT](LICENSE) license.
