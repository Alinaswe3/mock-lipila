package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Alinaswe3/mock-lipila/internal/admin"
	"github.com/Alinaswe3/mock-lipila/internal/api"
	"github.com/Alinaswe3/mock-lipila/internal/database"
	"github.com/Alinaswe3/mock-lipila/internal/simulator"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "lipila.db", "SQLite database path")
	flag.Parse()

	// Environment variables override flags
	if port := os.Getenv("PORT"); port != "" {
		*addr = ":" + port
	}
	if dp := os.Getenv("DB_PATH"); dp != "" {
		*dbPath = dp
	}

	// ── Database ──────────────────────────────────────────────
	log.Println("opening database:", *dbPath)
	db, err := database.New(*dbPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer func() {
		log.Println("closing database connection")
		db.Close()
	}()

	// Run migrations (create tables, seed config)
	if err := db.InitDB(); err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	log.Println("database initialized")

	// Seed a default test wallet if none exist
	wallet, created, err := db.SeedDefaultWallet()
	if err != nil {
		log.Fatalf("failed to seed default wallet: %v", err)
	}
	if created {
		log.Printf("new test wallet created — API key: %s", wallet.APIKey)
	} else {
		log.Printf("existing wallet found: %s (till: %s)", wallet.Name, wallet.TillNumber)
	}

	// ── Simulator ─────────────────────────────────────────────
	sim := simulator.NewSimulator(db)

	// ── Routes ────────────────────────────────────────────────
	mux := http.NewServeMux()

	// Root redirect to admin dashboard
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
	})

	// Admin UI routes (no auth required for mock)
	adminHandlers := &admin.Handlers{DB: db}
	adminHandlers.RegisterRoutes(mux)

	// API routes with auth middleware chain
	apiRouter := api.NewRouter(db, sim)
	mux.Handle("/api/", apiRouter)
	mux.Handle("/health", apiRouter)

	// ── Server ────────────────────────────────────────────────
	server := &http.Server{
		Addr:         *addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown on SIGINT / SIGTERM
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Println("────────────────────────────────────────")
		log.Printf("  Lipila Mock Payment Gateway")
		log.Printf("  Server:   http://localhost%s/", *addr)
		log.Printf("  Admin UI: http://localhost%s/admin/", *addr)
		log.Printf("  API base: http://localhost%s/api/v1/", *addr)
		log.Printf("  Health:   http://localhost%s/health", *addr)
		log.Println("────────────────────────────────────────")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	sig := <-done
	log.Printf("received signal %v, shutting down...", sig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("forced shutdown: %v", err)
	}

	log.Println("all in-flight requests completed")
	log.Println("server stopped")
}
