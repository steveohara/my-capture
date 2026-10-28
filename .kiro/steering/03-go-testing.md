---
inclusion: fileMatch
---

# Go testing conventions

Full source: [`.github/copilot-instructions.md`](../../.github/copilot-instructions.md#testing)

## e2e tests

All end-to-end tests must have `//go:build e2e` as the first line and live
in `tests/e2e/`. They are excluded from the default `go test ./...` run and
only execute via `make test-e2e`.

Follow the naming convention `scenario_<what_fails>_test.go`. The file's
package doc comment must explain:
- What failure mode the test exercises.
- What the expected system behaviour is.
- Any setup requirements.

## Unit tests

Place unit tests alongside the code they test (`foo_test.go` next to
`foo.go`). Use table-driven tests for functions with multiple input/output
cases. Test names read as sentences, e.g.
`TestPutIfVersion_ReturnsVersionConflictOnStaleWrite`.

## Integration tests (this repo specifically)

Per the [README](../../README.md#running-tests), Go tests in this repo are
integration tests requiring a live PostgreSQL instance:

```bash
make infra-up
make api-migrate-up
make api-test
```

Each test run creates an isolated organisation UUID so tests do not
interfere with one another (`TEST_DATABASE_URL` controls the target
database). There is no mock database.
