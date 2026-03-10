package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strings"

	"github.com/Alinaswe3/mock-lipila/internal/database"
	"github.com/Alinaswe3/mock-lipila/internal/simulator"
)

// Handlers holds dependencies for API endpoint handlers.
type Handlers struct {
	DB  *database.DB
	Sim *simulator.Simulator
}

// MobileMoneyCollectionRequest is the JSON body for POST /api/v1/collections/mobile-money.
type MobileMoneyCollectionRequest struct {
	ReferenceID   string          `json:"referenceId"`
	Amount        float64         `json:"amount"`
	AccountNumber string          `json:"accountNumber"`
	Currency      string          `json:"currency"`
	Narration     string          `json:"narration"`
	CustomerInfo  json.RawMessage `json:"customerInfo,omitempty"`
}

// CardCollectionRequest is the JSON body for POST /api/v1/collections/card.
type CardCollectionRequest struct {
	CustomerInfo      CardCustomerInfo      `json:"customerInfo"`
	CollectionRequest CardCollectionDetails `json:"collectionRequest"`
}

// CardCustomerInfo holds customer details for card payments.
type CardCustomerInfo struct {
	FirstName   string `json:"firstName"`
	LastName    string `json:"lastName"`
	PhoneNumber string `json:"phoneNumber"`
	Email       string `json:"email"`
	City        string `json:"city"`
	Country     string `json:"country"`
	Address     string `json:"address"`
	Zip         string `json:"zip"`
}

// CardCollectionDetails holds the payment details for card collection.
type CardCollectionDetails struct {
	ReferenceID   string  `json:"referenceId"`
	Amount        float64 `json:"amount"`
	AccountNumber string  `json:"accountNumber"`
	Currency      string  `json:"currency"`
	BackURL       string  `json:"backUrl"`
	RedirectURL   string  `json:"redirectUrl"`
	Narration     string  `json:"narration"`
}

// MobileMoneyDisbursementRequest is the JSON body for POST /api/v1/disbursements/mobile-money.
type MobileMoneyDisbursementRequest struct {
	ReferenceID   string  `json:"referenceId"`
	Amount        float64 `json:"amount"`
	AccountNumber string  `json:"accountNumber"`
	Currency      string  `json:"currency"`
	Narration     string  `json:"narration,omitempty"`
}

// BankDisbursementRequest is the JSON body for POST /api/v1/disbursements/bank.
type BankDisbursementRequest struct {
	ReferenceID       string  `json:"referenceId"`
	Amount            float64 `json:"amount"`
	Currency          string  `json:"currency"`
	Narration         string  `json:"narration"`
	AccountNumber     string  `json:"accountNumber"`
	SwiftCode         string  `json:"swiftCode"`
	FirstName         string  `json:"firstName"`
	LastName          string  `json:"lastName"`
	AccountHolderName string  `json:"accountHolderName"`
	PhoneNumber       string  `json:"phoneNumber"`
	Email             string  `json:"email,omitempty"`
}

// DisbursementFeeRate is the percentage fee charged on mobile money disbursements (1.5%).
const DisbursementFeeRate = 0.015

// BankDisbursementFeeRate is the percentage fee charged on bank disbursements (2.5%).
const BankDisbursementFeeRate = 0.025

// TransactionResponse is the JSON response for a transaction.
type TransactionResponse struct {
	ID                 string  `json:"id"`
	ReferenceID        string  `json:"referenceId"`
	Identifier         string  `json:"identifier"`
	Type               string  `json:"type"`
	PaymentType        string  `json:"paymentType"`
	Status             string  `json:"status"`
	Amount             float64 `json:"amount"`
	Currency           string  `json:"currency"`
	AccountNumber      string  `json:"accountNumber"`
	Narration          string  `json:"narration"`
	IPAddress          string  `json:"ipAddress,omitempty"`
	ExternalID         *string `json:"externalId,omitempty"`
	CardRedirectionURL *string `json:"cardRedirectionUrl,omitempty"`
	Message            *string `json:"message,omitempty"`
	CreatedAt          string  `json:"createdAt"`
}

