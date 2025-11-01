# Compozy SDK Codegen

This package relies on generated helpers for functional options, resource loading, registration, and client-backed execution. To regenerate the Go sources run:

```bash
cd sdk/compozy
go generate
```

The generator lives under `sdk/internal/sdkcodegen` and produces:

- `options_generated.go`
- `engine_execution.go`
- `engine_loading.go`
- `engine_registration.go`

Generated files are formatted and deterministic; rerunning `go generate` should not introduce diffs if the source spec has not changed.
