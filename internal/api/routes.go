package api

import (
	"net/http"

	"github.com/Alinaswe3/mock-lipila/internal/database"
	"github.com/Alinaswe3/mock-lipila/internal/simulator"
)

// NewRouter creates and returns the API router with all routes registered.
func NewRouter(db *database.DB, sim *simulator.Simulator) http.Handler {
	handlers := &Handlers{DB: db, Sim: sim}

	mux := http.NewServeMux()

	// API endpoints (authenticated + rate limited)
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/api/v1/collections/mobile-money", handlers.HandleMobileMoneyCollection)
	apiMux.HandleFunc("/api/v1/collections/card", handlers.HandleCardCollection)
	apiMux.HandleFunc("/api/v1/collections/check-status", handlers.HandleCheckCollectionStatus)
	apiMux.HandleFunc("/api/v1/disbursements/mobile-money", handlers.HandleMobileMoneyDisbursement)
	apiMux.HandleFunc("/api/v1/disbursements/bank", handlers.HandleBankDisbursement)
	apiMux.HandleFunc("/api/v1/disbursements/check-status", handlers.HandleCheckDisbursementStatus)
	apiMux.HandleFunc("/api/v1/merchants/balance", handlers.HandleMerchantBalance)

	// Middleware chain for API routes: rate limit -> auth -> body limit -> JSON content type
	authed := RateLimitMiddleware(AuthMiddleware(db)(LimitRequestBody(JSONContentType(apiMux))))
	mux.Handle("/api/", authed)

	// Health check (no auth required)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Wrap everything with CORS and logging
	return LoggingMiddleware(CORSMiddleware(mux))
}
