package email

import "testing"

func TestSenderAddressUsesConfiguredEmail(t *testing.T) {
	got := senderAddress(Config{SMTPEmail: "noreply@example.com"})
	if got != "noreply@example.com" {
		t.Fatalf("senderAddress() = %q, want configured email", got)
	}
}

func TestSenderAddressDefaultsForLocalSMTPWithoutAuth(t *testing.T) {
	got := senderAddress(Config{})
	if got != "subber@localhost" {
		t.Fatalf("senderAddress() = %q, want local default", got)
	}
}
