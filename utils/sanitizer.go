package utils

import "github.com/microcosm-cc/bluemonday"

// SanitizeInput cleans user input to prevent XSS attacks.
// For SQL injection, always use prepared statements with your database driver.
// This function is for preventing HTML/JS injection in rendered content.
func SanitizeInput(input string) string {
	p := bluemonday.UGCPolicy()
	return p.Sanitize(input)
}
