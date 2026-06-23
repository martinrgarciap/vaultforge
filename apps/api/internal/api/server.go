package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

const maxHTTPHeaderBytes = 32 * 1024

func newHTTPServer(
	address string,
	handler http.Handler,
	config HTTPConfig,
) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: config.ReadHeaderTimeout(),
		ReadTimeout:       config.ReadTimeout(),
		WriteTimeout:      config.WriteTimeout(),
		IdleTimeout:       config.IdleTimeout(),
		MaxHeaderBytes:    maxHTTPHeaderBytes,
	}
}

func (app *Application) Run() error {
	server := newHTTPServer(
		app.config.Addr,
		app.Routes(),
		app.config.HTTP,
	)

	signalContext, stop := signal.NotifyContext(
		context.Background(),
		os.Interrupt,
		syscall.SIGTERM,
	)
	defer stop()

	serverErrors := make(chan error, 1)

	go func() {
		app.logger.Infow(
			"server started",
			"addr", app.config.Addr,
			"env", app.config.Env,
		)

		err := server.ListenAndServe()

		if err != nil &&
			!errors.Is(
				err,
				http.ErrServerClosed,
			) {
			serverErrors <- err

			return
		}

		serverErrors <- nil
	}()

	select {
	case err := <-serverErrors:
		return err

	case <-signalContext.Done():
		app.logger.Infow(
			"shutdown signal received",
		)

		stop()
	}

	shutdownContext, cancel := context.WithTimeout(
		context.Background(),
		app.config.HTTP.ShutdownTimeout(),
	)
	defer cancel()

	if err := server.Shutdown(
		shutdownContext,
	); err != nil {
		return fmt.Errorf(
			"shut down HTTP server: %w",
			err,
		)
	}

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
