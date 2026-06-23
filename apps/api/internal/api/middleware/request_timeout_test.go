package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRequestTimeoutAllowsFastHandler(
	t *testing.T,
) {
	t.Parallel()

	handler := RequestTimeout(
		time.Second,
	)(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			if _, ok :=
				r.Context().Deadline(); !ok {
				t.Fatal(
					"request context omitted its deadline",
				)
			}

			w.WriteHeader(http.StatusNoContent)
		}),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/health/live",
		nil,
	)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusNoContent,
		)
	}
}

func TestRequestTimeoutCancelsSlowHandler(
	t *testing.T,
) {
	t.Parallel()

	const requestTimeout = 25 * time.Millisecond

	contextResult := make(chan error, 1)

	handler := RequestTimeout(
		requestTimeout,
	)(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			<-r.Context().Done()

			contextResult <- r.Context().Err()

			w.WriteHeader(
				http.StatusServiceUnavailable,
			)
		}),
	)

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/vaults",
		nil,
	)

	recorder := httptest.NewRecorder()

	startedAt := time.Now()
	handler.ServeHTTP(recorder, request)
	elapsed := time.Since(startedAt)

	if recorder.Code !=
		http.StatusServiceUnavailable {
		t.Fatalf(
			"status = %d, want %d",
			recorder.Code,
			http.StatusServiceUnavailable,
		)
	}

	select {
	case contextErr := <-contextResult:
		if !errors.Is(
			contextErr,
			context.DeadlineExceeded,
		) {
			t.Fatalf(
				"context error = %v, want %v",
				contextErr,
				context.DeadlineExceeded,
			)
		}

	case <-time.After(time.Second):
		t.Fatal(
			"request context was not canceled",
		)
	}

	if elapsed < requestTimeout {
		t.Fatalf(
			"handler returned after %v, before timeout %v",
			elapsed,
			requestTimeout,
		)
	}

	if elapsed > time.Second {
		t.Fatalf(
			"handler took too long to observe cancellation: %v",
			elapsed,
		)
	}
}
