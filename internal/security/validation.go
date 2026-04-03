package security

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

var (
	ErrHoneypotTriggered = errors.New("spam detected")
	ErrFormTooFast       = errors.New("form submitted too quickly")
	ErrInvalidEmail      = errors.New("invalid email address")
	ErrNameTooLong       = errors.New("name exceeds maximum length")
	ErrNameTooShort      = errors.New("name is required")
	ErrMessageTooLong    = errors.New("message exceeds maximum length")
	ErrMessageTooShort   = errors.New("message is too short")
	ErrInvalidInput      = errors.New("invalid input detected")
)

const (
	MaxNameLength    = 100
	MaxEmailLength   = 254
	MaxMessageLength = 5000
	MinMessageLength = 10
)

// Email regex pattern (RFC 5322 simplified)
var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// Validator handles input validation and sanitization
type Validator struct {
	minFormTimeSeconds int
}

// NewValidator creates a new validator
func NewValidator(minFormTimeSeconds int) *Validator {
	return &Validator{
		minFormTimeSeconds: minFormTimeSeconds,
	}
}

// ContactRequest represents an incoming contact form request
type ContactRequest struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Message   string `json:"message"`
	Website   string `json:"website"`   // Honeypot field
	Timestamp int64  `json:"timestamp"` // Form load timestamp (Unix seconds)
}

// ValidatedContact represents a validated and sanitized contact form
type ValidatedContact struct {
	Name    string
	Email   string
	Message string
}

// Validate checks the contact request for spam and validates all fields
func (v *Validator) Validate(req ContactRequest) (*ValidatedContact, error) {
	// Check honeypot field - should be empty
	if req.Website != "" {
		return nil, ErrHoneypotTriggered
	}

	// Check timestamp - form should be open for at least N seconds
	if req.Timestamp > 0 {
		formLoadTime := time.Unix(req.Timestamp, 0)
		if time.Since(formLoadTime) < time.Duration(v.minFormTimeSeconds)*time.Second {
			return nil, ErrFormTooFast
		}
	}

	// Sanitize inputs
	name := sanitizeInput(req.Name)
	email := sanitizeInput(req.Email)
	message := sanitizeInput(req.Message)

	// Validate name
	if len(name) == 0 {
		return nil, ErrNameTooShort
	}
	if len(name) > MaxNameLength {
		return nil, ErrNameTooLong
	}

	// Validate email
	if len(email) > MaxEmailLength {
		return nil, ErrInvalidEmail
	}
	if !emailRegex.MatchString(email) {
		return nil, ErrInvalidEmail
	}

	// Validate message
	if len(message) < MinMessageLength {
		return nil, ErrMessageTooShort
	}
	if len(message) > MaxMessageLength {
		return nil, ErrMessageTooLong
	}

	// Check for email header injection attempts
	if containsHeaderInjection(name) || containsHeaderInjection(email) || containsHeaderInjection(message) {
		return nil, ErrInvalidInput
	}

	return &ValidatedContact{
		Name:    name,
		Email:   email,
		Message: message,
	}, nil
}

// sanitizeInput removes leading/trailing whitespace and normalizes line endings
func sanitizeInput(input string) string {
	// Trim whitespace
	s := strings.TrimSpace(input)

	// Normalize line endings to \n
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")

	return s
}

// containsHeaderInjection checks for email header injection attempts
func containsHeaderInjection(input string) bool {
	lower := strings.ToLower(input)

	// Check for common header injection patterns
	injectionPatterns := []string{
		"bcc:",
		"cc:",
		"to:",
		"from:",
		"subject:",
		"content-type:",
		"mime-version:",
	}

	for _, pattern := range injectionPatterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}

	return false
}
