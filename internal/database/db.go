package database

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the sql.DB connection pool.
type DB struct {
	*sql.DB
}

// New opens a SQLite database at the given path with connection pooling configured.
func New(dbPath string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=ON")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	// SQLite performs best with a single writer, but multiple readers are fine.
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	return &DB{sqlDB}, nil
}

// InitDB creates all tables if they don't exist and seeds defaults.
func (db *DB) InitDB() error {
	if err := db.createTables(); err != nil {
		return fmt.Errorf("creating tables: %w", err)
	}
	if err := db.seedSimulationConfig(); err != nil {
		return fmt.Errorf("seeding simulation config: %w", err)
	}
	log.Println("database initialized")
	return nil
}

// ResetDB drops and recreates all tables, then seeds defaults.
func (db *DB) ResetDB() error {
	drops := []string{
		`DROP TABLE IF EXISTS callback_logs`,
		`DROP TABLE IF EXISTS transactions`,
		`DROP TABLE IF EXISTS wallets`,
		`DROP TABLE IF EXISTS simulation_config`,
	}
	for _, d := range drops {
		if _, err := db.Exec(d); err != nil {
			return fmt.Errorf("dropping table: %w\nSQL: %s", err, d)
		}
	}
	log.Println("all tables dropped")
	return db.InitDB()
}

func (db *DB) createTables() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS wallets (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			balance REAL NOT NULL DEFAULT 0,
			currency TEXT NOT NULL DEFAULT 'ZMW',
			api_key TEXT NOT NULL UNIQUE,
			till_number TEXT NOT NULL UNIQUE,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS transactions (
			id TEXT PRIMARY KEY,
			wallet_id TEXT NOT NULL,
			reference_id TEXT NOT NULL UNIQUE,
			identifier TEXT NOT NULL UNIQUE,
			type TEXT NOT NULL CHECK(type IN ('Collection','Disbursement','Settlement','Allocation','Transfer')),
			payment_type TEXT NOT NULL CHECK(payment_type IN ('Card','AirtelMoney','MtnMoney','ZamtelKwacha','Bank')),
			status TEXT NOT NULL DEFAULT 'Pending' CHECK(status IN ('Pending','Successful','Failed')),
			amount REAL NOT NULL,
			currency TEXT NOT NULL DEFAULT 'ZMW',
			account_number TEXT NOT NULL DEFAULT '',
			narration TEXT NOT NULL DEFAULT '',
			ip_address TEXT NOT NULL DEFAULT '',
			external_id TEXT,
			callback_url TEXT,
			card_redirect_url TEXT,
			message TEXT,
			customer_info TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (wallet_id) REFERENCES wallets(id)
		)`,
		`CREATE TABLE IF NOT EXISTS simulation_config (
			id INTEGER PRIMARY KEY CHECK(id = 1),
			mtn_success_rate INTEGER NOT NULL DEFAULT 80 CHECK(mtn_success_rate BETWEEN 0 AND 100),
			airtel_success_rate INTEGER NOT NULL DEFAULT 85 CHECK(airtel_success_rate BETWEEN 0 AND 100),
			zamtel_success_rate INTEGER NOT NULL DEFAULT 75 CHECK(zamtel_success_rate BETWEEN 0 AND 100),
			card_success_rate INTEGER NOT NULL DEFAULT 90 CHECK(card_success_rate BETWEEN 0 AND 100),
			bank_success_rate INTEGER NOT NULL DEFAULT 95 CHECK(bank_success_rate BETWEEN 0 AND 100),
			processing_delay_seconds INTEGER NOT NULL DEFAULT 3,
			enable_random_timeouts INTEGER NOT NULL DEFAULT 1,
			timeout_probability INTEGER NOT NULL DEFAULT 5 CHECK(timeout_probability BETWEEN 0 AND 100)
		)`,
		`CREATE TABLE IF NOT EXISTS callback_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			transaction_id TEXT NOT NULL,
			url TEXT NOT NULL,
			status_code INTEGER NOT NULL DEFAULT 0,
			response_body TEXT NOT NULL DEFAULT '',
			error TEXT NOT NULL DEFAULT '',
			attempted_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (transaction_id) REFERENCES transactions(id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_wallet_id ON transactions(wallet_id)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_reference_id ON transactions(reference_id)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_identifier ON transactions(identifier)`,
		`CREATE INDEX IF NOT EXISTS idx_transactions_status ON transactions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_wallets_api_key ON wallets(api_key)`,
		`CREATE INDEX IF NOT EXISTS idx_wallets_till_number ON wallets(till_number)`,
		`CREATE INDEX IF NOT EXISTS idx_callback_logs_transaction_id ON callback_logs(transaction_id)`,
	}

	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return fmt.Errorf("executing statement: %w\nSQL: %s", err, s)
		}
	}
	return nil
}

func (db *DB) seedSimulationConfig() error {
	_, err := db.Exec(`INSERT OR IGNORE INTO simulation_config (id) VALUES (1)`)
	if err != nil {
		return fmt.Errorf("seeding simulation config: %w", err)
	}
	return nil
}

// SeedDefaultWallet creates an initial test wallet if none exists.
// Returns the wallet and whether it was newly created.
func (db *DB) SeedDefaultWallet() (*Wallet, bool, error) {
	var count int
	if err := db.QueryRow(`SELECT COUNT(*) FROM wallets`).Scan(&count); err != nil {
		return nil, false, fmt.Errorf("counting wallets: %w", err)
	}
	if count > 0 {
		w := &Wallet{}
		err := db.QueryRow(`SELECT id, name, balance, currency, api_key, till_number, is_active, created_at FROM wallets LIMIT 1`).Scan(
			&w.ID, &w.Name, &w.Balance, &w.Currency, &w.APIKey, &w.TillNumber, &w.IsActive, &w.CreatedAt,
		)
		if err != nil {
			return nil, false, fmt.Errorf("fetching existing wallet: %w", err)
		}
		return w, false, nil
	}

	w := &Wallet{
		ID:         generateUUID(),
		Name:       "Test Merchant",
		Balance:    10000.00,
		Currency:   "ZMW",
		APIKey:     "Lsk_" + generateHex(16),
		TillNumber: "100001",
		IsActive:   true,
	}

	_, err := db.Exec(
		`INSERT INTO wallets (id, name, balance, currency, api_key, till_number, is_active) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.Name, w.Balance, w.Currency, w.APIKey, w.TillNumber, w.IsActive,
	)
	if err != nil {
		return nil, false, fmt.Errorf("inserting default wallet: %w", err)
	}

	log.Printf("seeded default wallet: name=%s till=%s api_key=%s", w.Name, w.TillNumber, w.APIKey)
	return w, true, nil
}