// HandleMobileMoneyCollection processes POST /api/v1/collections/mobile-money.
func (h *Handlers) HandleMobileMoneyCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error":   "METHOD_NOT_ALLOWED",
			"message": "only POST is allowed",
		})
		return
	}

	var req MobileMoneyCollectionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	// Validate required fields
	missing := collectMissing(map[string]string{
		"accountNumber": req.AccountNumber,
		"currency":      req.Currency,
		"narration":     req.Narration,
	})
	if len(missing) > 0 {
		writeValidationError(w, "missing required fields: "+strings.Join(missing, ", "))
		return
	}

	if err := ValidateReferenceId(req.ReferenceID); err != nil {
		writeValidationError(w, err.Error())
		return
	}
	if err := ValidateAmount(req.Amount); err != nil {
		writeValidationError(w, err.Error())
		return
	}
	if err := ValidateCurrency(req.Currency); err != nil {
		writeValidationError(w, err.Error())
		return
	}

	// Validate phone number and detect MNO
	paymentType, err := ValidatePhoneNumber(req.AccountNumber)
	if err != nil {
		writeValidationError(w, err.Error())
		return
	}

	// Check for duplicate referenceId
	if !h.checkDuplicateRef(w, req.ReferenceID) {
		return
	}

	walletID := getWalletID(r)

	var customerInfoPtr *json.RawMessage
	if len(req.CustomerInfo) > 0 {
		customerInfoPtr = &req.CustomerInfo
	}

	txn := &database.Transaction{
		ID:            database.GenerateUUID(),
		WalletID:      walletID,
		ReferenceID:   req.ReferenceID,
		Identifier:    database.GenerateLipilaIdentifier(database.TypeCollection),
		Type:          database.TypeCollection,
		PaymentType:   paymentType,
		Status:        database.StatusPending,
		Amount:        req.Amount,
		Currency:      req.Currency,
		AccountNumber: req.AccountNumber,
		Narration:     req.Narration,
		IPAddress:     clientIP(r),
		CallbackURL:   callbackURLFromHeader(r),
		CustomerInfo:  customerInfoPtr,
	}

	if err := h.DB.InsertTransaction(txn); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "SERVER_ERROR",
			"message": "failed to create transaction",
		})
		return
	}

	h.Sim.ProcessCollection(txn)

	writeJSON(w, http.StatusOK, TransactionResponse{
		ID:            txn.ID,
		ReferenceID:   txn.ReferenceID,
		Identifier:    txn.Identifier,
		Type:          txn.Type,
		PaymentType:   txn.PaymentType,
		Status:        txn.Status,
		Amount:        txn.Amount,
		Currency:      txn.Currency,
		AccountNumber: txn.AccountNumber,
		Narration:     txn.Narration,
		CreatedAt:     txn.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// HandleCardCollection processes POST /api/v1/collections/card.
func (h *Handlers) HandleCardCollection(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error":   "METHOD_NOT_ALLOWED",
			"message": "only POST is allowed",
		})
		return
	}

	var req CardCollectionRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	// Validate customerInfo required fields
	ci := req.CustomerInfo
	missing := collectMissing(map[string]string{
		"customerInfo.firstName":   ci.FirstName,
		"customerInfo.lastName":    ci.LastName,
		"customerInfo.phoneNumber": ci.PhoneNumber,
		"customerInfo.email":       ci.Email,
		"customerInfo.city":        ci.City,
		"customerInfo.country":     ci.Country,
		"customerInfo.address":     ci.Address,
		"customerInfo.zip":         ci.Zip,
	})

	// Validate collectionRequest required fields
	cr := req.CollectionRequest
	crMissing := collectMissing(map[string]string{
		"collectionRequest.accountNumber": cr.AccountNumber,
		"collectionRequest.currency":      cr.Currency,
		"collectionRequest.backUrl":       cr.BackURL,
		"collectionRequest.redirectUrl":   cr.RedirectURL,
		"collectionRequest.narration":     cr.Narration,
	})
	missing = append(missing, crMissing...)

	if len(missing) > 0 {
		writeValidationError(w, "missing required fields: "+strings.Join(missing, ", "))
		return
	}

	if err := ValidateReferenceId(cr.ReferenceID); err != nil {
		writeValidationError(w, "collectionRequest.referenceId: "+err.Error())
		return
	}
	if err := ValidateAmount(cr.Amount); err != nil {
		writeValidationError(w, "collectionRequest.amount: "+err.Error())
		return
	}
	if err := ValidateCountryCode(ci.Country); err != nil {
		writeValidationError(w, "customerInfo.country: "+err.Error())
		return
	}
	if err := ValidateCurrency(cr.Currency); err != nil {
		writeValidationError(w, err.Error())
		return
	}

	// Check for duplicate referenceId
	if !h.checkDuplicateRef(w, cr.ReferenceID) {
		return
	}

	// Marshal customer info to JSON for storage
	customerInfoJSON, err := json.Marshal(ci)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "SERVER_ERROR",
			"message": "failed to process customer info",
		})
		return
	}

	walletID := getWalletID(r)
	cardRedirectURL := "https://secure.3gdirectpay.com/payv2.php?ID=" + database.GenerateUUID()

	var customerInfoPtr *json.RawMessage
	if len(customerInfoJSON) > 0 {
		temp := json.RawMessage(customerInfoJSON)
		customerInfoPtr = &temp
	}

	txn := &database.Transaction{
		ID:              database.GenerateUUID(),
		WalletID:        walletID,
		ReferenceID:     cr.ReferenceID,
		Identifier:      database.GenerateLipilaIdentifier(database.TypeCollection),
		Type:            database.TypeCollection,
		PaymentType:     database.PayCard,
		Status:          database.StatusPending,
		Amount:          cr.Amount,
		Currency:        cr.Currency,
		AccountNumber:   cr.AccountNumber,
		Narration:       cr.Narration,
		IPAddress:       clientIP(r),
		CallbackURL:     callbackURLFromHeader(r),
		CardRedirectURL: &cardRedirectURL,
		CustomerInfo:    customerInfoPtr,
	}

	if err := h.DB.InsertTransaction(txn); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "SERVER_ERROR",
			"message": "failed to create transaction",
		})
		return
	}

	h.Sim.ProcessCardCollection(txn)

	writeJSON(w, http.StatusOK, TransactionResponse{
		ID:                 txn.ID,
		ReferenceID:        txn.ReferenceID,
		Identifier:         txn.Identifier,
		Type:               txn.Type,
		PaymentType:        txn.PaymentType,
		Status:             txn.Status,
		Amount:             txn.Amount,
		Currency:           txn.Currency,
		AccountNumber:      txn.AccountNumber,
		Narration:          txn.Narration,
		CardRedirectionURL: txn.CardRedirectURL,
		CreatedAt:          txn.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// HandleCheckCollectionStatus returns the status of a collection transaction.
// GET /api/v1/collections/check-status?referenceId=xxx
func (h *Handlers) HandleCheckCollectionStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error":   "METHOD_NOT_ALLOWED",
			"message": "only GET is allowed",
		})
		return
	}

	refID := r.URL.Query().Get("referenceId")
	if refID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "BAD_REQUEST",
			"message": "referenceId query parameter is required",
		})
		return
	}

	txn, err := h.DB.GetTransactionByReferenceID(refID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error":   "NOT_FOUND",
			"message": "transaction not found",
		})
		return
	}

	// Verify the transaction belongs to the authenticated wallet
	walletID := getWalletID(r)
	if txn.WalletID != walletID {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error":   "NOT_FOUND",
			"message": "transaction not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, TransactionResponse{
		ID:                 txn.ID,
		ReferenceID:        txn.ReferenceID,
		Identifier:         txn.Identifier,
		Type:               txn.Type,
		PaymentType:        txn.PaymentType,
		Status:             txn.Status,
		Amount:             txn.Amount,
		Currency:           txn.Currency,
		AccountNumber:      txn.AccountNumber,
		Narration:          txn.Narration,
		IPAddress:          txn.IPAddress,
		ExternalID:         txn.ExternalID,
		CardRedirectionURL: txn.CardRedirectURL,
		Message:            txn.Message,
		CreatedAt:          txn.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// HandleCheckDisbursementStatus returns the status of a disbursement transaction.
// GET /api/v1/disbursements/check-status?referenceId=xxx
func (h *Handlers) HandleCheckDisbursementStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error":   "METHOD_NOT_ALLOWED",
			"message": "only GET is allowed",
		})
		return
	}

	refID := r.URL.Query().Get("referenceId")
	if refID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "BAD_REQUEST",
			"message": "referenceId query parameter is required",
		})
		return
	}

	txn, err := h.DB.GetTransactionByReferenceID(refID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error":   "NOT_FOUND",
			"message": "transaction not found",
		})
		return
	}

	// Verify the transaction is a disbursement
	if txn.Type != database.TypeDisbursement {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error":   "NOT_FOUND",
			"message": "transaction not found",
		})
		return
	}

	// Verify the transaction belongs to the authenticated wallet
	walletID := getWalletID(r)
	if txn.WalletID != walletID {
		writeJSON(w, http.StatusNotFound, map[string]string{
			"error":   "NOT_FOUND",
			"message": "transaction not found",
		})
		return
	}

	writeJSON(w, http.StatusOK, TransactionResponse{
		ID:            txn.ID,
		ReferenceID:   txn.ReferenceID,
		Identifier:    txn.Identifier,
		Type:          txn.Type,
		PaymentType:   txn.PaymentType,
		Status:        txn.Status,
		Amount:        txn.Amount,
		Currency:      txn.Currency,
		AccountNumber: txn.AccountNumber,
		Narration:     txn.Narration,
		IPAddress:     txn.IPAddress,
		ExternalID:    txn.ExternalID,
		Message:       txn.Message,
		CreatedAt:     txn.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// HandleMobileMoneyDisbursement processes POST /api/v1/disbursements/mobile-money.
func (h *Handlers) HandleMobileMoneyDisbursement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error":   "METHOD_NOT_ALLOWED",
			"message": "only POST is allowed",
		})
		return
	}

	var req MobileMoneyDisbursementRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	// Validate required fields
	missing := collectMissing(map[string]string{
		"accountNumber": req.AccountNumber,
		"currency":      req.Currency,
	})
	if len(missing) > 0 {
		writeValidationError(w, "missing required fields: "+strings.Join(missing, ", "))
		return
	}

	if err := ValidateReferenceId(req.ReferenceID); err != nil {
		writeValidationError(w, err.Error())
		return
	}
	if err := ValidateAmount(req.Amount); err != nil {
		writeValidationError(w, err.Error())
		return
	}
	if err := ValidateCurrency(req.Currency); err != nil {
		writeValidationError(w, err.Error())
		return
	}

	// Validate phone number and detect MNO
	paymentType, err := ValidatePhoneNumber(req.AccountNumber)
	if err != nil {
		writeValidationError(w, err.Error())
		return
	}

	// Check for duplicate referenceId
	if !h.checkDuplicateRef(w, req.ReferenceID) {
		return
	}

	walletID := getWalletID(r)

	// Calculate fee (1.5%) and total deduction
	fee := math.Ceil(req.Amount*DisbursementFeeRate*100) / 100 // round up to 2 decimal places
	totalDeduction := req.Amount + fee

	// Deduct amount + fee from wallet balance
	if err := h.DB.DeductWalletBalance(walletID, totalDeduction); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "INSUFFICIENT_BALANCE",
			"message": fmt.Sprintf("insufficient wallet balance: requires %.2f (amount %.2f + fee %.2f)", totalDeduction, req.Amount, fee),
		})
		return
	}

	txn := &database.Transaction{
		ID:            database.GenerateUUID(),
		WalletID:      walletID,
		ReferenceID:   req.ReferenceID,
		Identifier:    database.GenerateLipilaIdentifier(database.TypeDisbursement),
		Type:          database.TypeDisbursement,
		PaymentType:   paymentType,
		Status:        database.StatusPending,
		Amount:        req.Amount,
		Currency:      req.Currency,
		AccountNumber: req.AccountNumber,
		Narration:     req.Narration,
		IPAddress:     clientIP(r),
		CallbackURL:   callbackURLFromHeader(r),
	}

	if err := h.DB.InsertTransaction(txn); err != nil {
		// Refund the deducted amount on insert failure
		h.DB.RefundWalletBalance(walletID, totalDeduction)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "SERVER_ERROR",
			"message": "failed to create transaction",
		})
		return
	}

	h.Sim.ProcessDisbursement(txn)

	writeJSON(w, http.StatusOK, TransactionResponse{
		ID:            txn.ID,
		ReferenceID:   txn.ReferenceID,
		Identifier:    txn.Identifier,
		Type:          txn.Type,
		PaymentType:   txn.PaymentType,
		Status:        txn.Status,
		Amount:        txn.Amount,
		Currency:      txn.Currency,
		AccountNumber: txn.AccountNumber,
		Narration:     txn.Narration,
		CreatedAt:     txn.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// HandleBankDisbursement processes POST /api/v1/disbursements/bank.
func (h *Handlers) HandleBankDisbursement(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error":   "METHOD_NOT_ALLOWED",
			"message": "only POST is allowed",
		})
		return
	}

	var req BankDisbursementRequest
	if !decodeJSONBody(w, r, &req) {
		return
	}

	// Validate required fields
	missing := collectMissing(map[string]string{
		"currency":          req.Currency,
		"narration":         req.Narration,
		"accountNumber":     req.AccountNumber,
		"swiftCode":         req.SwiftCode,
		"firstName":         req.FirstName,
		"lastName":          req.LastName,
		"accountHolderName": req.AccountHolderName,
		"phoneNumber":       req.PhoneNumber,
	})
	if len(missing) > 0 {
		writeValidationError(w, "missing required fields: "+strings.Join(missing, ", "))
		return
	}

	if err := ValidateReferenceId(req.ReferenceID); err != nil {
		writeValidationError(w, err.Error())
		return
	}
	if err := ValidateAmount(req.Amount); err != nil {
		writeValidationError(w, err.Error())
		return
	}
	if err := ValidateCurrency(req.Currency); err != nil {
		writeValidationError(w, err.Error())
		return
	}
	if err := ValidateSwiftCode(req.SwiftCode); err != nil {
		writeValidationError(w, err.Error())
		return
	}

	// Check for duplicate referenceId
	if !h.checkDuplicateRef(w, req.ReferenceID) {
		return
	}

	walletID := getWalletID(r)

	// Calculate fee (2.5%) and total deduction
	fee := math.Ceil(req.Amount*BankDisbursementFeeRate*100) / 100
	totalDeduction := req.Amount + fee

	// Deduct amount + fee from wallet balance
	if err := h.DB.DeductWalletBalance(walletID, totalDeduction); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "INSUFFICIENT_BALANCE",
			"message": fmt.Sprintf("insufficient wallet balance: requires %.2f (amount %.2f + fee %.2f)", totalDeduction, req.Amount, fee),
		})
		return
	}

	// Marshal bank details as customerInfo for storage
	bankDetails := map[string]string{
		"firstName":         req.FirstName,
		"lastName":          req.LastName,
		"accountHolderName": req.AccountHolderName,
		"phoneNumber":       req.PhoneNumber,
		"swiftCode":         req.SwiftCode,
	}
	if req.Email != "" {
		bankDetails["email"] = req.Email
	}
	customerInfoJSON, err := json.Marshal(bankDetails)
	if err != nil {
		h.DB.RefundWalletBalance(walletID, totalDeduction)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "SERVER_ERROR",
			"message": "failed to process bank details",
		})
		return
	}

	txn := &database.Transaction{
		ID:            database.GenerateUUID(),
		WalletID:      walletID,
		ReferenceID:   req.ReferenceID,
		Identifier:    database.GenerateLipilaIdentifier(database.TypeDisbursement),
		Type:          database.TypeDisbursement,
		PaymentType:   database.PayBank,
		Status:        database.StatusPending,
		Amount:        req.Amount,
		Currency:      req.Currency,
		AccountNumber: req.AccountNumber,
		Narration:     req.Narration,
		IPAddress:     clientIP(r),
		CallbackURL:   callbackURLFromHeader(r),
		CustomerInfo:  func() *json.RawMessage {
			if len(customerInfoJSON) > 0 {
				temp := json.RawMessage(customerInfoJSON)
				return &temp
			}
			return nil
		}(),
	}

	if err := h.DB.InsertTransaction(txn); err != nil {
		h.DB.RefundWalletBalance(walletID, totalDeduction)
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "SERVER_ERROR",
			"message": "failed to create transaction",
		})
		return
	}

	h.Sim.ProcessBankDisbursement(txn)

	writeJSON(w, http.StatusOK, TransactionResponse{
		ID:            txn.ID,
		ReferenceID:   txn.ReferenceID,
		Identifier:    txn.Identifier,
		Type:          txn.Type,
		PaymentType:   txn.PaymentType,
		Status:        txn.Status,
		Amount:        txn.Amount,
		Currency:      txn.Currency,
		AccountNumber: txn.AccountNumber,
		Narration:     txn.Narration,
		CreatedAt:     txn.CreatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// HandleMerchantBalance returns the authenticated wallet's current balance.
// GET /api/v1/merchants/balance
func (h *Handlers) HandleMerchantBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
			"error":   "METHOD_NOT_ALLOWED",
			"message": "only GET is allowed",
		})
		return
	}

	walletID := getWalletID(r)
	wallet, err := h.DB.GetWalletByID(walletID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "SERVER_ERROR",
			"message": "failed to retrieve wallet",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "Wallet balance retrieved successfully.",
		"data": map[string]interface{}{
			"balance": wallet.Balance,
		},
	})
}

// detectMNO returns the payment type based on the Zambian phone prefix.
func detectMNO(accountNumber string) string {
	if len(accountNumber) < 5 {
		return ""
	}
	prefix := accountNumber[:5]
	switch prefix {
	case "26096", "26076":
		return database.PayMtnMoney
	case "26097", "26077":
		return database.PayAirtelMoney
	case "26095", "26075":
		return database.PayZamtelKwacha
	default:
		return ""
	}
}

// callbackURLFromHeader reads the callback URL from the callbackUrl request header.
func callbackURLFromHeader(r *http.Request) *string {
	if v := r.Header.Get("callbackUrl"); v != "" {
		return &v
	}
	return nil
}

// getWalletID extracts the wallet ID from the request context (set by auth middleware).
func getWalletID(r *http.Request) string {
	if id, ok := r.Context().Value(walletIDContextKey).(string); ok {
		return id
	}
	return ""
}
