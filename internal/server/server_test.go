package server

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/baniol/docs-mcp/internal/config"
	"github.com/baniol/docs-mcp/internal/handlers"
	"github.com/baniol/docs-mcp/internal/search"
	"github.com/baniol/docs-mcp/internal/utils"
)

type mockSearcher struct{}

func (m *mockSearcher) Index(path, name, content string) {}
func (m *mockSearcher) Rebuild(docs []search.IndexDoc)   {}
func (m *mockSearcher) Search(q string, max, ss, spr int) []search.SearchResult {
	return []search.SearchResult{{Path: "doc.md", Name: "Doc", Score: 1.0, Snippets: []string{"snippet"}}}
}

func testServer() *Server {
	cfg := &config.Config{
		GithubRepo:             "test/repo",
		GithubBranch:           "master",
		DocsPath:               "docs",
		IncludeGithubLinks:     false,
		SnippetSize:            200,
		SnippetsPerResult:      2,
		MaxDocumentLength:      8000,
		LargeDocumentThreshold: 10000,
		Port:                   8099,
		WebhookSecret:          "secret",
	}
	h := handlers.New(cfg, nil, &mockSearcher{}, utils.NewCache(time.Minute))
	return New(cfg, h)
}

func TestHealthEndpoint(t *testing.T) {
	s := testServer()
	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var body map[string]any
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ok" {
		t.Errorf("expected status=ok, got %v", body["status"])
	}
}

func TestMCPInitialize(t *testing.T) {
	s := testServer()
	payload := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`
	r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(payload))
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != nil {
		t.Errorf("unexpected error: %+v", resp.Error)
	}
}

func TestMCPToolsList(t *testing.T) {
	s := testServer()
	payload := `{"jsonrpc":"2.0","id":2,"method":"tools/list"}`
	r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(payload))
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMCPUnknownMethod(t *testing.T) {
	s := testServer()
	payload := `{"jsonrpc":"2.0","id":3,"method":"unknown/method"}`
	r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(payload))
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)

	var resp Response
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error == nil {
		t.Error("expected error for unknown method")
	}
	if resp.Error.Code != -32601 {
		t.Errorf("expected code -32601, got %d", resp.Error.Code)
	}
}

func TestAPIKeyMiddleware_Missing(t *testing.T) {
	cfg := &config.Config{
		GithubRepo: "test/repo",
		APIKeys:    []string{"secret-key"},
		Port:       8099,
	}
	h := handlers.New(cfg, nil, &mockSearcher{}, utils.NewCache(time.Minute))
	s := New(cfg, h)

	r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestAPIKeyMiddleware_Valid(t *testing.T) {
	cfg := &config.Config{
		GithubRepo:             "test/repo",
		APIKeys:                []string{"secret-key"},
		Port:                   8099,
		SnippetSize:            200,
		SnippetsPerResult:      2,
		MaxDocumentLength:      8000,
		LargeDocumentThreshold: 10000,
	}
	h := handlers.New(cfg, nil, &mockSearcher{}, utils.NewCache(time.Minute))
	s := New(cfg, h)

	r := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/list"}`))
	r.Header.Set("Authorization", "Bearer secret-key")
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestWebhookSignatureVerification(t *testing.T) {
	body := []byte(`{"ref":"refs/heads/master","commits":[]}`)
	secret := "topsecret"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if !verifySignature(body, secret, sig) {
		t.Error("valid signature should verify")
	}
	if verifySignature(body, secret, "sha256=badhash") {
		t.Error("bad signature should not verify")
	}
}

func TestWebhookEndpoint_WrongBranch(t *testing.T) {
	s := testServer()
	payload := `{"ref":"refs/heads/feature","commits":[]}`
	mac := hmac.New(sha256.New, []byte("secret"))
	mac.Write([]byte(payload))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	r := httptest.NewRequest(http.MethodPost, "/webhook/github", bytes.NewBufferString(payload))
	r.Header.Set("X-GitHub-Event", "push")
	r.Header.Set("X-Hub-Signature-256", sig)
	w := httptest.NewRecorder()
	s.routes().ServeHTTP(w, r)

	var body map[string]string
	json.NewDecoder(w.Body).Decode(&body)
	if body["status"] != "ignored" {
		t.Errorf("expected ignored, got %v", body)
	}
}
