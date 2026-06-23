package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

type deadlineCapturingPinger struct {
	deadline    time.Time
	hasDeadline bool
}

func (pinger *deadlineCapturingPinger) Ping(ctx context.Context) error {
	pinger.deadline, pinger.hasDeadline = ctx.Deadline()

	return nil
}

func TestReadyAppliesShortDependencyDeadline(t *testing.T) {
	t.Parallel()

	pinger := &deadlineCapturingPinger{}
	handler := NewHealthCheckHandler("test", pinger)

	request := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	recorder := httptest.NewRecorder()

	startedAt := time.Now()
	handler.Ready(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}

	if !pinger.hasDeadline {
		t.Fatal("readiness dependency context had no deadline")
	}

	deadlineDuration := pinger.deadline.Sub(startedAt)

	if deadlineDuration <= 0 || deadlineDuration > readinessTimeout+100*time.Millisecond {
		t.Fatalf("readiness deadline = %v, want between zero and %v", deadlineDuration, readinessTimeout)
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
