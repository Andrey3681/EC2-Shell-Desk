---
name: conventional-commits
description: Enforces Conventional Commits 1.0.0 in English imperative mood after inspecting staged diffs.
---

# Conventional Commits Skill

Use this skill when preparing Git commits or reviewing commit messages.

## Guidelines
1. Inspect `git status` and `git diff --staged` before crafting the commit message.
2. Format: `<type>(<scope>): <short imperative English summary>` (max ~72 chars for subject, no trailing period).
3. Allowed types: `feat`, `fix`, `docs`, `refactor`, `test`, `perf`, `style`, `build`, `ci`, `chore`, `revert`.
4. Recommended scopes: `auth`, `vault`, `hosts`, `ssh`, `sftp`, `terminal`, `ui`, `sqlite`, `crypto`, `windows`, `docs`, `ci`, `deps`.
5. Local validation runs via `lefthook.yml` -> `scripts/verify-commit-msg.ps1`.
6. Use `BREAKING CHANGE:` footer if an exported binding, schema, or vault format breaks backwards compatibility.
