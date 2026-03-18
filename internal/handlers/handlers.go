package handlers

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/baniol/docs-mcp/internal/config"
	"github.com/baniol/docs-mcp/internal/docproc"
	"github.com/baniol/docs-mcp/internal/repo"
	"github.com/baniol/docs-mcp/internal/search"
	"github.com/baniol/docs-mcp/internal/utils"
)

// RepoClient is the interface for accessing the documentation repository.
type RepoClient interface {
	ListDocs(includeSubdirs bool) ([]repo.DocumentInfo, error)
	GetDocContent(docPath string) (string, error)
	Sync() (bool, error)
}

// Handler wraps all dependencies for MCP tool handling.
type Handler struct {
	cfg      *config.Config
	repo     RepoClient
	searcher search.Searcher
	cache    *utils.Cache
}

func New(cfg *config.Config, repoClient RepoClient, searcher search.Searcher, cache *utils.Cache) *Handler {
	return &Handler{
		cfg:      cfg,
		repo:     repoClient,
		searcher: searcher,
		cache:    cache,
	}
}

// ListTools returns the available MCP tools.
func (h *Handler) ListTools() []Tool {
	return []Tool{
		{
			Name: "query_infrastructure_docs",
			Description: "Smart query tool for infrastructure documentation. " +
				"Automatically determines whether to search, read, list, or navigate based on your question. " +
				"Use this for natural language queries.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]SchemaProperty{
					"query": {
						Type:        "string",
						Description: "Your question or request in natural language (e.g., 'show me database docs', 'how to add S3 access', 'read the README', 'list all docs')",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "search_docs",
			Description: "Search documentation using BM25 full-text search. Returns ranked snippets — use this first to find relevant documents before reading them in full.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]SchemaProperty{
					"query": {
						Type:        "string",
						Description: "Search query",
					},
					"max_results": {
						Type:        "integer",
						Description: "Maximum number of results to return (default: 5, max: 20)",
					},
				},
				Required: []string{"query"},
			},
		},
		{
			Name:        "get_document",
			Description: "Read the full content of a documentation file by its path. Large documents are truncated — use get_section for a specific part.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]SchemaProperty{
					"path": {
						Type:        "string",
						Description: "Relative path to the document (e.g., 'common/curl.md')",
					},
				},
				Required: []string{"path"},
			},
		},
		{
			Name:        "get_section",
			Description: "Read a specific section of a document by heading name. More token-efficient than reading the full document when you need a specific part.",
			InputSchema: ToolInputSchema{
				Type: "object",
				Properties: map[string]SchemaProperty{
					"path": {
						Type:        "string",
						Description: "Relative path to the document (e.g., 'common/curl.md')",
					},
					"heading": {
						Type:        "string",
						Description: "Heading text to find (case-insensitive, partial match supported)",
					},
				},
				Required: []string{"path", "heading"},
			},
		},
		{
			Name:        "list_docs",
			Description: "List available documentation files. Returns up to 200 files — use search_docs to find specific documents in large repositories.",
			InputSchema: ToolInputSchema{
				Type:       "object",
				Properties: map[string]SchemaProperty{},
				Required:   []string{},
			},
		},
	}
}

// SmartQuery routes a natural language query to the appropriate handler.
func (h *Handler) SmartQuery(query string) []TextContent {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return text("Please provide a query. Examples:\n- 'list all docs'\n- 'show me database documentation'\n- 'read the README'\n- 'how to add S3 access'")
	}

	switch {
	case containsAny(q, "list", "show all", "what docs", "available docs"):
		return h.ListDocs(true)
	case containsAny(q, "navigation", "structure", "overview", "table of contents"):
		return h.GetNavigation()
	case containsAny(q, "read", "show me", "get me", "display") &&
		containsAny(q, "readme", "main", "overview"):
		if strings.Contains(q, "readme") {
			return h.ReadDoc("README.md", 0)
		}
		return h.SearchDocs(query, 5)
	default:
		maxResults := 5
		if len(strings.Fields(q)) > 3 {
			maxResults = 10
		}
		return h.SearchDocs(query, maxResults)
	}
}

// ListDocs returns a formatted list of all documentation files (capped at 200).
func (h *Handler) ListDocs(includeSubdirs bool) []TextContent {
	const maxList = 200
	cacheKey := fmt.Sprintf("list_docs_%v", includeSubdirs)
	if v, ok := h.cache.Get(cacheKey); ok {
		return text(v.(string))
	}

	docs, err := h.repo.ListDocs(includeSubdirs)
	if err != nil {
		slog.Error("list docs failed", "err", err)
		return text("Failed to list documentation files: " + err.Error())
	}
	if len(docs) == 0 {
		return text("No documentation files found.")
	}

	truncated := len(docs) > maxList
	if truncated {
		docs = docs[:maxList]
	}

	items := make([]utils.DocListItem, len(docs))
	for i, d := range docs {
		items[i] = utils.DocListItem{Name: d.Name, Path: d.Path, Size: d.Size}
	}

	header := fmt.Sprintf("Found %d documentation files", len(docs))
	if truncated {
		header += fmt.Sprintf(" (showing first %d — use search_docs to find specific documents)", maxList)
	}
	result := fmt.Sprintf("%s:\n\n%s", header,
		utils.FormatDocumentList(items, h.cfg.GithubRepo, h.cfg.GithubBranch, h.cfg.DocsPath, h.cfg.IncludeGithubLinks))
	h.cache.Set(cacheKey, result)
	return text(result)
}

