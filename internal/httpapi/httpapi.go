// Package httpapi is the JSON API: it turns HTTP requests into calls on the
// services and their results into responses. It holds no business rules.
//
// In 3.1 it has one route, GET /healthz. 3.3 brings chi and the tenant routes
// under /api/o/{slug}/.
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
)

// Pinger is what the health check needs from the database: one round trip.
// *pgxpool.Pool meets it without naming it, and tests pass a fake.
type Pinger interface {
	Ping(ctx context.Context) error
}

// NewHandler returns the API's routes. db is checked by /healthz; log
// receives what a client must not see, such as why the database is down.
//
// The patterns name a method (Go 1.22), so the mux itself answers a wrong
// method with 405 and an unknown path with 404.
func NewHandler(db Pinger, log *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", healthz(db, log))
	return mux
}
