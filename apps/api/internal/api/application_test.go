package api

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewApplicationStoresServicesAndCreatesHandlers(t *testing.T) {
	sessionService := newTestSessionService()

	app := NewApplication(
		Config{
			Env:         "test",
			Addr:        ":8080",
			DatabaseURL: "postgres://test",
		},
		zap.NewNop().Sugar(),
		&testDatabasePinger{},
		newAllowingTestRequestLimiter(),
		&routeTestAuthService{},
		sessionService,
		nil,
		nil,
	)

	if app.sessionService != sessionService {
		t.Fatal("application did not retain the supplied session service")
	}

	if app.sessionHandler == nil {
		t.Fatal("application did not create the session handler")
	}

	if app.vaultHandler == nil {
		t.Fatal("application did not create the vault handler")
	}

	if app.itemHandler == nil {
		t.Fatal("application did not create the item handler")
	}
}
