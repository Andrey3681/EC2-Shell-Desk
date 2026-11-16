# Contributing to EC2 Shell Desk

Thank you for contributing to **EC2 Shell Desk**. Please follow these engineering, security, and documentation standards.

## 1. Development Environment Setup

1. Run `task doctor` on Windows PowerShell to verify Go, Wails CLI, Node.js, WebView2, and linters.
2. Run `task setup` to install Go tools (`wails`, `gofumpt`, `golangci-lint`, `gomarkdoc`) and frontend dependencies.
3. Optional: install local Git hooks with `lefthook install`.

## 2. Branching & Pull Request Workflow

- `main`: Always releasable and passing `task check`.
- Feature/fix branches: `<type>/<short-kebab-description>` (e.g., `feat/vault-pem-import`, `fix/sftp-cancel-leak`).
- Before opening a PR, run `task check` and ensure all tests pass with `-race`.

## 3. Coding & Language Rules

- **Strict English Codebase:** All Go/TypeScript identifiers, struct fields, database columns, inline comments, and Go doc comments MUST be written in **English**.
- **Hexagonal Boundaries:** `internal/core/` must remain dependency-free from SQLite, SSH/SFTP libraries, Wails, or UI frameworks.
- **No Sensitive Logging:** Never log private keys, passphrases, passwords, or decrypted secrets.
- **Go Docstrings:** Every exported type, function, method, and constant must begin its comment with its exact symbol name (`// HostInstance represents...`) so `gomarkdoc` generates clean API docs.

## 4. Commit Message Standard (Conventional Commits 1.0.0)

Commit messages MUST be in **English** and **imperative mood**:

```text
<type>(<scope>): <short imperative summary>

[optional body explaining why]

[optional footer: Refs #id | BREAKING CHANGE: ...]
```

- **Allowed types:** `feat`, `fix`, `docs`, `refactor`, `test`, `perf`, `style`, `build`, `ci`, `chore`, `revert`
- **Recommended scopes:** `auth`, `vault`, `hosts`, `ssh`, `sftp`, `terminal`, `ui`, `sqlite`, `crypto`, `windows`, `docs`, `ci`, `deps`
- **Valid examples:**
  - `feat(auth): bootstrap initial local user on first launch`
  - `feat(vault): encrypt imported PEM keys with AES-256-GCM`
  - `fix(sftp): cancel active transfer goroutine when tab closes`
  - `docs(security): document Argon2id key derivation parameters`

## 5. Documentation & Changelog

- Update `README.md`, `docs/`, and `CHANGELOG.md` (`[Unreleased]`) whenever features, security flows, schemas, or build commands change.
- Run `task doc` to regenerate `docs/api/` and PlantUML diagrams when exported Go APIs or diagrams change.
