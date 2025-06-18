package notifications

import (
	"regexp"
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
