---
inclusion: always
---

## Documentation Requirements

Documentation is **mandatory** in this codebase. Every file, every exported symbol, and every non-trivial unexported symbol must have a comment. These are not optional - the codebase is a teaching reference.

The documentation must be clear, concise, and informative. It should explain the purpose of the code, how it works, and any important details that a reader would need to understand it. Use Go's standard doc comment format (`// ...`) and avoid using em-dashes or other non-standard punctuation.

The command line help, SKILL.md and README.md must always be consistent with the code and with each other. If you change the name of a command, flag, or environment variable in the code, update the documentation in all three places to match.

Never use the em-dash character "-".

### Package (file-level) comments

Every `.go` file must begin with a `// Package ...` doc comment before the `package` declaration.

The package comment must explain:
1. **What** this package/service does in one sentence
2. **Why** it exists - its role in the overall architecture
3. **How** it fits in - what it connects to, what protocol/pattern it uses
4. Any non-obvious design decisions or trade-offs

Use Go's standard `//` doc comment format. For multi-section package docs, use `#` headings inside the comment block:

```go
// Package client provides the HTTP client used by all jcli commands to
// communicate with the Jira REST API v2.
//
// WHY A SEPARATE PACKAGE?
// Keeping the HTTP layer separate from the command layer means commands can
// be tested without making real network calls, and the client can be reused
// across commands without duplicating request logic.
//   - All API calls go through the single Client type, which holds the base
//     URL, credentials, and a shared http.Client.
//   - Error responses are decoded into a structured JiraError type so callers
//     get actionable messages rather than raw status codes.
package client
```

### Function and method comments

Every exported function, method, type, constant, and variable must have a doc comment starting with the symbol name:

```go
// GetIssue fetches a single issue by key from the Jira REST API.
// The fields returned depend on the Jira instance configuration; use
// --output json to inspect the full response.
func (c *Client) GetIssue(ctx context.Context, key string) (*Issue, error) {
```

For unexported functions, add a comment when the logic is non-obvious, the function has important preconditions, or it plays a significant architectural role:

```go
// buildJQL constructs a JQL query string from the supplied filter options.
// An empty filter returns an empty string, which Jira treats as "all issues".
func buildJQL(f IssueFilter) string {
```

Short unexported helpers (< 5 lines, name is self-explanatory) do not need a comment:

```go
func boolPtr(b bool) *bool { return &b }
```

### Struct and type comments

Every exported struct must explain its purpose, its concurrency safety (if relevant), and any important invariants:

```go
// Client is the HTTP client used to communicate with the Jira REST API.
// It holds the base URL, credentials, and a shared http.Client instance.
//
// Client is safe for concurrent use. Create one instance at startup and
// share it across all commands.
type Client struct {
```

Every exported struct field must have an inline comment explaining what it stores, what units it uses, and any constraints:

```go
// Issue represents a Jira issue as returned by GET /rest/api/2/issue/{key}.
type Issue struct {
    // Key is the human-readable issue identifier, e.g. "PROJ-123".
    Key string `json:"key"`

    // Fields contains all field values for the issue. The exact set of fields
    // depends on the Jira project configuration and the fields query parameter.
    Fields IssueFields `json:"fields"`
}
```

### Constants and variables

Group related constants with a `const (...)` block. Comment the block and each constant:

```go
const (
    // apiBase is the path prefix for all Jira REST API v2 endpoints.
    // Prepended to every request path inside Client methods.
    apiBase = "/rest/api/2"

    // agileBase is the path prefix for Jira Agile (board/sprint) endpoints.
    agileBase = "/rest/agile/1.0"
)
```

Sentinel errors must have comments explaining when they are returned and what the caller should do:

```go
// ErrNotFound is returned when the Jira API responds with HTTP 404.
// The caller should check whether the issue key or project key is correct.
var ErrNotFound = errors.New("resource not found")
```

### Inline comments

