package request

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequireEmptyBodyAcceptsAbsentBody(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(http.MethodPost, "/v1/auth/refresh", nil)
	recorder := httptest.NewRecorder()

	if err := RequireEmptyBody(recorder, request); err != nil {
		t.Fatalf("RequireEmptyBody() error = %v", err)
	}
}

func TestRequireEmptyBodyAcceptsEmptyBody(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/refresh",
		strings.NewReader(""),
	)
	recorder := httptest.NewRecorder()

	if err := RequireEmptyBody(recorder, request); err != nil {
		t.Fatalf("RequireEmptyBody() error = %v", err)
	}
}

func TestRequireEmptyBodyRejectsNonEmptyBody(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/refresh",
		strings.NewReader(`{"refreshToken":"not-accepted-here"}`),
	)
	recorder := httptest.NewRecorder()

	err := RequireEmptyBody(recorder, request)
	if !errors.Is(err, ErrBodyNotAllowed) {
		t.Fatalf("RequireEmptyBody() error = %v, want ErrBodyNotAllowed", err)
	}
}

func TestRequireEmptyBodyRejectsLargeBody(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/auth/refresh",
		strings.NewReader(strings.Repeat("a", 1024*1024)),
	)
	recorder := httptest.NewRecorder()

	err := RequireEmptyBody(recorder, request)
	if !errors.Is(err, ErrBodyNotAllowed) {
		t.Fatalf("RequireEmptyBody() error = %v, want ErrBodyNotAllowed", err)
	}
}
