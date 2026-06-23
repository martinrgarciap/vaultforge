package api

import (
	"net/http"
	"testing"
	"time"
)

func TestNewHTTPServerUsesConfiguredTimeouts(
	t *testing.T,
) {
	t.Parallel()

	config, err := newHTTPConfig(
		20*time.Second,
		4*time.Second,
		12*time.Second,
		45*time.Second,
		90*time.Second,
		8*time.Second,
	)
	if err != nil {
		t.Fatalf(
			"create HTTP configuration: %v",
			err,
		)
	}

	handler := http.HandlerFunc(func(
		http.ResponseWriter,
		*http.Request,
	) {
	})

	server := newHTTPServer(
		"127.0.0.1:8082",
		handler,
		config,
	)

	if server.Addr != "127.0.0.1:8082" {
		t.Fatalf(
			"address = %q",
			server.Addr,
		)
	}

	if server.Handler == nil {
		t.Fatal("server handler was nil")
	}

	if server.ReadHeaderTimeout !=
		4*time.Second {
		t.Fatalf(
			"read-header timeout = %v",
			server.ReadHeaderTimeout,
		)
	}

	if server.ReadTimeout !=
		12*time.Second {
		t.Fatalf(
			"read timeout = %v",
			server.ReadTimeout,
		)
	}

	if server.WriteTimeout !=
		45*time.Second {
		t.Fatalf(
			"write timeout = %v",
			server.WriteTimeout,
		)
	}

	if server.IdleTimeout !=
		90*time.Second {
		t.Fatalf(
			"idle timeout = %v",
			server.IdleTimeout,
		)
	}

	if server.MaxHeaderBytes != maxHTTPHeaderBytes {
		t.Fatalf(
			"maximum header bytes = %d, want %d",
			server.MaxHeaderBytes,
			maxHTTPHeaderBytes,
		)
	}
}

func TestNewHTTPServerUsesSafeZeroValueDefaults(
	t *testing.T,
) {
	t.Parallel()

	server := newHTTPServer(
		":8080",
		http.NotFoundHandler(),
		HTTPConfig{},
	)

	if server.ReadHeaderTimeout !=
		defaultReadHeaderTimeout {
		t.Fatal(
			"zero-value read-header timeout was unsafe",
		)
	}

	if server.ReadTimeout !=
		defaultReadTimeout {
		t.Fatal("zero-value read timeout was unsafe")
	}

	if server.WriteTimeout !=
		defaultWriteTimeout {
		t.Fatal("zero-value write timeout was unsafe")
	}

	if server.IdleTimeout !=
		defaultIdleTimeout {
		t.Fatal("zero-value idle timeout was unsafe")
	}
}
