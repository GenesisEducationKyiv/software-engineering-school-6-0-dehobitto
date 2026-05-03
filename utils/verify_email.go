// Package utils provides common validation helpers.
package utils

import "regexp"

var regEmail = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

// IsValidEmail checks whether the given string is a valid email address.
func IsValidEmail(email string) bool {
	return regEmail.MatchString(email)
}
