# go-pgpool

Tuned pgx v5 connection pools (package `pgpool`): `Connect(ctx, ConnectionString)` opens a pool for PostgreSQL/CockroachDB — an empty string defers to the libpq `PG*` env vars, `~/.pgpass`, and `~/.pg_service.conf` — and `HealthChecker` adapts a pool to the common health-check shape (`Name()`/`Check(ctx)`). Generic — lives in `gomatic`; extracted from xto-email's `go-config` during the xto repo split (see `xto-email/_projects/specs/repo-split/`). Pairs with `go-app/flags/pg` (flag-bound Config whose `ConnString()` feeds `Connect`).

- Library repo (`library.go` marker); flat single-package layout at the root; deps: pgx v5, `gomatic/go-error` (sentinels).
- Gate: shared Makefile from `nicerobot/tools.repository` — gofumpt, vet, staticcheck, golangci-lint, govulncheck, gocognit ≤ 7, 100% coverage. Never edit the distributed `Makefile`/`.golangci.yaml`/`.github` in-tree.
- Public docs live in `docs.go-pgpool`; the README is exactly badges + the docs link.
