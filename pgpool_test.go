package pgpool

import (
	"context"
	"errors"
	"testing"
	"time"

	errs "github.com/gomatic/go-error"
	"github.com/jackc/pgx/v5/pgxpool"
)

// errBoom is a stand-in cause used to verify error wrapping.
const errBoom errs.Const = "boom"

// newUnconnectedPool builds a real pgxpool.Pool that never establishes a
// connection (pgx connects lazily), so it is safe to construct and Close in
// unit tests without a live database.
func newUnconnectedPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig("postgres://u:p@127.0.0.1:1/db?sslmode=disable")
	if err != nil {
		t.Fatalf("ParseConfig: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewWithConfig: %v", err)
	}
	return pool
}

func TestBuildPoolConfigParseError(t *testing.T) {
	t.Parallel()

	_, err := buildPoolConfig("://not a valid url")
	if !errors.Is(err, ErrParseDatabaseConfig) {
		t.Fatalf("buildPoolConfig error = %v, want %v", err, ErrParseDatabaseConfig)
	}
}

func TestBuildPoolConfigAppliesTuning(t *testing.T) {
	t.Parallel()

	cfg, err := buildPoolConfig("postgres://u:p@127.0.0.1:5432/db?sslmode=disable")
	if err != nil {
		t.Fatalf("buildPoolConfig unexpected error: %v", err)
	}
	checks := []struct {
		got  any
		want any
		name string
	}{
		{name: "MaxConns", got: cfg.MaxConns, want: maxConns},
		{name: "MinConns", got: cfg.MinConns, want: minConns},
		{name: "MaxConnLifetime", got: cfg.MaxConnLifetime, want: maxConnLifetime},
		{name: "MaxConnIdleTime", got: cfg.MaxConnIdleTime, want: maxConnIdleTime},
		{name: "HealthCheckPeriod", got: cfg.HealthCheckPeriod, want: healthCheckPeriod},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestConnectDBParseError(t *testing.T) {
	t.Parallel()

	failOpen := func(context.Context, *pgxpool.Config) (*pgxpool.Pool, error) {
		t.Fatal("opener must not be called when parsing fails")
		return nil, nil
	}
	okPing := func(context.Context, *pgxpool.Pool) error { return nil }

	_, err := connect(context.Background(), "://bad", failOpen, okPing)
	if !errors.Is(err, ErrParseDatabaseConfig) {
		t.Fatalf("connect error = %v, want %v", err, ErrParseDatabaseConfig)
	}
}

func TestConnectDBOpenError(t *testing.T) {
	t.Parallel()

	open := func(context.Context, *pgxpool.Config) (*pgxpool.Pool, error) {
		return nil, errBoom
	}
	okPing := func(context.Context, *pgxpool.Pool) error { return nil }

	_, err := connect(context.Background(), "postgres://u@127.0.0.1:5432/db", open, okPing)
	if !errors.Is(err, ErrCreateConnectionPool) {
		t.Fatalf("connect error = %v, want %v", err, ErrCreateConnectionPool)
	}
	if !errors.Is(err, errBoom) {
		t.Fatalf("connect error = %v, want wrapped cause %v", err, errBoom)
	}
}

func TestConnectDBPingError(t *testing.T) {
	t.Parallel()

	pool := newUnconnectedPool(t)
	open := func(context.Context, *pgxpool.Config) (*pgxpool.Pool, error) {
		return pool, nil
	}
	failPing := func(context.Context, *pgxpool.Pool) error { return errBoom }

	_, err := connect(context.Background(), "postgres://u@127.0.0.1:5432/db", open, failPing)
	if !errors.Is(err, ErrPingDatabase) {
		t.Fatalf("connect error = %v, want %v", err, ErrPingDatabase)
	}
	if !errors.Is(err, errBoom) {
		t.Fatalf("connect error = %v, want wrapped cause %v", err, errBoom)
	}
}

func TestConnectDBSuccess(t *testing.T) {
	t.Parallel()

	pool := newUnconnectedPool(t)
	t.Cleanup(pool.Close)
	open := func(context.Context, *pgxpool.Config) (*pgxpool.Pool, error) {
		return pool, nil
	}
	okPing := func(context.Context, *pgxpool.Pool) error { return nil }

	got, err := connect(context.Background(), "postgres://u@127.0.0.1:5432/db", open, okPing)
	if err != nil {
		t.Fatalf("connect unexpected error: %v", err)
	}
	if got != pool {
		t.Fatalf("connect returned %p, want %p", got, pool)
	}
}

func TestConnectDBWrapperParseError(t *testing.T) {
	t.Parallel()

	// Exercises the exported Connect wrapper; an invalid URL fails before any
	// connection is attempted.
	_, err := Connect(context.Background(), "://bad")
	if !errors.Is(err, ErrParseDatabaseConfig) {
		t.Fatalf("Connect error = %v, want %v", err, ErrParseDatabaseConfig)
	}
}

func TestPingPoolUnreachable(t *testing.T) {
	t.Parallel()

	// pingPool adapts (*pgxpool.Pool).Ping; a lazily-constructed pool aimed at a
	// closed port makes the ping fail without a live database.
	pool := newUnconnectedPool(t)
	defer pool.Close()
	if err := pingPool(context.Background(), pool); err == nil {
		t.Fatal("pingPool = nil error, want a connection failure")
	}
}

func TestDBHealthCheckerName(t *testing.T) {
	t.Parallel()

	h := HealthChecker{}
	if got := h.Name(); got != "database" {
		t.Fatalf("Name() = %q, want %q", got, "database")
	}
}

func TestCheckHealthSuccess(t *testing.T) {
	t.Parallel()

	err := checkHealth(context.Background(), func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("checkHealth unexpected error: %v", err)
	}
}

func TestCheckHealthFailure(t *testing.T) {
	t.Parallel()

	err := checkHealth(context.Background(), func(context.Context) error { return errBoom })
	if !errors.Is(err, ErrDatabaseHealthCheck) {
		t.Fatalf("checkHealth error = %v, want %v", err, ErrDatabaseHealthCheck)
	}
	if !errors.Is(err, errBoom) {
		t.Fatalf("checkHealth error = %v, want wrapped cause %v", err, errBoom)
	}
}

func TestDBHealthCheckerCheckDelegates(t *testing.T) {
	t.Parallel()

	// HealthChecker.Check delegates to h.Pool.Ping. Against an unconnected
	// pool with a short deadline the ping fails, exercising the wired method
	// value and confirming the error is wrapped as ErrDatabaseHealthCheck.
	pool := newUnconnectedPool(t)
	t.Cleanup(pool.Close)
	h := HealthChecker{Pool: pool}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := h.Check(ctx); !errors.Is(err, ErrDatabaseHealthCheck) {
		t.Fatalf("Check() error = %v, want %v", err, ErrDatabaseHealthCheck)
	}
}
