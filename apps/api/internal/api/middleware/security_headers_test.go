package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	nextHandler := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.WriteHeader(http.StatusOK)
	})

	handler := SecurityHeaders(nextHandler)

	request := httptest.NewRequest(
		http.MethodGet,
		"/health",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	tests := []struct {
		header   string
		expected string
	}{
		{
			header:   "X-Content-Type-Options",
			expected: "nosniff",
		},
		{
			header:   "X-Frame-Options",
			expected: "DENY",
		},
		{
			header:   "Referrer-Policy",
			expected: "no-referrer",
		},
	}

	for _, test := range tests {
		t.Run(test.header, func(t *testing.T) {
			actual := recorder.Header().Get(test.header)

			if actual != test.expected {
				t.Errorf(
					"expected %s to be %q, got %q",
					test.header,
					test.expected,
					actual,
				)
			}
		})
	}

	if recorder.Code != http.StatusOK {
		t.Errorf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}
}
