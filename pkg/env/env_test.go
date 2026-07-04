package env

import (
	"reflect"
	"testing"
	"time"
)

func TestStringBoolIntCSVAndDuration(t *testing.T) {
	t.Setenv("ENV_STRING", "value")
	t.Setenv("ENV_BOOL", "true")
	t.Setenv("ENV_INT", "42")
	t.Setenv("ENV_CSV", "kafka:9092, kafka:9093,")
	t.Setenv("ENV_DURATION", "45s")
	t.Setenv("ENV_DURATION_LIST", "1m, 10m")

	if got := String("ENV_STRING", "fallback"); got != "value" {
		t.Fatalf("String() = %q, want value", got)
	}
	if got := Bool("ENV_BOOL", false); !got {
		t.Fatal("Bool() = false, want true")
	}
	if got := Int("ENV_INT", 1); got != 42 {
		t.Fatalf("Int() = %d, want 42", got)
	}
	if got := CSV("ENV_CSV", "fallback"); !reflect.DeepEqual(got, []string{"kafka:9092", "kafka:9093"}) {
		t.Fatalf("CSV() = %#v", got)
	}
	if got := Duration("ENV_DURATION", time.Second); got != 45*time.Second {
		t.Fatalf("Duration() = %s, want 45s", got)
	}
	if got := DurationList("ENV_DURATION_LIST", nil); !reflect.DeepEqual(got, []time.Duration{time.Minute, 10 * time.Minute}) {
		t.Fatalf("DurationList() = %#v", got)
	}
}

func TestInvalidValuesReturnFallbacks(t *testing.T) {
	t.Setenv("BAD_BOOL", "not-bool")
	t.Setenv("BAD_INT", "not-int")
	t.Setenv("BAD_DURATION", "not-duration")
	t.Setenv("BAD_DURATION_LIST", "bad, also-bad")

	if got := Bool("BAD_BOOL", true); !got {
		t.Fatal("Bool() should return fallback")
	}
	if got := Int("BAD_INT", 7); got != 7 {
		t.Fatalf("Int() = %d, want fallback 7", got)
	}
	if got := Duration("BAD_DURATION", 5*time.Second); got != 5*time.Second {
		t.Fatalf("Duration() = %s, want fallback 5s", got)
	}
	fallback := []time.Duration{time.Second}
	if got := DurationList("BAD_DURATION_LIST", fallback); !reflect.DeepEqual(got, fallback) {
		t.Fatalf("DurationList() = %#v, want fallback", got)
	}
}
