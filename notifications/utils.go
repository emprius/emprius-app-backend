package notifications

import (
	"regexp"
	"strings"
)

const (
	// EmailRegexTemplate is the regular expression used to validate email addresses.
	EmailRegexTemplate = `^[\w.\+\.\-]+@([\w\-]+\.)+[\w]{2,}$`
)

var emailRegex = regexp.MustCompile(EmailRegexTemplate)

// ValidEmail helper function allows to validate an email address.
func ValidEmail(email string) bool {
	return emailRegex.MatchString(email)
}

// Return a string with n stars (⭐️) for ratings, where n is between 1 and 5.
// Used by the notifications to show ratings in the emails.
func Stars(score int32) string {
	if score < 1 || score > 100 {
		return "Invalid input: score must be between 1 and 100"
	}

	// Convert score to 1-5 star scale
	stars := (score-1)/20 + 1

	filled := strings.Repeat("⭐️", int(stars))
	empty := strings.Repeat("☆", 5-int(stars))
	return filled + empty
}