// --- Wallet operations ---

// GetWalletByAPIKey looks up a wallet by its API key.
func (db *DB) GetWalletByAPIKey(apiKey string) (*Wallet, error) {
	w := &Wallet{}
	err := db.QueryRow(
		`SELECT id, name, balance, currency, api_key, till_number, is_active, created_at FROM wallets WHERE api_key = ?`, apiKey,
	).Scan(&w.ID, &w.Name, &w.Balance, &w.Currency, &w.APIKey, &w.TillNumber, &w.IsActive, &w.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("querying wallet by api key: %w", err)
	}
	return w, nil
}

// GetWalletByID looks up a wallet by ID.
func (db *DB) GetWalletByID(id string) (*Wallet, error) {
	w := &Wallet{}
	err := db.QueryRow(
		`SELECT id, name, balance, currency, api_key, till_number, is_active, created_at FROM wallets WHERE id = ?`, id,
	).Scan(&w.ID, &w.Name, &w.Balance, &w.Currency, &w.APIKey, &w.TillNumber, &w.IsActive, &w.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("querying wallet by id: %w", err)
	}
	return w, nil
}

// ListWallets returns all wallets.
func (db *DB) ListWallets() ([]Wallet, error) {
	rows, err := db.Query(`SELECT id, name, balance, currency, api_key, till_number, is_active, created_at FROM wallets ORDER BY created_at DESC`)
	if err != nil {
		return nil, fmt.Errorf("listing wallets: %w", err)
	}
	defer rows.Close()

	var wallets []Wallet
	for rows.Next() {
		var w Wallet
		if err := rows.Scan(&w.ID, &w.Name, &w.Balance, &w.Currency, &w.APIKey, &w.TillNumber, &w.IsActive, &w.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning wallet: %w", err)
		}
		wallets = append(wallets, w)
	}
	return wallets, rows.Err()
}

