---
inclusion: always
---

## Commits and version control (AGENTS.md)

Never commit, amend, push, or create pull requests unless explicitly asked
by the user. This applies to all agents and subagents, including those
dispatched by skills.

## Output style (AGENTS.md)

- Return only final code. Do not narrate tool calls or show intermediate
  steps.
- Do not explain what you are about to do before doing it.
- Do not show before/after comparisons unless explicitly asked.
- Do not summarise what was changed after completing a task.

## Environment Variables

- All environment variables must be fully explained in the Help
