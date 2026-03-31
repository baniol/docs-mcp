package search

import (
	"math"
	"sort"
	"strings"
	"sync"
)

const (
	bm25K1 = 1.5
	bm25B  = 0.75
)

type docEntry struct {
	path     string
	name     string
	content  string
	termFreq map[string]int
	length   int
}

// BM25Index implements the Searcher interface using BM25 ranking.
type BM25Index struct {
	mu        sync.RWMutex
	docs      map[string]*docEntry
	df        map[string]int // document frequency per term
	avgDocLen float64
}

func NewBM25Index() *BM25Index {
	return &BM25Index{
		docs: make(map[string]*docEntry),
		df:   make(map[string]int),
	}
}

func (b *BM25Index) Index(path, name, content string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.indexLocked(path, name, content)
}

func (b *BM25Index) indexLocked(path, name, content string) {
	// Remove old entry's df contributions
	if old, ok := b.docs[path]; ok {
		for term := range old.termFreq {
			b.df[term]--
			if b.df[term] <= 0 {
				delete(b.df, term)
			}
		}
	}

	tokens := tokenize(content)
	tf := make(map[string]int, len(tokens))
	for _, t := range tokens {
		tf[t]++
	}

	entry := &docEntry{
		path:     path,
		name:     name,
		content:  content,
		termFreq: tf,
		length:   len(tokens),
	}
	b.docs[path] = entry

	// Update df
	for term := range tf {
		b.df[term]++
	}

	b.recomputeAvgLen()
}

func (b *BM25Index) recomputeAvgLen() {
	if len(b.docs) == 0 {
		b.avgDocLen = 0
		return
	}
	total := 0
	for _, d := range b.docs {
		total += d.length
	}
	b.avgDocLen = float64(total) / float64(len(b.docs))
}

func (b *BM25Index) score(entry *docEntry, queryTerms []string) float64 {
	N := float64(len(b.docs))
	score := 0.0
	for _, term := range queryTerms {
		df := float64(b.df[term])
		if df == 0 {
			continue
		}
		idf := math.Log((N-df+0.5)/(df+0.5) + 1)
		tf := float64(entry.termFreq[term])
		dl := float64(entry.length)
		numerator := tf * (bm25K1 + 1)
		denominator := tf + bm25K1*(1-bm25B+bm25B*dl/b.avgDocLen)
		score += idf * (numerator / denominator)
	}
	// Boost for name/path matches
	nameLower := strings.ToLower(entry.name)
	pathLower := strings.ToLower(entry.path)
	for _, term := range queryTerms {
		if strings.Contains(nameLower, term) {
			score += 3.0
		} else if strings.Contains(pathLower, term) {
			score += 1.5
		}
	}
	return score
}

func (b *BM25Index) Search(query string, maxResults, snippetSize, snippetsPerResult int) []SearchResult {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.docs) == 0 {
		return nil
	}
	queryTerms := tokenize(query)
	if len(queryTerms) == 0 {
		return nil
	}

	type scored struct {
		entry *docEntry
		score float64
	}
	var candidates []scored
	for _, entry := range b.docs {
		s := b.score(entry, queryTerms)
		if s > 0 {
			candidates = append(candidates, scored{entry, s})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	if maxResults < len(candidates) {
		candidates = candidates[:maxResults]
	}

	results := make([]SearchResult, 0, len(candidates))
	for _, c := range candidates {
		snippets := extractSnippets(c.entry.content, queryTerms, snippetSize, snippetsPerResult)
		results = append(results, SearchResult{
			Path:     c.entry.path,
			Name:     c.entry.name,
			Score:    c.score,
			Snippets: snippets,
		})
	}
	return results
}

func (b *BM25Index) Rebuild(docs []IndexDoc) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.docs = make(map[string]*docEntry)
	b.df = make(map[string]int)
	b.avgDocLen = 0
	for _, d := range docs {
		b.indexLocked(d.Path, d.Name, d.Content)
	}
}

var stopWords = map[string]bool{
	"the": true, "a": true, "an": true, "and": true, "or": true,
	"but": true, "in": true, "on": true, "at": true, "to": true,
	"for": true, "of": true, "with": true, "by": true, "is": true,
	"it": true, "be": true, "as": true, "are": true, "was": true,
}

// tokenize lowercases and splits text into words, removing stop words.
func tokenize(text string) []string {
	lower := strings.ToLower(text)
	fields := strings.FieldsFunc(lower, func(r rune) bool {
		return !('a' <= r && r <= 'z') && !('0' <= r && r <= '9') && r != '-' && r != '_'
	})

	var tokens []string
	for _, f := range fields {
		if len(f) > 1 && !stopWords[f] {
			tokens = append(tokens, f)
		}
	}
	return tokens
}

func extractSnippets(content string, queryTerms []string, snippetSize, maxSnippets int) []string {
	lower := strings.ToLower(content)
	type pos struct{ start, end int }
	var positions []pos

	for _, term := range queryTerms {
		idx := 0
		for {
			p := strings.Index(lower[idx:], term)
			if p < 0 {
				break
			}
			abs := idx + p
			positions = append(positions, pos{abs, abs + len(term)})
			idx = abs + 1
		}
	}

	if len(positions) == 0 {
		// Return beginning of content
		end := snippetSize
		if end > len(content) {
			end = len(content)
		}
		return []string{content[:end]}
	}

	// Sort by position
	sort.Slice(positions, func(i, j int) bool { return positions[i].start < positions[j].start })

	var snippets []string
	usedRanges := make([]pos, 0)

	for _, p := range positions {
		if len(snippets) >= maxSnippets {
			break
		}
		ctx := (snippetSize - (p.end - p.start)) / 2
		sStart := p.start - ctx
		if sStart < 0 {
			sStart = 0
		}
		sEnd := p.end + ctx
		if sEnd > len(content) {
			sEnd = len(content)
		}

		// Check overlap with already used ranges
		overlaps := false
		for _, used := range usedRanges {
			if !(sEnd <= used.start || sStart >= used.end) {
				overlaps = true
				break
			}
		}
		if overlaps {
			continue
		}

		snippet := strings.TrimSpace(content[sStart:sEnd])
		if sStart > 0 {
			snippet = "..." + snippet
		}
		if sEnd < len(content) {
			snippet += "..."
		}
		snippets = append(snippets, snippet)
		usedRanges = append(usedRanges, pos{sStart, sEnd})
	}
	return snippets
}
