---
inclusion: fileMatch
---

# Go documentation requirements

Full source: [`.github/copilot-instructions.md`](../../.github/copilot-instructions.md#documentation-requirements)

This is a Go teaching-reference codebase. Documentation is mandatory, not
optional: every file, every exported symbol, and every non-trivial
unexported symbol must have a comment. Never use the em-dash character.

## Package (file-level) comments

Every `.go` file starts with a `// Package ...` doc comment before the
`package` declaration. For `package main` service files, use
`// Package main implements the <ServiceName> - ...`.

Must explain:
1. **What** the package/service does in one sentence.
2. **Why** it exists - its role in the architecture.
3. **How** it fits in - what it connects to, what protocol/pattern it uses.
4. Any non-obvious design decisions or trade-offs.

Use `#` headings inside the comment block for multi-section docs, e.g. a
`WHY X?` section explaining a technology choice.

## Function and method comments

Every exported function, method, type, constant, and variable has a doc
comment starting with the symbol name, e.g.:

```go
// Connect creates a new database connection pool and waits for PostgreSQL
// to accept connections, retrying for up to 30 seconds.
func Connect(ctx context.Context, dsn string) (*Pool, error) {
```

Unexported functions get a comment when the logic is non-obvious, has
important preconditions, or plays a significant architectural role. Short
unexported helpers (< 5 lines, self-explanatory name) do not need one.

## Struct and type comments

Every exported struct explains its purpose, concurrency safety (if
relevant), and important invariants. Every exported struct field has an
inline comment explaining what it stores, units, and constraints.

## Constants and variables

Group related constants in a commented `const (...)` block; comment each
constant. Sentinel errors must explain when they are returned and what the
caller should do.

## Inline comments

Explain **why**, not **what** - the code already says what it does. Use
`// -- Label ---` section dividers (em-dash characters in the divider only,
matching the existing codebase convention) to break long files into logical
sections, e.g.:

```go
// -- Event handlers ----------------------------------------------------

// -- Dispatch ------------------------------------------------------------
```
