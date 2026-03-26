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

func TestExtractSection(t *testing.T) {
	tests := []struct {
		name    string
		heading string
		want    string
		empty   bool
	}{
		{
			name:    "exact match",
			heading: "Section 1",
			want:    "Some content here.",
		},
		{
			name:    "case insensitive",
			heading: "section 1",
			want:    "Some content here.",
		},
		{
			name:    "partial match",
			heading: "Section 2",
			want:    "More content.",
		},
		{
			name:    "not found",
			heading: "Nonexistent",
			empty:   true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractSection(testDoc, tc.heading)
			if tc.empty {
				if got != "" {
					t.Errorf("expected empty, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tc.want) {
				t.Errorf("expected section to contain %q, got %q", tc.want, got)
			}
		})
	}
}

func TestExtractSection_NestedHeadings(t *testing.T) {
	doc := "# Top\n\n## Parent\n\nParent text.\n\n### Child\n\nChild text.\n\n## Sibling\n\nSibling text.\n"
	section := ExtractSection(doc, "Parent")
	if !strings.Contains(section, "Child text.") {
		t.Error("section should include nested sub-headings")
	}
	if strings.Contains(section, "Sibling text.") {
		t.Error("section should stop at same-level heading")
	}
}
