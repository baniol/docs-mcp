package utils

import "fmt"

func FormatFileSize(bytes int64) string {
	switch {
	case bytes < 1024:
		return fmt.Sprintf("%d B", bytes)
	case bytes < 1024*1024:
		return fmt.Sprintf("%.1f KB", float64(bytes)/1024)
	case bytes < 1024*1024*1024:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1024*1024))
	default:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1024*1024*1024))
	}
}

// DocListItem represents a document for formatting.
type DocListItem struct {
	Name        string
	Path        string
	Size        int64
	Description string
}

func FormatDocumentList(docs []DocListItem, githubRepo, branch, docsPath string, includeLinks bool) string {
	if len(docs) == 0 {
		return "No documents found."
	}
	var buf []byte
	for i, doc := range docs {
		if i > 0 {
			buf = append(buf, '\n')
		}
		line := fmt.Sprintf("* **%s** (%s)", doc.Name, FormatFileSize(doc.Size))
		if doc.Description != "" {
			line += " - " + doc.Description
		}
		buf = append(buf, line...)
		if includeLinks {
			link := fmt.Sprintf("\n  [View on GitHub](https://github.com/%s/blob/%s/%s/%s)", githubRepo, branch, docsPath, doc.Path)
			buf = append(buf, link...)
		}
	}
	return string(buf)
}

// SearchResultItem represents a search result for formatting.
type SearchResultItem struct {
	Name     string
	Path     string
	Score    float64
	Snippets []string
}

func FormatSearchResults(results []SearchResultItem, githubRepo, branch, docsPath string, includeLinks bool) string {
	if len(results) == 0 {
		return "No results found."
	}
	var buf []byte
	for i, r := range results {
		if i > 0 {
			buf = append(buf, '\n')
		}
		line := fmt.Sprintf("%d. **%s** (%s)", i+1, r.Name, r.Path)
		buf = append(buf, line...)
		if includeLinks {
			link := fmt.Sprintf("\n   https://github.com/%s/blob/%s/%s/%s", githubRepo, branch, docsPath, r.Path)
			buf = append(buf, link...)
		}
		for _, s := range r.Snippets {
			buf = append(buf, "\n   "...)
			buf = append(buf, s...)
		}
	}
	return string(buf)
}
