package sessioncookie

import "net/http"

const (
	refreshCookieName = "vaultforge_refresh"
	csrfCookieName    = "vaultforge_csrf"
	csrfHeaderName    = "X-CSRF-Token"
	refreshCookiePath = "/v1/auth/refresh"
	csrfCookiePath    = "/"
)

type Config struct {
	secure bool
}

func NewConfig(secure bool) Config {
	return Config{secure: secure}
}

func (config Config) RefreshCookieName() string {
	return refreshCookieName
}

func (config Config) CSRFCookieName() string {
	return csrfCookieName
}

func (config Config) CSRFHeaderName() string {
	return csrfHeaderName
}

func (config Config) RefreshCookiePath() string {
	return refreshCookiePath
}

func (config Config) CSRFCookiePath() string {
	return csrfCookiePath
}

func (config Config) SameSite() http.SameSite {
	return http.SameSiteStrictMode
}

func (config Config) Secure() bool {
	return config.secure
}
