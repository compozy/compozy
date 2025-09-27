# cp\_\_ Native Tool Migration Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/tool/builtin/registry.go` - Registers native cp\_\_ builtin definitions and surfaces kill switch.
- `engine/tool/builtin/filesystem/*.go` - Filesystem tool implementations (read, write, delete, list, grep).
- `engine/tool/builtin/exec/exec.go` - Native exec tool handling allowlists and argument schemas.
- `engine/tool/builtin/fetch/fetch.go` - HTTP fetch tool with timeout/body caps.
- `engine/llm/service.go` - Boot-time registration flow for builtin tools and kill switch wiring.

### Integration Points

- `pkg/config/native_tools.go` - Configuration structs for root sandbox, exec allowlist, kill switch.
- `pkg/logger` & `engine/infra/monitoring` - Observability hooks for metrics and structured logging.
- `engine/runtime/bun_manager.go` - Legacy runner reference for kill-switch fallback.

### Documentation Files

- `docs/native-tools.md` (new) - Developer-facing guide for cp\_\_ tools.
- `docs/changelog.md` - Release communication.
- CLI templates under `pkg/template/templates/` - Scaffolding references.

## Tasks

- [x] 1.0 Establish builtin tool framework and shared validation utilities
- [x] 2.0 Implement filesystem cp\_\_ tools with sandboxing and limits
- [x] 3.0 Implement cp\_\_ exec tool with absolute-path allowlist and Windows fallback
- [x] 4.0 Implement cp\_\_ fetch tool with HTTP safety limits
- [x] 5.0 Integrate builtin registration, kill switch, and configuration surface
- [ ] 6.0 Instrument observability and canonical error catalog
- [ ] 7.0 Build verification suite and tests
- [ ] 8.0 Update documentation, templates, and migration guidance

## Execution Plan

- Critical Path: 1.0 → 2.0 → 5.0 → 7.0
- Parallel Track A: 3.0 ↔ 4.0 (after 1.0) feeding into 5.0
- Parallel Track B: 6.0 (after 2.0 & 3.0) can progress alongside 5.0
- Parallel Track C: 8.0 starts after naming established in 1.0 and completes post-5.0
