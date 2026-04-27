package utils

import "testing"

func TestIsValidEmail(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"user@example.com", true},
		{"test.name+tag@domain.org", true},
		{"a@b.co", true},
		{"", false},
		{"plaintext", false},
		{"@domain.com", false},
		{"user@", false},
		{"user@.com", false},
		{"user@domain", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsValidEmail(tt.input)
			if got != tt.want {
				t.Errorf("IsValidEmail(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
