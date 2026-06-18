package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
)

type errorResponseBody struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	} `json:"error"`
}

func TestRecoverPanicReturnsInternalServerError(t *testing.T) {
	logger := zap.NewNop().Sugar()

	panicHandler := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		panic("sensitive panic value")
	})

	handler := chimiddleware.RequestID(
		RecoverPanic(logger)(panicHandler),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/panic",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusInternalServerError,
			recorder.Code,
		)
	}

	contentType := recorder.Header().Get("Content-Type")

	if contentType != "application/json" {
		t.Errorf(
			"expected content type application/json, got %q",
			contentType,
		)
	}

	var body errorResponseBody

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.Error.Code != "internal_server_error" {
		t.Errorf(
			"expected error code internal_server_error, got %q",
			body.Error.Code,
		)
	}

	if body.Error.Message != "The server encountered a problem." {
		t.Errorf(
			"unexpected public message %q",
			body.Error.Message,
		)
	}

	if body.Error.RequestID == "" {
		t.Error("expected response to contain a request ID")
	}

	if strings.Contains(
		recorder.Body.String(),
		"sensitive panic value",
	) {
		t.Error("panic value was exposed in the response")
	}
}

func TestRecoverPanicAllowsNormalResponse(t *testing.T) {
	logger := zap.NewNop().Sugar()

	normalHandler := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := RecoverPanic(logger)(normalHandler)

	request := httptest.NewRequest(
		http.MethodGet,
		"/normal",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusNoContent,
			recorder.Code,
		)
	}
}
