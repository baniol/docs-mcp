package config

import (
	"os"
	"testing"
)

func TestLoad_MissingToken_DefaultRepo(t *testing.T) {
	// Empty token should work for default public repo
	os.Unsetenv("GITHUB_TOKEN")
	os.Unsetenv("GITHUB_REPO")
	os.Unsetenv("DOCS_PATH")
	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error with default repo: %v", err)
	}
	if c.GithubRepo != "tldr-pages/tldr" {
		t.Errorf("expected default repo=tldr-pages/tldr, got %s", c.GithubRepo)
	}
}

func TestLoad_MissingToken_CustomRepo(t *testing.T) {
	// Token is optional for any repo — auth errors surface at clone/pull time
	os.Unsetenv("GITHUB_TOKEN")
	t.Setenv("GITHUB_REPO", "owner/some-public-repo")
	os.Unsetenv("DOCS_PATH")
	_, err := Load()
	if err != nil {
		t.Fatalf("unexpected error when GITHUB_TOKEN is missing: %v", err)
	}
}

func setRequiredEnvs(t *testing.T) {
	t.Helper()
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GITHUB_REPO", "owner/repo")
	t.Setenv("DOCS_PATH", "docs")
}

func TestLoad_DefaultRepoAndDocsPath(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	os.Unsetenv("GITHUB_REPO")
	os.Unsetenv("DOCS_PATH")
	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.GithubRepo != "tldr-pages/tldr" {
		t.Errorf("expected default repo=tldr-pages/tldr, got %s", c.GithubRepo)
	}
	if c.DocsPath != "pages" {
		t.Errorf("expected default docs_path=pages, got %s", c.DocsPath)
	}
}

func TestLoad_Defaults(t *testing.T) {
	setRequiredEnvs(t)

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.GithubBranch != "main" {
		t.Errorf("expected branch=main, got %s", c.GithubBranch)
	}
	if c.CacheTTL != 300 {
		t.Errorf("expected cache_ttl=300, got %d", c.CacheTTL)
	}
	if c.Port != 8000 {
		t.Errorf("expected port=8000, got %d", c.Port)
	}
	if c.ChunkSize != 800 {
		t.Errorf("expected chunk_size=800, got %d", c.ChunkSize)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	setRequiredEnvs(t)
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "DEBUG")

	c, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Port != 9090 {
		t.Errorf("expected port=9090, got %d", c.Port)
	}
	if c.LogLevel != "DEBUG" {
		t.Errorf("expected log_level=DEBUG, got %s", c.LogLevel)
	}
}