Use inline comments to explain **why**, not **what**. The code already says what it does.

```go
// Good: explains the reason for the special case
if status == 204 {
    return nil // 204 No Content is a success with no body to decode
}

// Bad: restates what the code already says
if status == 204 {
    return nil // return nil
}
```

Use section dividers with `// -----------------------------------------------------------------------` to break long files into logical sections. This is the established convention in this codebase:

```go
// -----------------------------------------------------------------------
// Issues -- /rest/api/2/issue
// -----------------------------------------------------------------------

func (c *Client) GetIssue(...) { ... }

// -----------------------------------------------------------------------
// Projects -- /rest/api/2/project
// -----------------------------------------------------------------------

func (c *Client) GetProject(...) { ... }
```

---

## Code Style

### Error handling

Always wrap errors with context. Use `fmt.Errorf("operation: %w", err)` - never discard or swallow errors silently unless documented:

```go
// Good
if err := c.get(ctx, path, &issue); err != nil {
    return fmt.Errorf("get issue %s: %w", key, err)
}

// Bad - no context
if err := c.get(ctx, path, &issue); err != nil {
    return err
}
```

### Logging

`jcli` is a CLI tool, not a server. Do not use a logging framework. Write user-facing messages directly to `os.Stderr` using `fmt.Fprintf`. Reserve `os.Stdout` for command output so it can be piped cleanly.

```go
fmt.Fprintf(os.Stderr, "warning: field %q not found in edit metadata\n", fieldID)
```

Never use the standard `log` package in command or client code.

### Imports

Group imports in three blocks separated by blank lines:
1. Standard library
2. Third-party packages
3. Internal packages (`github.com/vonage-technology/jcli/...`)

```go
import (
    "context"
    "fmt"
    "net/http"

    "github.com/spf13/cobra"

    "github.com/vonage-technology/jcli/internal/client"
    "github.com/vonage-technology/jcli/internal/output"
)
```

### Configuration

All user configuration comes from `~/.config/jcli/config.properties`. Use the `cmd.NewClient()` helper to obtain a configured client and resolved output format in every command `RunE` function. Do not read config values directly in leaf commands.

```go
RunE: func(c *cobra.Command, args []string) error {
    cl, cfg := cmd.NewClient()
    ...
}
```

### Concurrency

Use `context.Context` as the first parameter in every function that does I/O. Pass the context received from the Cobra `RunE` function (via `context.Background()`) through to all client calls.

---

## Testing

### Unit tests

Place unit tests alongside the code they test (`foo_test.go` next to `foo.go`). Use table-driven tests for functions with multiple input/output cases. Test names should read as sentences: `TestBuildJQL_FiltersByProjectAndAssignee`.

### Integration / e2e tests

End-to-end tests that make real HTTP calls to a Jira instance must have `//go:build e2e` as the first line and live in `tests/e2e/`. They are excluded from the default `go test ./...` run.

---

## Architecture Rules

These rules encode the key invariants of the system. Never violate them without updating the relevant documentation.

1. **Secrets never in code.** All credentials come from `~/.config/config.properties` or environment variables. No hardcoded tokens or passwords.
3. **Three-way documentation consistency.** Command help text, README.md, and `.agents/skills/jira/SKILL.md` must always agree. Update all three when adding or changing a command.
4. Never auto-commit or create releases. All changes must be explicitly requested and tagged with a version.

## Steering file map

| File | Covers |
|---|---|
| `01-go-documentation.md` | Go package/function/struct/const doc-comment requirements |
| `02-go-code-style.md` | Go error handling, logging, imports, env vars, concurrency |
| `03-go-testing.md` | Go e2e (`//go:build e2e`) and unit test conventions |
| `08-general-rules.md` | Delete-confirmation UX, dialog implementation, commit policy |

## Non-negotiable project-wide rule

Never use the em-dash character (`-`, U+2014) in code comments, section
dividers, or generated prose in this repository. Use plain hyphens instead.
