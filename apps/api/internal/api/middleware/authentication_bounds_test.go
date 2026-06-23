package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBearerTokenAcceptsMaximumLength(
	t *testing.T,
) {
	t.Parallel()

	tokenValue := strings.Repeat(
		"a",
		maxBearerTokenBytes,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/vaults",
		nil,
	)
	request.Header.Set(
		"Authorization",
		"Bearer "+tokenValue,
	)

	gotToken, ok := bearerToken(request)
	if !ok {
		t.Fatal(
			"maximum-length bearer token was rejected",
		)
	}

	if gotToken != tokenValue {
		t.Fatal(
			"bearer token changed during parsing",
		)
	}
}

func TestBearerTokenRejectsOversizedValue(
	t *testing.T,
) {
	t.Parallel()

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/vaults",
		nil,
	)
	request.Header.Set(
		"Authorization",
		"Bearer "+
			strings.Repeat(
				"a",
				maxBearerTokenBytes+1,
			),
	)

	if _, ok := bearerToken(request); ok {
		t.Fatal(
			"oversized bearer token was accepted",
		)
	}
}
