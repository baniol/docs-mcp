package server

import (
	"net/http"
)

// apiKeyMiddleware checks Bearer token against allowed keys.
// If no keys are configured, all requests are allowed.
func apiKeyMiddleware(keys []string) func(http.Handler) http.Handler {
	keySet := make(map[string]bool, len(keys))
	for _, k := range keys {
		if k != "" {
			keySet[k] = true
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(keySet) == 0 {
				next.ServeHTTP(w, r)
				return
			}
			auth := r.Header.Get("Authorization")
			if len(auth) > 7 && auth[:7] == "Bearer " {
				token := auth[7:]
				if keySet[token] {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
}

// corsMiddleware adds permissive CORS headers.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
