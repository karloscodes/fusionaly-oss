package events

import "strings"

// IsSelfReferral reports whether a referrer hostname is the site's own host:
// the same host, with or without "www.". A parent domain or another
// subdomain is a different site (see prepareTempEvent for subdomain
// tracking).
func IsSelfReferral(hostname, websiteDomain string) bool {
	if hostname == "" || websiteDomain == "" {
		return false
	}
	return withoutWWW(hostname) == withoutWWW(websiteDomain)
}

func withoutWWW(host string) string {
	return strings.TrimPrefix(strings.ToLower(host), "www.")
}
