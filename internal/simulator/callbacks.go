package simulator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Alinaswe3/mock-lipila/internal/database"
)

const (
	maxCallbackRetries = 3
	callbackTimeout    = 10 * time.Second
)

// CallbackPayload is the JSON body sent to callback URLs.
type CallbackPayload struct {
	ID            string  `json:"id"`
	ReferenceID   string  `json:"referenceId"`
	Identifier    string  `json:"identifier"`
	Type          string  `json:"type"`
	PaymentType   string  `json:"paymentType"`
	Status        string  `json:"status"`
	Amount        float64 `json:"amount"`
	Currency      string  `json:"currency"`
	AccountNumber string  `json:"accountNumber"`
	Narration     string  `json:"narration"`
	ExternalID    *string `json:"externalId,omitempty"`
	Message       *string `json:"message,omitempty"`
	Timestamp     string  `json:"timestamp"`
}

// sendCallback sends a POST request to the transaction's callback URL with retry logic.
// Retries up to 3 times with exponential backoff (2s, 4s, 8s).
func (s *Simulator) sendCallback(txn *database.Transaction) {
	if txn.CallbackURL == nil || *txn.CallbackURL == "" {
		return
	}

	payload := CallbackPayload{
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
		ExternalID:    txn.ExternalID,
		Message:       txn.Message,
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("callback: error marshaling payload for %s: %v", txn.ReferenceID, err)
		return
	}

	client := &http.Client{Timeout: callbackTimeout}
	backoff := 2 * time.Second

	for attempt := 1; attempt <= maxCallbackRetries; attempt++ {
		cl := &database.CallbackLog{
			TransactionID: txn.ID,
			URL:           *txn.CallbackURL,
		}

		resp, err := client.Post(*txn.CallbackURL, "application/json", bytes.NewReader(body))
		if err != nil {
			cl.Error = fmt.Sprintf("attempt %d: request failed: %v", attempt, err)
			log.Printf("callback: attempt %d/%d failed for %s: %v", attempt, maxCallbackRetries, txn.ReferenceID, err)
			s.DB.InsertCallbackLog(cl)

			if attempt < maxCallbackRetries {
				time.Sleep(backoff)
				backoff *= 2
			}
			continue
		}

		var respBody bytes.Buffer
		respBody.ReadFrom(resp.Body)
		resp.Body.Close()

		cl.StatusCode = resp.StatusCode
		cl.ResponseBody = respBody.String()
		s.DB.InsertCallbackLog(cl)

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			log.Printf("callback: delivered for %s on attempt %d (status %d)", txn.ReferenceID, attempt, resp.StatusCode)
			return
		}

		log.Printf("callback: attempt %d/%d non-2xx for %s (status %d)", attempt, maxCallbackRetries, txn.ReferenceID, resp.StatusCode)
		if attempt < maxCallbackRetries {
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	log.Printf("callback: all %d attempts exhausted for %s", maxCallbackRetries, txn.ReferenceID)
}
