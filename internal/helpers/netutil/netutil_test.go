package netutil

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	cases := []struct {
		ip      string
		private bool
	}{
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},
		{"127.0.0.1", true},
		{"127.0.0.2", true},
		{"169.254.169.254", true},  // AWS/GCP IMDS link-local
		{"100.64.0.1", true},       // CGN (carrier-grade NAT)
		{"100.127.255.255", true},  // CGN upper bound
		{"::1", true},              // IPv6 loopback
		{"fc00::1", true},          // IPv6 ULA
		{"fd00::1", true},          // IPv6 ULA (fd prefix)
		{"fe80::1", true},          // IPv6 link-local
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"2001:4860:4860::8888", false}, // Google DNS IPv6
		{"172.15.255.255", false},       // just below 172.16/12
		{"172.32.0.0", false},           // just above 172.31/12
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.ip, func(t *testing.T) {
			ip := net.ParseIP(tc.ip)
			if ip == nil {
				t.Fatalf("net.ParseIP returned nil for %q", tc.ip)
			}
			got := IsPrivateIP(ip)
			if got != tc.private {
				t.Errorf("IsPrivateIP(%s) = %v, want %v", tc.ip, got, tc.private)
			}
		})
	}
}

// TestValidateURL_BlockedSchemes verifies that non-http(s) schemes are rejected
// without any network I/O.
func TestValidateURL_BlockedSchemes(t *testing.T) {
	cases := []string{
		"file:///etc/passwd",
		"ftp://internal.host/",
		"gopher://host/",
	}
	for _, raw := range cases {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			if err := ValidateURL(raw); err == nil {
				t.Errorf("ValidateURL(%q) returned nil, want error", raw)
			}
		})
	}
}

// TestValidateURL_PrivateIPLiterals checks that IP-literal URLs for private
// ranges are rejected. net.LookupHost on an IP literal returns that same IP
// without an actual DNS query, so these tests work in sandboxed environments.
func TestValidateURL_PrivateIPLiterals(t *testing.T) {
	cases := []string{
		"http://127.0.0.1/",              // loopback IPv4
		"http://127.0.0.2/",              // loopback IPv4 (non-zero host)
		"http://10.0.0.1/",               // RFC 1918 class A
		"http://192.168.1.1/",            // RFC 1918 class C
		"http://172.16.0.1/",             // RFC 1918 class B lower
		"http://172.31.255.255/",         // RFC 1918 class B upper
		"http://169.254.169.254/",        // AWS/GCP IMDS link-local
		"http://100.64.0.1/",             // CGN (RFC 6598)
		"http://[::1]/",                  // loopback IPv6
		"http://[fc00::1]/",              // IPv6 ULA
		"http://[fd00::1]/",              // IPv6 ULA (fd prefix)
		"http://[fe80::1]/",              // IPv6 link-local
	}
	for _, raw := range cases {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			if err := ValidateURL(raw); err == nil {
				t.Errorf("ValidateURL(%q) returned nil, want error for private address", raw)
			}
		})
	}
}

// TestValidateURL_MalformedURLs checks that obviously invalid URLs are handled.
func TestValidateURL_MalformedURLs(t *testing.T) {
	cases := []string{
		"http://",           // no host
		"not-a-url",        // no scheme
		"://missing-scheme", // malformed
	}
	for _, raw := range cases {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			if err := ValidateURL(raw); err == nil {
				t.Errorf("ValidateURL(%q) returned nil, want error", raw)
			}
		})
	}
}

// TestValidateURL_HostnameLoopback ensures that the well-known loopback
// hostname resolves to a blocked address. This relies on the system resolver
// returning 127.0.0.1 for "localhost", which is true on virtually all
// platforms including CI runners.
func TestValidateURL_HostnameLoopback(t *testing.T) {
	// If localhost does not resolve (extremely unusual), the function returns
	// nil (unresolvable hosts are allowed through). Skip rather than fail.
	addrs, err := net.LookupHost("localhost")
	if err != nil || len(addrs) == 0 {
		t.Skip("localhost does not resolve on this system")
	}
	if err := ValidateURL("http://localhost/"); err == nil {
		t.Errorf("ValidateURL(\"http://localhost/\") returned nil, want error")
	}
}
