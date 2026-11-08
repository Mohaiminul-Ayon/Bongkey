package orders

import (
	"regexp"
	"strings"
)

// mobilePattern is a Bangladesh mobile number in local form: 01, an operator digit 3-9, then 8 digits.
var mobilePattern = regexp.MustCompile(`^01[3-9][0-9]{8}$`)

// NormalizeMobile turns "+880 1712-345678" or "8801712345678" into the local 01XXXXXXXXX form; anything else is returned trimmed.
func NormalizeMobile(value string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, value)
	if strings.HasPrefix(digits, "880") && len(digits) == 13 {
		return digits[2:]
	}
	if len(digits) == 11 {
		return digits
	}
	return strings.TrimSpace(value)
}

// ValidMobile reports whether value is already a normalized Bangladesh mobile number.
func ValidMobile(value string) bool {
	return mobilePattern.MatchString(value)
}
