package helpers

import (
	"testing"
	"time"
)

func TestNowUTC_IsUTC(t *testing.T) {
	now := NowUTC()
	if now.Location() != time.UTC {
		t.Errorf("expected UTC, got %v", now.Location())
	}
	if time.Since(now) > time.Second {
		t.Error("NowUTC should return current time")
	}
}

func TestTodayUTC_MidnightUTC(t *testing.T) {
	today := TodayUTC()
	if today.Location() != time.UTC {
		t.Errorf("expected UTC, got %v", today.Location())
	}
	if today.Hour() != 0 || today.Minute() != 0 || today.Second() != 0 || today.Nanosecond() != 0 {
		t.Errorf("expected midnight, got %v", today)
	}
	now := time.Now().UTC()
	if today.Year() != now.Year() || today.Month() != now.Month() || today.Day() != now.Day() {
		t.Errorf("date mismatch: today=%v now=%v", today, now)
	}
}

func TestParseUTC(t *testing.T) {
	const layout = "2006-01-02"

	t.Run("valid date", func(t *testing.T) {
		parsed, err := ParseUTC(layout, "2024-03-15")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if parsed.Location() != time.UTC {
			t.Errorf("expected UTC, got %v", parsed.Location())
		}
		if parsed.Year() != 2024 || parsed.Month() != 3 || parsed.Day() != 15 {
			t.Errorf("wrong date: %v", parsed)
		}
	})

	t.Run("invalid date", func(t *testing.T) {
		if _, err := ParseUTC(layout, "not-a-date"); err == nil {
			t.Error("expected error for invalid date")
		}
	})
}
