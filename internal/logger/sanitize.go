package logger

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"

	"subber/internal/requestid"
)

const hashPrefixLength = 16

// EmailHash returns a stable irreversible hash suitable for log correlation.
func EmailHash(email string) string {
	return hashString(strings.ToLower(strings.TrimSpace(email)))
}

// IPHash returns a stable irreversible hash of a client IP address.
func IPHash(ip string) string {
	parsed := net.ParseIP(strings.TrimSpace(ip))
	if parsed == nil {
		return ""
	}
	return hashString(parsed.String())
}

func hashString(value string) string {
	if value == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:hashPrefixLength]
}

// WithEmailHash attaches a safe email correlation field without logging PII.
func WithEmailHash(log Logger, email string) Logger {
	if log == nil {
		log = NewNoop()
	}
	if hash := EmailHash(email); hash != "" {
		return log.WithField("email_hash", hash)
	}
	return log
}

// WithRequestID attaches the request_id from context when present.
func WithRequestID(log Logger, ctx context.Context) Logger {
	if log == nil {
		log = NewNoop()
	}
	if id, ok := requestid.FromContext(ctx); ok {
		return log.WithField("request_id", id)
	}
	return log
}
