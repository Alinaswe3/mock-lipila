package database

import (
	"encoding/json"
	"time"
)

// Transaction type constants.
const (
	TypeCollection   = "Collection"
	TypeDisbursement = "Disbursement"
	TypeSettlement   = "Settlement"
	TypeAllocation   = "Allocation"
	TypeTransfer     = "Transfer"
)

// Payment type constants.
const (
	PayCard         = "Card"
	PayAirtelMoney  = "AirtelMoney"
	PayMtnMoney     = "MtnMoney"
	PayZamtelKwacha = "ZamtelKwacha"
	PayBank         = "Bank"
)

// Transaction status constants.
const (
	StatusPending    = "Pending"
	StatusSuccessful = "Successful"
	StatusFailed     = "Failed"
)

// Wallet represents a merchant wallet.
type Wallet struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Balance    float64   `json:"balance"`
	Currency   string    `json:"currency"`
	APIKey     string    `json:"api_key"`
	TillNumber string    `json:"till_number"`
	IsActive   bool      `json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
}

// Transaction represents a payment transaction record.
type Transaction struct {
	ID              string          `json:"id"`
	WalletID        string          `json:"wallet_id"`
	ReferenceID     string          `json:"reference_id"`
	Identifier      string          `json:"identifier"`
	Type            string          `json:"type"`
	PaymentType     string          `json:"payment_type"`
	Status          string          `json:"status"`
	Amount          float64         `json:"amount"`
	Currency        string          `json:"currency"`
	AccountNumber   string          `json:"account_number"`
	Narration       string          `json:"narration"`
	IPAddress       string          `json:"ip_address"`
	ExternalID      *string         `json:"external_id,omitempty"`
	CallbackURL     *string         `json:"callback_url,omitempty"`
	CardRedirectURL *string         `json:"card_redirect_url,omitempty"`
	Message         *string         `json:"message,omitempty"`
	CustomerInfo    *json.RawMessage `json:"customer_info,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
}

// SimulationConfig controls how the simulator behaves.
type SimulationConfig struct {
	ID                     int  `json:"id"`
	MtnSuccessRate         int  `json:"mtn_success_rate"`
	AirtelSuccessRate      int  `json:"airtel_success_rate"`
	ZamtelSuccessRate      int  `json:"zamtel_success_rate"`
	CardSuccessRate        int  `json:"card_success_rate"`
	BankSuccessRate        int  `json:"bank_success_rate"`
	ProcessingDelaySeconds int  `json:"processing_delay_seconds"`
	EnableRandomTimeouts   bool `json:"enable_random_timeouts"`
	TimeoutProbability     int  `json:"timeout_probability"`
}

// CallbackLog records each callback delivery attempt.
type CallbackLog struct {
	ID            int64
	TransactionID string
	URL           string
	StatusCode    int
	ResponseBody  string
	Error         string
	AttemptedAt   time.Time
}
