# GoMock/mockgen Adoption Plan (go:generate)

This plan standardizes mocks generation using `//go:generate` and GoMock's `mockgen`.

References

- GoMock library: /uber-go/mock
- mockgen CLI: github.com/golang/mock/mockgen

## Conventions

- Add `//go:generate` near each interface declaration file.
- Destination under a sibling `mocks/` folder with `package mocks`.
- Prefer `-source $GOFILE` for stability; use explicit file when needed.
- Run via `go generate ./...` or `go generate <pkg>`.

Optional setup for reproducibility (Go 1.24+): pin mockgen as a tool.

```go
// tools.go (root or /internal/tools)
//go:build tools

package tools

import (
	_ "github.com/golang/mock/mockgen"
)
```

Then `go mod tidy` to record the tool dependency. Alternatively, install locally:

```bash
go install github.com/golang/mock/mockgen@latest
```

---

## engine/task2

Interfaces

- `engine/task2/shared/response_handler.go`: `OutputTransformer`
- `engine/task2/shared/interfaces.go`: `ParentStatusManager`
- `engine/task2/shared/state_repository.go`: `StateRepository`
- `engine/task2/contracts/normalizer.go`: `TaskNormalizer`
- `engine/task2/contracts/factory.go`: `NormalizerFactory`

Add to interface files:

```go
//go:generate mockgen -source $GOFILE -destination mocks/mock_output_transformer.go -package mocks
```

```go
//go:generate mockgen -source $GOFILE -destination mocks/mock_parent_status_manager.go -package mocks
```

```go
//go:generate mockgen -source $GOFILE -destination mocks/mock_state_repository.go -package mocks
```

```go
//go:generate mockgen -source $GOFILE -destination mocks/mock_task_normalizer.go -package mocks
```

```go
//go:generate mockgen -source $GOFILE -destination mocks/mock_normalizer_factory.go -package mocks
```

Notes

- Replace manual Testify mocks under `test/integration/task2/**` with generated ones from `engine/task2/.../mocks`.

---

## engine/workflow/schedule

Interfaces

- `engine/workflow/schedule/manager.go`: `Manager`
- Any interfaces in `engine/workflow/schedule/router/*.go`

Add to each file with interfaces:

```go
//go:generate mockgen -source $GOFILE -destination mocks/mock_schedule.go -package mocks
```

Adjust filename if multiple interfaces exist (e.g., `mock_manager.go`, `mock_router.go`).

---

## engine/webhook

Interfaces

- `engine/webhook/verify.go`: `Verifier`
- `engine/webhook/redis.go`: `Service`, `RedisClient`
- `engine/webhook/router.go`: `Processor`
- `engine/webhook/registry.go`: `Lookup`

Add to each file:

```go
//go:generate mockgen -source $GOFILE -destination mocks/mock_webhook.go -package mocks
```

Use separate files if preferred (e.g., `mock_verifier.go`, `mock_redis.go`).

---

## engine/auth

Interfaces

- `engine/auth/uc/repository.go`: `Repository`

Add:

```go
//go:generate mockgen -source $GOFILE -destination mocks/mock_repository.go -package mocks
```

---

## engine/infra

Interfaces (selected)

- `engine/infra/server/router/idempotency.go`: `APIIdempotency`
- `engine/infra/server/router/helpers.go`: `WorkflowRunner`
- `engine/infra/server/server.go`: `MCPProxy`
- `engine/infra/server/config/service.go`: `Service`
- `engine/infra/postgres/taskrepo.go`: `DB`
- `engine/infra/cache/*.go`: `RedisInterface`, `Publisher`, `Subscriber`, `NotificationSystem`, `LockManager`, `Lock`, `KV`, `Lists`, `Hashes`, `KeyIterator`, `KeysProvider`, `AtomicListWithMetadata`, `Capable`

Add to each file with interfaces:

```go
//go:generate mockgen -source $GOFILE -destination mocks/mock_infra.go -package mocks
```

For large groups, split per-topic (e.g., `mock_cache.go`).

---

## pkg/release

Interfaces

- `pkg/release/internal/service/*.go`: `NpmService`, `GoReleaserService`, `CliffService`
- `pkg/release/internal/repository/*.go`: `StateRepository`, `GithubExtendedRepository`, `GithubRepository`, `GitExtendedRepository`, `GitRepository`, `FileSystemRepository`

Add to each file:

```go
//go:generate mockgen -source $GOFILE -destination ../mocks/mock_$(basename $GOFILE .go).go -package mocks
```

If the destination needs to be within the same directory, use `mocks/` sibling and adjust path accordingly.

---

## pkg/mcp-proxy

If an `MCPClient` (or similar) interface file exists (e.g., `client.go`), add:

```go
//go:generate mockgen -source $GOFILE -destination mocks/mock_client.go -package mocks
```

---

## engine/memory

Interfaces spread across `engine/memory/instance/*.go`, `engine/memory/uc/*.go`.

Add to each interface file:

```go
//go:generate mockgen -source $GOFILE -destination mocks/mock_memory.go -package mocks
```

Split by concern if needed (e.g., `mock_instance.go`, `mock_uc.go`).

---

## CLI

Interfaces (if any) under `cli/api/*.go`.

Add:

```go
//go:generate mockgen -source $GOFILE -destination mocks/mock_api.go -package mocks
```

---

## Running generation

- One-off for a package: `go generate ./engine/task2/shared`
- All packages: `go generate ./...`

Optional Make target:

```make
.PHONY: generate-mocks
generate-mocks:
	go generate ./...
```

---

## Migration checklist

- Replace hand-written Testify mocks with generated mocks.
- Remove obsolete `*_mocks_test.go` and ad-hoc mock types.
- Prefer custom gomock matchers for complex args.
- Keep hand-crafted fakes only if they encode non-trivial behavior.
