package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/baniol/docs-mcp/internal/config"
	"github.com/baniol/docs-mcp/internal/search"
	"github.com/baniol/docs-mcp/internal/utils"
)

// mockSearcher allows controlling search results in tests.
type mockSearcher struct {
	results []search.SearchResult
	rebuilt []search.IndexDoc
}

func (m *mockSearcher) Index(path, name, content string) {}
func (m *mockSearcher) Search(query string, maxResults, snippetSize, snippetsPerResult int) []search.SearchResult {
	return m.results
}
func (m *mockSearcher) Rebuild(docs []search.IndexDoc) { m.rebuilt = docs }

func testConfig() *config.Config {
	return &config.Config{
		GithubRepo:             "test/repo",
		GithubBranch:           "master",
		DocsPath:               "docs",
		IncludeGithubLinks:     false,
		SnippetSize:            200,
		MaxSnippetSize:         400,
		SnippetsPerResult:      2,
		MaxDocumentLength:      8000,
		LargeDocumentThreshold: 10000,
	}
}

func TestSmartQuery_List(t *testing.T) {
	// Test SearchDocs routing
	ms := &mockSearcher{results: []search.SearchResult{
		{Path: "doc.md", Name: "Doc", Score: 1.5, Snippets: []string{"snippet"}},
	}}
	h2 := &Handler{
		cfg:      testConfig(),
		searcher: ms,
		cache:    utils.NewCache(time.Minute),
	}
	result := h2.SearchDocs("kubernetes", 5)
	if len(result) == 0 || result[0].Type != "text" {
		t.Error("expected text result")
	}
	if !strings.Contains(result[0].Text, "kubernetes") {
		t.Errorf("expected query in result, got: %s", result[0].Text)
	}
}

func TestSmartQuery_EmptyQuery(t *testing.T) {
	h := &Handler{
		cfg:      testConfig(),
		searcher: &mockSearcher{},
		cache:    utils.NewCache(time.Minute),
	}
	result := h.SmartQuery("")
	if len(result) == 0 {
		t.Fatal("expected result")
	}
	if !strings.Contains(result[0].Text, "Please provide") {
		t.Errorf("expected help text, got: %s", result[0].Text)
	}
}

func TestSearchDocs_NoResults(t *testing.T) {
	h := &Handler{
		cfg:      testConfig(),
		searcher: &mockSearcher{results: nil},
		cache:    utils.NewCache(time.Minute),
	}
	result := h.SearchDocs("unknownterm", 5)
	if !strings.Contains(result[0].Text, "No results found") {
		t.Errorf("expected 'No results found', got: %s", result[0].Text)
	}
}

func TestSearchDocs_Caching(t *testing.T) {
	calls := 0
	ms := &mockSearcher{}
	ms.results = []search.SearchResult{{Path: "a.md", Name: "A", Score: 1.0}}

	h := &Handler{
		cfg:      testConfig(),
		searcher: ms,
		cache:    utils.NewCache(time.Minute),
	}

	h.SearchDocs("test", 5)
	calls++
	// Second call should hit cache
	ms.results = nil // change results to confirm cache is used
	result := h.SearchDocs("test", 5)
	_ = calls
	if strings.Contains(result[0].Text, "No results") {
		t.Error("second call should use cached result, not re-search")
	}
}

func TestListTools(t *testing.T) {
	h := &Handler{cfg: testConfig(), searcher: &mockSearcher{}, cache: utils.NewCache(time.Minute)}
	tools := h.ListTools()

	wantNames := []string{"query_infrastructure_docs", "search_docs", "get_document", "get_section", "list_docs"}
	if len(tools) != len(wantNames) {
		t.Fatalf("expected %d tools, got %d", len(wantNames), len(tools))
	}
	for i, name := range wantNames {
		if tools[i].Name != name {
			t.Errorf("tools[%d]: expected %s, got %s", i, name, tools[i].Name)
		}
	}
}
