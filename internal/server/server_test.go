package server_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"testing"
	"time"

	"nicoaravena/cow-book-app/internal/server"
)

// discard is a logger for tests that don't look at the log.
var discard = slog.New(slog.DiscardHandler)

// listen opens a listener on a free port, chosen by the OS because the port
// is 0, so tests never collide with each other or a running cowbook serve.
func listen(t *testing.T) net.Listener {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return ln
}

// start runs server.Run on h in a goroutine and returns the server's URL, the
// function that stands for Ctrl-C, and the channel Run's result arrives on.
func start(t *testing.T, h http.Handler, shutdownTimeout time.Duration) (string, context.CancelFunc, <-chan error) {
	t.Helper()
	ln := listen(t)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	done := make(chan error, 1)
	go func() {
		done <- server.Run(ctx, server.New(h, discard), ln, discard, shutdownTimeout)
	}()
	return "http://" + ln.Addr().String(), cancel, done
}

// wait returns Run's result, or fails the test if it takes too long.
func wait(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return")
		return nil
	}
}

// blocking returns a handler that signals entered when a request arrives and
// answers only once release is closed.
func blocking() (h http.Handler, entered chan struct{}, release chan struct{}) {
	entered = make(chan struct{})
	release = make(chan struct{})
	h = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(entered)
		<-release
		io.WriteString(w, "done")
	})
	return h, entered, release
}

// A request in flight when the signal comes still gets its answer, and Run
// then returns nil: a clean exit.
func TestRunFinishesRequestsInFlight(t *testing.T) {
	h, entered, release := blocking()
	url, stop, done := start(t, h, 5*time.Second)

	type result struct {
		status int
		body   string
		err    error
	}
	got := make(chan result, 1)
	go func() {
		resp, err := http.Get(url)
		if err != nil {
			got <- result{err: err}
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		got <- result{resp.StatusCode, string(body), err}
	}()

	<-entered
	stop()
	// Shutdown has closed the listener but waits for the request.
	select {
	case err := <-done:
		t.Fatalf("Run returned %v while a request was in flight", err)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	r := <-got
	if r.err != nil || r.status != http.StatusOK || r.body != "done" {
		t.Errorf("request = %d %q, %v; want 200 \"done\", nil", r.status, r.body, r.err)
	}
	if err := wait(t, done); err != nil {
		t.Errorf("Run() = %v, want nil", err)
	}
}

// A request that outlasts the shutdown timeout makes Run give up and say so.
func TestRunShutdownTimeout(t *testing.T) {
	h, entered, release := blocking()
	defer close(release)
	url, stop, done := start(t, h, 50*time.Millisecond)

	go func() {
		if resp, err := http.Get(url); err == nil {
			resp.Body.Close()
		}
	}()

	<-entered
	stop()
	err := wait(t, done)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Run() = %v, want an error wrapping context.DeadlineExceeded", err)
	}
}

// If serving fails, Run returns at once with the error instead of waiting for
// a signal that may never come.
func TestRunServeFails(t *testing.T) {
	ln := listen(t)
	ln.Close()

	done := make(chan error, 1)
	go func() {
		done <- server.Run(context.Background(), server.New(http.NotFoundHandler(), discard), ln, discard, time.Second)
	}()
	err := wait(t, done)
	if err == nil || errors.Is(err, http.ErrServerClosed) {
		t.Errorf("Run() = %v, want the listener's error", err)
	}
}

// New sets every timeout, so a slow or idle client can't hold a connection
// open forever.
func TestNewSetsTimeouts(t *testing.T) {
	srv := server.New(http.NotFoundHandler(), discard)
	for name, d := range map[string]time.Duration{
		"ReadHeaderTimeout": srv.ReadHeaderTimeout,
		"ReadTimeout":       srv.ReadTimeout,
		"WriteTimeout":      srv.WriteTimeout,
		"IdleTimeout":       srv.IdleTimeout,
	} {
		if d <= 0 {
			t.Errorf("%s = %v, want > 0", name, d)
		}
	}
	if srv.ErrorLog == nil {
		t.Error("ErrorLog is nil, want the server's own errors sent to slog")
	}
}
