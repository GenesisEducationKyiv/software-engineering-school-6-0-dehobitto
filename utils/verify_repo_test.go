package utils

import "testing"

func TestIsValidRepo(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"golang/go", true},
		{"owner/repo", true},
		{"my-org/my-repo", true},
		{"user.name/repo.name", true},
		{"", false},
		{"noslash", false},
		{"/repo", false},
		{"owner/", false},
		{"a/b/c", false},
		{"owner//repo", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := IsValidRepo(tt.input)
			if got != tt.want {
				t.Errorf("IsValidRepo(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
