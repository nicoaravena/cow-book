package httpapi_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nicoaravena/cow-book-app/internal/httpapi"
)

// fakePinger stands in for the pool. It returns err and remembers whether the
// context it was given had a deadline.
type fakePinger struct {
	err         error
	hadDeadline bool
}

func (p *fakePinger) Ping(ctx context.Context) error {
	_, p.hadDeadline = ctx.Deadline()
	return p.err
}

// serve sends one request to a handler built on db and returns the response
// and everything logged while it ran.
func serve(t *testing.T, db httpapi.Pinger, method, path string) (*httptest.ResponseRecorder, string) {
	t.Helper()
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	h := httpapi.NewHandler(db, log)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec, logs.String()
}

func TestHealthzOK(t *testing.T) {
	db := &fakePinger{}
	rec, _ := serve(t, db, http.MethodGet, "/healthz")

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Body.String(), `{"status":"ok"}`+"\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	if got, want := rec.Header().Get("Content-Type"), "application/json"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	if !db.hadDeadline {
		t.Error("Ping got a context without a deadline; a stuck database would hang the check")
	}
}

func TestHealthzDatabaseDown(t *testing.T) {
	db := &fakePinger{err: errors.New("dial tcp 127.0.0.1:5432: connection refused")}
	rec, logs := serve(t, db, http.MethodGet, "/healthz")

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if got, want := rec.Body.String(), `{"status":"unavailable"}`+"\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	// The reason goes to the log, never to whoever is asking.
	if strings.Contains(rec.Body.String(), "refused") {
		t.Errorf("body leaks the error: %q", rec.Body.String())
	}
	if !strings.Contains(logs, "level=WARN") || !strings.Contains(logs, "connection refused") {
		t.Errorf("log = %q, want a WARN line with the error", logs)
	}
}

func TestHealthzMethods(t *testing.T) {
	tests := []struct {
		method string
		want   int
	}{
		{http.MethodGet, http.StatusOK},
		// A GET pattern also answers HEAD.
		{http.MethodHead, http.StatusOK},
		{http.MethodPost, http.StatusMethodNotAllowed},
		{http.MethodDelete, http.StatusMethodNotAllowed},
	}
	for _, tt := range tests {
		t.Run(tt.method, func(t *testing.T) {
			rec, _ := serve(t, &fakePinger{}, tt.method, "/healthz")
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestUnknownPath(t *testing.T) {
	rec, _ := serve(t, &fakePinger{}, http.MethodGet, "/nope")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
