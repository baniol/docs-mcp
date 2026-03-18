package search

import (
	"testing"
)

func TestBM25_BasicSearch(t *testing.T) {
	idx := NewBM25Index(800, 100)
	idx.Index("doc1.md", "Kubernetes Guide", "Kubernetes is an orchestration tool for containers. Pods are the basic unit.")
	idx.Index("doc2.md", "Database Guide", "PostgreSQL is a relational database. It supports ACID transactions.")
	idx.Index("doc3.md", "Networking", "VPN configuration and firewall rules for the infrastructure.")

	results := idx.Search("kubernetes containers", 5, 200, 1)
	if len(results) == 0 {
		t.Fatal("expected results for 'kubernetes containers'")
	}
	if results[0].Path != "doc1.md" {
		t.Errorf("expected doc1.md as top result, got %s", results[0].Path)
	}
}

func TestBM25_Ranking(t *testing.T) {
	idx := NewBM25Index(800, 100)
	// doc1 has more occurrences of "database"
	idx.Index("doc1.md", "Database Deep Dive", "database database database PostgreSQL database configuration database")
	idx.Index("doc2.md", "Quick Intro", "This mentions database once")
	idx.Index("doc3.md", "Unrelated", "Something completely different about networking")

	results := idx.Search("database", 5, 200, 1)
	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}
	if results[0].Path != "doc1.md" {
		t.Errorf("expected doc1.md ranked first, got %s", results[0].Path)
	}
}

func TestBM25_NoResults(t *testing.T) {
	idx := NewBM25Index(800, 100)
	idx.Index("doc1.md", "Guide", "Some content about infrastructure")

	results := idx.Search("zzzyyyxxx", 5, 200, 1)
	if len(results) != 0 {
		t.Errorf("expected 0 results for nonsense query, got %d", len(results))
	}
}

func TestBM25_Rebuild(t *testing.T) {
	idx := NewBM25Index(800, 100)
	idx.Index("old.md", "Old", "old content")

	idx.Rebuild([]IndexDoc{
		{Path: "new.md", Name: "New", Content: "new content"},
	})

	results := idx.Search("old", 5, 200, 1)
	if len(results) != 0 {
		t.Error("expected old doc to be gone after rebuild")
	}
	results = idx.Search("new", 5, 200, 1)
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'new', got %d", len(results))
	}
}

func TestChunkDocument(t *testing.T) {
	content := "Para one.\n\nPara two.\n\nPara three.\n\nPara four.\n\nPara five."
	chunks := ChunkDocument(content, 30, 10)
	if len(chunks) < 2 {
		t.Errorf("expected multiple chunks for long content, got %d", len(chunks))
	}
	for _, c := range chunks {
		if c.Text == "" {
			t.Error("got empty chunk")
		}
	}
}