// UpdateWalletBalance updates a wallet's balance.
func (db *DB) UpdateWalletBalance(id string, newBalance float64) error {
	_, err := db.Exec(`UPDATE wallets SET balance = ? WHERE id = ?`, newBalance, id)
	if err != nil {
		return fmt.Errorf("updating wallet balance: %w", err)
	}
	return nil
}

// DeductWalletBalance atomically deducts an amount from a wallet if sufficient balance exists.
// Returns an error if balance is insufficient.
func (db *DB) DeductWalletBalance(id string, amount float64) error {
	result, err := db.Exec(
		`UPDATE wallets SET balance = balance - ? WHERE id = ? AND balance >= ?`,
		amount, id, amount,
	)
	if err != nil {
		return fmt.Errorf("deducting wallet balance: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("insufficient balance")
	}
	return nil
}

// RefundWalletBalance adds an amount back to a wallet's balance.
func (db *DB) RefundWalletBalance(id string, amount float64) error {
	_, err := db.Exec(`UPDATE wallets SET balance = balance + ? WHERE id = ?`, amount, id)
	if err != nil {
		return fmt.Errorf("refunding wallet balance: %w", err)
	}
	return nil
}

// CreateWallet creates a new wallet with an auto-generated ID, API key, and till number.
// API key format: "Lsk_" + 32 random hex characters. Till number: 6 random digits.
func (db *DB) CreateWallet(name string, balance float64, currency string) (*Wallet, error) {
	w := &Wallet{
		ID:         generateUUID(),
		Name:       name,
		Balance:    balance,
		Currency:   currency,
		APIKey:     "Lsk_" + generateHex(16),
		TillNumber: generateRandomTill(),
		IsActive:   true,
	}

	_, err := db.Exec(
		`INSERT INTO wallets (id, name, balance, currency, api_key, till_number, is_active) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.Name, w.Balance, w.Currency, w.APIKey, w.TillNumber, w.IsActive,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting wallet: %w", err)
	}
	return w, nil
}

// ToggleWalletActive flips the is_active flag on a wallet.
func (db *DB) ToggleWalletActive(id string) error {
	_, err := db.Exec(`UPDATE wallets SET is_active = NOT is_active WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("toggling wallet active: %w", err)
	}
	return nil
}

// ListTransactionsByWalletID returns all transactions for a specific wallet.
func (db *DB) ListTransactionsByWalletID(walletID string, limit, offset int) ([]Transaction, error) {
	rows, err := db.Query(
		`SELECT id, wallet_id, reference_id, identifier, type, payment_type, status,
		 amount, currency, account_number, narration, ip_address, external_id, callback_url,
		 card_redirect_url, message, customer_info, created_at, updated_at
		 FROM transactions WHERE wallet_id = ? ORDER BY created_at DESC LIMIT ? OFFSET ?`, walletID, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("listing transactions by wallet: %w", err)
	}
	defer rows.Close()

	var txns []Transaction
	for rows.Next() {
		var t Transaction
		var customerInfoSQL sql.NullString // Intermediate for scanning customer_info
		if err := rows.Scan(
			&t.ID, &t.WalletID, &t.ReferenceID, &t.Identifier, &t.Type, &t.PaymentType, &t.Status,
			&t.Amount, &t.Currency, &t.AccountNumber, &t.Narration, &t.IPAddress, &t.ExternalID,
			&t.CallbackURL, &t.CardRedirectURL, &t.Message, &customerInfoSQL, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning transaction: %w", err)
		}
		if customerInfoSQL.Valid {
			ci := json.RawMessage(customerInfoSQL.String)
			t.CustomerInfo = &ci
		} else {
			t.CustomerInfo = nil
		}
		txns = append(txns, t)
	}
	return txns, rows.Err()
}

// ValidateAPIKey checks if an API key belongs to an active wallet.
func (db *DB) ValidateAPIKey(apiKey string) (bool, error) {
	var isActive bool
	err := db.QueryRow(`SELECT is_active FROM wallets WHERE api_key = ?`, apiKey).Scan(&isActive)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("validating api key: %w", err)
	}
	return isActive, nil
}

// --- Transaction operations ---

