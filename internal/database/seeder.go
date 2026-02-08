package database

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"time"

	"encoding/json"


	"github.com/google/uuid"
)

// SeedTestData seeds the database with various test scenarios.
func SeedTestData(db *DB) error {
	log.Println("Seeding test data...")

	// Clear existing data (optional, for clean test runs)
	// if err := clearData(db); err != nil {
	// 	return fmt.Errorf("failed to clear existing data: %w", err)
	// }

	// Seed Wallets
	walletLowBalanceID := uuid.New().String()
	if err := createWallet(db, Wallet{
		ID:         walletLowBalanceID,
		Name:       "Low Balance Wallet",
		Balance:    10.00,
		Currency:   "ZMW",
		APIKey:     uuid.New().String(),
		TillNumber: "987654",
		IsActive:   true,
		CreatedAt:  time.Now(),
	}); err != nil {
		return fmt.Errorf("failed to seed low balance wallet: %w", err)
	}

	walletHighBalanceID := uuid.New().String()
	if err := createWallet(db, Wallet{
		ID:         walletHighBalanceID,
		Name:       "High Balance Wallet",
		Balance:    10000.00,
		Currency:   "ZMW",
		APIKey:     uuid.New().String(),
		TillNumber: "123456",
		IsActive:   true,
		CreatedAt:  time.Now(),
	}); err != nil {
		return fmt.Errorf("failed to seed high balance wallet: %w", err)
	}

	// Seed Transactions for various statuses and types
	paymentTypes := []string{PayCard, PayAirtelMoney, PayMtnMoney, PayZamtelKwacha, PayBank}
	statuses := []string{StatusPending, StatusSuccessful, StatusFailed}
	errorMessages := []string{
		"Insufficient funds",
		"Transaction declined by provider",
		"Invalid account number",
		"Service temporarily unavailable",
		"Timeout from external system",
		"Duplicate transaction reference",
	}

	for _, status := range statuses {
		for _, pType := range paymentTypes {
			// Successful transaction
			if err := createTransaction(db, Transaction{
				ID:          uuid.New().String(),
				WalletID:    walletHighBalanceID,
				ReferenceID: fmt.Sprintf("REF-%s-%s-%s", status, pType, uuid.New().String()[:4]),
				Type:        TypeCollection,
				PaymentType: pType,
				Status:      status,
				Amount:      float64(rand.Intn(100)+1) * 10.0, // Random amount
				Currency:    "ZMW",
				AccountNumber: func() string {
					if pType == PayBank {
						return fmt.Sprintf("BANK%d", rand.Intn(999999))
					}
					return fmt.Sprintf("09%d", 700000000+rand.Intn(99999999))
				}(),
				Narration: fmt.Sprintf("%s %s transaction", status, pType),
				CreatedAt: time.Now().Add(-time.Duration(rand.Intn(24*30)) * time.Hour), // Last 30 days
				UpdatedAt: time.Now().Add(-time.Duration(rand.Intn(24*30)) * time.Hour),
				Message: func() *string {
					if status == StatusFailed {
						msg := errorMessages[rand.Intn(len(errorMessages))]
						return &msg
					}
					return nil
				}(),
			}); err != nil {
				return fmt.Errorf("failed to seed %s %s transaction: %w", status, pType, err)
			}
		}
	}

	// Seed old pending transactions (simulate timeouts)
	for i := 0; i < 5; i++ {
		oldPendingTime := time.Now().Add(-time.Hour * 24 * time.Duration(rand.Intn(60)+30)) // 30-90 days ago
		if err := createTransaction(db, Transaction{
			ID:            uuid.New().String(),
			WalletID:      walletHighBalanceID,
			ReferenceID:   fmt.Sprintf("OLD-PENDING-%s", uuid.New().String()[:4]),
			Type:          TypeCollection,
			PaymentType:   paymentTypes[rand.Intn(len(paymentTypes))],
			Status:        StatusPending,
			Amount:        float64(rand.Intn(50)+1) * 5.0,
			Currency:      "ZMW",
			AccountNumber: fmt.Sprintf("09%d", 700000000+rand.Intn(99999999)),
			Narration:     "Old pending transaction",
			CreatedAt:     oldPendingTime,
			UpdatedAt:     oldPendingTime,
		}); err != nil {
			return fmt.Errorf("failed to seed old pending transaction %d: %w", i, err)
		}
	}

	// Seed transactions for wallet with low balance (to test insufficient funds)
	for i := 0; i < 3; i++ {
		if err := createTransaction(db, Transaction{
			ID:            uuid.New().String(),
			WalletID:      walletLowBalanceID,
			ReferenceID:   fmt.Sprintf("LOW-BAL-%s", uuid.New().String()[:4]),
			Type:          TypeCollection,
			PaymentType:   paymentTypes[rand.Intn(len(paymentTypes))],
			Status:        StatusPending, // Can be pending, then fail later
			Amount:        float64(rand.Intn(20)+1) * 10.0,
			Currency:      "ZMW",
			AccountNumber: fmt.Sprintf("09%d", 700000000+rand.Intn(99999999)),
			Narration:     "Transaction from low balance wallet",
			CreatedAt:     time.Now().Add(-time.Duration(rand.Intn(24*7)) * time.Hour),
			UpdatedAt:     time.Now().Add(-time.Duration(rand.Intn(24*7)) * time.Hour),
		}); err != nil {
			return fmt.Errorf("failed to seed low balance wallet transaction %d: %w", i, err)
		}
	}

	log.Println("Test data seeding complete.")
	return nil
}

