package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestLoggerLogsRequestDetailsWithoutSecrets(t *testing.T) {
	core, observedLogs := observer.New(zap.InfoLevel)
	logger := zap.New(core).Sugar()

	responseBody := `{"status":"created"}`

	nextHandler := http.HandlerFunc(func(
		w http.ResponseWriter,
		r *http.Request,
	) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		if _, err := w.Write([]byte(responseBody)); err != nil {
			t.Fatalf("failed to write test response: %v", err)
		}
	})

	handler := chimiddleware.RequestID(
		RequestLogger(logger)(nextHandler),
	)

	requestBody := strings.NewReader(
		`{"password":"never-log-this","vault_value":"secret-value"}`,
	)

	request := httptest.NewRequest(
		http.MethodPost,
		"/vaults?include=metadata",
		requestBody,
	)

	request.Header.Set(
		"Authorization",
		"Bearer super-secret-token",
	)

	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			recorder.Code,
		)
	}

	entries := observedLogs.All()

	if len(entries) != 1 {
		t.Fatalf(
			"expected 1 log entry, got %d",
			len(entries),
		)
	}

	entry := entries[0]

	if entry.Message != "HTTP request completed" {
		t.Errorf(
			"expected log message %q, got %q",
			"HTTP request completed",
			entry.Message,
		)
	}

	fields := entry.ContextMap()

	if fields["method"] != http.MethodPost {
		t.Errorf(
			"expected method POST, got %v",
			fields["method"],
		)
	}

	if fields["path"] != "/vaults" {
		t.Errorf(
			"expected path /vaults, got %v",
			fields["path"],
		)
	}

	if fmt.Sprint(fields["status"]) != "201" {
		t.Errorf(
			"expected status 201, got %v",
			fields["status"],
		)
	}

	requestID, ok := fields["request_id"].(string)
	if !ok || requestID == "" {
		t.Error("expected log entry to contain a request ID")
	}

	if _, exists := fields["duration_ms"]; !exists {
		t.Error("expected log entry to contain duration_ms")
	}

	if _, exists := fields["bytes"]; !exists {
		t.Error("expected log entry to contain bytes")
	}

	encodedFields, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("failed to encode log fields: %v", err)
	}

	logOutput := entry.Message + string(encodedFields)

	sensitiveValues := []string{
		"super-secret-token",
		"never-log-this",
		"secret-value",
		"include=metadata",
	}

	for _, value := range sensitiveValues {
		if strings.Contains(logOutput, value) {
			t.Errorf(
				"sensitive value %q was written to logs",
				value,
			)
		}
	}
}
