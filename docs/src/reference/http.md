# HTTP API

`cowbook serve` runs the HTTP server, configured from the environment, until Ctrl-C or SIGTERM.
Every M3 step adds routes here. See [3.1](../concepts/3.1-serve.md) for how it's built.

```sh
make serve                     # go run ./cmd/cowbook serve, with .env loaded
go run ./cmd/cowbook serve -h  # lists the variables below
```

## Configuration

| Variable | Default | Means |
| --- | --- | --- |
| `DATABASE_URL` | none, required | Postgres connection string. `serve` pings it at startup and exits if it can't connect |
| `COWBOOK_ADDR` | `localhost:8080` | Address to listen on. `localhost` keeps it off the network; port `0` picks a free one, shown in the `listening` log line |
| `COWBOOK_LOG_LEVEL` | `info` | `debug`, `info`, `warn` or `error`, in any case |

Fixed in code: request timeouts (headers 5 s, request 15 s, response 15 s, idle connection
60 s) and the shutdown timeout (10 s). Logs are `log/slog` text on stderr.

| Exit code | Means |
| --- | --- |
| `0` | Stopped by a signal after a clean shutdown, or `-h` |
| `1` | Bad or missing configuration, Postgres unreachable at startup, the address taken, or requests still running after the shutdown timeout |
| `2` | A stray argument |
| `130`, `143` | Killed by a second SIGINT or SIGTERM during the shutdown |

## Endpoints

| Method and path | Answers | Since |
| --- | --- | --- |
| `GET /healthz` | `200 {"status":"ok"}` when Postgres answers a ping within 2 s, else `503 {"status":"unavailable"}`; the reason goes to the log as a `WARN` line | 3.1 |

Every route names its method: another method on a known path gets `405` with an `Allow` header,
an unknown path `404`. Responses are `application/json`.
