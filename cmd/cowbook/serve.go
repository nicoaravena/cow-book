package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"nicoaravena/cow-book-app/internal/httpapi"
	"nicoaravena/cow-book-app/internal/postgres"
	"nicoaravena/cow-book-app/internal/server"
)

// shutdownTimeout is how long serve waits for requests in flight after a
// signal before it gives up on them.
const shutdownTimeout = 10 * time.Second

// startupPingTimeout bounds the first ping, so a database that never answers
// stops serve instead of hanging it.
const startupPingTimeout = 5 * time.Second

// serveConfig is everything serve reads from the environment.
type serveConfig struct {
	DatabaseURL string     // DATABASE_URL, required
	Addr        string     // COWBOOK_ADDR, default localhost:8080
	LogLevel    slog.Level // COWBOOK_LOG_LEVEL: debug, info, warn or error; default info
}

// loadServeConfig reads the config through getenv, which is os.Getenv in the
// binary and a map in the tests.
func loadServeConfig(getenv func(string) string) (serveConfig, error) {
	cfg := serveConfig{
		DatabaseURL: getenv("DATABASE_URL"),
		Addr:        getenv("COWBOOK_ADDR"),
		LogLevel:    slog.LevelInfo,
	}
	if cfg.DatabaseURL == "" {
		return serveConfig{}, errors.New("DATABASE_URL is not set")
	}
	// Bound to this machine only by default: nothing here is fit for a
	// network until login (M4) and the database roles (5.1).
	if cfg.Addr == "" {
		cfg.Addr = "localhost:8080"
	}
	if s := getenv("COWBOOK_LOG_LEVEL"); s != "" {
		// UnmarshalText accepts debug, info, warn and error in any case.
		if err := cfg.LogLevel.UnmarshalText([]byte(s)); err != nil {
			return serveConfig{}, fmt.Errorf("COWBOOK_LOG_LEVEL: %w", err)
		}
	}
	return cfg, nil
}

// runServe runs the HTTP server until SIGINT (Ctrl-C) or SIGTERM, then shuts
// it down gracefully. It takes no flags yet; parsing an empty set still gives
// it -h and refuses stray arguments.
func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), `usage: cowbook serve

Serves the HTTP API until Ctrl-C or SIGTERM. Configured from the environment:
  DATABASE_URL       Postgres connection string (required)
  COWBOOK_ADDR       address to listen on (default localhost:8080)
  COWBOOK_LOG_LEVEL  debug, info, warn or error (default info)`)
	}
	if err := fs.Parse(args); errors.Is(err, flag.ErrHelp) {
		return nil
	} else if err != nil {
		return errUsage
	}
	if fs.NArg() > 0 {
		return usageError(fs, "unexpected argument %q", fs.Arg(0))
	}

	cfg, err := loadServeConfig(os.Getenv)
	if err != nil {
		return err
	}
	// Logs go to stderr, as text; stdout stays free for a command's output.
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: cfg.LogLevel}))

	// ctx ends on the first SIGINT or SIGTERM. AfterFunc then calls stop,
	// which hands the signals back to Go's default handling: a second Ctrl-C
	// kills the process even while it waits for requests to finish.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	context.AfterFunc(ctx, stop)

	pool, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	// Deferred calls run last in, first out, but this one is only reached
	// after server.Run has returned, so the pool closes once no request can
	// use it.
	defer pool.Close()

	// The pool connects lazily; one ping now turns a wrong URL or a stopped
	// database into an exit at startup rather than 503s later.
	pingCtx, cancel := context.WithTimeout(ctx, startupPingTimeout)
	err = pool.Ping(pingCtx)
	cancel()
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}

	ln, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		return err
	}
	// ln.Addr is the real address, with the port the OS picked for :0.
	log.Info("listening", "addr", ln.Addr().String())

	srv := server.New(httpapi.NewHandler(pool, log), log)
	if err := server.Run(ctx, srv, ln, log, shutdownTimeout); err != nil {
		return err
	}
	log.Info("stopped")
	return nil
}
