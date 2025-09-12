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

## Tasks (≤7 parent tasks)

- [x] **1.0 Domain Model & Core Interfaces**
  - [x] Define `Attachment` and `Resolved` interfaces in `engine/attachment/resolver.go`
  - [x] Implement `baseAttachment` struct and all concrete types (`ImageAttachment`, `PDFAttachment`, `AudioAttachment`, `VideoAttachment`, `URLAttachment`, `FileAttachment`) in `engine/attachment/config.go`
  - [x] Add pluralized source support: `URLs []string` and `Paths []string` fields to applicable concrete types
  - [x] Implement polymorphic `UnmarshalYAML`/`UnmarshalJSON` with `type` discriminator and alias normalization (`document` → `pdf`)
  - [x] Add struct-level validation to ensure exactly one of `url`, `path`, `urls`, or `paths` is present per attachment
  - [x] Design `Resolved` interface with robust `Cleanup()` method for all resource handles (file descriptors, temp files)
  - [x] Success: Unit tests for polymorphic unmarshaling, validation errors, and interface contracts

- [x] **2.0 Resolvers & Resource Management**
  - [x] Implement MIME detection logic (`mimedetect.go`) using `net/http.DetectContentType` + `mimetype` fallback
  - [x] Implement shared HTTP helper (`resolver_http.go`) with timeouts, redirects, size caps, and context cancellation
  - [x] Implement shared Filesystem helper (`resolver_fs.go`) with CWD-awareness and path traversal prevention
  - [x] Implement resolver factory (`resolver_factory.go`) to select resolvers based on `Attachment.Type()`
  - [x] Implement per-type resolvers (`resolver_image.go`, `resolver_pdf.go`, `resolver_audio.go`, `resolver_video.go`, `resolver_url.go`) with type-specific allowlists and limits
  - [x] Ensure all file-based `Resolved` handles correctly manage temp files and implement `Cleanup()`
  - [x] Success: Unit tests for MIME detection, URL/path resolution, limits, cleanup, and context cancellation

- [x] **3.0 Normalization & Template Integration**
  - [x] Implement structural normalization (`normalize.go`) to expand `paths`/`urls` into individual attachments
  - [x] Implement glob pattern support for `paths` using `github.com/bmatcuk/doublestar/v4`
  - [x] Implement two-phase template engine integration (`context_normalization.go`):
    - [x] Phase 1: Evaluate templates with deferral of unresolved `.tasks.*` references during normalization
    - [x] Phase 2: Re-evaluate deferred templates at execution time with full runtime context
  - [x] Add metadata inheritance from pluralized sources to expanded individual attachments
  - [x] Success: Unit tests for glob expansion, template deferral logic, and metadata inheritance

- [ ] **4.0 Global Configuration & Schema Integration**
  - [x] Implement all global settings (`attachments.*`) in `pkg/config` per global-config.mdc pattern:
    - [x] Register fields in `pkg/config/definition/schema.go` (Path, Default, CLIFlag, EnvVar, Type, Help)
    - [x] Add typed structs in `pkg/config/config.go` with full tags (`koanf`, `json`, `yaml`, `mapstructure`, `env`, `validate`)
    - [x] Map from registry in appropriate `build<Section>Config(...)`
    - [x] Add CLI visibility (`cli/helpers/flag_categories.go`) and diagnostics (`cli/cmd/config/config.go`)
  - [x] Add global limits: `max_download_size_bytes`, `download_timeout`, `max_redirects`, `allowed_mime_types.*`, `temp_dir_quota_bytes`
  - [x] Embed `attachment.Config` into `Task`, `Agent`, and `Action` configurations
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

- [ ] **6.0 Tests & Examples**
  - [ ] Unit tests:
    - [ ] Resolver factory selection by `Attachment.Type()`
    - [ ] Per-type resolvers (success, size/timeout limits, redirects, MIME allowlist)
    - [ ] Resource cleanup and context cancellation
    - [ ] Merge logic ordering and de-duplication
    - [ ] Path traversal prevention (filesystem resolver)
  - [ ] Integration tests (no external network I/O):
    - [ ] Router flow with attachments driving branch selection (image/audio/video)
    - [ ] End-to-end LLM request assembly verifying `ContentPart` mapping
    - [ ] Global `attachments.*` limits enforced via config/env
    - [ ] Template deferral/evaluation across workflow context
  - [ ] Examples:
    - [ ] Rename `examples/pokemon-img` → `examples/pokemon`
    - [ ] One workflow with a router → 3 tasks (analyze-image | analyze-audio | analyze-video)
    - [ ] Seed example media (small CC‑licensed) acquired via Perplexity; examples only
    - [ ] Adjust to new attachments spec
  - [ ] Success: `make lint` + tests pass; integration tests stable; example runs locally

- [ ] **7.0 Documentation**
  - [ ] Create dedicated category `docs/content/docs/core/attachments/`
    - [ ] `meta.json` with pages: `overview`, `configuration`, `router-patterns`
  - [ ] Author pages mirroring Signals style (concise, pattern‑oriented):
    - [ ] Overview: concepts, types, scope resolution (task/agent/action)
    - [ ] Configuration: YAML schema and examples (paths/urls, globbing, template deferral)
    - [ ] Router patterns: branch by attachment type; best practices
  - [ ] Update `docs/content/docs/core/meta.json` to include `attachments`
  - [ ] Cross‑link from tasks and agents docs where relevant
  - [ ] Success: Docs build cleanly; links valid; content matches techspec

## Execution Plan

- **Critical Path:** 1.0 → 2.0 → 3.0 → 4.0 → 5.0 → 6.0
- **Parallelization Opportunities:**
  - Tasks 2.0 and 4.0 can be developed in parallel after 1.0 is complete
  - Task 3.0 requires completion of both 1.0 and 4.0 (needs config types and template engine)
  - Task 6.0 testing can begin incrementally as each prior task completes
  - Documentation drafting (part of 6.0) can start after 3.0

## Notes

- MUST READ: `.cursor/rules/critical-validation.mdc` before starting.
- Treat user-provided `mime` as hint; validation uses detected MIME.
