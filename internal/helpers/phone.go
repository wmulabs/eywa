package helpers

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nyaruka/phonenumbers"
)

var ErrInvalidPhone = errors.New("invalid phone number")

// defaultRegion is the ISO 3166-1 alpha-2 country code used when the number
// has no country code prefix (e.g. "MX", "BR", "US", "GB", "DE").
// Pass "" to attempt parsing without a default region.
func NormalizePhone(number, defaultRegion string) (string, error) {
	number = strings.TrimSpace(number)
	if number == "" {
		return "", ErrInvalidPhone
	}

	region := strings.ToUpper(defaultRegion)
	parsed, err := phonenumbers.Parse(number, region)
	if err != nil {
		return "", fmt.Errorf("%w: %s", ErrInvalidPhone, err.Error())
	}
	if !phonenumbers.IsValidNumber(parsed) {
		return "", ErrInvalidPhone
	}
	return phonenumbers.Format(parsed, phonenumbers.E164), nil
}
