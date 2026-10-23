# fbcli - FileBrowser Quantum CLI

A command-line client for [FileBrowser Quantum](https://github.com/gtsteffaniak/filebrowser), the multi-source FileBrowser v2 server. Manage remote files, transfer directories, and keep folders in sync from the terminal.

This release is a full migration to the **FileBrowser Quantum v2 API** and targets `ghcr.io/gtsteffaniak/filebrowser:stable` (**v2.0.1-stable**).

> [!IMPORTANT]
> The legacy `filebrowser/filebrowser` v1 API is **not supported**. `fbcli` now uses `/api/auth/login`, `/api/resources?path=...&source=...`, and `/api/settings/sources`.

[![Go Version](https://img.shields.io/badge/Go-1.24.4-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](#license)

## Features

- Full file operations: list, upload, download, mkdir, delete, rename/move
- Multi-source support with automatic source discovery
- Directory downloads as server-side zip archives
- Regex ignore patterns for uploads, downloads, deletes, and syncs
- Bidirectional sync (`syncto` / `syncfrom`) with exact-size comparison and SHA-256 checks for small files
- Secure password prompt, TOTP support, and short-lived session token caching
- Script-friendly listing output (`ls -s`)
- Exact handling of spaces, unicode, and URL-special characters in remote paths

## Compatibility

| Component | Supported |
|-----------|-----------|
| Server | FileBrowser Quantum v2.0.1-stable (`:stable`); newer v2.x releases should work |
| Legacy FileBrowser v1 | No |
| Linux | amd64, arm64 |
| Windows | amd64 |
| macOS | Build from source |

The integration workflow runs against the same `:stable` container image used in deployment.

## Installation

### Pre-built binaries

Download a release from the [releases page](https://github.com/johnwmail/fbcli/releases).

### Build from source

```bash
git clone https://github.com/johnwmail/fbcli.git
cd fbcli
go build -o fbcli .
```

### Go install

```bash
go install github.com/johnwmail/fbcli@latest
```

## Configuration

`fbcli` reads connection settings from environment variables. Missing values are requested interactively (except during `show`).

```bash
export FILEBROWSER_URL="https://files.example.com"
export FILEBROWSER_USERNAME="alice"
export FILEBROWSER_PASSWORD="your-password"

# Optional: choose a source when the account can access several.
export FILEBROWSER_SOURCE="media"

# Optional: TOTP code when two-factor authentication is enabled.
export FILEBROWSER_TOTP="123456"
```

| Variable | Description | Required |
|----------|-------------|----------|
| `FILEBROWSER_URL` | Server URL, including any base path | Yes |
| `FILEBROWSER_USERNAME` | Login username | Yes |
| `FILEBROWSER_PASSWORD` | Login password | Yes |
| `FILEBROWSER_SOURCE` | Source name to use | No* |
| `FILEBROWSER_TOTP` | Current TOTP code | No |

\* If omitted, `fbcli` calls `/api/settings/sources`. A single accessible source is used automatically. With multiple sources the first name alphabetically is used and a note is printed to stderr. Use `--source` or `FILEBROWSER_SOURCE` to choose explicitly.

`FILEBROWSER_URL` may include a base path, for example `https://example.com/files`; endpoints are appended as `/files/api/...`.

### Source permissions

The account needs the relevant per-source permissions:

- `view` for `ls` and sync planning
- `download` for `download` and syncfrom
- `create` for `upload` and `mkdir`
- `modify` for `rename`/`mv`
- `delete` for `rm`/`delete`

A read-only source rejects all write operations with HTTP 403. If the account has no sources, `fbcli` exits with an explanatory error.

### Session token cache

Successful logins are cached in the user cache directory (for example `~/.cache/fbcli/sessions.json` on Linux, `~/Library/Caches/fbcli/sessions.json` on macOS, and `%LocalAppData%\fbcli\sessions.json` on Windows; mode `0600`). This prevents a new login for every command and avoids the server's login rate limiter during loops and scripts. Passwords are never stored.

To force a fresh login, delete the cache file or the `fbcli` cache directory. Set `XDG_CACHE_HOME` to relocate it on Linux.

The login endpoint is rate limited per IP and username. If a login is rate limited, `fbcli` waits briefly and retries once; longer waits are reported to the caller.

## Commands

### Listing

#### `ls [-i <regex>] [-l] [-s] [remote_path]`

```bash
fbcli ls /                       # compact multi-column listing
fbcli ls -l /documents           # modified time and index size
fbcli ls -s /logs                # one name per line (scripts)
fbcli ls -i '.*\.log$' /var      # skip log files
fbcli ls -s /docs/report.pdf     # list a single file
```

Aliases: `list`, `dir` (both default to the detailed format).

`-l` shows the size reported by the server index. For files this can be disk usage rather than exact byte length (for example 4 KiB for a 6-byte file). Synchronization uses the download endpoint's `Content-Length` for exact comparisons.

### Transfers

#### `upload, up [-i <regex>] <local_path> [remote_dir]`

```bash
fbcli upload report.pdf /documents
fbcli up -i '\.git' ./project /projects
fbcli upload new.txt /docs/old.txt   # overwrite an existing remote file
```

- If the destination exists as a **file**, it is treated as the target file path.
- Otherwise the destination is treated as a directory and the local base name is preserved.
- Missing remote parent directories are created automatically.
- A directory upload preserves empty directories.
- Files matching `-i` are skipped; matching directories and their subtrees are skipped.

#### `download, down, dl [-i <regex>] [-z] <remote_path> [local_path]`

```bash
fbcli download /documents/report.pdf
fbcli download -z /projects/webapp          # save webapp.zip
mkdir -p ./backup && fbcli download /projects/webapp ./backup
fbcli dl -i 'node_modules' /src ./copy      # recursive download, skipping matches
```

- Files are downloaded to `local_path`; if `local_path` is an existing directory, the remote base name is used inside it.
- Directories are downloaded as a zip archive by default. The archive contains the remote directory as its top-level entry.
- `-z` explicitly requests the zip path; it cannot be combined with `-i`.
- With `-i`, `fbcli` walks the tree and downloads files individually so ignored entries can be skipped. The target is a local directory.

### Directory operations

#### `mkdir, md <remote_path>...`

Creates one or more directories, including parents. Creating an existing directory succeeds. The source root cannot be created or replaced.

```bash
fbcli mkdir /new-folder
fbcli md /dir1 /dir2 /dir3
```

#### `rm, delete [-i <regex>] <remote_path>...`

```bash
fbcli rm /old-file.txt
fbcli delete /tmp/file1 /tmp/file2
fbcli rm -i 'error\.log' /logs        # remove everything except error.log
```

Directories are deleted recursively. `fbcli` refuses to delete the source root. With `-i`, matching entries are preserved; a directory is also removed when nothing inside it was preserved.

#### `rename, mv <old_path> <new_path>`

```bash
fbcli rename /old-name.txt /new-name.txt
fbcli mv /temp/file.txt /documents/file.txt
```

Moves and renames use the v2 JSON PATCH API. The destination parent directory must already exist.

### Synchronization

#### `syncto, to [-i <regex>] <local_path> <remote_path>`

Makes the remote directory mirror the local directory:

- New and changed local files are uploaded.
- Remote entries not present locally are deleted.
- Ignored names are left untouched on both sides.
- Files are compared by exact byte size from the download endpoint; files smaller than 1 MiB are additionally verified with SHA-256.
- A second run over unchanged files reports `already in sync`.

```bash
fbcli syncto ./local-folder /remote-backup
fbcli to -i '\.(git|DS_Store)' ./project /projects/my-app
```

#### `syncfrom, from [-i <regex>] <remote_path> <local_path>`

Makes the local directory mirror the remote directory. It performs the same comparison and deletion rules in the opposite direction.

```bash
fbcli syncfrom /server-configs ./local-backup
fbcli from -i '^temp' /workspace ./local-workspace
```

Sync is intentionally one-way and destructive by default: extra entries on the destination side are deleted. Use `-i` for anything that must be preserved.

### Configuration

#### `show`

Displays the configured URL, username, source, redacted password, and version. `show` does not contact the server and therefore prints `(auto)` when no source is configured.

```bash
fbcli show
fbcli show --source media
```

## Global options

| Option | Description |
|--------|-------------|
| `-i, --ignore <regex>` | Skip names matching a Go regular expression |
| `-S, --source <name>` | Override `FILEBROWSER_SOURCE` for this command |
| `-z, --zip` | Download a directory as a zip archive |
| `-s, --script` | Print plain names, one per line |
| `-l, --long` | Detailed listing format |
| `--` | Treat all following arguments as paths |
| `-h, --help` | Show help |
| `-v, --version` | Show version |

Options may appear before or after positional paths. Use `--` when a path starts with `-`.

## Ignore patterns

`-i` accepts any Go regular expression and matches the **base name** of each file or directory.

```bash
-i '.*\.log$'              # log files
-i '(temp|tmp|cache)'      # temporary names
-i '\.(git|svn|hg)'        # VCS directories
-i '^\.'                   # dotfiles
```

## Scripting

Use `-s` for one name per line without colors:

```bash
fbcli ls -s /data | while read -r name; do
    fbcli download "/data/$name" ./downloads/
done
```

Exit codes:

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Usage error, authentication failure, network error, or API error |

Server failures are printed as `HTTP <status>: <message>`. Common statuses:

- `401` — missing, expired, or revoked credentials
- `403` — source permission denied or read-only source
- `404` — path or source not found
- `409` — resource already exists where the operation requires a new path
- `429` — login or request rate limit

## Troubleshooting

### `source "..." not found`

The account cannot access that source name, or it was typed incorrectly. Run `fbcli ls /` with no `FILEBROWSER_SOURCE` to let the CLI auto-detect, or ask an administrator which source names the account can access.

### `the account has no accessible sources`

The user exists but has no backend scopes. Grant source access in the Quantum admin UI.

### Login returns `401`

Check the username/password and TOTP code. Delete the session cache to rule out a stale token:

```bash
rm -f "${XDG_CACHE_HOME:-$HOME/.cache}/fbcli/sessions.json"
```

### Login returns `429`

The server rate-limits credential attempts. Reuse the session cache, wait for the `Retry-After` period, and avoid parallel batches of `fbcli` commands that each need a fresh login.

### Upload returns `403`

The account lacks `create`/`modify` permission for that source, or the source is read-only.

### Download returns `403`

The account lacks `download` permission for the source, or the source is not readable by the account.

### `ls -l` size looks wrong

The v2 index reports disk usage for files. Use synchronization (which uses exact `Content-Length`) or download the file when exact byte size matters.

## Migrating from the legacy FileBrowser API

| Legacy (v1) | FileBrowser Quantum (v2) |
|-------------|--------------------------|
| `POST /api/login` with JSON body | `POST /api/auth/login?username=...` with `X-Password` header |
| `/api/resources/<path>` | `/api/resources?path=<path>&source=<name>` |
| `/api/raw/<path>` | `/api/resources/download?source=<name>&file=<path>` |
| `POST /api/resources/<path>/` (mkdir) | `POST /api/resources?path=...&source=...&isDir=true` |
| Query-string rename (`PATCH ...?action=rename`) | JSON PATCH body with `items`, `action`, `overwrite`, `rename` |
| Single implicit source | Multiple named sources (`/api/settings/sources`) |
| Public share paths under `/api/public` | Public share API under `/public/api` |

### Migration checklist

1. Upgrade the server to FileBrowser Quantum v2.0.1-stable or newer.
2. Point `FILEBROWSER_URL` at the new server, including any base path.
3. Set `FILEBROWSER_SOURCE` if the account can access more than one source.
4. Verify permissions for the source (`view`, `download`, `create`, `modify`, `delete`).
5. Run `fbcli show`, then `fbcli ls -s /`.
6. Run the integration suite against the deployment: `./testsuite/run-tests.bash`.
7. Update scripts that hard-coded the old `/api/login`, `/api/raw`, or path-in-URL endpoints.

## Project layout

| File | Responsibility |
|------|----------------|
| `fbcli.go` | CLI entry point, options, prompts, `show`, help |
| `client.go` | HTTP client, auth, source discovery, API errors |
| `resource.go` | List, upload, download, mkdir, delete, rename |
| `sync.go` | One-way synchronization |
| `output.go` | Listing formats, colors, hashing helpers |
| `session.go` | Session token cache |
| `fbcli_test.go` | Unit tests |
| `testsuite/` | Real-server integration tests |

## Testing

### Unit tests

```bash
go test ./...
go vet ./...
golangci-lint run
```

### Integration tests

```bash
go build -o fbcli .
export FILEBROWSER_URL=http://localhost:8080
export FILEBROWSER_USERNAME=admin
export FILEBROWSER_PASSWORD=testpass

./testsuite/run-tests.bash          # summary only
./testsuite/run-tests.bash -d       # detailed output
./testsuite/run-tests.bash -l       # list test files (offline)
```

See [testsuite/README.md](testsuite/README.md) for container setup, environment variables, the full coverage list, and framework helpers.

The GitHub Actions workflow starts `ghcr.io/gtsteffaniak/filebrowser:stable`, sets `FILEBROWSER_ADMIN_PASSWORD`, waits for `/health`, builds `fbcli`, and runs the complete suite on Linux amd64 and arm64. The Windows job builds the binary and runs the unit tests and offline smoke checks.

## Development

```bash
gofmt -w .
go vet ./...
go test ./...
golangci-lint run
go build -o fbcli .
```

Cross-compile for release:

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o fbcli-linux-amd64 .
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o fbcli-linux-arm64 .
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -o fbcli-windows-amd64.exe .
```

Release tags (`v*`) run verification, build Linux amd64/arm64 and Windows amd64 archives, generate checksums, and publish a GitHub release.

## License

MIT — see [LICENSE](LICENSE).
