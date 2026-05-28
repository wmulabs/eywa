package helpers

import (
	"fmt"
	"time"
)

// NowUTC ensures all timestamps across the system are UTC — never use time.Now() directly.
func NowUTC() time.Time {
	return time.Now().UTC()
}

func TodayUTC() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func ParseUTC(layout, value string) (time.Time, error) {
	t, err := time.Parse(layout, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse time %q: %w", value, err)
	}
	return t.UTC(), nil
}
