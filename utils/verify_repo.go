package utils

import "regexp"

var regRepo = regexp.MustCompile(`^[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+$`)

// IsValidRepo checks whether the given string matches the owner/repo format.
func IsValidRepo(repo string) bool {
	return regRepo.MatchString(repo)
}
