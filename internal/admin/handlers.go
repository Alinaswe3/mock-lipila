package admin

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/Alinaswe3/mock-lipila/internal/database"
)

// Handlers holds dependencies for admin UI handlers.
type Handlers struct {
	DB *database.DB
}

// RegisterRoutes registers admin routes on the given mux.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/admin/", h.HandleDashboard)
	mux.HandleFunc("/admin/create-wallet", h.HandleCreateWallet)
	mux.HandleFunc("/admin/update-config", h.HandleUpdateConfig)
	mux.HandleFunc("/admin/reset-db", h.HandleResetDB)
	mux.HandleFunc("GET /admin/reset", h.HandleResetConfirm)
	mux.HandleFunc("POST /admin/reset", h.HandleResetExecute)
	mux.HandleFunc("GET /admin/config", h.HandleConfigPage)
	mux.HandleFunc("POST /admin/config", h.HandleConfigUpdate)
	mux.HandleFunc("GET /admin/wallets/new", h.HandleNewWalletForm)
	mux.HandleFunc("POST /admin/wallets", h.HandleCreateWalletDedicated)
	mux.HandleFunc("POST /admin/wallets/{id}/toggle", h.HandleToggleWallet)
	mux.HandleFunc("GET /admin/wallets/{id}", h.HandleWalletDetail)
	mux.HandleFunc("POST /admin/seed-test-data", h.HandleSeedTestData)
	mux.HandleFunc("POST /admin/seed-stuck-pending", h.HandleSeedStuckPending)
	mux.HandleFunc("POST /admin/seed-random-history", h.HandleSeedRandomHistory)
}

