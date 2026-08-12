# Learnings

## 2026-08-12 — Scope route directives by site kind

- A shared Caddy route writer must explicitly constrain feature-specific directives to the site kinds that support them, even when the UI normally leaves unsupported fields empty. API callers can bypass UI assumptions.
- Preserve legacy behavior when adding a kind guard: this project treats an empty site kind as a proxy site in some existing data.
- New form copy and validation messages should use locale IDs from the first implementation, and feature changes should avoid unrelated formatter churn in large modal files.
