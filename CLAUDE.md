# docs-mcp

Go MCP server providing smart access to GitHub-hosted documentation via BM25 search, git-based sync, and MCP JSON-RPC 2.0.

All code, comments, and commits must be in English.

## Commands

Run `make help` for all available targets. Key commands:

```bash
make build          # Build binary to ./bin/docs-mcp
make run            # Build and run locally (requires .env)
make test           # Run all tests
make test-coverage  # Run tests with coverage report
make lint           # Run golangci-lint
make fmt            # Format with gofmt + goimports
make fmt-check      # Check formatting (CI mode)
make vet            # Run go vet
make check-all      # Run all checks (fmt, vet, lint, tests)
make clean          # Remove build artifacts
make setup-hooks    # Configure git hooks
make bump-patch         # vX.Y.Z → vX.Y.Z+1
make bump-minor         # vX.Y.Z → vX.Y+1.0
make bump-major         # vX.Y.Z → vX+1.0.0
```

Short aliases: `make t` (test), `make ca` (check-all)

## Architecture

```
cmd/server/         — Entry point: wires dependencies, starts HTTP server
internal/
  config/           — Env-based config (config.Load())
  repo/             — Git clone/pull via go-git
  docproc/          — Markdown parsing, chunking
  search/           — BM25 index + query
  syncer/           — Background sync loop
  handlers/         — MCP JSON-RPC handlers (tools: search_docs, get_document, list_docs)
  server/           — HTTP server setup, routing (chi)
  utils/            — Shared helpers
```

### Request flow

1. MCP client sends JSON-RPC 2.0 POST to `/mcp`
2. `handlers` dispatches to tool handler
3. `search` runs BM25 query against in-memory index
4. Response returned as JSON-RPC result

### Background sync

`syncer` polls `SYNC_INTERVAL` seconds, pulls latest git changes, re-indexes docs via `docproc` + `search`.

Webhook push (`POST /webhook`) triggers immediate sync.

## Conventions

### Go style
- Standard Go project layout (`cmd/`, `internal/`)
- Error handling: always wrap with context — `fmt.Errorf("doing X: %w", err)`
- Logging: `log/slog` with structured key-value pairs; no `fmt.Printf` in production code
- No globals except `slog.Default()` — pass dependencies explicitly
- Interfaces defined at the consumer side (in the package that uses them)
- Table-driven tests with `t.Run()` subtests

### Adding a new MCP tool
1. Add handler function in `internal/handlers/`
2. Register in the dispatch map in `handlers/mcp.go`
3. Add integration test in `handlers/*_test.go`

### Adding a new config option
1. Add field to `internal/config/config.go`
2. Read from env with sensible default
3. Document in README.md config table

### Dependency management
- `go mod tidy` before committing new deps
- Prefer stdlib; add external deps only when clearly justified

## Testing

```bash
make test           # go test ./...
make test-coverage  # with -coverprofile, opens HTML report
make test-race      # with -race flag
```

Tests live next to the code (`foo_test.go`). Integration tests use `testdata/` fixtures.

## Git

### Commits
Concise, imperative, "why"-focused. Examples:
- `add BM25 re-ranking for multi-term queries`
- `fix race condition in syncer shutdown`
- `config: make GITHUB_REPO required`

Do not add `Co-Authored-By` lines.

After each commit with user-facing changes, update `## [Unreleased]` in `CHANGELOG.md` (use Added/Changed/Fixed/Removed sections per [Keep a Changelog](https://keepachangelog.com/)).

### Pre-commit hook
Run `make setup-hooks` once. Hook runs `make check-all` before each commit.

### Releases & tags

Tags follow [Semantic Versioning](https://semver.org/): `vMAJOR.MINOR.PATCH`

```bash
make bump-patch   # vX.Y.Z → vX.Y.Z+1  (bug fixes, internal)
make bump-minor   # vX.Y.Z → vX.Y+1.0  (new features, backward-compatible)
make bump-major   # vX.Y.Z → vX+1.0.0  (breaking changes)
```

Each command reads the latest git tag, calculates the next version, moves `[Unreleased]` in CHANGELOG.md to the new version, commits, and creates an annotated tag. Then push: `git push && git push origin <tag>`

### Changelog

`CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com/) format. Add entries to `## [Unreleased]` during development — `make bump-*` handles the rest.

Categories: `Added`, `Changed`, `Fixed`, `Removed`, `Security`, `Deprecated`
