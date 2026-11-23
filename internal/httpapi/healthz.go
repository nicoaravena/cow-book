package httpapi

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// pingTimeout bounds the health check, so a database that accepts the
// connection and then hangs gives a 503 instead of a request that never ends.
const pingTimeout = 2 * time.Second

// healthz answers 200 when the database answers a ping, 503 when it doesn't.
// The reason goes to the log only: the endpoint has no login, and a
// connection error can name hosts and ports.
func healthz(db Pinger, log *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Derived from the request's context, so a client that gives up also
		// stops the ping.
		ctx, cancel := context.WithTimeout(r.Context(), pingTimeout)
		defer cancel()

		w.Header().Set("Content-Type", "application/json")
		if err := db.Ping(ctx); err != nil {
			log.Warn("health check failed", "err", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			io.WriteString(w, `{"status":"unavailable"}`+"\n")
			return
		}
		io.WriteString(w, `{"status":"ok"}`+"\n")
	})
}
