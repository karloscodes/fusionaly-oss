package clientip

import "testing"

func TestResolve(t *testing.T) {
	const kamal = "172.18.0.2:41234" // kamal-proxy in the Docker network

	cases := []struct {
		name, remoteAddr, forwardedFor, want string
	}{
		{"behind kamal-proxy: the address it appended", kamal, "203.0.113.7", "203.0.113.7"},
		{"a spoofed entry on the left is ignored", kamal, "1.2.3.4, 203.0.113.7", "203.0.113.7"},
		{"behind Cloudflare: skip its edge", kamal, "1.2.3.4, 203.0.113.7, 162.158.1.1", "203.0.113.7"},
		{"a direct request cannot claim an address", "198.51.100.9:5000", "203.0.113.7", "198.51.100.9"},
		{"IPv6 client", kamal, "2001:db8::1", "2001:db8::1"},
		{"IPv6 behind Cloudflare", kamal, "2001:db8::1, 2606:4700::6810:1", "2001:db8::1"},
		{"entries with ports and quotes", kamal, `"203.0.113.7:443"`, "203.0.113.7"},
		{"IPv4-mapped IPv6 peer", "[::ffff:172.18.0.2]:80", "203.0.113.7", "203.0.113.7"},
		{"garbage entries are skipped", kamal, "not-an-ip, 203.0.113.7, unknown", "203.0.113.7"},
		{"development: no header", "127.0.0.1:5555", "", "127.0.0.1"},
		{"only proxies in the header: fall back to the peer", kamal, "10.0.0.5", "172.18.0.2"},
		{"unreadable peer", "", "203.0.113.7", "127.0.0.1"},
		{"Fiber test connection acts as the local proxy", "0.0.0.0:0", "203.0.113.7", "203.0.113.7"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolve(tc.remoteAddr, tc.forwardedFor)

			if got != tc.want {
				t.Errorf("resolve(%q, %q) = %q, want %q", tc.remoteAddr, tc.forwardedFor, got, tc.want)
			}
		})
	}
}
