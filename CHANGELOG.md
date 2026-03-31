# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- MIT license
- GitHub Actions CI pipeline (format, vet, lint, test, build on Go 1.22/1.23)

### Fixed
- Race condition: concurrent webhook and background syncer could corrupt git worktree (added mutex to repo.Client)
- Webhook endpoint accepted unbounded request body (added 1 MB limit via MaxBytesReader)

## [v0.1.0] - 2026-03-18

### Added
- BM25 full-text search with configurable chunking and snippet extraction
- Git-based doc sync with background polling and webhook trigger
- MCP JSON-RPC 2.0 server: `search_docs`, `get_document`, `list_docs` tools
- Bearer token auth via `API_KEYS` env var
- GitHub webhook verification (`GITHUB_WEBHOOK_SECRET`)
- `.env` file support via godotenv
- Configurable chunk size, overlap, snippet size, and document limits
