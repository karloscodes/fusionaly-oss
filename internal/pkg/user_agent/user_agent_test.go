package user_agent_test

import (
	"testing"

	"fusionaly/internal/pkg/user_agent"
)

func TestParseUserAgent(t *testing.T) {
	testCases := []struct {
		name            string
		userAgent       string
		expectedBrowser string
		expectedOS      string
		expectedMobile  bool
		expectedTablet  bool
		expectedDesktop bool
	}{
		{
			name:            "Chrome on Windows",
			userAgent:       "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			expectedBrowser: "Chrome",
			expectedOS:      "Windows",
			expectedMobile:  false,
			expectedTablet:  false,
			expectedDesktop: true,
		},
		{
			name:            "Safari on iPhone",
			userAgent:       "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
			expectedBrowser: "Mobile Safari",
			expectedOS:      "iOS",
			expectedMobile:  true,
			expectedTablet:  false,
			expectedDesktop: false,
		},
		{
			name:            "Chrome on Android",
			userAgent:       "Mozilla/5.0 (Linux; Android 11; SM-G998B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.120 Mobile Safari/537.36",
			expectedBrowser: "Chrome Mobile",
			expectedOS:      "Android",
			expectedMobile:  true,
			expectedTablet:  false,
			expectedDesktop: false,
		},
		{
			name:            "Safari on iPad",
			userAgent:       "Mozilla/5.0 (iPad; CPU OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
			expectedBrowser: "Mobile Safari",
			expectedOS:      "iPadOS",
			expectedMobile:  false,
			expectedTablet:  true,
			expectedDesktop: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := user_agent.ParseUserAgent(tc.userAgent)

			t.Logf("Input: %s", tc.userAgent)
			t.Logf("Parsed - Browser: %s, OS: %s, Device: %s, Mobile: %v, Tablet: %v, Desktop: %v",
				result.Browser, result.OS, result.Device, result.Mobile, result.Tablet, result.Desktop)

			if result.Browser != tc.expectedBrowser {
				t.Errorf("Expected browser %s, got %s", tc.expectedBrowser, result.Browser)
			}
			if result.OS != tc.expectedOS {
				t.Errorf("Expected OS %s, got %s", tc.expectedOS, result.OS)
			}
			if result.Mobile != tc.expectedMobile {
				t.Errorf("Expected mobile %v, got %v", tc.expectedMobile, result.Mobile)
			}
			if result.Tablet != tc.expectedTablet {
				t.Errorf("Expected tablet %v, got %v", tc.expectedTablet, result.Tablet)
			}
			if result.Desktop != tc.expectedDesktop {
				t.Errorf("Expected desktop %v, got %v", tc.expectedDesktop, result.Desktop)
			}
		})
	}
}
