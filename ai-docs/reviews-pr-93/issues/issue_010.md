# Issue 010

- Status: RESOLVED
- Disposition: VALID
- File: `internal/cli/extension/install_source_test.go`
- Review request: stop calling `t.Fatalf` from the `httptest` server handler goroutine.
- Validation: the handler called `t.Fatalf` directly on an unexpected request path.
- Action taken: replaced it with a buffered error channel, returned an HTTP error from the handler, and surfaced any failure back on the test goroutine.
