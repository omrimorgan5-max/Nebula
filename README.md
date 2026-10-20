# Nebula

A private notes and file space. The Go server serves the browser app and API from one origin. Accounts use SQLite, note content is stored in SQLite and as HTML files, and uploaded files are stored separately for each username.

## Run locally

Requires Go 1.24 or newer.

```sh
go run .
```

Open `http://localhost:8080`. Local HTTP uses a non-secure session cookie; HTTPS requests automatically receive a `Secure` cookie. Set `DATA_DIR` to change the data directory.

New accounts get this private directory layout:

```text
files/<username>/file-uploads/
files/<username>/notes/
```

Each note is mirrored to `notes/note-<id>.html`; note titles and ownership metadata, credentials, sessions, and file indexes live in `files/nebula.db`. Passwords are salted and hashed. User directories and file contents are created with owner-only permissions. Upload limit is 25 MB per file.

## Source layout

- `main.go` is the executable entry point.
- `internal/server/` contains the server setup and feature-specific auth, notes, posts, profile, files, and HTTP modules.
- `internal/server/web/templates/` contains the shared page layout and separate page/component templates.
- `internal/server/web/static/js/` and `internal/server/web/static/css/` contain feature-level frontend modules and design-area stylesheets.

Go, JavaScript, and HTML source files are kept under 300 lines. The CSS modules preserve the existing stylesheet formatting and cascade order.

## Docker

```sh
docker compose up -d nebula
```

Open `http://localhost:8080`. The named `nebula-data` volume persists account, note, and upload data across container updates.

## Cloudflare Tunnel

1. Create a remotely managed tunnel in the Cloudflare Zero Trust dashboard and copy its tunnel token.
2. Copy `.env.example` to `.env` and set `CLOUDFLARE_TUNNEL_TOKEN` to the token. Keep `.env` private and out of source control.
3. In the tunnel's **Public Hostnames**, add your domain or subdomain and route it to `http://nebula:8080`.
4. Start both services with `docker compose --profile tunnel up -d --build`.

Cloudflare terminates public HTTPS and connects through the tunnel to the app container. The app recognizes the forwarded HTTPS scheme and marks its session cookie `Secure`. Do not expose the app port publicly on an untrusted host; for a tunnel-only deployment, remove the `ports` mapping from `compose.yaml`.

## Notes editor

The editor supports font family, bold, and italic formatting. Notes are sanitized on the server to retain only those formatting elements. Sessions are HttpOnly and SameSite cookies, and note/file API queries are scoped to the signed-in account.

## Dashboard and writing

The dashboard summarizes your private notes, posts, and files and links to recent work. Notes and posts support searchable tags; notes can also be searched from the sidebar and pinned. Existing databases are upgraded in place when the app starts. Posts use **Draft** and **Ready · private** states; neither state exposes a public URL.

Unsaved note edits are kept in browser storage scoped to the signed-in username and note, restored after reload, and retried automatically when the server is unavailable. Successfully synced drafts are removed from browser storage.