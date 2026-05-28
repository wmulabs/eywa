package helpers

import (
	"testing"
)

func TestNormalizePhone(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		defaultRegion string
		want          string
		wantErr       bool
	}{
		// Full E.164 — no default region needed
		{name: "MX E164", input: "+525512345678", defaultRegion: "", want: "+525512345678"},
		{name: "BR E164", input: "+5511987654321", defaultRegion: "", want: "+5511987654321"},
		{name: "US E164", input: "+12125551234", defaultRegion: "", want: "+12125551234"},
		{name: "GB E164", input: "+447911123456", defaultRegion: "", want: "+447911123456"},
		{name: "DE E164", input: "+4915112345678", defaultRegion: "", want: "+4915112345678"},
		{name: "CO E164", input: "+573001234567", defaultRegion: "", want: "+573001234567"},
		{name: "CL E164", input: "+56987654321", defaultRegion: "", want: "+56987654321"},
		{name: "PE E164", input: "+51987654321", defaultRegion: "", want: "+51987654321"},

		// Local format with default region
		{name: "MX local", input: "5512345678", defaultRegion: "MX", want: "+525512345678"},
		{name: "BR local", input: "11987654321", defaultRegion: "BR", want: "+5511987654321"},
		{name: "US local", input: "2125551234", defaultRegion: "US", want: "+12125551234"},
		{name: "GB local", input: "07911123456", defaultRegion: "GB", want: "+447911123456"},

		// Invalid
		{name: "empty", input: "", defaultRegion: "", wantErr: true},
		{name: "garbage", input: "notaphone", defaultRegion: "", wantErr: true},
		{name: "plus only", input: "+", defaultRegion: "", wantErr: true},
		// Parses but fails IsValidNumber
		{name: "invalid US number", input: "+10000000000", defaultRegion: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizePhone(tc.input, tc.defaultRegion)
			if tc.wantErr {
				if err == nil {
					t.Errorf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
