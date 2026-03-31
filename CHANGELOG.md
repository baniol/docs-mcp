# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.1.0] - 2026-03-18

### Added
- BM25 full-text search with configurable chunking and snippet extraction
- Git-based doc sync with background polling and webhook trigger
- MCP JSON-RPC 2.0 server: `search_docs`, `get_document`, `list_docs` tools
- Bearer token auth via `API_KEYS` env var
- GitHub webhook verification (`GITHUB_WEBHOOK_SECRET`)
- `.env` file support via godotenv
- Configurable chunk size, overlap, snippet size, and document limits
- MIT license
- GitHub Actions CI pipeline (format, vet, lint, test, build on Go 1.22/1.23)
- Cache max entry limit with eviction of oldest entry (`CACHE_MAX_ENTRIES`, default 1000)
- Background cache eviction goroutine removes expired entries proactively
- Server version injected at build time via `-ldflags` (from `git describe --tags`)
- Context propagation: handlers respect client disconnection via `r.Context()`

### Fixed
- Race condition: concurrent webhook and background syncer could corrupt git worktree (added mutex to repo.Client)
- Webhook endpoint accepted unbounded request body (added 1 MB limit via MaxBytesReader)
- ReadDoc cache misses when `maxLength` varied — cache key now includes `maxLength`
- Large documents were not cached after truncation — all processed content is now cached

### Changed
- API key comparison uses `subtle.ConstantTimeCompare` instead of map lookup (timing side-channel hardening)
- `NewCache()` now requires a `maxEntries` parameter
