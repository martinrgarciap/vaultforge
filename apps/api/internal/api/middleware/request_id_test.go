package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type requestIDTestResult struct {
	contextValue string
	headerValues []string
}

func TestBoundedRequestIDPreservesValidIncomingID(
	t *testing.T,
) {
	t.Parallel()

	validID := strings.Repeat(
		"a",
		maxIncomingRequestIDBytes,
	)

	result := serveRequestIDTest(
		t,
		func(request *http.Request) {
			request.Header.Set(
				requestIDHeader,
				validID,
			)
		},
	)

	if result.contextValue != validID {
		t.Fatalf(
			"request ID = %q, want supplied value",
			result.contextValue,
		)
	}

	if len(result.headerValues) != 1 ||
		result.headerValues[0] != validID {
		t.Fatalf(
			"request header values = %#v",
			result.headerValues,
		)
	}
}

func TestBoundedRequestIDReplacesOversizedID(
	t *testing.T,
) {
	t.Parallel()

	result := serveRequestIDTest(
		t,
		func(request *http.Request) {
			request.Header.Set(
				requestIDHeader,
				strings.Repeat(
					"a",
					maxIncomingRequestIDBytes+1,
				),
			)
		},
	)

	assertGeneratedRequestID(t, result)
}

func TestBoundedRequestIDReplacesUnsafeID(
	t *testing.T,
) {
	t.Parallel()

	result := serveRequestIDTest(
		t,
		func(request *http.Request) {
			request.Header.Set(
				requestIDHeader,
				"attacker supplied request id",
			)
		},
	)

	assertGeneratedRequestID(t, result)
}

func TestBoundedRequestIDReplacesMultipleIDs(
	t *testing.T,
) {
	t.Parallel()

	result := serveRequestIDTest(
		t,
		func(request *http.Request) {
			request.Header.Add(
				requestIDHeader,
				"first-request-id",
			)
			request.Header.Add(
				requestIDHeader,
				"second-request-id",
			)
		},
	)

	assertGeneratedRequestID(t, result)
}

func serveRequestIDTest(
	t *testing.T,
	configure func(*http.Request),
) requestIDTestResult {
	t.Helper()

	var result requestIDTestResult

	handler := BoundedRequestID(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			result.contextValue =
				chimiddleware.GetReqID(
					r.Context(),
				)

			result.headerValues = append(
				[]string(nil),
				r.Header.Values(
					requestIDHeader,
				)...,
			)

			w.WriteHeader(http.StatusNoContent)
		}),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/health/live",
		nil,
	)

	configure(request)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusNoContent,
		)
	}

	return result
}

func assertGeneratedRequestID(
	t *testing.T,
	result requestIDTestResult,
) {
	t.Helper()

	if result.contextValue == "" {
		t.Fatal(
			"invalid incoming request ID was not replaced",
		)
	}

	if len(result.headerValues) != 0 {
		t.Fatalf(
			"invalid incoming header remained: %#v",
			result.headerValues,
		)
	}
}
