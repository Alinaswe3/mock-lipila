package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"
)

// maxRequestBodySize is the maximum allowed request body size (1 MB).
const maxRequestBodySize = 1 << 20

var (
	phoneRegex     = regexp.MustCompile(`^260[0-9]{9}$`)
	swiftRegex     = regexp.MustCompile(`^[A-Z]{6}[A-Z0-9]{2}([A-Z0-9]{3})?$`)
	refIDAllowed   = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
	countryRegex   = regexp.MustCompile(`^[A-Z]{2}$`)
)

// ValidatePhoneNumber validates a Zambian phone number (260XXXXXXXXX) and returns the detected MNO.
func ValidatePhoneNumber(phone string) (string, error) {
	if !phoneRegex.MatchString(phone) {
		return "", fmt.Errorf("accountNumber must be in Zambian format: 260XXXXXXXXX (12 digits)")
	}
	provider := detectMNO(phone)
	if provider == "" {
		return "", fmt.Errorf("unrecognized mobile network for accountNumber prefix")
	}
	return provider, nil
}

// ValidateAmount checks that the amount is positive and within a reasonable range.
func ValidateAmount(amount float64) error {
	if amount <= 0 {
		return fmt.Errorf("amount must be greater than 0")
	}
	if amount > 999_999_999.99 {
		return fmt.Errorf("amount must not exceed 999999999.99")
	}
	return nil
}

// ValidateCurrency checks that the currency code is supported (ZMW or USD).
func ValidateCurrency(currency string) error {
	switch strings.ToUpper(currency) {
	case "ZMW", "USD":
		return nil
	default:
		return fmt.Errorf("currency must be ZMW or USD")
	}
}

// ValidateReferenceId checks that the reference ID is non-empty, reasonably sized,
// and contains only safe characters (alphanumeric, hyphens, underscores, dots).
func ValidateReferenceId(refID string) error {
	if refID == "" {
		return fmt.Errorf("referenceId is required")
	}
	if utf8.RuneCountInString(refID) > 100 {
		return fmt.Errorf("referenceId must not exceed 100 characters")
	}
	if !refIDAllowed.MatchString(refID) {
		return fmt.Errorf("referenceId contains invalid characters (only alphanumeric, hyphens, underscores, and dots allowed)")
	}
	return nil
}

// ValidateSwiftCode checks that a SWIFT/BIC code matches the expected format.
func ValidateSwiftCode(code string) error {
	if !swiftRegex.MatchString(code) {
		return fmt.Errorf("swiftCode must be 8 or 11 characters (e.g. ABORZMLU or ABORZMLUXXX)")
	}
	return nil
}

// ValidateCountryCode checks that a country code is a valid 2-letter ISO code.
func ValidateCountryCode(code string) error {
	if !countryRegex.MatchString(code) {
		return fmt.Errorf("country must be a 2-letter country code (e.g. ZM, US)")
	}
	return nil
}

// checkDuplicateRef checks if a referenceId already exists and writes an error response if so.
// Returns true if the referenceId is available (safe to proceed), false if a response was written.
func (h *Handlers) checkDuplicateRef(w http.ResponseWriter, refID string) bool {
	_, err := h.DB.GetTransactionByReferenceID(refID)
	if err == nil {
		writeValidationError(w, "duplicate referenceId: a transaction with this referenceId already exists")
		return false
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "SERVER_ERROR",
			"message": "failed to validate referenceId",
		})
		return false
	}
	return true
}

// writeValidationError writes a 400 BAD_REQUEST response with the given message.
func writeValidationError(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusBadRequest, map[string]string{
		"error":   "BAD_REQUEST",
		"message": msg,
	})
}

// LimitRequestBody wraps an http.Handler to enforce a maximum request body size.
func LimitRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)
		}
		next.ServeHTTP(w, r)
	})
}

// isMaxBytesError checks if an error is caused by exceeding the request body size limit.
func isMaxBytesError(err error) bool {
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		return true
	}
	return err != nil && strings.Contains(err.Error(), "http: request body too large")
}

// decodeJSONBody reads and decodes the JSON request body. Returns true on success.
// On failure, writes the appropriate error response and returns false.
func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		if isMaxBytesError(err) {
			writeJSON(w, http.StatusRequestEntityTooLarge, map[string]string{
				"error":   "PAYLOAD_TOO_LARGE",
				"message": fmt.Sprintf("request body must not exceed %d bytes", maxRequestBodySize),
			})
		} else if errors.Is(err, io.EOF) {
			writeValidationError(w, "request body is empty")
		} else {
			writeValidationError(w, "invalid JSON body")
		}
		return false
	}
	return true
}

// collectMissing checks a list of field name/value pairs and returns the names of empty ones.
func collectMissing(fields map[string]string) []string {
	var missing []string
	for name, val := range fields {
		if val == "" {
			missing = append(missing, name)
		}
	}
	return missing
}
