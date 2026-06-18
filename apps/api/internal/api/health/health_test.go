package health

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLiveReturnsHealthStatus(t *testing.T) {
	handler := NewHealthCheckHandler("test")

	request := httptest.NewRequest(
		http.MethodGet,
		"/health/live",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.HealthCheck(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	contentType := recorder.Header().Get("Content-Type")

	if contentType != "application/json" {
		t.Errorf(
			"expected application/json content type, got %q",
			contentType,
		)
	}

	var body healthResponse

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf(
			"expected status ok, got %q",
			body.Status,
		)
	}

	if body.Environment != "test" {
		t.Errorf(
			"expected environment test, got %q",
			body.Environment,
		)
	}
}
