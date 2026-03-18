package config

import (
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config holds all configuration for the MCP server.
type Config struct {
	// GitHub
	GithubToken   string
	GithubRepo    string
	DocsPath      string
	GithubBranch  string
	WebhookSecret string

	// Server
	CacheTTL int
	LogLevel string
	Port     int

	// Repo
	RepoPath     string
	SyncInterval int // seconds

	// Search & Token Optimization
	ChunkSize              int
	ChunkOverlap           int
	SnippetSize            int
	MaxSnippetSize         int
	SnippetsPerResult      int
	MaxDocumentLength      int
	LargeDocumentThreshold int

	// Auth
	APIKeys []string

	// Feature flags
	IncludeGithubLinks bool
}

func Load() (*Config, error) {
	c := &Config{
		GithubToken:            env("GITHUB_TOKEN", ""),
		GithubRepo:             env("GITHUB_REPO", "tldr-pages/tldr"),
		DocsPath:               env("DOCS_PATH", "pages"),
		GithubBranch:           env("GITHUB_BRANCH", "main"),
		WebhookSecret:          env("GITHUB_WEBHOOK_SECRET", ""),
		CacheTTL:               envInt("CACHE_TTL", 300),
		LogLevel:               env("LOG_LEVEL", "INFO"),
		Port:                   envInt("PORT", 8000),
		RepoPath:               env("REPO_PATH", "/tmp/docs-mcp-repo"),
		SyncInterval:           envInt("SYNC_INTERVAL", 1800),
		ChunkSize:              envInt("CHUNK_SIZE", 800),
		ChunkOverlap:           envInt("CHUNK_OVERLAP", 100),
		SnippetSize:            envInt("SNIPPET_SIZE", 400),
		MaxSnippetSize:         envInt("MAX_SNIPPET_SIZE", 600),
		SnippetsPerResult:      envInt("SNIPPETS_PER_RESULT", 2),
		MaxDocumentLength:      envInt("MAX_DOCUMENT_LENGTH", 8000),
		LargeDocumentThreshold: envInt("LARGE_DOCUMENT_THRESHOLD", 10000),
		IncludeGithubLinks:     envBool("INCLUDE_GITHUB_LINKS", true),
	}

	if keys := env("API_KEYS", ""); keys != "" {
		for _, k := range splitNonEmpty(keys, ',') {
			c.APIKeys = append(c.APIKeys, k)
		}
	}

	// Trim whitespace/quotes that can sneak in from .env files
	c.GithubToken = strings.Trim(strings.TrimSpace(c.GithubToken), `"'`)

	return c, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			slog.Warn("invalid integer env var, using default", "key", key, "value", v, "default", fallback)
			return fallback
		}
		return n
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}

func splitNonEmpty(s string, sep rune) []string {
	return strings.FieldsFunc(s, func(r rune) bool { return r == sep })
}
