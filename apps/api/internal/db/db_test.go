package db

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNewRejectsMalformedConfigurationSafely(
	t *testing.T,
) {
	t.Parallel()

	const secretMarker = "synthetic-database-secret-marker"

	_, err := New(
		context.Background(),
		"not-a-postgres-url://"+secretMarker,
	)
	if err == nil {
		t.Fatal(
			"expected malformed configuration to fail",
		)
	}

	if strings.Contains(
		err.Error(),
		secretMarker,
	) {
		t.Fatal(
			"database configuration error exposed the URL",
		)
	}
}

func TestNewMapsUnavailableDatabaseSafely(
	t *testing.T,
) {
	t.Parallel()

	const secretMarker = "synthetic-database-password-marker"

	ctx, cancel := context.WithTimeout(
		context.Background(),
		time.Second,
	)
	defer cancel()

	_, err := New(
		ctx,
		"postgres://vaultforge:"+
			secretMarker+
			"@127.0.0.1:1/vaultforge"+
			"?sslmode=disable",
	)
	if err == nil {
		t.Fatal(
			"expected unavailable database to fail",
		)
	}

	errorText := err.Error()

	for _, forbidden := range []string{
		secretMarker,
		"127.0.0.1:1",
		"connection refused",
		"dial tcp",
	} {
		if strings.Contains(
			errorText,
			forbidden,
		) {
			t.Fatalf(
				"database error exposed %q",
				forbidden,
			)
		}
	}
}

func TestNewFailsSafelyWithCanceledContext(
	t *testing.T,
) {
	t.Parallel()

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := New(
		ctx,
		"postgres://vaultforge:test"+
			"@127.0.0.1:1/vaultforge"+
			"?sslmode=disable",
	)
	if err == nil {
		t.Fatal(
			"expected canceled startup to fail",
		)
	}

	if strings.Contains(
		err.Error(),
		"127.0.0.1:1",
	) {
		t.Fatal(
			"canceled startup exposed database details",
		)
	}
}
