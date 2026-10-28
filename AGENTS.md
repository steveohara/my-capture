# Agent Instructions

## Mandatory session startup

At the start of every session, before doing anything else, read all of the
following steering files in order. They are the canonical coding standards for
this repository and you must follow them for every task:

- `.kiro/steering/00-copilot-instructions.md`
- `.kiro/steering/01-go-documentation.md`
- `.kiro/steering/02-go-code-style.md`
- `.kiro/steering/03-go-testing.md`
- `.kiro/steering/08-general-rules.md`

## Commits
Never commit, amend, push, or create PRs unless explicitly asked by the user.
This applies to all agents and subagents, including those dispatched by skills.

## Output style
- Return only final code. Do not narrate tool calls or show intermediate steps.
- Do not explain what you are about to do before doing it.
- Do not show before/after comparisons unless explicitly asked.
- Do not summarise what was changed after completing a task.
