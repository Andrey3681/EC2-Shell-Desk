# Rule: English Codebase, Documentation & Conventional Commits

- ALL Go and TypeScript identifiers, package names, file names, structs, struct fields, interfaces, functions, methods, variables, constants, SQL table/column names, inline comments (`//`), and doc comments MUST be written in **English**.
- Every exported Go type, function, method, and constant MUST have a Go doc comment starting with its exact name (e.g., `// HostInstance represents...`) to remain compatible with `gomarkdoc`.
- Do not modify or remove existing comments/docstrings unrelated to your changes.
- Follow **Conventional Commits 1.0.0** in English imperative mood (`<type>(<scope>): <short summary>`), and inspect staged diffs before committing.
- Run `task check` (`gofumpt`, `golangci-lint`, `go test`, `gomarkdoc`) before marking implementation tasks complete.
