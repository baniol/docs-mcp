# docs-mcp

Go MCP server providing smart access to any GitHub-hosted documentation via BM25 search, git-based repo sync, and MCP JSON-RPC 2.0 protocol.

## Requirements

- Go 1.22+
- GitHub Personal Access Token *(optional for public repos, required for private)*

## Installation

```bash
git clone https://github.com/baniol/docs-mcp
cd docs-mcp
go build -o docs-mcp ./cmd/server
```

## Configuration

All settings are read from environment variables. `GITHUB_TOKEN`, `GITHUB_REPO`, and `DOCS_PATH` are required.

| Variable                   | Default                      | Description                                      |
|----------------------------|------------------------------|--------------------------------------------------|
| `GITHUB_TOKEN`             | *(empty)*                    | GitHub Personal Access Token *(required for private repos only)* |
| `GITHUB_REPO`              | `tldr-pages/tldr`            | Repository to clone, e.g. `owner/repo`           |
| `DOCS_PATH`                | `pages`                      | Path to docs inside the repo, e.g. `docs`        |
| `GITHUB_BRANCH`            | `main`                       | Branch to track                                  |
| `REPO_PATH`                | `/tmp/docs-mcp-repo`         | Local path where the repo is cloned              |
| `PORT`                     | `8000`                       | HTTP server port                                 |
| `LOG_LEVEL`                | `INFO`                       | Log level: `DEBUG`, `INFO`, `WARN`, `ERROR`      |
| `CACHE_TTL`                | `300`                        | Response cache TTL in seconds                    |
| `CACHE_MAX_ENTRIES`        | `1000`                       | Maximum number of cached responses               |
| `SYNC_INTERVAL`            | `1800`                       | Background git pull interval in seconds          |
| `API_KEYS`                 | *(empty)*                    | Comma-separated Bearer tokens; empty = no auth   |
| `GITHUB_WEBHOOK_SECRET`    | *(empty)*                    | HMAC secret for GitHub webhook verification      |
| `CHUNK_SIZE`               | `800`                        | Document chunk size for BM25 indexing (chars)    |
| `CHUNK_OVERLAP`            | `100`                        | Overlap between chunks (chars)                   |
| `SNIPPET_SIZE`             | `400`                        | Search result snippet size (chars)               |
| `SNIPPETS_PER_RESULT`      | `2`                          | Number of snippets per search result             |
| `MAX_DOCUMENT_LENGTH`      | `8000`                       | Max document length before truncation (chars)    |
| `LARGE_DOCUMENT_THRESHOLD` | `10000`                      | Threshold for smart truncation (chars)           |
| `INCLUDE_GITHUB_LINKS`     | `true`                       | Include GitHub links in responses                |

Create a `.env` file or export variables directly:

```bash
# For public repos (e.g., tldr-pages/tldr), no token needed:
export GITHUB_REPO=tldr-pages/tldr
export DOCS_PATH=pages

# For private repos, you must provide a token:
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxx
export GITHUB_REPO=your-private/repo
export DOCS_PATH=path/to/docs
```

## Running

```bash
# With env vars (public repo - no token needed)
GITHUB_REPO=tldr-pages/tldr DOCS_PATH=pages ./docs-mcp

# Private repo (requires token)
GITHUB_TOKEN=ghp_xxx GITHUB_REPO=owner/repo DOCS_PATH=path/to/docs ./docs-mcp

# With .env file (export manually or use direnv)
source .env && ./docs-mcp
```

On first start the server will:
1. Clone the repository (shallow, single-branch)
2. Build the BM25 search index
3. Start background sync every `SYNC_INTERVAL` seconds
4. Listen for HTTP requests

## Endpoints

### `GET /health`
Returns server status.

```json
{"status": "ok", "service": "docs-mcp"}
```

### `POST /mcp`
MCP JSON-RPC 2.0 protocol endpoint. Used by Cursor, Claude Desktop, etc.

```bash
# Initialize
curl -X POST http://localhost:8000/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'

# List tools
curl -X POST http://localhost:8000/mcp \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list"}'

# Call tool
curl -X POST http://localhost:8000/mcp \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"query_infrastructure_docs","arguments":{"query":"how to configure S3 access"}}}'
```

### `POST /mcp/query`
Convenience endpoint for direct queries.

```bash
curl -X POST http://localhost:8000/mcp/query \
  -H "Content-Type: application/json" \
  -d '{"query": "how to add S3 bucket access"}'
```

### `POST /mcp/tools/list`
Returns available MCP tools.

### `POST /webhook/github`
Receives GitHub push webhooks to trigger immediate sync + index rebuild.

Configure in GitHub: `Settings → Webhooks → Payload URL: https://your-host/webhook/github`

## Connecting from Cursor / Claude Desktop

### Cursor (`~/.cursor/mcp.json`)

```json
{
  "mcpServers": {
    "infra-docs": {
      "url": "http://your-server:8000/mcp"
    }
  }
}
```

### Claude Desktop (`~/Library/Application Support/Claude/claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "infra-docs": {
      "url": "http://your-server:8000/mcp"
    }
  }
}
```

## Authentication

Set `API_KEYS` to require a Bearer token on `/mcp`, `/mcp/query`, and `/mcp/tools/list`:

```bash
API_KEYS=my-secret-key ./docs-mcp
```

Then pass `Authorization: Bearer my-secret-key` in requests. The `/health` and `/webhook/github` endpoints are not protected by API key auth.

## Search Ranking & Tags

Search uses BM25 ranking with boosts for matches in document name (+3.0) and path (+1.5).

Documents can optionally include YAML frontmatter with tags to improve search relevance for terms not present in the body text:

```markdown
---
tags: ["backup", "storage", "s3", "cost-optimization"]
---

# Backup Costs Analysis
...
```

Tags get a +5.0 score boost when they match a query term — useful for synonyms or concepts implied by the content but not stated explicitly (e.g. tagging an S3 document with `storage`). Documents without frontmatter work normally.

## Development

```bash
# Run tests
go test ./...

# Run with verbose output
go test -v ./...
```

```bash
# Build
go build ./cmd/server

# Run locally (public repo - no token needed)
GITHUB_REPO=tldr-pages/tldr DOCS_PATH=pages go run ./cmd/server

# Private repo (requires token)
GITHUB_TOKEN=ghp_xxx GITHUB_REPO=owner/repo DOCS_PATH=path/to/docs go run ./cmd/server
```

## Project Structure

```
cmd/server/main.go          entry point, wires all components
internal/
  config/                   env var loading
  utils/                    TTL cache, formatting, path validation
  repo/                     git clone/pull, file reading
  docproc/                  markdown parsing (TOC, summary, navigation)
  search/                   BM25 ranking, chunker, Searcher interface
  syncer/                   background git sync goroutine
  handlers/                 MCP tool handlers, smart query routing
  server/                   HTTP server, JSON-RPC 2.0, webhook
```

## License

[MIT](LICENSE)
