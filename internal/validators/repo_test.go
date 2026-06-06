package validators

import "testing"

func TestIsValidRepo(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"simple repo", "golang/go", true},
		{"generic owner repo", "owner/repo", true},
		{"hyphenated names", "my-org/my-repo", true},
		{"dotted names", "user.name/repo.name", true},
		{"underscores", "my_org/my_repo", true},
		{"uppercase", "Owner/Repo", true},
		{"empty", "", false},
		{"missing slash", "noslash", false},
		{"missing owner", "/repo", false},
		{"missing repo", "owner/", false},
		{"too many parts", "a/b/c", false},
		{"empty path part", "owner//repo", false},
		{"space in owner", "my org/repo", false},
		{"space in repo", "owner/my repo", false},
		{"newline suffix", "owner/repo\n", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValidRepo(tt.input)
			if got != tt.want {
				t.Errorf("IsValidRepo(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
