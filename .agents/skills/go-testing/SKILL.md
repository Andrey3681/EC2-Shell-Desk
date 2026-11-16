---
name: go-testing
description: TDD workflow, table-driven Go tests, in-memory port fakes, and embedded SSH/SFTP test servers.
---

# Go Testing Skill

Use this skill when writing unit or integration tests in Go (`*_test.go`).

## Guidelines
1. **TDD & Table-Driven Tests:**
   - Use `tests := []struct{ name string; ... }{ ... }` with `t.Run(tc.name, func(t *testing.T) { ... })`.
   - Test happy paths, boundary cases, invalid inputs, context cancellation, and cross-user isolation (`userA` cannot access `userB` resources).
2. **In-Memory Fakes for Ports:**
   - Test `internal/services/` using in-memory fakes implementing `ports.UserRepository`, `ports.KeyRepository`, `ports.HostRepository`, `ports.SSHSessionManager`, and `ports.SFTPClient`.
3. **No Real Cloud Dependency:**
   - Never require live AWS EC2 instances in automated tests.
   - For `sshclient` and `sftpclient` adapter tests, spin up an in-memory Go SSH/SFTP test server on `127.0.0.1:0` with ephemeral test keys generated inside the test or loaded from `testdata/`.
4. **Security Assertions:**
   - Assert that encrypted `.pem` blobs stored in SQLite do not contain plaintext PEM headers (`BEGIN ... PRIVATE KEY`).
   - Assert that log output buffers never contain passphrases, passwords, or private key bytes.
5. **Verification Command:** Run `go test ./... -race -cover -count=1`.
