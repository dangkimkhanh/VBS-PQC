package utils

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"regexp"
	"strings"
)

func randomFrom(charset string, length int) string {
	b := make([]byte, length)
	max := big.NewInt(int64(len(charset)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			panic(fmt.Errorf("secure random generation failed: %w", err))
		}
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

// GenerateRandomCode returns an unbiased random token for links and sessions.
func GenerateRandomCode(length int) string {
	return randomFrom("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", length)
}

// GenerateNumericCode returns a digits-only one-time code (OTP).
func GenerateNumericCode(length int) string {
	return randomFrom("0123456789", length)
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// EmailFilter matches an email case-insensitively so records created before
// emails were normalized are still found.
func EmailFilter(email string) interface{} {
	return map[string]interface{}{"$regex": "^" + regexp.QuoteMeta(NormalizeEmail(email)) + "$", "$options": "i"}
}
