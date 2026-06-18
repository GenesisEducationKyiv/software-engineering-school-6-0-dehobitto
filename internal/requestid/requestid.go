package requestid

import (
	"context"
	"regexp"

	"github.com/google/uuid"
)

const Header = "X-Request-ID"

type contextKey struct{}

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// New returns a new request identifier.
func New() string {
	return uuid.NewString()
}

// Normalize returns id when it is safe to propagate, otherwise a fresh id.
func Normalize(id string) string {
	if validRequestID.MatchString(id) {
		return id
	}
	return New()
}

// WithContext stores id in ctx.
func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext returns the request id stored in ctx.
func FromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(contextKey{}).(string)
	return id, ok && id != ""
}
