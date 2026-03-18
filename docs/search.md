# Search Architecture

## Overview

The server uses a `Searcher` interface (`internal/search/types.go`) that decouples the search implementation from the rest of the codebase. Swapping the search backend requires only a new implementation of this interface — no changes to handlers, server, or syncer.

```go
type Searcher interface {
    Index(path, name, content string)
    Search(query string, maxResults int, snippetSize int, snippetsPerResult int) []SearchResult
    Rebuild(docs []IndexDoc)
}
```

---

## Current Implementation: BM25

**File:** `internal/search/bm25.go`

BM25 (Best Match 25) is a classical information retrieval algorithm used by Elasticsearch and Solr as their default ranking function. It scores documents based on term frequency and inverse document frequency, with length normalization.

### How it works

1. At startup, all markdown files are chunked into ~800 character overlapping segments
2. Each chunk is tokenized (lowercased, stop words removed)
3. At query time, BM25 scores every document against query terms
4. Top N documents are returned with relevant snippets

Name and path matches receive a score boost (3x and 1.5x respectively).

### Strengths

- Zero external dependencies, zero network calls
- Works offline, no API keys, no IAM roles needed
- Fast — pure in-memory computation
- Excellent for exact technical terms: `terraform`, `S3`, `EKS`, `helm`, `ingress`, service names, environment names

### Weaknesses

- No semantic understanding — query `"połącz z bazą"` won't match `"database connection string"`
- Language mismatch — Polish queries against English docs return weak results
- Synonyms not handled — `"storage"` won't match `"S3 bucket"` unless both words appear

### When it's sufficient

For infrastructure documentation queried by engineers who know the technical terminology, BM25 performs comparably to semantic search. The vocabulary is precise and consistent — people search for `"RDS snapshot"` not `"how do I back up my relational database"`.

---

## Optional Upgrade: AWS Bedrock Embeddings

**Status:** Not implemented. Interface is ready, implementation is ~150 lines.

### How it would work

1. On indexing, each document chunk is sent to Bedrock to get a vector embedding (1536 dimensions)
2. Embeddings are stored in memory (and optionally persisted to disk)
3. At query time, the query is embedded and cosine similarity is computed against all chunk embeddings
4. Documents are ranked by their best matching chunk

### Recommended model

`amazon.titan-embed-text-v2` — good quality, low latency, available in most regions.

Alternative: `cohere.embed-english-v3` — slightly better quality for English technical text.

### Configuration

```bash
AWS_REGION=eu-west-1
USE_BEDROCK=true
# No API keys needed — uses IAM role attached to the k8s pod (IRSA)
```

### IAM policy required

```json
{
  "Effect": "Allow",
  "Action": "bedrock:InvokeModel",
  "Resource": "arn:aws:bedrock:eu-west-1::foundation-model/amazon.titan-embed-text-v2:0"
}
```

### Trade-offs vs BM25

| | BM25 | Bedrock |
|---|---|---|
| Setup | zero | IAM role + region |
| Latency (indexing) | ~1s for 100 docs | ~30s for 100 docs (API calls) |
| Latency (query) | <1ms | ~100ms (1 API call) |
| AWS cost | free | ~$0.001 per full reindex |
| Semantic queries | no | yes |
| Works offline | yes | no |
| Docker image size | no change | no change (AWS SDK ~10MB) |

### When to upgrade

Switch to Bedrock if in practice you observe:
- Queries that clearly describe a concept but don't match exact keywords in the docs
- Team members using Polish queries against English documentation
- Low relevance scores on valid queries

### Implementation sketch

```go
// internal/search/bedrock.go
type BedrockSearcher struct {
    client    *bedrockruntime.Client
    modelID   string
    documents map[string]docEntry     // path -> content + embeddings
    chunkSize int
}

func (b *BedrockSearcher) embed(text string) ([]float32, error) {
    // POST to Bedrock, parse response, return vector
}

func (b *BedrockSearcher) Search(query string, maxResults, snippetSize, snippetsPerResult int) []SearchResult {
    queryVec, _ := b.embed(query)
    // cosine similarity against all chunk embeddings
    // group by document, return top N
}
```

The rest of `main.go` wiring would be:

```go
if cfg.UseBedrock {
    searcher = search.NewBedrockSearcher(cfg)
} else {
    searcher = search.NewBM25Index(cfg.ChunkSize, cfg.ChunkOverlap)
}
```
