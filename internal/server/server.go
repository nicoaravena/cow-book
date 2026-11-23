// Package server runs an http.Handler until its context ends, then shuts it
// down gracefully. It knows nothing about routes: the API now, and the web
// pages from M7, are handlers the composition root passes in.
package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// An http.Server's timeouts are all zero, meaning none, unless set. Without
// them a client that sends headers one byte a minute (slowloris), or never
// reads the answer, holds a connection and a goroutine forever.
const (
	readHeaderTimeout = 5 * time.Second  // request line and headers
	readTimeout       = 15 * time.Second // the whole request, body included
	writeTimeout      = 15 * time.Second // from the end of the headers to the end of the answer
	idleTimeout       = 60 * time.Second // a kept-alive connection between requests
)

// New returns a server for h with the timeouts set. The server's own errors,
// such as a failed TLS handshake or a panic in a handler, go to log at error
// level instead of the standard logger.
func New(h http.Handler, log *slog.Logger) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelError),
	}
}

// Run serves on ln until ctx ends, then shuts srv down: it stops accepting
// connections, waits up to shutdownTimeout for requests in flight, and
// returns nil if they all finished. If serving fails first, Run returns that
// error without waiting for ctx.
func Run(ctx context.Context, srv *http.Server, ln net.Listener, log *slog.Logger, shutdownTimeout time.Duration) error {
	// Serve blocks until the server stops, so it runs in its own goroutine.
	// The buffer of 1 lets it send its result even if nobody receives it.
	served := make(chan error, 1)
	go func() { served <- srv.Serve(ln) }()

	select {
	case err := <-served:
		// Serve never returns nil; before Shutdown it's a real failure.
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	log.Info("shutting down", "timeout", shutdownTimeout)
	// ctx is already cancelled, so the deadline is built on a copy without
	// its cancellation, as WithTenant does for its rollback.
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		// Requests still running are cut off when the process exits.
		return fmt.Errorf("shutdown: %w", err)
	}
	// After Shutdown, Serve returns ErrServerClosed at once: a clean stop.
	if err := <-served; !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("serve: %w", err)
	}
	return nil
}
