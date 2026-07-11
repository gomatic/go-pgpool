// Package pgpool opens tuned pgx v5 connection pools for PostgreSQL and
// CockroachDB and provides the matching health checker. An empty
// ConnectionString defers to the standard libpq environment variables (PGHOST,
// PGPORT, PGUSER, PGPASSWORD, PGDATABASE, PGSSLMODE, ...) plus ~/.pgpass and
// ~/.pg_service.conf, matching psql's behavior.
package pgpool

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ConnectionString is a libpq connection string (URL form or KV form) used to
// configure a database connection pool.
type ConnectionString string

// Pool capacity and lifecycle tuning applied to every connection pool.
const (
	maxConns          int32 = 25
	minConns          int32 = 5
	maxConnLifetime         = 1 * time.Hour
	maxConnIdleTime         = 30 * time.Minute
	healthCheckPeriod       = 30 * time.Second
)

// poolOpener constructs a connection pool from a parsed configuration. It
// matches pgxpool.NewWithConfig so the production wrapper can pass that function
// directly while tests substitute a fake.
type poolOpener func(context.Context, *pgxpool.Config) (*pgxpool.Pool, error)

// poolPinger verifies connectivity of an open pool. The production wrapper
// adapts (*pgxpool.Pool).Ping to it while tests substitute a fake.
type poolPinger func(context.Context, *pgxpool.Pool) error

// Connect creates a pgx v5 connection pool configured for CockroachDB
// or PostgreSQL.
//
// When databaseURL is empty, pgx uses the standard libpq environment
// variables to build the connection: PGHOST, PGPORT, PGUSER, PGPASSWORD,
// PGDATABASE, PGSSLMODE, PGAPPNAME, PGCONNECT_TIMEOUT, etc. It also
// honors ~/.pgpass and ~/.pg_service.conf, matching psql's behavior.
//
// When databaseURL is set, it overrides the env vars (URL form:
// postgres://user:pass@host:port/db?sslmode=disable, or libpq KV form:
// "host=localhost port=5432 user=alice dbname=xto sslmode=disable").
func Connect(ctx context.Context, databaseURL ConnectionString) (*pgxpool.Pool, error) {
	return connect(ctx, databaseURL, pgxpool.NewWithConfig, pingPool)
}

// pingPool adapts the (*pgxpool.Pool).Ping method to the context-first
// poolPinger seam.
func pingPool(ctx context.Context, pool *pgxpool.Pool) error {
	return pool.Ping(ctx)
}

// connect is the dependency-injected core of Connect. The exported wrapper
// supplies the production pool opener and pinger.
func connect(
	ctx context.Context,
	databaseURL ConnectionString,
	open poolOpener,
	ping poolPinger,
) (*pgxpool.Pool, error) {
	cfg, err := buildPoolConfig(databaseURL)
	if err != nil {
		return nil, err
	}

	pool, err := open(ctx, cfg)
	if err != nil {
		return nil, ErrCreateConnectionPool.With(err)
	}

	if err := ping(ctx, pool); err != nil {
		pool.Close()
		return nil, ErrPingDatabase.With(err)
	}

	return pool, nil
}

// buildPoolConfig parses the connection string and applies the pool tuning.
//
// Enabling the prepared statement cache requires no work here: pgx v5 enables
// the statement cache by default on each connection (256-entry LRU in describe
// mode).
func buildPoolConfig(databaseURL ConnectionString) (*pgxpool.Config, error) {
	cfg, err := pgxpool.ParseConfig(string(databaseURL))
	if err != nil {
		return nil, ErrParseDatabaseConfig.With(err)
	}

	cfg.MaxConns = maxConns
	cfg.MinConns = minConns
	cfg.MaxConnLifetime = maxConnLifetime
	cfg.MaxConnIdleTime = maxConnIdleTime
	cfg.HealthCheckPeriod = healthCheckPeriod

	return cfg, nil
}

// HealthChecker implements health checking against a pgxpool.Pool.
// It satisfies the common health-checker interface shape (Name() string,
// Check(ctx context.Context) error).
type HealthChecker struct {
	Pool *pgxpool.Pool
}

// Name returns the identifier for this health check.
func (h HealthChecker) Name() string {
	return "database"
}

// Check pings the database pool to verify connectivity.
func (h HealthChecker) Check(ctx context.Context) error {
	return checkHealth(ctx, h.Pool.Ping)
}

// checkHealth runs a ping and wraps any failure as ErrDatabaseHealthCheck. It
// is the dependency-injected core of Check, taking the ping operation so both
// branches are reachable in tests without a live database.
func checkHealth(ctx context.Context, ping func(context.Context) error) error {
	if err := ping(ctx); err != nil {
		return ErrDatabaseHealthCheck.With(err)
	}
	return nil
}
