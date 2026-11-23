package main

import (
	"log/slog"
	"testing"
)

// env returns a getenv that reads from m instead of the process environment,
// so each case sees exactly the variables it names.
func env(m map[string]string) func(string) string {
	return func(key string) string { return m[key] }
}

func TestLoadServeConfig(t *testing.T) {
	const url = "postgres://cowbook@localhost:5432/cowbook"
	tests := []struct {
		name    string
		env     map[string]string
		want    serveConfig
		wantErr bool
	}{
		{
			name: "defaults",
			env:  map[string]string{"DATABASE_URL": url},
			want: serveConfig{DatabaseURL: url, Addr: "localhost:8080", LogLevel: slog.LevelInfo},
		},
		{
			name: "all set",
			env: map[string]string{
				"DATABASE_URL":      url,
				"COWBOOK_ADDR":      ":9090",
				"COWBOOK_LOG_LEVEL": "debug",
			},
			want: serveConfig{DatabaseURL: url, Addr: ":9090", LogLevel: slog.LevelDebug},
		},
		{
			name: "level in capitals",
			env:  map[string]string{"DATABASE_URL": url, "COWBOOK_LOG_LEVEL": "WARN"},
			want: serveConfig{DatabaseURL: url, Addr: "localhost:8080", LogLevel: slog.LevelWarn},
		},
		{
			name:    "no database",
			env:     map[string]string{},
			wantErr: true,
		},
		{
			name:    "unknown level",
			env:     map[string]string{"DATABASE_URL": url, "COWBOOK_LOG_LEVEL": "loud"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := loadServeConfig(env(tt.env))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("loadServeConfig() = %+v, nil; want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("loadServeConfig() error = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("loadServeConfig() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
