// Package netutil provides shared network validation utilities.
package netutil

import (
	"context"
	"fmt"
	"net"
	"net/url"
)

// ValidateURL checks that rawURL uses http or https, has a non-empty host,
// and does not resolve to a private, loopback, or link-local address.
//
// Note: DNS resolution is performed once here and again by the underlying
// http.Client. This creates a TOCTOU window where a DNS rebinding attack could
// change the resolved address between validation and the actual connection.
// Callers that require full protection against rebinding should use a custom
// net.Dialer that re-validates the IP at dial time.
func ValidateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("URL scheme not allowed: %s", parsed.Scheme)
	}
	hostname := parsed.Hostname()
	if hostname == "" {
		return fmt.Errorf("URL has no host")
	}
	// Use background context: SSRF validation must not be skipped due to a cancelled caller context.
	addrs, err := net.DefaultResolver.LookupHost(context.Background(), hostname)
	if err != nil {
		// Cannot resolve — let the http.Client handle the failure.
		return nil
	}
	for _, addr := range addrs {
		ip := net.ParseIP(addr)
		if ip == nil {
			continue
		}
		if IsPrivateIP(ip) {
			return fmt.Errorf("requests to private network addresses are not allowed")
		}
	}
	return nil
}

// IsPrivateIP reports whether ip is a loopback, link-local, or private address.
func IsPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	for _, r := range privateRanges {
		if r.Contains(ip) {
			return true
		}
	}
	return false
}

var privateRanges = []net.IPNet{
	parseCIDR("0.0.0.0/8"),
	parseCIDR("10.0.0.0/8"),
	parseCIDR("172.16.0.0/12"),
	parseCIDR("192.168.0.0/16"),
	parseCIDR("127.0.0.0/8"),
	parseCIDR("::1/128"),
	parseCIDR("169.254.0.0/16"),
	parseCIDR("100.64.0.0/10"),
	parseCIDR("fc00::/7"),
}

func parseCIDR(s string) net.IPNet {
	_, ipNet, err := net.ParseCIDR(s)
	if err != nil {
		panic("invalid CIDR in privateRanges: " + s)
	}
	return *ipNet
}
