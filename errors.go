package pgpool

import errs "github.com/gomatic/go-error"

// Sentinel errors this package emits, matchable with errors.Is. The Const
// mechanism is owned by gomatic/go-error. Keep sorted alphabetically.
const (
	ErrCreateConnectionPool errs.Const = "creating connection pool"
	ErrDatabaseHealthCheck  errs.Const = "database health check failed"
	ErrParseDatabaseConfig  errs.Const = "parsing database config"
	ErrPingDatabase         errs.Const = "pinging database"
)
