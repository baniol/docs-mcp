package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/baniol/docs-mcp/internal/config"
	"github.com/baniol/docs-mcp/internal/handlers"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server is the HTTP server.
type Server struct {
	cfg     *config.Config
	handler *handlers.Handler
	http    *http.Server
}

func New(cfg *config.Config, h *handlers.Handler) *Server {
	s := &Server{cfg: cfg, handler: h}
	s.http = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      s.routes(),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	return s
}

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	authMiddleware := apiKeyMiddleware(s.cfg.APIKeys)

	r.Get("/health", s.healthHandler)
	r.With(authMiddleware).Post("/mcp", s.mcpHandler)
	r.With(authMiddleware).Post("/mcp/query", s.queryHandler)
	r.With(authMiddleware).Post("/mcp/tools/list", s.toolsListHandler)
	r.Post("/webhook/github", s.webhookHandler)

	return r
}

func (s *Server) Start() error {
	slog.Info("server listening", "addr", s.http.Addr)
	return s.http.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// healthHandler — GET /health
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"service": "docs-mcp",
	})
}

// toolsListHandler — POST /mcp/tools/list
func (s *Server) toolsListHandler(w http.ResponseWriter, r *http.Request) {
	tools := s.handler.ListTools()
	writeJSON(w, http.StatusOK, map[string]any{"tools": tools})
}

// queryHandler — POST /mcp/query  (convenience endpoint)
func (s *Server) queryHandler(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query string `json:"query"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}
	content := s.handler.SmartQuery(body.Query)
	writeJSON(w, http.StatusOK, map[string]any{
		"query":   body.Query,
		"content": content,
	})
}

// mcpHandler — POST /mcp  (JSON-RPC 2.0)
func (s *Server) mcpHandler(w http.ResponseWriter, r *http.Request) {
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusOK, errResponse(nil, -32700, "parse error"))
		return
	}

	slog.Info("mcp request", "method", req.Method, "id", req.ID)

	switch req.Method {
	case "initialize":
		writeJSON(w, http.StatusOK, okResponse(req.ID, map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities": map[string]any{
				"tools":     map[string]any{"listChanged": true},
				"resources": map[string]any{},
				"prompts":   map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "docs-mcp",
				"version": "1.0.0",
			},
		}))

	case "tools/list":
		tools := s.handler.ListTools()
		writeJSON(w, http.StatusOK, okResponse(req.ID, map[string]any{"tools": tools}))

	case "tools/call":
		params := parseToolsCallParams(req.Params)
		if params == nil {
			writeJSON(w, http.StatusOK, errResponse(req.ID, -32602, "invalid params"))
			return
		}
		content, err := s.handler.CallTool(params.Name, params.Arguments)
		if err != nil {
			writeJSON(w, http.StatusOK, errResponse(req.ID, -32601, err.Error()))
			return
		}
		writeJSON(w, http.StatusOK, okResponse(req.ID, map[string]any{"content": content}))

	case "notifications/initialized":
		// Notification — no response body needed
		w.WriteHeader(http.StatusOK)

	default:
		writeJSON(w, http.StatusOK, errResponse(req.ID, -32601, "unknown method: "+req.Method))
	}
}

// webhookHandler — POST /webhook/github
func (s *Server) webhookHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}

	if s.cfg.WebhookSecret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if !verifySignature(body, s.cfg.WebhookSecret, sig) {
			http.Error(w, "invalid signature", http.StatusUnauthorized)
			return
		}
	}

	event := r.Header.Get("X-GitHub-Event")
	if event != "push" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "event": event})
		return
	}

	var payload struct {
		Ref     string `json:"ref"`
		Commits []struct {
			Modified []string `json:"modified"`
			Added    []string `json:"added"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	if branch != s.cfg.GithubBranch {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "wrong branch"})
		return
	}

	docsChanged := false
	for _, commit := range payload.Commits {
		for _, f := range append(commit.Modified, commit.Added...) {
			if strings.Contains(f, s.cfg.DocsPath) {
				docsChanged = true
				break
			}
		}
	}
	if !docsChanged {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ignored", "reason": "no doc changes"})
		return
	}

	slog.Info("webhook: docs changed, triggering sync+rebuild")
	go func() {
		if err := s.handler.SyncRepo(); err != nil {
			slog.Error("webhook sync failed", "err", err)
		}
		if err := s.handler.BuildIndex(); err != nil {
			slog.Error("webhook rebuild failed", "err", err)
		}
		s.handler.InvalidateCache()
	}()

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "message": "rebuild triggered"})
}

func verifySignature(body []byte, secret, header string) bool {
	if !strings.HasPrefix(header, "sha256=") {
		return false
	}
	got := header[7:]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(got), []byte(expected))
}

func parseToolsCallParams(raw any) *ToolsCallParams {
	b, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var p ToolsCallParams
	if err := json.Unmarshal(b, &p); err != nil {
		return nil
	}
	return &p
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
