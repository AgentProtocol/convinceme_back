package server

import (
	"regexp"
)

// isStrongPassword checks if a password meets the strength requirements
func isStrongPassword(password string) bool {
	// Check for minimum length (already handled by binding)
	if len(password) < 8 {
		return false
	}

	// Check for at least one uppercase letter
	uppercase := regexp.MustCompile(`[A-Z]`)
	if !uppercase.MatchString(password) {
		return false
	}

	// Check for at least one lowercase letter
	lowercase := regexp.MustCompile(`[a-z]`)
	if !lowercase.MatchString(password) {
		return false
	}

	// Check for at least one digit
	digit := regexp.MustCompile(`[0-9]`)
	if !digit.MatchString(password) {
		return false
	}

	// Check for at least one special character
	special := regexp.MustCompile(`[^a-zA-Z0-9]`)
	if !special.MatchString(password) {
		return false
	}

	return true
}