// HandleDashboard renders the unified admin dashboard.
func (h *Handlers) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	wallets, err := h.DB.ListWallets()
	if err != nil {
		log.Printf("admin: error listing wallets: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	txns, err := h.DB.ListTransactions(20, 0)
	if err != nil {
		log.Printf("admin: error listing transactions: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	cfg, err := h.DB.GetSimulationConfig()
	if err != nil {
		log.Printf("admin: error loading config: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Wallets":      wallets,
		"Transactions": txns,
		"Config":       cfg,
		"Flash":        r.URL.Query().Get("flash"),
		"Error":        r.URL.Query().Get("error"),
	}

	if err := DashboardTmpl.Execute(w, data); err != nil {
		log.Printf("admin: error rendering dashboard: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleCreateWallet processes the create wallet form.
func (h *Handlers) HandleCreateWallet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
		return
	}

	name := r.FormValue("name")
	currency := r.FormValue("currency")
	balanceStr := r.FormValue("balance")

	if name == "" || currency == "" || balanceStr == "" {
		http.Redirect(w, r, "/admin/?error=All+fields+are+required", http.StatusSeeOther)
		return
	}

	balance, err := strconv.ParseFloat(balanceStr, 64)
	if err != nil || balance < 0 {
		http.Redirect(w, r, "/admin/?error=Invalid+balance+value", http.StatusSeeOther)
		return
	}

	wallet, err := h.DB.CreateWallet(name, balance, currency)
	if err != nil {
		log.Printf("admin: error creating wallet: %v", err)
		http.Redirect(w, r, "/admin/?error=Failed+to+create+wallet", http.StatusSeeOther)
		return
	}

	flash := fmt.Sprintf("Wallet+created.+API+Key:+%s", wallet.APIKey)
	http.Redirect(w, r, "/admin/?flash="+flash, http.StatusSeeOther)
}

// HandleNewWalletForm renders the dedicated create wallet form.
func (h *Handlers) HandleNewWalletForm(w http.ResponseWriter, r *http.Request) {
	data := map[string]any{
		"Error": r.URL.Query().Get("error"),
	}
	if err := NewWalletTmpl.Execute(w, data); err != nil {
		log.Printf("admin: error rendering new wallet form: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleCreateWalletDedicated processes the dedicated create wallet form.
func (h *Handlers) HandleCreateWalletDedicated(w http.ResponseWriter, r *http.Request) {
	name := r.FormValue("name")
	currency := r.FormValue("currency")
	balanceStr := r.FormValue("balance")

	if name == "" || currency == "" || balanceStr == "" {
		http.Redirect(w, r, "/admin/wallets/new?error=All+fields+are+required", http.StatusSeeOther)
		return
	}

	balance, err := strconv.ParseFloat(balanceStr, 64)
	if err != nil || balance < 0 {
		http.Redirect(w, r, "/admin/wallets/new?error=Invalid+balance+value", http.StatusSeeOther)
		return
	}

	wallet, err := h.DB.CreateWallet(name, balance, currency)
	if err != nil {
		log.Printf("admin: error creating wallet: %v", err)
		http.Redirect(w, r, "/admin/wallets/new?error=Failed+to+create+wallet", http.StatusSeeOther)
		return
	}

	flash := fmt.Sprintf("Wallet+created.+API+Key:+%s", wallet.APIKey)
	http.Redirect(w, r, "/admin/?flash="+flash, http.StatusSeeOther)
}

// HandleToggleWallet toggles a wallet's active/inactive status.
func (h *Handlers) HandleToggleWallet(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Redirect(w, r, "/admin/?error=Missing+wallet+ID", http.StatusSeeOther)
		return
	}

	if err := h.DB.ToggleWalletActive(id); err != nil {
		log.Printf("admin: error toggling wallet %s: %v", id, err)
		http.Redirect(w, r, "/admin/?error=Failed+to+toggle+wallet", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/?flash=Wallet+status+toggled", http.StatusSeeOther)
}

// HandleWalletDetail shows a single wallet with its transactions.
func (h *Handlers) HandleWalletDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Redirect(w, r, "/admin/?error=Missing+wallet+ID", http.StatusSeeOther)
		return
	}

	wallet, err := h.DB.GetWalletByID(id)
	if err != nil {
		log.Printf("admin: error fetching wallet %s: %v", id, err)
		http.Redirect(w, r, "/admin/?error=Wallet+not+found", http.StatusSeeOther)
		return
	}

	txns, err := h.DB.ListTransactionsByWalletID(id, 50, 0)
	if err != nil {
		log.Printf("admin: error listing transactions for wallet %s: %v", id, err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Wallet":       wallet,
		"Transactions": txns,
	}

	if err := WalletDetailTmpl.Execute(w, data); err != nil {
		log.Printf("admin: error rendering wallet detail: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleConfigPage renders the dedicated simulation config editing page.
func (h *Handlers) HandleConfigPage(w http.ResponseWriter, r *http.Request) {
	cfg, err := h.DB.GetSimulationConfig()
	if err != nil {
		log.Printf("admin: error loading config: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"Config": cfg,
		"Flash":  r.URL.Query().Get("flash"),
		"Error":  r.URL.Query().Get("error"),
	}

	if err := ConfigTmpl.Execute(w, data); err != nil {
		log.Printf("admin: error rendering config page: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// clampInt clamps a value between min and max.
func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// HandleConfigUpdate processes the dedicated config form with validation.
func (h *Handlers) HandleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	parseInt := func(key string) int {
		v, _ := strconv.Atoi(r.FormValue(key))
		return v
	}

	mtn := clampInt(parseInt("mtn_success_rate"), 0, 100)
	airtel := clampInt(parseInt("airtel_success_rate"), 0, 100)
	zamtel := clampInt(parseInt("zamtel_success_rate"), 0, 100)
	card := clampInt(parseInt("card_success_rate"), 0, 100)
	bank := clampInt(parseInt("bank_success_rate"), 0, 100)
	delay := parseInt("processing_delay_seconds")
	if delay < 0 {
		delay = 0
	}
	timeoutProb := clampInt(parseInt("timeout_probability"), 0, 100)

	cfg := &database.SimulationConfig{
		MtnSuccessRate:         mtn,
		AirtelSuccessRate:      airtel,
		ZamtelSuccessRate:      zamtel,
		CardSuccessRate:        card,
		BankSuccessRate:        bank,
		ProcessingDelaySeconds: delay,
		EnableRandomTimeouts:   r.FormValue("enable_random_timeouts") == "on",
		TimeoutProbability:     timeoutProb,
	}

	if err := h.DB.UpdateSimulationConfig(cfg); err != nil {
		log.Printf("admin: error updating config: %v", err)
		http.Redirect(w, r, "/admin/config?error=Failed+to+update+config", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/?flash=Simulation+config+updated", http.StatusSeeOther)
}

// HandleUpdateConfig processes the simulation config update form.
func (h *Handlers) HandleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
		return
	}

	parseInt := func(key string) int {
		v, _ := strconv.Atoi(r.FormValue(key))
		return v
	}

	cfg := &database.SimulationConfig{
		MtnSuccessRate:         parseInt("mtn_success_rate"),
		AirtelSuccessRate:      parseInt("airtel_success_rate"),
		ZamtelSuccessRate:      parseInt("zamtel_success_rate"),
		CardSuccessRate:        parseInt("card_success_rate"),
		BankSuccessRate:        parseInt("bank_success_rate"),
		ProcessingDelaySeconds: parseInt("processing_delay_seconds"),
		EnableRandomTimeouts:   r.FormValue("enable_random_timeouts") == "on",
		TimeoutProbability:     parseInt("timeout_probability"),
	}

	if err := h.DB.UpdateSimulationConfig(cfg); err != nil {
		log.Printf("admin: error updating config: %v", err)
		http.Redirect(w, r, "/admin/?error=Failed+to+update+config", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/?flash=Simulation+config+updated", http.StatusSeeOther)
}

// HandleResetConfirm renders the "Are you sure?" confirmation page.
func (h *Handlers) HandleResetConfirm(w http.ResponseWriter, r *http.Request) {
	if err := ResetConfirmTmpl.Execute(w, nil); err != nil {
		log.Printf("admin: error rendering reset confirm page: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// HandleResetExecute performs the actual database reset.
func (h *Handlers) HandleResetExecute(w http.ResponseWriter, r *http.Request) {
	if err := h.DB.ResetDB(); err != nil {
		log.Printf("admin: error resetting database: %v", err)
		http.Redirect(w, r, "/admin/?error=Failed+to+reset+database", http.StatusSeeOther)
		return
	}

	wallet, _, err := h.DB.SeedDefaultWallet()
	if err != nil {
		log.Printf("admin: error seeding wallet after reset: %v", err)
		http.Redirect(w, r, "/admin/?flash=Database+reset+successfully", http.StatusSeeOther)
		return
	}

	log.Printf("admin: re-seeded default wallet after reset: %s", wallet.APIKey)
	http.Redirect(w, r, "/admin/?flash=Database+reset+successfully.+New+API+Key:+"+wallet.APIKey, http.StatusSeeOther)
}

// HandleResetDB resets the entire database.
func (h *Handlers) HandleResetDB(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
		return
	}

	if err := h.DB.ResetDB(); err != nil {
		log.Printf("admin: error resetting database: %v", err)
		http.Redirect(w, r, "/admin/?error=Failed+to+reset+database", http.StatusSeeOther)
		return
	}

	// Re-seed a default wallet after reset
	wallet, _, err := h.DB.SeedDefaultWallet()
	if err != nil {
		log.Printf("admin: error seeding wallet after reset: %v", err)
	} else {
		log.Printf("admin: re-seeded default wallet after reset: %s", wallet.APIKey)
	}

	http.Redirect(w, r, "/admin/?flash=Database+reset+successfully.+New+API+Key:+"+wallet.APIKey, http.StatusSeeOther)
}

// getFirstWalletID returns the ID of the first wallet, or an error if none exist.
func (h *Handlers) getFirstWalletID() (string, error) {
	wallets, err := h.DB.ListWallets()
	if err != nil {
		return "", err
	}
	if len(wallets) == 0 {
		return "", fmt.Errorf("no wallets exist")
	}
	return wallets[0].ID, nil
}

// HandleSeedTestData seeds comprehensive test transactions.
func (h *Handlers) HandleSeedTestData(w http.ResponseWriter, r *http.Request) {
	walletID, err := h.getFirstWalletID()
	if err != nil {
		http.Redirect(w, r, "/admin/?error=No+wallets+exist.+Create+one+first.", http.StatusSeeOther)
		return
	}

	count, err := h.DB.SeedTestData(walletID)
	if err != nil {
		log.Printf("admin: error seeding test data: %v", err)
		http.Redirect(w, r, "/admin/?error=Failed+to+seed+test+data", http.StatusSeeOther)
		return
	}

	flash := fmt.Sprintf("Seeded+%d+test+records+(transactions+and+wallets)", count)
	http.Redirect(w, r, "/admin/?flash="+flash, http.StatusSeeOther)
}

// HandleSeedStuckPending creates old pending transactions to simulate timeouts.
func (h *Handlers) HandleSeedStuckPending(w http.ResponseWriter, r *http.Request) {
	walletID, err := h.getFirstWalletID()
	if err != nil {
		http.Redirect(w, r, "/admin/?error=No+wallets+exist.+Create+one+first.", http.StatusSeeOther)
		return
	}

	count, err := h.DB.SeedStuckPending(walletID)
	if err != nil {
		log.Printf("admin: error seeding stuck pending: %v", err)
		http.Redirect(w, r, "/admin/?error=Failed+to+seed+stuck+pending", http.StatusSeeOther)
		return
	}

	flash := fmt.Sprintf("Created+%d+stuck+pending+transactions", count)
	http.Redirect(w, r, "/admin/?flash="+flash, http.StatusSeeOther)
}

// HandleSeedRandomHistory generates random transaction history over the last 30 days.
func (h *Handlers) HandleSeedRandomHistory(w http.ResponseWriter, r *http.Request) {
	walletID, err := h.getFirstWalletID()
	if err != nil {
		http.Redirect(w, r, "/admin/?error=No+wallets+exist.+Create+one+first.", http.StatusSeeOther)
		return
	}

	count, err := h.DB.SeedRandomHistory(walletID, 50)
	if err != nil {
		log.Printf("admin: error seeding random history: %v", err)
		http.Redirect(w, r, "/admin/?error=Failed+to+seed+random+history", http.StatusSeeOther)
		return
	}

	flash := fmt.Sprintf("Generated+%d+random+transactions+over+last+30+days", count)
	http.Redirect(w, r, "/admin/?flash="+flash, http.StatusSeeOther)
}
