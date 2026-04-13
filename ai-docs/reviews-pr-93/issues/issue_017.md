# Issue 017

- Status: RESOLVED
- Disposition: VALID
- File: `internal/setup/reusable_agents_test_helpers_test.go`
- Review request: build reusable-agent fixtures under `root/agents/<name>` to match the declared `agents/*` pattern.
- Validation: the helper created fixtures directly under `root/<name>`.
- Action taken: the helper now writes fixtures under `root/agents/<name>`.
