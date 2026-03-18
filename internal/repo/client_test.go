package repo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/baniol/docs-mcp/internal/config"
)

func setupTestRepo(t *testing.T) (*Client, string) {
	t.Helper()
	dir := t.TempDir()
	docsDir := filepath.Join(dir, "docs")
	if err := os.Mkdir(docsDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Create test markdown files
	files := map[string]string{
		"README.md":     "# README\nThis is the readme.",
		"guide.md":      "# Guide\nThis is a guide.",
		"sub/nested.md": "# Nested\nNested document.",
	}
	for name, content := range files {
		path := filepath.Join(docsDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := &config.Config{
		RepoPath: dir,
		DocsPath: "docs",
	}
	// manually set docsPath for test
	c := &Client{
		cfg:      cfg,
		repoPath: dir,
		docsPath: docsDir,
	}
	return c, dir
}

func TestListDocs(t *testing.T) {
	c, _ := setupTestRepo(t)
	docs, err := c.ListDocs(true)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 3 {
		t.Errorf("expected 3 docs, got %d", len(docs))
	}
}

func TestGetDocContent(t *testing.T) {
	c, _ := setupTestRepo(t)
	content, err := c.GetDocContent("README.md")
	if err != nil {
		t.Fatal(err)
	}
	if content != "# README\nThis is the readme." {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestGetDocContent_PathTraversal(t *testing.T) {
	c, _ := setupTestRepo(t)
	_, err := c.GetDocContent("../../etc/passwd")
	if err == nil {
		t.Fatal("expected error for path traversal attempt")
	}
}

func TestGetDocContent_NotFound(t *testing.T) {
	c, _ := setupTestRepo(t)
	_, err := c.GetDocContent("nonexistent.md")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}