// ReadDoc returns the content of a document, with smart truncation for large docs.
func (h *Handler) ReadDoc(docPath string, maxLength int) []TextContent {
	if docPath == "" {
		return text("Error: doc_path parameter is required")
	}
	if maxLength == 0 {
		maxLength = h.cfg.MaxDocumentLength
	}

	cacheKey := "doc_content_" + docPath
	if v, ok := h.cache.Get(cacheKey); ok {
		cached := v.(string)
		if len(cached) <= maxLength {
			return text(cached)
		}
	}

	content, err := h.repo.GetDocContent(docPath)
	if err != nil {
		slog.Error("read doc failed", "path", docPath, "err", err)
		return text(fmt.Sprintf("Failed to read document '%s': %s", docPath, err.Error()))
	}
	originalLen := len(content)

	if originalLen > h.cfg.LargeDocumentThreshold {
		content = h.smartTruncate(content, docPath, originalLen, maxLength)
	} else if originalLen > maxLength {
		content = simpleTruncate(content, maxLength, originalLen)
	}

	if h.cfg.IncludeGithubLinks {
		link := fmt.Sprintf("https://github.com/%s/blob/%s/%s/%s",
			h.cfg.GithubRepo, h.cfg.GithubBranch, h.cfg.DocsPath, docPath)
		content = fmt.Sprintf("[View on GitHub](%s)\n\n%s", link, content)
	}

	if originalLen <= h.cfg.MaxDocumentLength {
		h.cache.Set(cacheKey, content)
	}
	return text(content)
}

func (h *Handler) smartTruncate(content, docPath string, originalLen, maxLength int) string {
	toc := docproc.ExtractTableOfContents(content)
	summary := docproc.ExtractSummary(content, 500)

	var parts []string

	lines := strings.Split(content, "\n")
	for _, line := range lines[:min(10, len(lines))] {
		if strings.HasPrefix(strings.TrimSpace(line), "# ") {
			parts = append(parts, line)
			break
		}
	}

	if summary != "" {
		parts = append(parts, "", "## Summary", "", summary)
	}

	if len(toc) > 0 {
		parts = append(parts, "", "## Table of Contents", "")
		for _, h := range toc {
			if h.Level > 6 {
				continue
			}
			indent := strings.Repeat("  ", h.Level-1)
			parts = append(parts, fmt.Sprintf("%s- %s", indent, h.Text))
			if len(parts) > 25 {
				break
			}
		}
	}

	current := strings.Join(parts, "\n")
	remaining := maxLength - len(current)
	if remaining > 1000 {
		end := remaining - 200
		if end > len(content) {
			end = len(content)
		}
		preview := content[:end]
		if idx := strings.LastIndex(preview, ". "); idx > end*4/5 {
			preview = preview[:idx+1]
		}
		parts = append(parts, "", "## Content Preview", "", preview)
		if originalLen > end {
			parts = append(parts, "",
				fmt.Sprintf("*[Document truncated. Full document is %d characters. Use specific section queries to read more.]*",
					originalLen))
		}
	}

	return strings.Join(parts, "\n")
}

func simpleTruncate(content string, maxLength, originalLen int) string {
	cut := content[:maxLength]
	if idx := strings.LastIndex(cut, ". "); idx > maxLength*9/10 {
		cut = cut[:idx+1]
	}
	return cut + fmt.Sprintf("\n\n*[Document truncated at %d characters. Original length: %d characters.]*",
		maxLength, originalLen)
}

// SearchDocs searches the index and returns formatted results.
func (h *Handler) SearchDocs(query string, maxResults int) []TextContent {
	if query == "" {
		return text("Error: query parameter is required")
	}

	cacheKey := fmt.Sprintf("search_%s_%d", query, maxResults)
	if v, ok := h.cache.Get(cacheKey); ok {
		return text(v.(string))
	}

	results := h.searcher.Search(query, maxResults,
		h.cfg.SnippetSize, h.cfg.SnippetsPerResult)

	if len(results) == 0 {
		return text(fmt.Sprintf("No results found for '%s'", query))
	}

	items := make([]utils.SearchResultItem, len(results))
	for i, r := range results {
		items[i] = utils.SearchResultItem{
			Name:     r.Name,
			Path:     r.Path,
			Score:    r.Score,
			Snippets: r.Snippets,
		}
	}
	result := fmt.Sprintf("Search results for '%s':\n\n%s", query,
		utils.FormatSearchResults(items, h.cfg.GithubRepo, h.cfg.GithubBranch, h.cfg.DocsPath, h.cfg.IncludeGithubLinks))
	h.cache.Set(cacheKey, result)
	return text(result)
}