// InsertTransaction inserts a new transaction.
func (db *DB) InsertTransaction(t *Transaction) error {
	_, err := db.Exec(
		`INSERT INTO transactions (id, wallet_id, reference_id, identifier, type, payment_type, status,
		 amount, currency, account_number, narration, ip_address, external_id, callback_url,
		 card_redirect_url, message, customer_info)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.ID, t.WalletID, t.ReferenceID, t.Identifier, t.Type, t.PaymentType, t.Status,
		t.Amount, t.Currency, t.AccountNumber, t.Narration, t.IPAddress, t.ExternalID,
		t.CallbackURL, t.CardRedirectURL, t.Message, t.CustomerInfo,
	)
	if err != nil {
		return fmt.Errorf("inserting transaction: %w", err)
	}
	return nil
}

// GetTransactionByReferenceID looks up a transaction by its client-provided reference ID.
func (db *DB) GetTransactionByReferenceID(refID string) (*Transaction, error) {
	return db.scanTransaction(`SELECT id, wallet_id, reference_id, identifier, type, payment_type, status,
		amount, currency, account_number, narration, ip_address, external_id, callback_url,
		card_redirect_url, message, customer_info, created_at, updated_at
		FROM transactions WHERE reference_id = ?`, refID)
}

// GetTransactionByIdentifier looks up a transaction by its Lipila identifier.
func (db *DB) GetTransactionByIdentifier(identifier string) (*Transaction, error) {
	return db.scanTransaction(`SELECT id, wallet_id, reference_id, identifier, type, payment_type, status,
		amount, currency, account_number, narration, ip_address, external_id, callback_url,
		card_redirect_url, message, customer_info, created_at, updated_at
		FROM transactions WHERE identifier = ?`, identifier)
}

// UpdateTransactionStatus updates the status and message of a transaction.
func (db *DB) UpdateTransactionStatus(id, status string, message *string) error {
	_, err := db.Exec(
		`UPDATE transactions SET status = ?, message = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, message, id,
	)
	if err != nil {
		return fmt.Errorf("updating transaction status: %w", err)
	}
	return nil
}

// SetTransactionExternalID sets the MNO external ID on a transaction.
func (db *DB) SetTransactionExternalID(id, externalID string) error {
	_, err := db.Exec(
		`UPDATE transactions SET external_id = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		externalID, id,
	)
	if err != nil {
		return fmt.Errorf("setting external id: %w", err)
	}
	return nil
}

// ListTransactions returns transactions ordered by creation time descending.
func (db *DB) ListTransactions(limit, offset int) ([]Transaction, error) {
	rows, err := db.Query(
		`SELECT id, wallet_id, reference_id, identifier, type, payment_type, status,
		 amount, currency, account_number, narration, ip_address, external_id, callback_url,
		 card_redirect_url, message, customer_info, created_at, updated_at
		 FROM transactions ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset,
	)
	if err != nil {
		return nil, fmt.Errorf("listing transactions: %w", err)
	}
	defer rows.Close()

	var txns []Transaction
	for rows.Next() {
		var t Transaction
		var customerInfoSQL sql.NullString // Intermediate for scanning customer_info
		if err := rows.Scan(
			&t.ID, &t.WalletID, &t.ReferenceID, &t.Identifier, &t.Type, &t.PaymentType, &t.Status,
			&t.Amount, &t.Currency, &t.AccountNumber, &t.Narration, &t.IPAddress, &t.ExternalID,
			&t.CallbackURL, &t.CardRedirectURL, &t.Message, &customerInfoSQL, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning transaction: %w", err)
		}
		if customerInfoSQL.Valid {
			ci := json.RawMessage(customerInfoSQL.String)
			t.CustomerInfo = &ci
		} else {
			t.CustomerInfo = nil
		}
		txns = append(txns, t)
	}
	return txns, rows.Err()
}

