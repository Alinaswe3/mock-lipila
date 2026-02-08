package simulator

import (
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/Alinaswe3/mock-lipila/internal/database"
)

// Fee rates charged on collections (deducted from wallet credit).
const (
	CollectionFeeRate     = 0.025 // 2.5% for mobile money collections
	CardCollectionFeeRate = 0.04  // 4% for card collections
)

// maxConcurrentSimulations limits how many goroutines the simulator runs at once.
const maxConcurrentSimulations = 50

// Simulator handles asynchronous transaction processing.
type Simulator struct {
	DB  *database.DB
	sem chan struct{} // semaphore to limit concurrent goroutines
}

// NewSimulator creates a new Simulator with a bounded goroutine pool.
func NewSimulator(db *database.DB) *Simulator {
	return &Simulator{
		DB:  db,
		sem: make(chan struct{}, maxConcurrentSimulations),
	}
}

// acquire reserves a slot in the goroutine pool.
func (s *Simulator) acquire() { s.sem <- struct{}{} }

// release frees a slot in the goroutine pool.
func (s *Simulator) release() { <-s.sem }

// ProcessCollection simulates processing a mobile money collection asynchronously.
// On success, the wallet is credited with amount minus the 2.5% collection fee.
func (s *Simulator) ProcessCollection(txn *database.Transaction) {
	s.acquire()
	go func() {
		defer s.release()
		cfg, err := s.DB.GetSimulationConfig()
		if err != nil {
			log.Printf("simulator: error loading config: %v", err)
			return
		}

		delay := time.Duration(cfg.ProcessingDelaySeconds) * time.Second
		time.Sleep(delay)

		// Check for random timeout
		if cfg.EnableRandomTimeouts && rand.Intn(100) < cfg.TimeoutProbability {
			msg := "transaction timed out"
			s.DB.UpdateTransactionStatus(txn.ID, database.StatusFailed, &msg)
			txn.Status = database.StatusFailed
			txn.Message = &msg
			log.Printf("simulator: collection %s -> timeout", txn.ReferenceID)
			s.sendCallback(txn)
			return
		}

		successRate := getSuccessRate(cfg, txn.PaymentType)
		if rand.Intn(100) < successRate {
			// Success — credit wallet with amount minus fee
			fee := math.Ceil(txn.Amount*CollectionFeeRate*100) / 100
			credit := txn.Amount - fee
			if err := s.DB.RefundWalletBalance(txn.WalletID, credit); err != nil {
				log.Printf("simulator: error crediting wallet for collection %s: %v", txn.ReferenceID, err)
			} else {
				log.Printf("simulator: credited %.2f to wallet (amount %.2f - fee %.2f) for collection %s",
					credit, txn.Amount, fee, txn.ReferenceID)
			}

			extID := database.GenerateMNOExternalID()
			s.DB.SetTransactionExternalID(txn.ID, extID)
			msg := successMessage(txn.PaymentType)
			s.DB.UpdateTransactionStatus(txn.ID, database.StatusSuccessful, &msg)
			txn.Status = database.StatusSuccessful
			txn.ExternalID = &extID
			txn.Message = &msg
			log.Printf("simulator: collection %s -> Successful (ext: %s)", txn.ReferenceID, extID)
		} else {
			// Failure
			msg := failureMessage(txn.PaymentType)
			s.DB.UpdateTransactionStatus(txn.ID, database.StatusFailed, &msg)
			txn.Status = database.StatusFailed
			txn.Message = &msg
			log.Printf("simulator: collection %s -> Failed (%s)", txn.ReferenceID, msg)
		}

		s.sendCallback(txn)
	}()
}

// ProcessCardCollection simulates processing a card collection asynchronously.
// On success, the wallet is credited with amount minus the 4% card collection fee.
func (s *Simulator) ProcessCardCollection(txn *database.Transaction) {
	s.acquire()
	go func() {
		defer s.release()
		cfg, err := s.DB.GetSimulationConfig()
		if err != nil {
			log.Printf("simulator: error loading config: %v", err)
			return
		}

		delay := time.Duration(cfg.ProcessingDelaySeconds) * time.Second
		time.Sleep(delay)

		if cfg.EnableRandomTimeouts && rand.Intn(100) < cfg.TimeoutProbability {
			msg := "transaction timed out"
			s.DB.UpdateTransactionStatus(txn.ID, database.StatusFailed, &msg)
			txn.Status = database.StatusFailed
			txn.Message = &msg
			log.Printf("simulator: card collection %s -> timeout", txn.ReferenceID)
			s.sendCallback(txn)
			return
		}

		if rand.Intn(100) < cfg.CardSuccessRate {
			// Success — credit wallet with amount minus 4% fee
			fee := math.Ceil(txn.Amount*CardCollectionFeeRate*100) / 100
			credit := txn.Amount - fee
			if err := s.DB.RefundWalletBalance(txn.WalletID, credit); err != nil {
				log.Printf("simulator: error crediting wallet for card collection %s: %v", txn.ReferenceID, err)
			} else {
				log.Printf("simulator: credited %.2f to wallet (amount %.2f - fee %.2f) for card collection %s",
					credit, txn.Amount, fee, txn.ReferenceID)
			}

			extID := database.GenerateMNOExternalID()
			s.DB.SetTransactionExternalID(txn.ID, extID)
			msg := "Card payment processed successfully"
			s.DB.UpdateTransactionStatus(txn.ID, database.StatusSuccessful, &msg)
			txn.Status = database.StatusSuccessful
			txn.ExternalID = &extID
			txn.Message = &msg
			log.Printf("simulator: card collection %s -> Successful (ext: %s)", txn.ReferenceID, extID)
		} else {
			msg := cardFailureMessage()
			s.DB.UpdateTransactionStatus(txn.ID, database.StatusFailed, &msg)
			txn.Status = database.StatusFailed
			txn.Message = &msg
			log.Printf("simulator: card collection %s -> Failed (%s)", txn.ReferenceID, msg)
		}

		s.sendCallback(txn)
	}()
}

var cardFailureMessages = []string{
	"Card declined by issuer",
	"Insufficient funds",
	"Card verification failed",
	"3D Secure authentication failed",
}

func cardFailureMessage() string {
	return cardFailureMessages[rand.Intn(len(cardFailureMessages))]
}

// getSuccessRate returns the configured success rate for the given payment type.
func getSuccessRate(cfg *database.SimulationConfig, paymentType string) int {
	switch paymentType {
	case database.PayMtnMoney:
		return cfg.MtnSuccessRate
	case database.PayAirtelMoney:
		return cfg.AirtelSuccessRate
	case database.PayZamtelKwacha:
		return cfg.ZamtelSuccessRate
	case database.PayCard:
		return cfg.CardSuccessRate
	case database.PayBank:
		return cfg.BankSuccessRate
	default:
		return 80
	}
}

func successMessage(paymentType string) string {
	switch paymentType {
	case database.PayMtnMoney:
		return "MTN Money payment received successfully"
	case database.PayAirtelMoney:
		return "Airtel Money payment received successfully"
	case database.PayZamtelKwacha:
		return "Zamtel Kwacha payment received successfully"
	default:
		return "payment received successfully"
	}
}

func failureMessage(paymentType string) string {
	switch paymentType {
	case database.PayMtnMoney:
		return "MTN Money: insufficient funds or subscriber not found"
	case database.PayAirtelMoney:
		return "Airtel Money: transaction declined by subscriber"
	case database.PayZamtelKwacha:
		return "Zamtel Kwacha: service temporarily unavailable"
	default:
		return "transaction failed"
	}
}
