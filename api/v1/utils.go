package v1

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/netip"
	"strings"

	"github.com/gofiber/fiber/v2"
	"log/slog"
)

func getClientIP(c *fiber.Ctx) string {
	// Try standard headers first
	if ip := selectPreferredIP(strings.Split(c.Get("X-Forwarded-For"), ",")); ip != "" {
		return ip
	}

	// Other reverse-proxy headers
	for _, header := range []string{
		"X-Real-IP",
		"CF-Connecting-IP",
		"True-Client-IP",
		"X-Client-IP",
	} {
		if value := c.Get(header); value != "" {
			if ip := selectPreferredIP([]string{value}); ip != "" {
				return ip
			}
		}
	}

	if forwarded := c.Get("Forwarded"); forwarded != "" {
		if ip := selectPreferredIP(parseForwardedHeader(forwarded)); ip != "" {
			return ip
		}
	}

	// Try the remote address from the request directly
	remoteAddr := c.Context().RemoteAddr().String()
	if remoteAddr != "" {
		// Extract IP from IP:port format
		host, _, err := net.SplitHostPort(remoteAddr)
		if err == nil && host != "" {
			parsedIP := net.ParseIP(host)
			if parsedIP != nil && !isPrivateIP(parsedIP) {
				return host
			}
		} else {
			// Try to use the address directly if SplitHostPort fails
			parsedIP := net.ParseIP(remoteAddr)
			if parsedIP != nil && !isPrivateIP(parsedIP) {
				return remoteAddr
			}
		}
	}

	// Finally, use Fiber's built-in method
	ip := c.IP()
	if ip != "" && ip != "0.0.0.0" && ip != "::" {
		parsedIP := net.ParseIP(strings.TrimSpace(ip))
		if parsedIP != nil && !isPrivateIP(parsedIP) {
			return ip
		}
	}

	// If we get here, we couldn't determine a public IP
	// In a production environment, you might want to log this occurrence
	slog.Default().Info("Fallback to loopback IP for request",
		slog.String("path", c.Path()),
		slog.Any("headers", c.GetReqHeaders()))
	return "127.0.0.1"
}

// Helper function to check if an IP is private
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Check if IP is private according to RFC 1918, RFC 4193, RFC 4291
	privateIPBlocks := []*net.IPNet{
		parseCIDR("10.0.0.0/8"),     // RFC 1918
		parseCIDR("172.16.0.0/12"),  // RFC 1918
		parseCIDR("192.168.0.0/16"), // RFC 1918
		parseCIDR("fc00::/7"),       // RFC 4193 Unique Local Addresses
		parseCIDR("fe80::/10"),      // RFC 4291 Link-Local
		parseCIDR("::1/128"),        // Loopback
		parseCIDR("127.0.0.0/8"),    // Loopback
	}

	for _, block := range privateIPBlocks {
		candidate := ip

		switch len(block.IP) {
		case net.IPv4len:
			if ip4 := ip.To4(); ip4 != nil {
				candidate = ip4
			} else {
				continue
			}
		case net.IPv6len:
			candidate = ip.To16()
			if candidate == nil {
				continue
			}
		}

		if block.Contains(candidate) {
			return true
		}
	}
	return false
}

// Helper function to safely parse CIDR notation
func parseCIDR(s string) *net.IPNet {
	_, block, _ := net.ParseCIDR(s)
	return block
}

func selectPreferredIP(values []string) string {
	var ipv6Fallback string

	for _, raw := range values {
		clean, parsed := normalizeIP(raw)
		if parsed == nil || isPrivateIP(parsed) {
			continue
		}

		if parsed.To4() != nil {
			return clean
		}

		if ipv6Fallback == "" {
			ipv6Fallback = clean
		}
	}

	return ipv6Fallback
}

func normalizeIP(raw string) (string, net.IP) {
	clean := strings.TrimSpace(raw)
	clean = strings.Trim(clean, "\"")
	if clean == "" {
		return "", nil
	}

	// Remove zone identifier if present (e.g. fe80::1%eth0)
	if percent := strings.Index(clean, "%"); percent != -1 {
		clean = clean[:percent]
	}

	// Try parsing addr:port (handles both IPv4:port and [IPv6]:port)
	if addrPort, err := netip.ParseAddrPort(clean); err == nil {
		addr := addrPort.Addr()
		if addr.Is4In6() {
			addr = addr.Unmap()
		}
		ipStr := addr.String()
		return ipStr, net.ParseIP(ipStr)
	}

	trimmed := clean
	if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
		trimmed = strings.TrimPrefix(trimmed, "[")
		trimmed = strings.TrimSuffix(trimmed, "]")
	}

	if addr, err := netip.ParseAddr(trimmed); err == nil {
		if addr.Is4In6() {
			addr = addr.Unmap()
		}
		ipStr := addr.String()
		return ipStr, net.ParseIP(ipStr)
	}

	if host, _, err := net.SplitHostPort(clean); err == nil {
		return normalizeIP(host)
	}

	return "", nil
}

func parseForwardedHeader(header string) []string {
	var candidates []string

	entries := strings.Split(header, ",")
	for _, entry := range entries {
		parts := strings.Split(entry, ";")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(strings.ToLower(part), "for=") {
				ip := strings.TrimPrefix(part, "for=")
				candidates = append(candidates, ip)
			}
		}
	}

	return candidates
}

// generateETag creates a strong ETag from content using SHA-256
func generateETag(content []byte) string {
	hash := sha256.Sum256(content)
	return `"` + hex.EncodeToString(hash[:]) + `"` // Quoted for strong ETag
}
