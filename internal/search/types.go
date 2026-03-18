package search

// SearchResult holds a ranked search result.
type SearchResult struct {
	Path     string
	Name     string
	Score    float64
	Snippets []string
}

// Searcher is the interface for searching indexed documents.
type Searcher interface {
	// Index adds or updates a document in the index.
	Index(path, name, content string)
	// Search returns ranked results for a query.
	Search(query string, maxResults int, snippetSize int, snippetsPerResult int) []SearchResult
	// Rebuild clears the index and re-indexes all provided documents.
	Rebuild(docs []IndexDoc)
}

// IndexDoc is the input for batch indexing.
type IndexDoc struct {
	Path    string
	Name    string
	Content string
}
