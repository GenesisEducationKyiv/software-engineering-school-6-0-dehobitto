package logger

import "testing"

func TestEmailHashIsStableAndCaseInsensitive(t *testing.T) {
	got := EmailHash(" User@Example.com ")
	want := EmailHash("user@example.com")

	if got == "" {
		t.Fatal("EmailHash returned empty hash")
	}
	if got != want {
		t.Fatalf("EmailHash = %q, want %q", got, want)
	}
	if got == "user@example.com" {
		t.Fatal("EmailHash must not return raw email")
	}
}

func TestIPHash(t *testing.T) {
	got := IPHash("127.0.0.1")

	if got == "" {
		t.Fatal("IPHash returned empty hash")
	}
	if got == "127.0.0.1" {
		t.Fatal("IPHash must not return raw IP")
	}
}

func TestIPHashInvalidIP(t *testing.T) {
	if got := IPHash("not-an-ip"); got != "" {
		t.Fatalf("IPHash = %q, want empty", got)
	}
}
