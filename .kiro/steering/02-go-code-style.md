---
inclusion: fileMatch
---

# Go code style

Full source: [`.github/copilot-instructions.md`](../../.github/copilot-instructions.md#code-style)

## Error handling

Always wrap errors with context via `fmt.Errorf("operation: %w", err)`.
Never discard or swallow errors silently unless documented.

Non-fatal errors (shadow writes, log publishes) are logged with `.Warn()`
and include a `(non-fatal)` note. Fatal startup errors use `log.Fatal()`.
Operational errors use `log.Error()`. Expected degraded states use
`log.Warn()`. Normal flow uses `log.Info()` or `log.Debug()`.

## Logging

Use `zerolog` structured logging throughout with relevant key-value fields,
e.g. `log.Info().Str("journey_id", journeyID)...Msg(...)`. Never use
`fmt.Println` or the standard `log` package in service code.

## Imports

Group imports in three blocks separated by blank lines:
1. Standard library
2. Third-party packages
3. Internal packages (`github.com/journey-system/...` / this module's path)

## Environment variables

All service configuration comes from environment variables with sensible
localhost defaults, via the established `envOrDefault` helper. Document
every environment variable in the package doc comment and in
`docs/architecture.md`.

## Concurrency

Use `context.Context` as the first parameter in every function doing I/O.
Cancel the context on shutdown via `signal.NotifyContext`. Goroutines that
are not trivially short must have a comment explaining their lifetime and
what stops them.