// createWallet inserts a new wallet into the database.
func createWallet(db *DB, wallet Wallet) error {
	stmt := `INSERT INTO wallets (id, name, balance, currency, api_key, till_number, is_active, created_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(stmt, wallet.ID, wallet.Name, wallet.Balance, wallet.Currency, wallet.APIKey, wallet.TillNumber, wallet.IsActive, wallet.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to insert wallet %s: %w", wallet.ID, err)
	}
	return nil
}

// createTransaction inserts a new transaction into the database.
func createTransaction(db *DB, tx Transaction) error {
	stmt := `INSERT INTO transactions (id, wallet_id, reference_id, identifier, type, payment_type, status, amount, currency, account_number, narration, ip_address, external_id, callback_url, card_redirect_url, message, customer_info, created_at, updated_at)
             VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// Ensure fields that can be null are handled correctly
	externalID := sql.NullString{}
	if tx.ExternalID != nil {
		externalID.String = *tx.ExternalID
		externalID.Valid = true
	}
	callbackURL := sql.NullString{}
	if tx.CallbackURL != nil {
		callbackURL.String = *tx.CallbackURL
		callbackURL.Valid = true
	}
	cardRedirectURL := sql.NullString{}
	if tx.CardRedirectURL != nil {
		cardRedirectURL.String = *tx.CardRedirectURL
		cardRedirectURL.Valid = true
	}
	message := sql.NullString{}
	if tx.Message != nil {
		message.String = *tx.Message
		message.Valid = true
	}

	// Default values for fields that might be empty
	if tx.Identifier == "" {
		tx.Identifier = uuid.New().String()
	}
	if tx.IPAddress == "" {
		tx.IPAddress = "127.0.0.1"
	}
	if tx.CustomerInfo == nil {
		emptyJSON := json.RawMessage("{}")
		tx.CustomerInfo = &emptyJSON
	}

	_, err := db.Exec(stmt,
		tx.ID,
		tx.WalletID,
		tx.ReferenceID,
		tx.Identifier,
		tx.Type,
		tx.PaymentType,
		tx.Status,
		tx.Amount,
		tx.Currency,
		tx.AccountNumber,
		tx.Narration,
		tx.IPAddress,
		externalID,
		callbackURL,
		cardRedirectURL,
		message,
		tx.CustomerInfo,
		tx.CreatedAt,
		tx.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert transaction %s: %w", tx.ID, err)
	}
	return nil
}

// clearData deletes all data from wallets and transactions tables. Use with caution.
func clearData(db *DB) error {
	log.Println("Clearing existing data...")
	_, err := db.Exec("DELETE FROM transactions")
	if err != nil {
		return fmt.Errorf("failed to delete transactions: %w", err)
	}
	_, err = db.Exec("DELETE FROM wallets")
	if err != nil {
		return fmt.Errorf("failed to delete wallets: %w", err)
	}
	log.Println("Data cleared.")
	return nil
}

