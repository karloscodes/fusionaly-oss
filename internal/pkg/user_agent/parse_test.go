package user_agent

import "testing"

func TestParseIsDeterministic(t *testing.T) {
	t.Run("gives the same device every time when two brands match", func(t *testing.T) {
		// Expected values come from Matomo's fixtures. parseUserAgent skips
		// the cache, so every call checks the brands again.
		ua := "com.google.android.youtube/5.3.32(Linux; U; Android 4.0.3; ro_RO; GOCLEVER NETBOOK R103 Build/IML74K) gzip"
		first := parseUserAgent(ua)

		for range 50 {
			again := parseUserAgent(ua)

			if again != first {
				t.Fatalf("got %+v, then %+v", first, again)
			}
		}
		if first.Device != "GOCLEVER" || !first.Desktop {
			t.Errorf("got %s desktop=%v, want GOCLEVER desktop", first.Device, first.Desktop)
		}
	})
}

func TestParseUserAgentCache(t *testing.T) {
	t.Run("returns the same result as a full parse", func(t *testing.T) {
		ua := "Mozilla/5.0 (iPad; CPU OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1"

		cached := ParseUserAgent(ua)
		again := ParseUserAgent(ua)

		if cached != parseUserAgent(ua) || again != cached {
			t.Errorf("cached %+v differs from a full parse %+v", cached, parseUserAgent(ua))
		}
	})
}
