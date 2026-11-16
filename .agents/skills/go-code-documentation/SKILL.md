---
name: go-code-documentation
description: Enforces English Go docstrings compatible with gomarkdoc and generates docs/api/ Markdown documentation.
---

# Go Code Documentation Skill

Use this skill whenever adding or modifying Go packages, exported types, functions, methods, or constants.

## Guidelines
1. **Strict English Docstrings:** Every package needs a `// Package <name> ...` comment (usually in `doc.go`), and every exported symbol MUST have a doc comment starting with its exact name:
   ```go
   // HostInstance represents a remote virtual machine or EC2 instance owned by a local user.
   type HostInstance struct { ... }
   ```
2. **Describe Behavior & Invariants:** Explain what the symbol does, concurrency expectations (if any), and ownership/error semantics without restating trivial field types.
3. **Generate API Docs:** Run `gomarkdoc -o "docs/api/{{.Dir}}.md" ./internal/...` (or `task doc`) to keep `docs/api/` synchronized.
4. **Lint Check:** `revive` (`exported` rule in `.golangci.yml`) enforces exported doc comments automatically.
