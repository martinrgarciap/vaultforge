package sessioncookie

import (
	"context"
	"errors"
	"net/http"
	"time"
)

var (
	ErrSessionCookieUnavailable = errors.New("session cookie transport is unavailable")
	ErrRefreshCookieInvalid     = errors.New("refresh cookie is missing or invalid")
	ErrCSRFValidationFailed     = errors.New("CSRF validation failed")
)

type Manager struct {
	config         Config
	tokenGenerator *CSRFTokenGenerator
}

func NewManager(config Config) *Manager {
	return &Manager{config: config, tokenGenerator: NewCSRFTokenGenerator()}
}

func (manager *Manager) GenerateCSRFToken(ctx context.Context) (CSRFToken, error) {
	if manager == nil || manager.tokenGenerator == nil {
		return CSRFToken{}, ErrSessionCookieUnavailable
	}

	token, err := manager.tokenGenerator.Generate(ctx)
	if err != nil {
		return CSRFToken{}, errors.Join(ErrSessionCookieUnavailable, err)
	}

	return token, nil
}

func (manager *Manager) Set(w http.ResponseWriter, refreshToken string, csrfToken CSRFToken, expiresAt time.Time) error {
	if manager == nil || refreshToken == "" || csrfToken.Value() == "" || expiresAt.IsZero() {
		return ErrSessionCookieUnavailable
	}

	http.SetCookie(w, &http.Cookie{
		Name:     manager.config.RefreshCookieName(),
		Value:    refreshToken,
		Path:     manager.config.RefreshCookiePath(),
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   manager.config.Secure(),
		SameSite: manager.config.SameSite(),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     manager.config.CSRFCookieName(),
		Value:    csrfToken.Value(),
		Path:     manager.config.CSRFCookiePath(),
		Expires:  expiresAt,
		HttpOnly: false,
		Secure:   manager.config.Secure(),
		SameSite: manager.config.SameSite(),
	})

	return nil
}

func (manager *Manager) RefreshToken(r *http.Request) (string, error) {
	if manager == nil || r == nil {
		return "", ErrRefreshCookieInvalid
	}

	cookies := cookiesNamed(r, manager.config.RefreshCookieName())
	if len(cookies) != 1 || cookies[0].Value == "" {
		return "", ErrRefreshCookieInvalid
	}

	return cookies[0].Value, nil
}

func (manager *Manager) ValidateCSRF(r *http.Request) error {
	if manager == nil || r == nil {
		return ErrCSRFValidationFailed
	}

	cookies := cookiesNamed(r, manager.config.CSRFCookieName())
	headers := r.Header.Values(manager.config.CSRFHeaderName())
	if len(cookies) != 1 || len(headers) != 1 {
		return ErrCSRFValidationFailed
	}

	cookieToken, err := ParseCSRFToken(cookies[0].Value)
	if err != nil {
		return ErrCSRFValidationFailed
	}

	headerToken, err := ParseCSRFToken(headers[0])
	if err != nil || !cookieToken.Equal(headerToken) {
		return ErrCSRFValidationFailed
	}

	return nil
}

func (manager *Manager) Clear(w http.ResponseWriter) {
	if manager == nil {
		return
	}

	expiresAt := time.Unix(1, 0).UTC()

	http.SetCookie(w, &http.Cookie{
		Name:     manager.config.RefreshCookieName(),
		Value:    "",
		Path:     manager.config.RefreshCookiePath(),
		Expires:  expiresAt,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   manager.config.Secure(),
		SameSite: manager.config.SameSite(),
	})

	http.SetCookie(w, &http.Cookie{
		Name:     manager.config.CSRFCookieName(),
		Value:    "",
		Path:     manager.config.CSRFCookiePath(),
		Expires:  expiresAt,
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   manager.config.Secure(),
		SameSite: manager.config.SameSite(),
	})
}

func cookiesNamed(r *http.Request, name string) []*http.Cookie {
	cookies := make([]*http.Cookie, 0, 1)
	for _, cookie := range r.Cookies() {
		if cookie.Name == name {
			cookies = append(cookies, cookie)
		}
	}

	return cookies
}
