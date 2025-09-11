# Attachments Feature: Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/attachment/config.go` — Attachment interfaces and type-specific structs with pluralized source support
- `engine/attachment/normalize.go` — Expansion/normalization: paths/urls → individual attachments with glob support
- `engine/attachment/resolver_factory.go` — Chooses resolver by `Attachment.Type()`
- `engine/attachment/resolver_*.go` — Per-type resolvers and shared helpers
- `engine/llm/orchestrator.go` — Build messages from attachments
- `engine/task/uc/exec_task.go` — Compute effective attachments and pass through
- `pkg/config/*` — Global limits, schema, env/CLI mapping

### Documentation Files

- `tasks/prd-attachments/_techspec.md` — Technical spec
- `tasks/prd-attachments/_prd.md` — Product requirements
- `docs/content/docs/*` — User documentation updates

## Tasks (≤6 parent tasks)

- [ ] **1.0 Domain Model & Core Interfaces**
  - [ ] Define `Attachment` and `Resolved` interfaces in `engine/attachment/resolver.go`
  - [ ] Implement `baseAttachment` struct and all concrete types (`ImageAttachment`, `PDFAttachment`, `AudioAttachment`, `VideoAttachment`, `URLAttachment`, `FileAttachment`) in `engine/attachment/config.go`
  - [ ] Add pluralized source support: `URLs []string` and `Paths []string` fields to applicable concrete types
  - [ ] Implement polymorphic `UnmarshalYAML`/`UnmarshalJSON` with `type` discriminator and alias normalization (`document` → `pdf`)
  - [ ] Add struct-level validation to ensure exactly one of `url`, `path`, `urls`, or `paths` is present per attachment
  - [ ] Design `Resolved` interface with robust `Cleanup()` method for all resource handles (file descriptors, temp files)
  - [ ] Success: Unit tests for polymorphic unmarshaling, validation errors, and interface contracts

- [ ] **2.0 Resolvers & Resource Management**
  - [ ] Implement MIME detection logic (`mimedetect.go`) using `net/http.DetectContentType` + `mimetype` fallback
  - [ ] Implement shared HTTP helper (`resolver_http.go`) with timeouts, redirects, size caps, and context cancellation
  - [ ] Implement shared Filesystem helper (`resolver_fs.go`) with CWD-awareness and path traversal prevention
  - [ ] Implement resolver factory (`resolver_factory.go`) to select resolvers based on `Attachment.Type()`
  - [ ] Implement per-type resolvers (`resolver_image.go`, `resolver_pdf.go`, `resolver_audio.go`, `resolver_video.go`, `resolver_url.go`) with type-specific allowlists and limits
  - [ ] Ensure all file-based `Resolved` handles correctly manage temp files and implement `Cleanup()`
  - [ ] Success: Unit tests for MIME detection, URL/path resolution, limits, cleanup, and context cancellation

- [ ] **3.0 Normalization & Template Integration**
  - [ ] Implement structural normalization (`normalize.go`) to expand `paths`/`urls` into individual attachments
  - [ ] Implement glob pattern support for `paths` using `github.com/bmatcuk/doublestar/v4`
  - [ ] Implement two-phase template engine integration (`context_normalization.go`):
    - [ ] Phase 1: Evaluate templates with deferral of unresolved `.tasks.*` references during normalization
    - [ ] Phase 2: Re-evaluate deferred templates at execution time with full runtime context
  - [ ] Add metadata inheritance from pluralized sources to expanded individual attachments
  - [ ] Success: Unit tests for glob expansion, template deferral logic, and metadata inheritance

- [ ] **4.0 Global Configuration & Schema Integration**
  - [ ] Implement all global settings (`attachments.*`) in `pkg/config` per global-config.mdc pattern:
    - [ ] Register fields in `pkg/config/definition/schema.go` (Path, Default, CLIFlag, EnvVar, Type, Help)
    - [ ] Add typed structs in `pkg/config/config.go` with full tags (`koanf`, `json`, `yaml`, `mapstructure`, `env`, `validate`)
    - [ ] Map from registry in appropriate `build<Section>Config(...)`
    - [ ] Add CLI visibility (`cli/helpers/flag_categories.go`) and diagnostics (`cli/cmd/config/config.go`)
  - [ ] Add global limits: `max_download_size_bytes`, `download_timeout`, `max_redirects`, `allowed_mime_types.*`, `temp_dir_quota_bytes`
  - [ ] Embed `attachment.Config` into `Task`, `Agent`, and `Action` configurations
  - [ ] Success: `compozy config show` displays attachments settings; validation works; env/CLI mapping functional

- [ ] **5.0 Execution Wiring & Orchestrator Integration**
  - [ ] Implement merge logic (`merge.go`) to compute effective attachments (task ∪ agent ∪ action) with deterministic order and de-duplication by canonical key
  - [ ] Apply normalization/expansion before merging to handle pluralized sources correctly
  - [ ] Implement `ToContentParts` helper (`to_llm_parts.go`) to map attachments to `llmadapter.ContentPart`:
    - [ ] `ImageAttachment` + URL → `ImageURLPart`
    - [ ] `ImageAttachment` + Path → `BinaryPart` with detected `image/*` MIME
  - [ ] Integrate into `engine/task/uc/exec_task.go` to compute effective attachments and pass to orchestrator
  - [ ] Update `engine/llm/orchestrator.go` to use attachments exclusively, removing legacy `image_url/images` handling
  - [ ] Success: Integration tests verify image parts appear correctly; legacy fields removed; expanded attachments work

- [ ] **6.0 Tests, Examples, and Documentation**
  - [ ] Add comprehensive unit tests:
    - [ ] Resolver factory selection tests (correct resolver chosen by `Attachment.Type()`)
    - [ ] Per-type resolver tests covering success cases, limits (size, timeout), and error handling
    - [ ] Resource cleanup tests: verify temp file cleanup on success, failure, and panic paths
    - [ ] Context cancellation tests: verify network requests cancelled immediately when context cancelled
    - [ ] Merge logic tests: ordering, de-duplication, override behavior
    - [ ] Path traversal prevention tests
  - [ ] Add integration tests:
    - [ ] End-to-end attachment resolution and LLM part generation
    - [ ] Template evaluation with workflow context
    - [ ] Global configuration limit enforcement
  - [ ] Update examples:
    - [ ] Migrate `examples/pokemon-img` to use new `attachments` syntax
    - [ ] Add examples showing pluralized sources (`paths`/`urls`) and glob patterns
    - [ ] Add template examples (workflow input, previous task outputs)
  - [ ] Update documentation:
    - [ ] Configuration guide with all attachment types and options
    - [ ] Provider support matrix for `BinaryPart` vs `ImageURLPart`
    - [ ] Migration guide from `image_url/images` to `attachments` with examples
    - [ ] Template integration examples and two-phase resolution explanation
  - [ ] Success: CI tests pass; examples run; documentation complete; migration path clear

## Execution Plan

- **Critical Path:** 1.0 → 2.0 → 3.0 → 4.0 → 5.0 → 6.0
- **Parallelization Opportunities:**
  - Tasks 2.0 and 4.0 can be developed in parallel after 1.0 is complete
  - Task 3.0 requires completion of both 1.0 and 4.0 (needs config types and template engine)
  - Task 6.0 testing can begin incrementally as each prior task completes
  - Documentation drafting (part of 6.0) can start after 3.0
  - Example migration (part of 6.0) can start after 5.0

## Notes

- MUST READ: `.cursor/rules/critical-validation.mdc` before starting.
- Treat user-provided `mime` as hint; validation uses detected MIME.
