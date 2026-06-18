package response

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSON(t *testing.T) {
	recorder := httptest.NewRecorder()

	data := struct {
		Status string `json:"status"`
	}{
		Status: "ok",
	}

	err := WriteJSON(
		recorder,
		http.StatusCreated,
		data,
	)
	if err != nil {
		t.Fatalf("WriteJSON returned an error: %v", err)
	}

	if recorder.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			recorder.Code,
		)
	}

	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf(
			"expected application/json, got %q",
			contentType,
		)
	}

	var body struct {
		Status string `json:"status"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body.Status != "ok" {
		t.Errorf(
			"expected status ok, got %q",
			body.Status,
		)
	}
}

func TestWriteError(t *testing.T) {
	recorder := httptest.NewRecorder()

	err := WriteError(
		recorder,
		http.StatusNotFound,
		"not_found",
		"The requested resource was not found.",
		"request-123",
	)
	if err != nil {
		t.Fatalf("WriteError returned an error: %v", err)
	}

	if recorder.Code != http.StatusNotFound {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusNotFound,
			recorder.Code,
		)
	}

	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json" {
		t.Errorf(
			"expected application/json, got %q",
			contentType,
		)
	}

	var body struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if body.Error.Code != "not_found" {
		t.Errorf(
			"expected code not_found, got %q",
			body.Error.Code,
		)
	}

	if body.Error.Message != "The requested resource was not found." {
		t.Errorf(
			"unexpected error message %q",
			body.Error.Message,
		)
	}

	if body.Error.RequestID != "request-123" {
		t.Errorf(
			"expected request ID request-123, got %q",
			body.Error.RequestID,
		)
	}
}

func TestWriteErrorOmitsEmptyRequestID(t *testing.T) {
	recorder := httptest.NewRecorder()

	err := WriteError(
		recorder,
		http.StatusBadRequest,
		"bad_request",
		"Invalid request.",
		"",
	)
	if err != nil {
		t.Fatalf("WriteError returned an error: %v", err)
	}

	var body struct {
		Error map[string]any `json:"error"`
	}

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}

	if _, exists := body.Error["request_id"]; exists {
		t.Error("expected empty request_id to be omitted")
	}
}
