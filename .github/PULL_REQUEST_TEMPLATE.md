## Summary
<!-- Briefly describe what this PR changes and why. -->

## Related Issue
<!-- Closes # -->

## Checklist
- [ ] Code identifiers, struct fields, SQL columns, and Go docstrings/comments are in **English**
- [ ] Hexagonal boundaries respected (`internal/core/` has no adapter/Wails/UI imports)
- [ ] Strict `owner_user_id` isolation preserved on all database queries and service actions
- [ ] No private keys, passphrases, passwords, or decrypted secrets logged or committed
- [ ] `task test` and `task doc` executed cleanly
- [ ] `CHANGELOG.md` and `docs/` updated if applicable
