package requestid

import (
	"context"
	"regexp"

	"github.com/google/uuid"
)

const Header = "X-Request-ID"

type contextKey struct{}

var validRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

func New() string {
	return uuid.NewString()
}

func Normalize(id string) string {
	if validRequestID.MatchString(id) {
		return id
	}
	return New()
}

func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

func FromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(contextKey{}).(string)
	return id, ok && id != ""
}