// ListTransactionsFiltered returns transactions matching the given filters.
// Any filter value that is empty string is ignored.
func (db *DB) ListTransactionsFiltered(status, txnType, paymentType string, limit, offset int) ([]Transaction, error) {
	query := `SELECT id, wallet_id, reference_id, identifier, type, payment_type, status,
		 amount, currency, account_number, narration, ip_address, external_id, callback_url,
		 card_redirect_url, message, customer_info, created_at, updated_at
		 FROM transactions WHERE 1=1`
	var args []any

	if status != "" {
		query += " AND status = ?"
		args = append(args, status)
	}
	if txnType != "" {
		query += " AND type = ?"
		args = append(args, txnType)
	}
	if paymentType != "" {
		query += " AND payment_type = ?"
		args = append(args, paymentType)
	}

	query += " ORDER BY created_at DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("listing filtered transactions: %w", err)
	}
	defer rows.Close()

	var txns []Transaction
	for rows.Next() {
		var t Transaction
		var customerInfoSQL sql.NullString // Intermediate for scanning customer_info
		if err := rows.Scan(
			&t.ID, &t.WalletID, &t.ReferenceID, &t.Identifier, &t.Type, &t.PaymentType, &t.Status,
			&t.Amount, &t.Currency, &t.AccountNumber, &t.Narration, &t.IPAddress, &t.ExternalID,
			&t.CallbackURL, &t.CardRedirectURL, &t.Message, &customerInfoSQL, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning transaction: %w", err)
		}
		if customerInfoSQL.Valid {
			ci := json.RawMessage(customerInfoSQL.String)
			t.CustomerInfo = &ci
		} else {
			t.CustomerInfo = nil
		}
		txns = append(txns, t)
	}
	return txns, rows.Err()
}

func (db *DB) scanTransaction(query string, args ...any) (*Transaction, error) {
	t := &Transaction{}
	var customerInfoSQL sql.NullString // Intermediate for scanning customer_info
	err := db.QueryRow(query, args...).Scan(
		&t.ID, &t.WalletID, &t.ReferenceID, &t.Identifier, &t.Type, &t.PaymentType, &t.Status,
		&t.Amount, &t.Currency, &t.AccountNumber, &t.Narration, &t.IPAddress, &t.ExternalID,
		&t.CallbackURL, &t.CardRedirectURL, &t.Message, &customerInfoSQL, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("querying transaction: %w", err)
	}
	if customerInfoSQL.Valid {
		ci := json.RawMessage(customerInfoSQL.String)
		t.CustomerInfo = &ci
	} else {
		t.CustomerInfo = nil
	}
	return t, nil
}

// --- Simulation config operations ---

// GetSimulationConfig returns the singleton simulation configuration.
func (db *DB) GetSimulationConfig() (*SimulationConfig, error) {
	c := &SimulationConfig{}
	err := db.QueryRow(
		`SELECT id, mtn_success_rate, airtel_success_rate, zamtel_success_rate,
		 card_success_rate, bank_success_rate, processing_delay_seconds,
		 enable_random_timeouts, timeout_probability FROM simulation_config WHERE id = 1`,
	).Scan(
		&c.ID, &c.MtnSuccessRate, &c.AirtelSuccessRate, &c.ZamtelSuccessRate,
		&c.CardSuccessRate, &c.BankSuccessRate, &c.ProcessingDelaySeconds,
		&c.EnableRandomTimeouts, &c.TimeoutProbability,
	)
	if err != nil {
		return nil, fmt.Errorf("querying simulation config: %w", err)
	}
	return c, nil
}

// UpdateSimulationConfig updates the simulation configuration.
func (db *DB) UpdateSimulationConfig(c *SimulationConfig) error {
	_, err := db.Exec(
		`UPDATE simulation_config SET
		 mtn_success_rate = ?, airtel_success_rate = ?, zamtel_success_rate = ?,
		 card_success_rate = ?, bank_success_rate = ?, processing_delay_seconds = ?,
		 enable_random_timeouts = ?, timeout_probability = ?
		 WHERE id = 1`,
		c.MtnSuccessRate, c.AirtelSuccessRate, c.ZamtelSuccessRate,
		c.CardSuccessRate, c.BankSuccessRate, c.ProcessingDelaySeconds,
		c.EnableRandomTimeouts, c.TimeoutProbability,
	)
	if err != nil {
		return fmt.Errorf("updating simulation config: %w", err)
	}
	return nil
}

// --- Callback log operations ---

// InsertCallbackLog records a callback delivery attempt.
func (db *DB) InsertCallbackLog(cl *CallbackLog) error {
	_, err := db.Exec(
		`INSERT INTO callback_logs (transaction_id, url, status_code, response_body, error)
		 VALUES (?, ?, ?, ?, ?)`,
		cl.TransactionID, cl.URL, cl.StatusCode, cl.ResponseBody, cl.Error,
	)
	if err != nil {
		return fmt.Errorf("inserting callback log: %w", err)
	}
	return nil
}

// --- Test data seeding ---

