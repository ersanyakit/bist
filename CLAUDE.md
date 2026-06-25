# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`hissebot` (Go 1.26 module) is a BIST (Borsa İstanbul / Turkish stock exchange) data-ingestion, financial/technical-analysis, and reporting engine. ~165k LOC, 978 Go files. README and most docs are in Turkish.

## Commands

```bash
go build ./...                 # build everything (a cgo sqlite const-qualifier warning is harmless/expected)
go test ./...                  # all tests (128 test files)
go test -race ./internal/...   # run when touching concurrency
go test ./internal/quant/...   # run one package
go test -run TestName ./internal/quant/...   # run a single test

go run ./cmd/hissebot help     # CLI entrypoint; all subcommands live here
```

Typical end-to-end for one ticker:
```bash
go run ./cmd/hissebot sync all-data
go run ./cmd/hissebot financials run -force      # import + calculate ratios
go run ./cmd/hissebot analyze -symbol ASELS -provider bistdb -timeframes 1D,1W,1M
go run ./cmd/hissebot serve api                  # Fiber v3 HTTP API (GET /healthz, /sectors, ...)
```

Codegen (indicator/pattern catalogs are generated, not hand-edited):
```bash
go run ./tools/indicator_catalog_gen
go run ./tools/pattern_catalog_gen
```

There is **no** Makefile, golangci-lint config, or test CI (the only workflow is `.github/workflows/kap-attachment-worker.yml`). `cgo` is required (C compiler) because of `mattn/go-sqlite3`.

## Architecture (the big picture)

The intended layer direction is **entry → infra → compute → domain/core**:

- `cmd/hissebot/main.go` — 1088-line flat subcommand router: `run()` switches on `args[0]`, each command parses its own `flag.FlagSet`. Also hosts HTTP servers (`api_server.go`, `report_server.go`). No central DI container — match this pattern when adding commands.
- `internal/services` (34 files) — external data fetchers (tradingview, kap, tcmb, tuik, mkk, news, pricequality). These call `internal/storage` to persist.
- `internal/ta` — **the gravity well: 66% of the codebase, 722 files, 30 subpackages** (analysis, indicators, patterns, formations, forecast, professional reports). `internal/quant` holds portfolio/rates/volatility math; `internal/analysis` is a thin parallel scoring facade.
- `internal/domain` — pure DDD aggregates (marketdata, financials, stocks, disclosures, macro, documents). Imports **no** infrastructure; keep it that way.
- `pkg/mathutil` — shared core (Max/Min/Clamp/SafeDiv), highest fan-in.

### Persistence is file-based, not a database

There is **no PostgreSQL at runtime.** Each stock's data lives in its own folder under `data/equities/{TICKER}/*.json`, read/written only through `internal/storage` `EquityStore` (`storage.NewEquityStore(root)`). The `migrations/*.sql` files are a **reference/conceptual schema only** — nothing connects to them. Do not add `database/sql` or an ORM. `data/` is gitignored.

KAP disclosure documents have a central cache under `data/equities/_kap/` (manifest with sha256, document registry with `version`+`latest` flags, failures file). Ingestion must be idempotent (safe to re-run). Every domain data structure carries a `SourceMeta` (`source`, `source_url`, `as_of_date`, `data_version`) for traceability — preserve this on new types.

## Known architectural debt (don't amplify it)

These are documented issues, not patterns to copy:

- **`internal/ta` imports upward into infrastructure** (`ta → services` 57 calls, `→ datasources` 32, `→ kapingest` 19, e.g. `ta/analysis/engine.go:15-16`). Don't add new direct `ta → services/*` dependencies; inject a small interface instead.
- **Repository ports (`internal/repositories/contracts.go`) are defined but bypassed** — production binds to concrete `internal/storage`.
- God files: `ta/storage/professional_report.go` (9,549 lines; `reportLabel` is 719 lines). Prefer data maps / codegen over giant switch tables.
- Decision math is untested: `internal/analysis/*/scoring.go` and `internal/confidence/score.go` (the `0.75` review gate) — add tests when changing these.

Full review with file:line evidence: `docs/review-2026-06-25.md`.

## Conventions

- Wrap errors as `fmt.Errorf("context: %w", err)`. No `panic`/`log.Fatal` in `internal/` library code (`log.Fatalf` only in `main()`).
- Thread the caller's `context.Context`; do not start a fresh `context.Background()` in hot paths (existing offenders: `engine.go:2566`, `ta/storage/writer.go:315`).
- Worker-pool reference implementation: `internal/services/financials/fetch.go:155-185`.
- All config comes from env via `internal/config/config.go` `Load()` (a flat 51-field struct); add new settings there. Secrets (KAP token, MKK/IsYatırım cookies, MQTT password) come from env, never hardcoded.
- Name financial/score thresholds as `const` rather than bare literals.

## Further docs

- `docs/index.md` — generated documentation hub (overview, architecture with a Mermaid dependency graph, source tree, API/command contracts, data models, dev/deploy guides). All in Turkish.
- `_bmad-output/project-context.md` — condensed AI implementation rules (Turkish).
- `README.md` — full command list and the `data/equities/{TICKER}/` file layout.
- `docs/ai_access_guide.md` — how AI agents access repo data via file/HTTP/MCP.
