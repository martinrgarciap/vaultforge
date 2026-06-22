package sessioncookie

import (
	"net/http"
	"testing"
)

func TestConfigUsesFixedCookieContract(t *testing.T) {
	t.Parallel()

	config := NewConfig(false)

	if config.RefreshCookieName() != "vaultforge_refresh" {
		t.Errorf("refresh cookie name = %q", config.RefreshCookieName())
	}

	if config.CSRFCookieName() != "vaultforge_csrf" {
		t.Errorf("CSRF cookie name = %q", config.CSRFCookieName())
	}

	if config.CSRFHeaderName() != "X-CSRF-Token" {
		t.Errorf("CSRF header name = %q", config.CSRFHeaderName())
	}

	if config.RefreshCookiePath() != "/v1/auth/refresh" {
		t.Errorf("refresh cookie path = %q", config.RefreshCookiePath())
	}

	if config.CSRFCookiePath() != "/" {
		t.Errorf("CSRF cookie path = %q", config.CSRFCookiePath())
	}

	if config.SameSite() != http.SameSiteStrictMode {
		t.Errorf("SameSite mode = %v, want Strict", config.SameSite())
	}
}

func TestConfigPreservesSecureSetting(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name   string
		secure bool
	}{
		{name: "local HTTP", secure: false},
		{name: "production HTTPS", secure: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			config := NewConfig(testCase.secure)

			if config.Secure() != testCase.secure {
				t.Errorf(
					"Secure() = %t, want %t",
					config.Secure(),
					testCase.secure,
				)
			}
		})
	}
}
