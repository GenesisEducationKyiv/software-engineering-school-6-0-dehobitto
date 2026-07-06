package requestid

import (
	"context"
	"testing"
)

func TestNormalizeKeepsValidRequestID(t *testing.T) {
	id := "req-123:abc.def"
	if got := Normalize(id); got != id {
		t.Fatalf("Normalize() = %q, want %q", got, id)
	}
}

func TestNormalizeReplacesInvalidRequestID(t *testing.T) {
	for _, raw := range []string{"", "bad header", "bad/slash", string(make([]byte, 129))} {
		got := Normalize(raw)
		if got == raw {
			t.Fatalf("Normalize(%q) returned original invalid value", raw)
		}
		if got == "" {
			t.Fatalf("Normalize(%q) returned empty value", raw)
		}
	}
}

func TestContextRoundTrip(t *testing.T) {
	ctx := WithContext(context.Background(), "req-1")
	got, ok := FromContext(ctx)
	if !ok || got != "req-1" {
		t.Fatalf("FromContext() = (%q, %v), want (req-1, true)", got, ok)
	}

	got, ok = FromContext(context.Background())
	if ok || got != "" {
		t.Fatalf("FromContext(empty) = (%q, %v), want empty false", got, ok)
	}
}
