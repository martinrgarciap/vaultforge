package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireLoopbackAllowsDirectLoopbackPeers(t *testing.T) {
	t.Parallel()

	addresses := []string{
		"127.0.0.1:54321",
		"[::1]:54321",
	}

	for _, address := range addresses {
		address := address

		t.Run(address, func(t *testing.T) {
			t.Parallel()

			called := false

			handler := RequireLoopback(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			}))

			request := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
			request.RemoteAddr = address

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
			}

			if !called {
				t.Fatal("loopback request did not reach handler")
			}
		})
	}
}

func TestRequireLoopbackRejectsExternalPeer(t *testing.T) {
	t.Parallel()

	called := false

	handler := RequireLoopback(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	request := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	request.RemoteAddr = "203.0.113.10:54321"

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}

	if called {
		t.Fatal("external request reached internal handler")
	}
}

func TestRequireLoopbackIgnoresForwardedLoopbackAddress(t *testing.T) {
	t.Parallel()

	called := false

	handler := RequireLoopback(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		called = true
	}))

	request := httptest.NewRequest(http.MethodGet, "/internal/metrics", nil)
	request.RemoteAddr = "203.0.113.10:54321"
	request.Header.Set("X-Forwarded-For", "127.0.0.1")
	request.Header.Set("X-Real-IP", "127.0.0.1")

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}

	if called {
		t.Fatal("spoofed forwarding header bypassed internal restriction")
	}
}
