package api

import "testing"

func TestLoadConfigCreatesSessionCookieConfig(t *testing.T) {
	testCases := []struct {
		name        string
		environment string
		secure      bool
	}{
		{
			name:        "development allows local HTTP",
			environment: "development",
			secure:      false,
		},
		{
			name:        "test allows local HTTP",
			environment: "test",
			secure:      false,
		},
		{
			name:        "production requires HTTPS",
			environment: "production",
			secure:      true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			setValidTokenEnvironment(t)

			t.Setenv("APP_ENV", testCase.environment)
			t.Setenv("DATABASE_URL", testDatabaseURL)

			cfg, err := LoadConfig()
			if err != nil {
				t.Fatalf("LoadConfig() error = %v", err)
			}

			if cfg.SessionCookies.Secure() != testCase.secure {
				t.Errorf(
					"session cookie Secure() = %t, want %t",
					cfg.SessionCookies.Secure(),
					testCase.secure,
				)
			}

			if cfg.SessionCookies.RefreshCookieName() != "vaultforge_refresh" {
				t.Errorf(
					"refresh cookie name = %q",
					cfg.SessionCookies.RefreshCookieName(),
				)
			}

			if cfg.SessionCookies.CSRFCookieName() != "vaultforge_csrf" {
				t.Errorf(
					"CSRF cookie name = %q",
					cfg.SessionCookies.CSRFCookieName(),
				)
			}
		})
	}
}
