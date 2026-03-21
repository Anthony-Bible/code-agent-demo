package tool

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		private bool
	}{
		// Loopback
		{"loopback", "127.0.0.1", true},
		{"loopback-other", "127.255.255.255", true},

		// RFC 1918 private ranges
		{"class-a-private", "10.0.0.1", true},
		{"class-b-private", "172.16.0.1", true},
		{"class-c-private", "192.168.1.1", true},

		// Link-local
		{"link-local", "169.254.1.1", true},

		// Multicast
		{"multicast", "224.0.0.1", true},

		// 0.0.0.0/8 "this network" (RFC 1122) — the regression range
		{"unspecified-exact", "0.0.0.0", true},
		{"this-network-0.0.0.1", "0.0.0.1", true},
		{"this-network-0.1.0.0", "0.1.0.0", true},
		{"this-network-0.255.255.255", "0.255.255.255", true},

		// IPv6
		{"ipv6-loopback", "::1", true},
		{"ipv6-link-local", "fe80::1", true},
		{"ipv6-unique-local", "fd00::1", true},

		// Public addresses — should NOT be private
		{"public-1.1.1.1", "1.1.1.1", false},
		{"public-8.8.8.8", "8.8.8.8", false},
		{"public-93.184.216.34", "93.184.216.34", false},
		{"public-ipv6", "2607:f8b0:4004:800::200e", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			got := isPrivateIP(ip)
			if got != tt.private {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
			}
		})
	}
}
