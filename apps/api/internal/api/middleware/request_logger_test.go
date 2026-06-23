package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	oteltrace "go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestRequestLoggerLogsSafeRequestDetails(t *testing.T) {
	core, observedLogs := observer.New(zap.InfoLevel)
	logger := zap.New(core).Sugar()

	responseBody := `{"status":"created"}`

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		if _, err := w.Write([]byte(responseBody)); err != nil {
			t.Fatalf("write test response: %v", err)
		}
	})

	router := chi.NewRouter()
	router.Use(chimiddleware.RequestID)
	router.Use(RequestLogger(logger))
	router.Post("/vaults", nextHandler)

	request := httptest.NewRequest(
		http.MethodPost,
		"/vaults?include=metadata",
		strings.NewReader(`{"password":"never-log-this","vault_value":"secret-value"}`),
	)

	request.Header.Set("Authorization", "Bearer super-secret-token")

	traceID, err := oteltrace.TraceIDFromHex("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("create trace ID: %v", err)
	}

	spanID, err := oteltrace.SpanIDFromHex("0123456789abcdef")
	if err != nil {
		t.Fatalf("create span ID: %v", err)
	}

	spanContext := oteltrace.NewSpanContext(oteltrace.SpanContextConfig{
		TraceID: traceID,
		SpanID:  spanID,
	})

	request = request.WithContext(
		oteltrace.ContextWithSpanContext(request.Context(), spanContext),
	)

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}

	entries := observedLogs.All()
	if len(entries) != 1 {
		t.Fatalf("log entry count = %d, want 1", len(entries))
	}

	entry := entries[0]
	if entry.Message != "HTTP request completed" {
		t.Fatalf("message = %q", entry.Message)
	}

	fields := entry.ContextMap()

	if fields["method"] != http.MethodPost {
		t.Fatalf("method = %v", fields["method"])
	}

	if fields["route"] != "/vaults" {
		t.Fatalf("route = %v", fields["route"])
	}

	if _, exists := fields["path"]; exists {
		t.Fatal("raw path field was logged")
	}

	if fmt.Sprint(fields["status"]) != "201" {
		t.Fatalf("status = %v", fields["status"])
	}

	if fields["trace_id"] != traceID.String() {
		t.Fatalf("trace ID = %v", fields["trace_id"])
	}

	requestID, ok := fields["request_id"].(string)
	if !ok || requestID == "" {
		t.Fatal("request ID was missing")
	}

	if _, exists := fields["duration_ms"]; !exists {
		t.Fatal("duration_ms was missing")
	}

	if _, exists := fields["bytes"]; !exists {
		t.Fatal("bytes was missing")
	}

	encodedFields, err := json.Marshal(fields)
	if err != nil {
		t.Fatalf("encode log fields: %v", err)
	}

	logOutput := entry.Message + string(encodedFields)

	for _, forbidden := range []string{
		"super-secret-token",
		"never-log-this",
		"secret-value",
		"include=metadata",
	} {
		if strings.Contains(logOutput, forbidden) {
			t.Fatalf("log exposed %q", forbidden)
		}
	}
}
