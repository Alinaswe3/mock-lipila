package simulator

import (
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/Alinaswe3/mock-lipila/internal/database"
)

// DisbursementFeeRate is the percentage fee charged on mobile money disbursements (1.5%).
const DisbursementFeeRate = 0.015

// BankDisbursementFeeRate is the percentage fee charged on bank disbursements (2.5%).
const BankDisbursementFeeRate = 0.025

// ProcessDisbursement simulates processing a mobile money disbursement asynchronously.
// Fee of 1.5% is pre-deducted by the handler; refunded on failure.
func (s *Simulator) ProcessDisbursement(txn *database.Transaction) {
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
			if err := s.DB.UpdateTransactionStatus(txn.ID, database.StatusFailed, &msg); err != nil {
				log.Printf("simulator: error updating disbursement %s: %v", txn.ReferenceID, err)
			}
			txn.Status = database.StatusFailed
			txn.Message = &msg
			log.Printf("simulator: disbursement %s -> timeout", txn.ReferenceID)
			s.refundDisbursement(txn, DisbursementFeeRate)
			s.sendCallback(txn)
			return
		}

		successRate := getSuccessRate(cfg, txn.PaymentType)

		if rand.Intn(100) < successRate {
			extID := database.GenerateMNOExternalID()
			s.DB.SetTransactionExternalID(txn.ID, extID)
			msg := disbursementSuccessMessage(txn.PaymentType)
			if err := s.DB.UpdateTransactionStatus(txn.ID, database.StatusSuccessful, &msg); err != nil {
				log.Printf("simulator: error updating disbursement %s: %v", txn.ReferenceID, err)
				return
			}
			txn.Status = database.StatusSuccessful
			txn.ExternalID = &extID
			txn.Message = &msg
			log.Printf("simulator: disbursement %s -> Successful (ext: %s)", txn.ReferenceID, extID)
		} else {
			msg := disbursementFailureMessage(txn.PaymentType)
			if err := s.DB.UpdateTransactionStatus(txn.ID, database.StatusFailed, &msg); err != nil {
				log.Printf("simulator: error updating disbursement %s: %v", txn.ReferenceID, err)
				return
			}
			txn.Status = database.StatusFailed
			txn.Message = &msg
			log.Printf("simulator: disbursement %s -> Failed (%s)", txn.ReferenceID, msg)
			s.refundDisbursement(txn, DisbursementFeeRate)
		}

		s.sendCallback(txn)
	}()
}

// ProcessBankDisbursement simulates processing a bank disbursement asynchronously.
// Fee of 2.5% is pre-deducted by the handler; refunded on failure.
func (s *Simulator) ProcessBankDisbursement(txn *database.Transaction) {
	s.acquire()
	go func() {
		defer s.release()
		cfg, err := s.DB.GetSimulationConfig()
		if err != nil {
			log.Printf("simulator: error loading config: %v", err)
			return
		}

		// Bank transfers take longer (2x delay)
		delay := time.Duration(cfg.ProcessingDelaySeconds*2) * time.Second
		time.Sleep(delay)

		if cfg.EnableRandomTimeouts && rand.Intn(100) < cfg.TimeoutProbability {
			msg := "transaction timed out"
			if err := s.DB.UpdateTransactionStatus(txn.ID, database.StatusFailed, &msg); err != nil {
				log.Printf("simulator: error updating bank disbursement %s: %v", txn.ReferenceID, err)
			}
			txn.Status = database.StatusFailed
			txn.Message = &msg
			log.Printf("simulator: bank disbursement %s -> timeout", txn.ReferenceID)
			s.refundDisbursement(txn, BankDisbursementFeeRate)
			s.sendCallback(txn)
			return
		}

		if rand.Intn(100) < cfg.BankSuccessRate {
			extID := database.GenerateMNOExternalID()
			s.DB.SetTransactionExternalID(txn.ID, extID)
			msg := "Bank transfer completed successfully"
			if err := s.DB.UpdateTransactionStatus(txn.ID, database.StatusSuccessful, &msg); err != nil {
				log.Printf("simulator: error updating bank disbursement %s: %v", txn.ReferenceID, err)
				return
			}
			txn.Status = database.StatusSuccessful
			txn.ExternalID = &extID
			txn.Message = &msg
			log.Printf("simulator: bank disbursement %s -> Successful (ext: %s)", txn.ReferenceID, extID)
		} else {
			msg := bankDisbursementFailureMessage()
			if err := s.DB.UpdateTransactionStatus(txn.ID, database.StatusFailed, &msg); err != nil {
				log.Printf("simulator: error updating bank disbursement %s: %v", txn.ReferenceID, err)
				return
			}
			txn.Status = database.StatusFailed
			txn.Message = &msg
			log.Printf("simulator: bank disbursement %s -> Failed (%s)", txn.ReferenceID, msg)
			s.refundDisbursement(txn, BankDisbursementFeeRate)
		}

		s.sendCallback(txn)
	}()
}

// refundDisbursement refunds the amount + fee back to the wallet on failure.
func (s *Simulator) refundDisbursement(txn *database.Transaction, feeRate float64) {
	fee := math.Ceil(txn.Amount*feeRate*100) / 100
	totalRefund := txn.Amount + fee
	if err := s.DB.RefundWalletBalance(txn.WalletID, totalRefund); err != nil {
		log.Printf("simulator: error refunding wallet %s for disbursement %s: %v", txn.WalletID, txn.ReferenceID, err)
	} else {
		log.Printf("simulator: refunded %.2f to wallet %s for failed disbursement %s", totalRefund, txn.WalletID, txn.ReferenceID)
	}
}

func disbursementSuccessMessage(paymentType string) string {
	switch paymentType {
	case database.PayMtnMoney:
		return "MTN Money disbursement completed successfully"
	case database.PayAirtelMoney:
		return "Airtel Money disbursement completed successfully"
	case database.PayZamtelKwacha:
		return "Zamtel Kwacha disbursement completed successfully"
	default:
		return "disbursement completed successfully"
	}
}

var mtnDisbursementFailures = []string{
	"LOW_BALANCE_OR_PAYEE_LIMIT_REACHED_OR_NOT_ALLOWED",
	"Failed to send status check to Mtn httpStatusCode=404",
}

var airtelDisbursementFailures = []string{
	"Transaction is not found",
	"User not found",
}

var zamtelDisbursementFailures = []string{
	"System internal error.",
	"Recipient wallet not active",
}

var bankDisbursementFailures = []string{
	"Invalid account number",
	"Account name mismatch",
	"Bank temporarily unavailable",
	"SWIFT code not recognized",
}

func disbursementFailureMessage(paymentType string) string {
	switch paymentType {
	case database.PayMtnMoney:
		return mtnDisbursementFailures[rand.Intn(len(mtnDisbursementFailures))]
	case database.PayAirtelMoney:
		return airtelDisbursementFailures[rand.Intn(len(airtelDisbursementFailures))]
	case database.PayZamtelKwacha:
		return zamtelDisbursementFailures[rand.Intn(len(zamtelDisbursementFailures))]
	default:
		return "disbursement failed"
	}
}

func bankDisbursementFailureMessage() string {
	return bankDisbursementFailures[rand.Intn(len(bankDisbursementFailures))]
}