// GetSection returns a specific section of a document identified by heading text.
func (h *Handler) GetSection(docPath, heading string) []TextContent {
	if docPath == "" {
		return text("Error: path parameter is required")
	}
	if heading == "" {
		return text("Error: heading parameter is required")
	}

	content, err := h.repo.GetDocContent(docPath)
	if err != nil {
		slog.Error("get section failed", "path", docPath, "err", err)
		return text(fmt.Sprintf("Failed to read document '%s': %s", docPath, err.Error()))
	}

	section := docproc.ExtractSection(content, heading)
	if section == "" {
		toc := docproc.ExtractTableOfContents(content)
		var headings []string
		for _, h := range toc {
			headings = append(headings, h.Text)
		}
		if len(headings) > 0 {
			return text(fmt.Sprintf("Section '%s' not found in '%s'.\nAvailable headings: %s",
				heading, docPath, strings.Join(headings, ", ")))
		}
		return text(fmt.Sprintf("Section '%s' not found in '%s'", heading, docPath))
	}

	if h.cfg.IncludeGithubLinks {
		anchor := "#" + strings.ToLower(strings.ReplaceAll(strings.TrimSpace(heading), " ", "-"))
		link := fmt.Sprintf("https://github.com/%s/blob/%s/%s/%s%s",
			h.cfg.GithubRepo, h.cfg.GithubBranch, h.cfg.DocsPath, docPath, anchor)
		section = fmt.Sprintf("[View on GitHub](%s)\n\n%s", link, section)
	}
	return text(section)
}

// GetNavigation extracts and returns navigation from README.md.
func (h *Handler) GetNavigation() []TextContent {
	const cacheKey = "navigation"
	if v, ok := h.cache.Get(cacheKey); ok {
		return text(v.(string))
	}

	readme, err := h.repo.GetDocContent("README.md")
	if err != nil {
		slog.Error("get navigation failed", "err", err)
		return text("Failed to get navigation structure: " + err.Error())
	}

	nav := docproc.ExtractNavigationFromReadme(readme)

	var parts []string
	if nav.Overview != "" {
		parts = append(parts, "## Overview\n"+nav.Overview+"\n")
	}
	if len(nav.Files) > 0 {
		parts = append(parts, "## Available Documentation\n")
		for _, f := range nav.Files {
			entry := "- "
			if f.Link != "" {
				entry += fmt.Sprintf("[%s](%s)", f.Name, f.Link)
			} else {
				entry += f.Name
			}
			if f.Description != "" {
				entry += "\n  - " + f.Description
			}
			parts = append(parts, entry)
		}
	}

	result := strings.Join(parts, "\n")
	if result == "" {
		result = "No navigation structure found in README.md"
	}
	h.cache.Set(cacheKey, result)
	return text(result)
}

// BuildIndex indexes all documents in the repo.
func (h *Handler) BuildIndex() error {
	docs, err := h.repo.ListDocs(true)
	if err != nil {
		return fmt.Errorf("list docs: %w", err)
	}

	indexDocs := make([]search.IndexDoc, 0, len(docs))
	for _, d := range docs {
		content, err := h.repo.GetDocContent(d.Path)
		if err != nil {
			slog.Warn("skipping doc during indexing", "path", d.Path, "err", err)
			continue
		}
		indexDocs = append(indexDocs, search.IndexDoc{
			Path:    d.Path,
			Name:    d.Name,
			Content: content,
		})
	}
	h.searcher.Rebuild(indexDocs)
	slog.Info("index built", "docs", len(indexDocs))
	return nil
}

// SyncRepo pulls latest changes from the remote repository.
func (h *Handler) SyncRepo() error {
	_, err := h.repo.Sync()
	return err
}

// CallTool dispatches a tool call by name.
func (h *Handler) CallTool(name string, args map[string]any) ([]TextContent, error) {
	switch name {
	case "query_infrastructure_docs":
		query, _ := args["query"].(string)
		return h.SmartQuery(query), nil
	case "search_docs":
		query, _ := args["query"].(string)
		maxResults := 5
		if v, ok := args["max_results"].(float64); ok && v > 0 {
			maxResults = int(v)
			if maxResults > 20 {
				maxResults = 20
			}
		}
		return h.SearchDocs(query, maxResults), nil
	case "get_document":
		path, _ := args["path"].(string)
		return h.ReadDoc(path, 0), nil
	case "get_section":
		path, _ := args["path"].(string)
		heading, _ := args["heading"].(string)
		return h.GetSection(path, heading), nil
	case "list_docs":
		return h.ListDocs(true), nil
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// InvalidateCache clears all cached responses.
func (h *Handler) InvalidateCache() {
	h.cache.Clear()
}

func text(s string) []TextContent {
	return []TextContent{{Type: "text", Text: s}}
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