// SeedTestData creates a comprehensive set of test transactions for all scenarios.
// It creates a low-balance wallet and populates the given wallet with varied transactions.
func (db *DB) SeedTestData(walletID string) (int, error) {
	now := time.Now().UTC()
	count := 0

	// Create a low-balance wallet for testing insufficient funds
	_, err := db.CreateWallet("Low Balance Merchant", 5.00, "ZMW")
	if err != nil {
		log.Printf("seed: low balance wallet may already exist: %v", err)
	} else {
		count++
	}

	type seedTxn struct {
		txnType     string
		paymentType string
		status      string
		amount      float64
		account     string
		narration   string
		message     *string
		createdAgo  time.Duration
	}

	msg := func(s string) *string { return &s }

	txns := []seedTxn{
		// Successful collections — each payment type
		{TypeCollection, PayMtnMoney, StatusSuccessful, 100.00, "260971234567", "MTN successful payment", msg("Transaction completed successfully"), 25 * 24 * time.Hour},
		{TypeCollection, PayAirtelMoney, StatusSuccessful, 250.00, "260971234568", "Airtel successful payment", msg("Transaction completed successfully"), 20 * 24 * time.Hour},
		{TypeCollection, PayZamtelKwacha, StatusSuccessful, 75.50, "260955123456", "Zamtel successful payment", msg("Transaction completed successfully"), 15 * 24 * time.Hour},
		{TypeCollection, PayCard, StatusSuccessful, 500.00, "260971234569", "Card successful payment", msg("Transaction completed successfully"), 10 * 24 * time.Hour},

		// Failed collections — each MNO with realistic error messages
		{TypeCollection, PayMtnMoney, StatusFailed, 200.00, "260961111111", "MTN failed payment", msg("LOW_BALANCE_OR_PAYEE_LIMIT_REACHED_OR_NOT_ALLOWED"), 22 * 24 * time.Hour},
		{TypeCollection, PayAirtelMoney, StatusFailed, 150.00, "260972222222", "Airtel failed payment", msg("User not found"), 18 * 24 * time.Hour},
		{TypeCollection, PayZamtelKwacha, StatusFailed, 80.00, "260953333333", "Zamtel failed payment", msg("System internal error."), 14 * 24 * time.Hour},
		{TypeCollection, PayCard, StatusFailed, 300.00, "260974444444", "Card failed payment", msg("Card declined by issuer"), 8 * 24 * time.Hour},
		{TypeCollection, PayMtnMoney, StatusFailed, 50.00, "260965555555", "MTN timeout failure", msg("Failed to send status check to Mtn httpStatusCode=404"), 5 * 24 * time.Hour},
		{TypeCollection, PayAirtelMoney, StatusFailed, 120.00, "260976666666", "Airtel not found", msg("Transaction is not found"), 3 * 24 * time.Hour},

		// Pending collections (simulate old stuck transactions)
		{TypeCollection, PayMtnMoney, StatusPending, 175.00, "260967777777", "MTN pending stuck", nil, 7 * 24 * time.Hour},
		{TypeCollection, PayAirtelMoney, StatusPending, 90.00, "260978888888", "Airtel pending stuck", nil, 5 * 24 * time.Hour},
		{TypeCollection, PayZamtelKwacha, StatusPending, 60.00, "260959999999", "Zamtel pending stuck", nil, 3 * 24 * time.Hour},

		// Successful disbursements — mobile money & bank
		{TypeDisbursement, PayMtnMoney, StatusSuccessful, 500.00, "260961234567", "MTN payout", msg("Disbursement completed"), 24 * 24 * time.Hour},
		{TypeDisbursement, PayAirtelMoney, StatusSuccessful, 300.00, "260971234500", "Airtel payout", msg("Disbursement completed"), 19 * 24 * time.Hour},
		{TypeDisbursement, PayBank, StatusSuccessful, 2000.00, "1234567890", "Bank transfer", msg("Disbursement completed"), 12 * 24 * time.Hour},

		// Failed disbursements
		{TypeDisbursement, PayMtnMoney, StatusFailed, 800.00, "260960000001", "MTN payout failed", msg("LOW_BALANCE_OR_PAYEE_LIMIT_REACHED_OR_NOT_ALLOWED"), 16 * 24 * time.Hour},
		{TypeDisbursement, PayBank, StatusFailed, 5000.00, "9876543210", "Bank transfer failed", msg("Invalid account number"), 6 * 24 * time.Hour},
		{TypeDisbursement, PayBank, StatusFailed, 3000.00, "5555555555", "Bank name mismatch", msg("Account name mismatch"), 4 * 24 * time.Hour},
		{TypeDisbursement, PayZamtelKwacha, StatusFailed, 150.00, "260950000002", "Zamtel payout failed", msg("Recipient wallet not active"), 2 * 24 * time.Hour},

		// Pending disbursements (stuck)
		{TypeDisbursement, PayMtnMoney, StatusPending, 400.00, "260960000003", "MTN payout pending", nil, 6 * 24 * time.Hour},
		{TypeDisbursement, PayBank, StatusPending, 1500.00, "1111111111", "Bank transfer pending", nil, 4 * 24 * time.Hour},
	}

	for _, s := range txns {
		createdAt := now.Add(-s.createdAgo)
		updatedAt := createdAt
		if s.status != StatusPending {
			updatedAt = createdAt.Add(3 * time.Second) // simulate processing delay
		}

		_, err := db.Exec(
			`INSERT INTO transactions (id, wallet_id, reference_id, identifier, type, payment_type, status,
			 amount, currency, account_number, narration, ip_address, external_id, message, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			generateUUID(), walletID, "seed-"+generateHex(8), GenerateLipilaIdentifier(s.txnType),
			s.txnType, s.paymentType, s.status,
			s.amount, "ZMW", s.account, s.narration, "127.0.0.1",
			GenerateMNOExternalID(), s.message,
			createdAt.Format("2006-01-02 15:04:05"), updatedAt.Format("2006-01-02 15:04:05"),
		)
		if err != nil {
			log.Printf("seed: error inserting test transaction: %v", err)
			continue
		}
		count++
	}

	return count, nil
}

// SeedStuckPending creates several old pending transactions to simulate timeouts.
func (db *DB) SeedStuckPending(walletID string) (int, error) {
	now := time.Now().UTC()
	count := 0

	stuckTxns := []struct {
		paymentType string
		account     string
		age         time.Duration
	}{
		{PayMtnMoney, "260961000001", 48 * time.Hour},
		{PayAirtelMoney, "260971000002", 36 * time.Hour},
		{PayZamtelKwacha, "260951000003", 24 * time.Hour},
		{PayMtnMoney, "260961000004", 12 * time.Hour},
		{PayCard, "260971000005", 6 * time.Hour},
		{PayBank, "0000001111", 72 * time.Hour},
		{PayMtnMoney, "260961000006", 96 * time.Hour},
	}

	for i, s := range stuckTxns {
		createdAt := now.Add(-s.age)
		_, err := db.Exec(
			`INSERT INTO transactions (id, wallet_id, reference_id, identifier, type, payment_type, status,
			 amount, currency, account_number, narration, ip_address, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			generateUUID(), walletID, fmt.Sprintf("stuck-%d-%s", i, generateHex(4)),
			GenerateLipilaIdentifier(TypeCollection),
			TypeCollection, s.paymentType, StatusPending,
			float64(50+i*25), "ZMW", s.account,
			fmt.Sprintf("Stuck pending #%d — %s ago", i+1, s.age),
			"127.0.0.1",
			createdAt.Format("2006-01-02 15:04:05"), createdAt.Format("2006-01-02 15:04:05"),
		)
		if err != nil {
			log.Printf("seed: error inserting stuck txn: %v", err)
			continue
		}
		count++
	}

	return count, nil
}

// SeedRandomHistory generates random transactions spread across the last 30 days.
func (db *DB) SeedRandomHistory(walletID string, numTxns int) (int, error) {
	now := time.Now().UTC()
	count := 0

	types := []struct {
		txnType     string
		paymentType string
	}{
		{TypeCollection, PayMtnMoney},
		{TypeCollection, PayAirtelMoney},
		{TypeCollection, PayZamtelKwacha},
		{TypeCollection, PayCard},
		{TypeDisbursement, PayMtnMoney},
		{TypeDisbursement, PayAirtelMoney},
		{TypeDisbursement, PayBank},
	}

	statuses := []struct {
		status string
		weight int // out of 100
	}{
		{StatusSuccessful, 70},
		{StatusFailed, 20},
		{StatusPending, 10},
	}

	failMsgs := map[string][]string{
		PayMtnMoney:     {"LOW_BALANCE_OR_PAYEE_LIMIT_REACHED_OR_NOT_ALLOWED", "Failed to send status check to Mtn httpStatusCode=404"},
		PayAirtelMoney:  {"User not found", "Transaction is not found"},
		PayZamtelKwacha: {"System internal error.", "Recipient wallet not active"},
		PayCard:         {"Card declined by issuer", "3D Secure authentication failed"},
		PayBank:         {"Invalid account number", "Account name mismatch", "Bank temporarily unavailable"},
	}

	for i := 0; i < numTxns; i++ {
		// Random bytes for deterministic-ish choices
		rb := make([]byte, 8)
		rand.Read(rb)

		tp := types[int(rb[0])%len(types)]
		daysAgo := int(rb[1]) % 30
		hoursAgo := int(rb[2]) % 24
		minsAgo := int(rb[3]) % 60
		createdAt := now.Add(-time.Duration(daysAgo)*24*time.Hour - time.Duration(hoursAgo)*time.Hour - time.Duration(minsAgo)*time.Minute)

		// Pick status by weight
		roll := int(rb[4]) % 100
		status := StatusSuccessful
		for _, sw := range statuses {
			roll -= sw.weight
			if roll < 0 {
				status = sw.status
				break
			}
		}

		amount := float64(10+int(rb[5])%990) + float64(int(rb[6])%100)/100.0
		account := fmt.Sprintf("2609%d%07d", 5+int(rb[7])%3, int(rb[0])%10000000)

		var message *string
		if status == StatusSuccessful {
			m := "Transaction completed successfully"
			message = &m
		} else if status == StatusFailed {
			msgs := failMsgs[tp.paymentType]
			if len(msgs) > 0 {
				m := msgs[int(rb[7])%len(msgs)]
				message = &m
			}
		}

		updatedAt := createdAt
		if status != StatusPending {
			updatedAt = createdAt.Add(3 * time.Second)
		}

		_, err := db.Exec(
			`INSERT INTO transactions (id, wallet_id, reference_id, identifier, type, payment_type, status,
			 amount, currency, account_number, narration, ip_address, external_id, message, created_at, updated_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			generateUUID(), walletID, fmt.Sprintf("rand-%d-%s", i, generateHex(4)),
			GenerateLipilaIdentifier(tp.txnType),
			tp.txnType, tp.paymentType, status,
			amount, "ZMW", account,
			fmt.Sprintf("Random test transaction #%d", i+1),
			"127.0.0.1", GenerateMNOExternalID(), message,
			createdAt.Format("2006-01-02 15:04:05"), updatedAt.Format("2006-01-02 15:04:05"),
		)
		if err != nil {
			log.Printf("seed: error inserting random txn %d: %v", i, err)
			continue
		}
		count++
	}

	return count, nil
}

// --- Helpers ---

// GenerateUUID produces a v4-style UUID string.
func GenerateUUID() string {
	return generateUUID()
}

func generateUUID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate random bytes: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 2
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func generateHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate random bytes: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// generateRandomTill generates a 6-digit random till number.
func generateRandomTill() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate random bytes: " + err.Error())
	}
	// Convert 3 bytes to a number 0-999999, format as 6 digits
	n := (int(b[0])<<16 | int(b[1])<<8 | int(b[2])) % 1000000
	return fmt.Sprintf("%06d", n)
}

// GenerateMNOExternalID creates an MNO-style external ID: MP{YYMMDD}.{HHMM}.C{XXXX}
func GenerateMNOExternalID() string {
	now := time.Now().UTC()
	suffix := generateHex(2)
	return fmt.Sprintf("MP%s.%s.C%s",
		now.Format("060102"),
		now.Format("1504"),
		strings.ToUpper(suffix),
	)
}

// GenerateLipilaIdentifier creates a Lipila-format identifier: LPLXC-YYYYMMDD-HHMMSS-XXXX
func GenerateLipilaIdentifier(txnType string) string {
	now := time.Now().UTC()
	prefix := "LPLXC" // Collection
	switch txnType {
	case TypeDisbursement:
		prefix = "LPLXD"
	case TypeSettlement:
		prefix = "LPLXS"
	case TypeAllocation:
		prefix = "LPLXA"
	case TypeTransfer:
		prefix = "LPLXT"
	}
	suffix := generateHex(2) // 4 hex chars
	return fmt.Sprintf("%s-%s-%s-%s",
		prefix,
		now.Format("20060102"),
		now.Format("150405"),
		suffix,
	)
}
