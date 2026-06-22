package sessioncookie

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestManagerSetsReadsAndValidatesSessionCookies(t *testing.T) {
	t.Parallel()

	config := NewConfig(true)
	manager := NewManager(config)

	csrfToken, err := manager.GenerateCSRFToken(context.Background())
	if err != nil {
		t.Fatalf("generate CSRF token: %v", err)
	}

	expiresAt := time.Date(2026, time.July, 22, 12, 0, 0, 0, time.UTC)
	recorder := httptest.NewRecorder()

	if err := manager.Set(recorder, "synthetic-refresh-token", csrfToken, expiresAt); err != nil {
		t.Fatalf("set session cookies: %v", err)
	}

	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookie count = %d, want 2", len(cookies))
	}

	refreshCookie := findCookie(t, cookies, config.RefreshCookieName())
	if refreshCookie.Value != "synthetic-refresh-token" {
		t.Fatal("refresh cookie did not contain the expected token")
	}

	if refreshCookie.Path != config.RefreshCookiePath() ||
		!refreshCookie.HttpOnly ||
		!refreshCookie.Secure ||
		refreshCookie.Domain != "" {
		t.Fatal("refresh cookie attributes did not match the secure contract")
	}

	if refreshCookie.SameSite != http.SameSiteStrictMode ||
		!refreshCookie.Expires.Equal(expiresAt) {
		t.Fatal("refresh cookie security or expiration did not match")
	}

	csrfCookie := findCookie(t, cookies, config.CSRFCookieName())
	if csrfCookie.Value != csrfToken.Value() {
		t.Fatal("CSRF cookie did not contain the generated token")
	}

	if csrfCookie.Path != config.CSRFCookiePath() ||
		csrfCookie.HttpOnly ||
		!csrfCookie.Secure ||
		csrfCookie.Domain != "" {
		t.Fatal("CSRF cookie attributes did not match the secure contract")
	}

	if csrfCookie.SameSite != http.SameSiteStrictMode ||
		!csrfCookie.Expires.Equal(expiresAt) {
		t.Fatal("CSRF cookie security or expiration did not match")
	}

	request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	request.AddCookie(refreshCookie)
	request.AddCookie(csrfCookie)
	request.Header.Set(config.CSRFHeaderName(), csrfToken.Value())

	refreshToken, err := manager.RefreshToken(request)
	if err != nil {
		t.Fatalf("read refresh cookie: %v", err)
	}

	if refreshToken != "synthetic-refresh-token" {
		t.Fatal("refresh cookie was not read correctly")
	}

	if err := manager.ValidateCSRF(request); err != nil {
		t.Fatalf("validate CSRF token: %v", err)
	}
}

