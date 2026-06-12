package hash

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

func EmailHash(email string) string {
	normalized := strings.ToLower(strings.TrimSpace(email))
	return TextHash(normalized)
}

func TextHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}
