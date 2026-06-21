package api

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewApplicationStoresSessionService(
	t *testing.T,
) {
	sessionService := newTestSessionService()

	app := NewApplication(
		Config{
			Env:         "test",
			Addr:        ":8080",
			DatabaseURL: "postgres://test",
		},
		zap.NewNop().Sugar(),
		&testDatabasePinger{},
		&routeTestAuthService{},
		sessionService,
	)

	if app.sessionService != sessionService {
		t.Fatal(
			"application did not retain the supplied session service",
		)
	}

	if app.sessionHandler == nil {
		t.Fatal(
			"application did not create the session handler",
		)
	}
}
