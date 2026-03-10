package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Alinaswe3/mock-lipila/internal/database"
)

type contextKey string

const (
	apiKeyContextKey   contextKey = "api_key"
	walletIDContextKey contextKey = "wallet_id"
)

// LoggingMiddleware logs method, path, client IP, status, and duration for each request.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.Printf("[%s] %s %s %s %d %s",
			start.Format("2006-01-02 15:04:05"),
			r.Method,
			r.URL.Path,
			clientIP(r),
			sw.status,
			time.Since(start),
		)
	})
}

// AuthMiddleware validates the x-api-key header against wallet API keys.
// 401 for missing/invalid key, 403 for inactive wallet.
func AuthMiddleware(db *database.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("x-api-key")
			if key == "" {
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error": "UNAUTHORIZED",
					"message": "missing x-api-key header",
				})
				return
			}

			if !strings.HasPrefix(key, "Lsk") {
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error": "UNAUTHORIZED",
					"message": "invalid API key format",
				})
				return
			}

			wallet, err := db.GetWalletByAPIKey(key)
			if err != nil {
				writeJSON(w, http.StatusUnauthorized, map[string]string{
					"error": "UNAUTHORIZED",
					"message": "API key not found",
				})
				return
			}

			if !wallet.IsActive {
				writeJSON(w, http.StatusForbidden, map[string]string{
					"error": "FORBIDDEN",
					"message": "API key is inactive or revoked",
				})
				return
			}

			ctx := context.WithValue(r.Context(), apiKeyContextKey, key)
			ctx = context.WithValue(ctx, walletIDContextKey, wallet.ID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RateLimitMiddleware enforces max 100 requests per minute per API key.
func RateLimitMiddleware(next http.Handler) http.Handler {
	var mu sync.Mutex
	type bucket struct {
		count  int
		window time.Time
	}
	clients := make(map[string]*bucket)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("x-api-key")
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}

		mu.Lock()
		now := time.Now()
		b, exists := clients[key]
		if !exists || now.Sub(b.window) > time.Minute {
			clients[key] = &bucket{count: 1, window: now}
			mu.Unlock()
			next.ServeHTTP(w, r)
			return
		}

		b.count++
		if b.count > 100 {
			mu.Unlock()
			w.Header().Set("Retry-After", "60")
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error":   "TOO_MANY_REQUESTS",
				"message": "rate limit exceeded, max 100 requests per minute",
			})
			return
		}
		mu.Unlock()

		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware allows all origins for testing.
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, x-api-key, callbackUrl")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// JSONContentType sets the Content-Type header to application/json.
func JSONContentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// --- helpers ---

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		return strings.SplitN(fwd, ",", 2)[0]
	}
	return r.RemoteAddr
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
