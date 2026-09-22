// Package clientip finds the real client address behind kamal-proxy and,
// optionally, Cloudflare.
//
// Proxies append the address they received the request from to
// X-Forwarded-For. So the entries on the right were written by proxies we
// run, and everything to the left of the first address a trusted proxy did
// not write is whatever the client sent. The client address is the rightmost
// entry that is not a trusted proxy.
package clientip

import (
	"net"
	"net/netip"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// trustedProxies are the networks a forwarding proxy can sit in: private and
// loopback ranges (kamal-proxy in the Docker network) and Cloudflare's edge,
// from https://www.cloudflare.com/ips/. A visitor never comes from these.
var trustedProxies = mustPrefixes(
	"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8",
	"fc00::/7", "fe80::/10", "::1/128",
	// The unspecified address is never a real peer: the kernel reports a
	// concrete one. Fiber's in-memory test connections use it, so tests can
	// play the part of the local proxy.
	"0.0.0.0/32", "::/128",
	"173.245.48.0/20", "103.21.244.0/22", "103.22.200.0/22", "103.31.4.0/22",
	"141.101.64.0/18", "108.162.192.0/18", "190.93.240.0/20", "188.114.96.0/20",
	"197.234.240.0/22", "198.41.128.0/17", "162.158.0.0/15", "104.16.0.0/13",
	"104.24.0.0/14", "172.64.0.0/13", "131.0.72.0/22",
	"2400:cb00::/32", "2606:4700::/32", "2803:f800::/32", "2405:b500::/32",
	"2405:8100::/32", "2a06:98c0::/29", "2c0f:f248::/32",
)

// FromRequest returns the client address of the request.
func FromRequest(c *fiber.Ctx) string {
	return resolve(c.Context().RemoteAddr().String(), c.Get("X-Forwarded-For"))
}

// resolve picks the client address from the direct peer and the
// X-Forwarded-For header.
func resolve(remoteAddr, forwardedFor string) string {
	peer, ok := parse(remoteAddr)
	if !ok {
		return "127.0.0.1"
	}

	// Only a trusted proxy may tell us who the client is. A request straight
	// from the internet can put anything in X-Forwarded-For.
	if !trusted(peer) {
		return peer.String()
	}

	entries := strings.Split(forwardedFor, ",")
	for i := len(entries) - 1; i >= 0; i-- {
		addr, ok := parse(entries[i])
		if ok && !trusted(addr) {
			return addr.String()
		}
	}

	// No forwarded client (development, or a proxy that sets no header).
	return peer.String()
}

func trusted(addr netip.Addr) bool {
	for _, prefix := range trustedProxies {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

// parse reads an address with or without a port, IPv4-mapped IPv6 as IPv4.
func parse(raw string) (netip.Addr, bool) {
	s := strings.Trim(strings.TrimSpace(raw), `"`)
	if host, _, err := net.SplitHostPort(s); err == nil {
		s = host
	}
	s = strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
	addr, err := netip.ParseAddr(s)
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap().WithZone(""), true
}

func mustPrefixes(cidrs ...string) []netip.Prefix {
	prefixes := make([]netip.Prefix, len(cidrs))
	for i, cidr := range cidrs {
		prefixes[i] = netip.MustParsePrefix(cidr)
	}
	return prefixes
}
