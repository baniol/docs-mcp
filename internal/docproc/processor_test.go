package docproc

import (
	"strings"
	"testing"
)

const testReadme = `# Infrastructure Docs

This repository contains infrastructure documentation.

## Navigation

| Document | Description |
|----------|-------------|
| README.md | Main readme |
| guide.md | Setup guide |

## Other Section
`

const testDoc = `# My Document

This is the first paragraph describing the document.

## Section 1

Some content here.

## Section 2

More content.
`

func TestExtractNavigationFromReadme(t *testing.T) {
	nav := ExtractNavigationFromReadme(testReadme)
	if nav.Overview == "" {
		t.Error("expected non-empty overview")
	}
	if len(nav.Files) < 2 {
		t.Errorf("expected at least 2 files, got %d", len(nav.Files))
	}
}

func TestExtractTableOfContents(t *testing.T) {
	toc := ExtractTableOfContents(testDoc)
	if len(toc) != 3 {
		t.Errorf("expected 3 headings, got %d", len(toc))
	}
	if toc[0].Level != 1 || toc[0].Text != "My Document" {
		t.Errorf("unexpected first heading: %+v", toc[0])
	}
}

func TestExtractSummary(t *testing.T) {
	summary := ExtractSummary(testDoc, 500)
	if !strings.Contains(summary, "first paragraph") {
		t.Errorf("expected summary to contain first paragraph text, got: %q", summary)
	}
}

func TestExtractDocumentMetadata(t *testing.T) {
	meta := ExtractDocumentMetadata(testDoc, "my_document.md")
	if meta.Title != "My Document" {
		t.Errorf("expected title 'My Document', got %q", meta.Title)
	}
	if meta.Description == "" {
		t.Error("expected non-empty description")
	}
	if meta.WordCount == 0 {
		t.Error("expected non-zero word count")
	}
}

func TestExtractSnippet(t *testing.T) {
	content := "abcdefghijklmnopqrstuvwxyz"
	snippet := ExtractSnippet(content, 10, 10)
	if len(snippet) > 20 { // 10 + possible "..."
		t.Errorf("snippet too long: %q", snippet)
	}
}

func TestSearchContent(t *testing.T) {
	content := "The quick brown fox jumps over the lazy dog"
	snippet := SearchContent(content, "fox", 20)
	if !strings.Contains(strings.ToLower(snippet), "fox") {
		t.Errorf("expected snippet to contain 'fox', got: %q", snippet)
	}
}
