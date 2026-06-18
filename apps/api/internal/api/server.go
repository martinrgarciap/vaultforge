package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 1 * time.Minute
	shutdownTimeout   = 5 * time.Second
)

func (app *Application) Run() error {
	server := &http.Server{
		Addr:              app.config.Addr,
		Handler:           app.Routes(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	// This context is cancelled when the application receives Ctrl+C
	// or a termination signal from the operating system.
	signalContext, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	// ListenAndServe blocks, so start the server in a goroutine.
	serverErrors := make(chan error, 1)

	go func() {
		app.logger.Infow(
			"server started",
			"addr", app.config.Addr,
			"env", app.config.Env,
		)

		err := server.ListenAndServe()

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
			return
		}

		serverErrors <- nil
	}()

	// Wait until either the server fails or a shutdown signal arrives.
	select {
	case err := <-serverErrors:
		return err

	case <-signalContext.Done():
		app.logger.Infow(
			"shutdown signal received",
		)
		// Restore the operating system's normal signal behavior.
		// A second Ctrl+C can then force the program to exit.
		stop()
	}

	// Give active requests up to five seconds to finish.
	shutdownContext, cancel := context.WithTimeout(
		context.Background(),
		shutdownTimeout,
	)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		return fmt.Errorf("shut down HTTP server: %w", err)
	}

	// Wait for ListenAndServe to return after Shutdown closes the server.
	if err := <-serverErrors; err != nil {
		return err
	}

	app.logger.Infow(
		"server stopped",
		"addr", app.config.Addr,
		"env", app.config.Env,
	)

	return nil
}
