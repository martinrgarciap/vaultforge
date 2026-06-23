package middleware

import (
	"net/http"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

const (
	requestIDHeader           = "X-Request-ID"
	maxIncomingRequestIDBytes = 128
)

func BoundedRequestID(next http.Handler) http.Handler {
	requestIDHandler := chimiddleware.RequestID(next)

	return http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		values := r.Header.Values(requestIDHeader)

		if len(values) != 1 ||
			!validIncomingRequestID(values[0]) {
			r.Header.Del(requestIDHeader)
		}

		requestIDHandler.ServeHTTP(w, r)
	})
}

func validIncomingRequestID(value string) bool {
	if value == "" ||
		len(value) > maxIncomingRequestIDBytes {
		return false
	}

	for index := 0; index < len(value); index++ {
		if !validRequestIDCharacter(value[index]) {
			return false
		}
	}

	return true
}

func validRequestIDCharacter(value byte) bool {
	if value >= 'a' && value <= 'z' {
		return true
	}

	if value >= 'A' && value <= 'Z' {
		return true
	}

	if value >= '0' && value <= '9' {
		return true
	}

	switch value {
	case '-', '.', '_', ':', '/':
		return true

	default:
		return false
	}
}
