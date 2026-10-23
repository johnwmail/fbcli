# fbcli integration test suite

This suite exercises the CLI against a running **FileBrowser Quantum v2** server. It is not a mock-based suite: uploads, downloads, deletes, renames, and syncs are performed against a real source and cleaned up afterwards.

The migration target is `ghcr.io/gtsteffaniak/filebrowser:stable` (currently v2.0.1-stable).

## Requirements

- Go 1.24+ and a built `./fbcli` binary
- Bash 4+
- A reachable FileBrowser Quantum v2 server
- An account with view, download, create, modify, and delete permissions on one source
- A writable source root for temporary `test-*` directories

The legacy `filebrowser/filebrowser` v1 server is **not** supported.

## Environment

| Variable | Required | Purpose |
|----------|----------|---------|
| `FILEBROWSER_URL` | Yes | Quantum server URL, including any base path |
| `FILEBROWSER_USERNAME` | Yes | Login username |
| `FILEBROWSER_PASSWORD` | Yes | Login password |
| `FILEBROWSER_SOURCE` | No | Source name; auto-detected when omitted |
| `FBCLI_TEST_SOURCE` | No | Optional valid source name used to test `--source` and env override paths |

`FBCLI_TEST_SOURCE` is separate from `FILEBROWSER_SOURCE` so the suite can exercise automatic source discovery in CI while still testing the explicit override when the source name is known.

## Running against the supported container

```bash
# Start Quantum v2.0.1-stable. --user root keeps /srv writable in the test container.
docker run -d --name fbcli-test --user root -p 8080:80 \
  -e FILEBROWSER_ADMIN_PASSWORD=testpass \
  ghcr.io/gtsteffaniak/filebrowser:stable

# Build and run the suite.
go build -o fbcli .
export FILEBROWSER_URL=http://localhost:8080
export FILEBROWSER_USERNAME=admin
export FILEBROWSER_PASSWORD=testpass
# Optional explicit-source checks:
export FBCLI_TEST_SOURCE=srv

./testsuite/run-tests.bash
```

To confirm the server version:

```bash
docker exec fbcli-test /home/filebrowser/filebrowser version
# Version: v2.0.1-stable
```

## Runner options

```bash
./testsuite/run-tests.bash              # summary only
./testsuite/run-tests.bash -d           # detailed assertion output
./testsuite/run-tests.bash -l           # list test files (offline)
./testsuite/run-tests.bash testsuite/test-ls.bash
./testsuite/run-tests.bash -d testsuite/test-ls.bash
```

The runner performs a preflight login/list before starting, so missing credentials, an unreachable server, an invalid source, or insufficient permissions fail immediately with a readable error.

## Test files

| File | Coverage |
|------|----------|
| `test-show-simple.bash` | `show`, offline behaviour, password redaction, `--source`, help/version |
| `test-ls.bash` | compact/long/script listings, ignore regex, aliases, single-file listing, empty directories |
| `test-upload.bash` | file/directory upload, content integrity, overwrite of an existing remote file, ignore patterns, alias |
| `test-download.bash` | file/directory download, zip archive, `-z`, existing local directory, ignored tree, incompatible flags, aliases |
| `test-mkdir.bash` | `mkdir`/`md`, multiple/nested directories, idempotency, invalid paths/source |
| `test-rename.bash` | rename/move files and directories, cross-directory moves, content preservation, missing destination parent |
| `test-rm.bash` | single/multiple/recursive deletion, ignore patterns, directory removal with `-i`, root protection, aliases |
| `test-syncto.bash` | content sync, idempotent repeat sync, remote-only deletion, ignore patterns, alias |
| `test-syncfrom.bash` | content sync, idempotent repeat sync, local-only deletion, remote updates, ignore patterns, alias |
| `test-quantum-v2.bash` | source selection, special-character paths, `--` handling, upload-to-existing-file, validation |
| `test-shell.bash` | sftp-style batch shell: pwd/cd, put/get, recursive transfers, zip, rename/rm, local commands, error exit |

### Shell testing

The shell is tested through stdin, so no pseudo-terminal is required:

```bash
printf 'pwd\nls -s /\ncd /tmp\nexit\n' | ./fbcli shell
```

`fbcli shell` still runs in batch mode when stdin is not a terminal. When run from a terminal, `fbcli` with no arguments starts the interactive shell. A batch command failure is reported and makes the process exit non-zero.

## Framework

`framework.bash` provides:

- `check_environment` — validates credentials, binary, and a working source before tests run
- `assert`, `assert_fails` — command success/failure, with output on failure
- `assert_exists`, `assert_not_exists` — local paths
- `assert_remote_exists`, `assert_remote_not_exists` — exact remote-name checks via `ls -s`
- `assert_contains`, `assert_not_contains` — literal output checks
- `assert_content`, `assert_remote_content` — content integrity checks
- `create_test_file`, `create_test_dir` — create and track fixtures
- `track_local`, `track_remote` — register cleanup paths
- `remote_path_exists` — exact existence check used by the assertions

Tests use the `test-` prefix for remote root directories and for local fixtures. Cleanup runs from an `EXIT` trap, removes tracked paths, then removes any leftover `test-*` remote directories and `test-*` local paths.

## Unit tests

The Go unit tests cover argument parsing, remote path normalization, zip-target selection, password redaction, JWT expiry parsing, list sorting, v2 login headers, source discovery, API retry metadata, resource URL construction, and safe mkdir behaviour:

```bash
go test ./...
```

## Adding a test

1. Create `testsuite/test-<feature>.bash`.
2. Source `framework.bash` and call `init_test "description"`.
3. Create fixtures with the helpers so cleanup is automatic.
4. Use `assert_remote_content`/`assert_content` for data integrity, not just existence.
5. Call `finish_test` at the end.
6. Run `gofmt -w .`, `go vet ./...`, `go test ./...`, `golangci-lint run`, and the new integration test before committing.

Use `FILEBROWSER_SOURCE` or `--source` only when a test specifically covers source selection; otherwise let the CLI auto-detect so the suite works on any single-source deployment.
