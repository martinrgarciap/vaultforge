package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func assertRouteRateLimitError(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
	wantStatus int,
	wantCode string,
) {
	t.Helper()

	if recorder.Code != wantStatus {
		t.Fatalf(
			"status = %d, want %d; body = %s",
			recorder.Code,
			wantStatus,
			recorder.Body.String(),
		)
	}

	var body errorResponseBody

	if err := json.NewDecoder(
		recorder.Body,
	).Decode(&body); err != nil {
		t.Fatalf(
			"decode rate-limit response: %v",
			err,
		)
	}

	if body.Error.Code != wantCode {
		t.Fatalf(
			"error code = %q, want %q",
			body.Error.Code,
			wantCode,
		)
	}

	if body.Error.Message == "" {
		t.Fatal("rate-limit response omitted its safe message")
	}

	if body.Error.RequestID == "" {
		t.Fatal("rate-limit response omitted its request ID")
	}
}