func TestManagerRejectsInvalidBrowserSessionState(t *testing.T) {
	t.Parallel()

	config := NewConfig(false)
	manager := NewManager(config)

	csrfToken, err := manager.GenerateCSRFToken(context.Background())
	if err != nil {
		t.Fatalf("generate CSRF token: %v", err)
	}

	t.Run("missing refresh cookie", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)

		_, err := manager.RefreshToken(request)
		if !errors.Is(err, ErrRefreshCookieInvalid) {
			t.Fatalf("RefreshToken() error = %v, want ErrRefreshCookieInvalid", err)
		}
	})

	t.Run("empty refresh cookie", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
		request.AddCookie(&http.Cookie{Name: config.RefreshCookieName()})

		_, err := manager.RefreshToken(request)
		if !errors.Is(err, ErrRefreshCookieInvalid) {
			t.Fatalf("RefreshToken() error = %v, want ErrRefreshCookieInvalid", err)
		}
	})

	t.Run("duplicate refresh cookie", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
		request.AddCookie(&http.Cookie{Name: config.RefreshCookieName(), Value: "first"})
		request.AddCookie(&http.Cookie{Name: config.RefreshCookieName(), Value: "second"})

		_, err := manager.RefreshToken(request)
		if !errors.Is(err, ErrRefreshCookieInvalid) {
			t.Fatalf("RefreshToken() error = %v, want ErrRefreshCookieInvalid", err)
		}
	})

	t.Run("missing CSRF cookie", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
		request.Header.Set(config.CSRFHeaderName(), csrfToken.Value())

		assertCSRFValidationFailed(t, manager.ValidateCSRF(request))
	})

	t.Run("missing CSRF header", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
		request.AddCookie(&http.Cookie{
			Name:  config.CSRFCookieName(),
			Value: csrfToken.Value(),
		})

		assertCSRFValidationFailed(t, manager.ValidateCSRF(request))
	})

	t.Run("duplicate CSRF cookie", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
		request.AddCookie(&http.Cookie{
			Name:  config.CSRFCookieName(),
			Value: csrfToken.Value(),
		})
		request.AddCookie(&http.Cookie{
			Name:  config.CSRFCookieName(),
			Value: csrfToken.Value(),
		})
		request.Header.Set(config.CSRFHeaderName(), csrfToken.Value())

		assertCSRFValidationFailed(t, manager.ValidateCSRF(request))
	})

	t.Run("duplicate CSRF header", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
		request.AddCookie(&http.Cookie{
			Name:  config.CSRFCookieName(),
			Value: csrfToken.Value(),
		})
		request.Header.Add(config.CSRFHeaderName(), csrfToken.Value())
		request.Header.Add(config.CSRFHeaderName(), csrfToken.Value())

		assertCSRFValidationFailed(t, manager.ValidateCSRF(request))
	})

	t.Run("malformed CSRF cookie", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
		request.AddCookie(&http.Cookie{
			Name:  config.CSRFCookieName(),
			Value: "malformed-csrf-cookie",
		})
		request.Header.Set(config.CSRFHeaderName(), csrfToken.Value())

		assertCSRFValidationFailed(t, manager.ValidateCSRF(request))
	})

	t.Run("malformed CSRF header", func(t *testing.T) {
		request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
		request.AddCookie(&http.Cookie{
			Name:  config.CSRFCookieName(),
			Value: csrfToken.Value(),
		})
		request.Header.Set(config.CSRFHeaderName(), "malformed-csrf-header")

		assertCSRFValidationFailed(t, manager.ValidateCSRF(request))
	})

	t.Run("mismatched CSRF token", func(t *testing.T) {
		otherToken, err := manager.GenerateCSRFToken(context.Background())
		if err != nil {
			t.Fatalf("generate other CSRF token: %v", err)
		}

		request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
		request.AddCookie(&http.Cookie{
			Name:  config.CSRFCookieName(),
			Value: csrfToken.Value(),
		})
		request.Header.Set(config.CSRFHeaderName(), otherToken.Value())

		assertCSRFValidationFailed(t, manager.ValidateCSRF(request))
	})
}

func TestManagerClearsSessionCookies(t *testing.T) {
	t.Parallel()

	config := NewConfig(true)
	manager := NewManager(config)
	recorder := httptest.NewRecorder()

	manager.Clear(recorder)

	cookies := recorder.Result().Cookies()
	if len(cookies) != 2 {
		t.Fatalf("cookie count = %d, want 2", len(cookies))
	}

	refreshCookie := findCookie(t, cookies, config.RefreshCookieName())
	if refreshCookie.Value != "" ||
		refreshCookie.MaxAge != -1 ||
		!refreshCookie.HttpOnly ||
		refreshCookie.Path != config.RefreshCookiePath() ||
		!refreshCookie.Secure {
		t.Fatal("refresh clearing cookie did not match the secure contract")
	}

	csrfCookie := findCookie(t, cookies, config.CSRFCookieName())
	if csrfCookie.Value != "" ||
		csrfCookie.MaxAge != -1 ||
		csrfCookie.HttpOnly ||
		csrfCookie.Path != config.CSRFCookiePath() ||
		!csrfCookie.Secure {
		t.Fatal("CSRF clearing cookie did not match the secure contract")
	}
}

func assertCSRFValidationFailed(t *testing.T, err error) {
	t.Helper()

	if !errors.Is(err, ErrCSRFValidationFailed) {
		t.Fatalf("ValidateCSRF() error = %v, want ErrCSRFValidationFailed", err)
	}
}

func findCookie(t *testing.T, cookies []*http.Cookie, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range cookies {
		if cookie.Name == name {
			return cookie
		}
	}

	t.Fatalf("cookie %q was not found", name)
	return nil
}
