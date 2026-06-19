package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeDatabasePinger struct {
	err   error
	calls int
}

func (pinger *fakeDatabasePinger) Ping(
	_ context.Context,
) error {
	pinger.calls++

	return pinger.err
}

func TestLiveReturnsHealthStatus(t *testing.T) {
	databasePinger := &fakeDatabasePinger{
		err: errors.New("database unavailable"),
	}

	handler := NewHealthCheckHandler(
		"test",
		databasePinger,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/health/live",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Live(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	if databasePinger.calls != 0 {
		t.Errorf(
			"expected liveness not to ping database, got %d calls",
			databasePinger.calls,
		)
	}

	body := decodeHealthResponse(t, recorder)

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

func TestReadyReturnsOKWhenDatabaseIsAvailable(
	t *testing.T,
) {
	databasePinger := &fakeDatabasePinger{}

	handler := NewHealthCheckHandler(
		"test",
		databasePinger,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/health/ready",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Ready(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			recorder.Code,
		)
	}

	if databasePinger.calls != 1 {
		t.Errorf(
			"expected one database ping, got %d",
			databasePinger.calls,
		)
	}

	body := decodeHealthResponse(t, recorder)

	if body.Status != "ok" {
		t.Errorf(
			"expected status ok, got %q",
			body.Status,
		)
	}
}

func TestReadyReturnsServiceUnavailableWhenDatabaseFails(
	t *testing.T,
) {
	databasePinger := &fakeDatabasePinger{
		err: errors.New(
			"database connection failed with secret details",
		),
	}

	handler := NewHealthCheckHandler(
		"test",
		databasePinger,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/health/ready",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Ready(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusServiceUnavailable,
			recorder.Code,
		)
	}

	if databasePinger.calls != 1 {
		t.Errorf(
			"expected one database ping, got %d",
			databasePinger.calls,
		)
	}

	body := decodeHealthResponse(t, recorder)

	if body.Status != "unavailable" {
		t.Errorf(
			"expected status unavailable, got %q",
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

func TestReadyReturnsServiceUnavailableWithoutDatabase(
	t *testing.T,
) {
	handler := NewHealthCheckHandler(
		"test",
		nil,
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/health/ready",
		nil,
	)

	recorder := httptest.NewRecorder()

	handler.Ready(recorder, request)

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusServiceUnavailable,
			recorder.Code,
		)
	}
}

func decodeHealthResponse(
	t *testing.T,
	recorder *httptest.ResponseRecorder,
) healthResponse {
	t.Helper()

	contentType := recorder.Header().Get("Content-Type")

	if contentType != "application/json" {
		t.Errorf(
			"expected application/json content type, got %q",
			contentType,
		)
	}

	var body healthResponse

	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf(
			"failed to decode response: %v",
			err,
		)
	}

	return body
}