// GenerateRandomTransactionHistory generates random transaction history for the last N days.
func GenerateRandomTransactionHistory(db *DB, walletID string, days int) error {
	log.Printf("Generating random transaction history for wallet %s for the last %d days...", walletID, days)

	paymentTypes := []string{PayCard, PayAirtelMoney, PayMtnMoney, PayZamtelKwacha, PayBank}
	statuses := []string{StatusPending, StatusSuccessful, StatusFailed}
	errorMessages := []string{
		"Insufficient funds",
		"Transaction declined by provider",
		"Invalid account number",
		"Service temporarily unavailable",
		"Timeout from external system",
		"Duplicate transaction reference",
		"Payment gateway error",
		"Customer cancelled transaction",
		"Fraudulent activity detected",
		"System maintenance",
	}

	// Get wallet currency
	var currency string
	err := db.QueryRow("SELECT currency FROM wallets WHERE id = ?", walletID).Scan(&currency)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("wallet with ID %s not found", walletID)
		}
		return fmt.Errorf("failed to get wallet currency: %w", err)
	}

	for i := 0; i < days; i++ {
		date := time.Now().AddDate(0, 0, -i)
		// Generate a random number of transactions for each day
		numTransactions := rand.Intn(10) + 5 // 5 to 14 transactions per day

		for j := 0; j < numTransactions; j++ {
			status := statuses[rand.Intn(len(statuses))]
			paymentType := paymentTypes[rand.Intn(len(paymentTypes))]
			amount := float64(rand.Intn(500)+1) * 10.0 // 10 to 5000 ZMW

			tx := Transaction{
				ID:          uuid.New().String(),
				WalletID:    walletID,
				ReferenceID: fmt.Sprintf("RAND-%s-%s-%d-%s", date.Format("0102"), status, j, uuid.New().String()[:4]),
				Type:        TypeCollection,
				PaymentType: paymentType,
				Status:      status,
				Amount:      amount,
				Currency:    currency,
				AccountNumber: func() string {
					if paymentType == PayBank {
						return fmt.Sprintf("BANK%d", rand.Intn(999999))
					}
					return fmt.Sprintf("09%d", 700000000+rand.Intn(99999999))
				}(),
				Narration: fmt.Sprintf("Random transaction on %s", date.Format("2006-01-02")),
				CreatedAt: date.Add(time.Duration(rand.Intn(24*60*60)) * time.Second), // Random time on that day
				UpdatedAt: date.Add(time.Duration(rand.Intn(24*60*60)) * time.Second),
				Message: func() *string {
					if status == StatusFailed {
						msg := errorMessages[rand.Intn(len(errorMessages))]
						return &msg
					}
					return nil
				}(),
			}

			if err := createTransaction(db, tx); err != nil {
				return fmt.Errorf("failed to generate random transaction for %s: %w", date.Format("2006-01-02"), err)
			}
		}
	}

	log.Printf("Random transaction history generation complete for wallet %s.", walletID)
	return nil
}

// SimulateStuckPendingTransactions changes the status of a few random pending transactions to failed or successful
// to simulate stuck transactions being processed by an external system.
func SimulateStuckPendingTransactions(db *DB) error {
	log.Println("Simulating stuck pending transactions...")

	rows, err := db.Query("SELECT id, wallet_id, amount, created_at FROM transactions WHERE status = ?", StatusPending)
	if err != nil {
		return fmt.Errorf("failed to query pending transactions: %w", err)
	}
	defer rows.Close()

	var pendingTransactions []Transaction
	for rows.Next() {
		var tx Transaction
		var createdAtStr string // Assuming created_at is TEXT in DB
		if err := rows.Scan(&tx.ID, &tx.WalletID, &tx.Amount, &createdAtStr); err != nil {
			log.Printf("Error scanning pending transaction: %v", err)
			continue
		}
		// Parse created_at string to time.Time
		tx.CreatedAt, err = time.Parse(time.RFC3339, createdAtStr) // Adjust format if needed
		if err != nil {
			log.Printf("Error parsing created_at timestamp for transaction %s: %v", tx.ID, err)
			continue
		}
		pendingTransactions = append(pendingTransactions, tx)
	}
	if err = rows.Err(); err != nil {
		return fmt.Errorf("error iterating pending transactions: %w", err)
	}

	if len(pendingTransactions) == 0 {
		log.Println("No pending transactions to simulate.")
		return nil
	}

	// Simulate processing for a few random pending transactions
	numToProcess := rand.Intn(len(pendingTransactions)/2) + 1 // Process 1 to half of pending
	if numToProcess > len(pendingTransactions) {
		numToProcess = len(pendingTransactions)
	}

	processedCount := 0
	for i := 0; i < numToProcess; i++ {
		idx := rand.Intn(len(pendingTransactions))
		tx := pendingTransactions[idx]

		newStatus := StatusSuccessful
		if rand.Intn(100) < 30 { // 30% chance to fail
			newStatus = StatusFailed
		}

		// Update transaction status
		stmt := `UPDATE transactions SET status = ?, updated_at = ? WHERE id = ?`
		_, err := db.Exec(stmt, newStatus, time.Now(), tx.ID)
		if err != nil {
			log.Printf("Failed to update status for transaction %s: %v", tx.ID, err)
			continue
		}
		processedCount++

		// If successful, update wallet balance (simplified for simulation)
		if newStatus == StatusSuccessful {
			updateWalletBalanceStmt := `UPDATE wallets SET balance = balance + ? WHERE id = ?`
			_, err := db.Exec(updateWalletBalanceStmt, tx.Amount, tx.WalletID)
			if err != nil {
				log.Printf("Failed to update wallet balance for successful transaction %s: %v", tx.ID, err)
			}
		}

		// Remove processed transaction from list to avoid reprocessing
		pendingTransactions = append(pendingTransactions[:idx], pendingTransactions[idx+1:]...)
	}

	log.Printf("Simulated processing for %d pending transactions.", processedCount)
	return nil
}
