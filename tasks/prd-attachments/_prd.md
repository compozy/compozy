# Product Requirements Document (PRD): Polymorphic Attachments

## Overview

Compozy will introduce a unified, polymorphic Attachments capability that replaces ad‑hoc image inputs and enables consistent, type‑safe configuration of external assets across Tasks, Agents, and Actions. The feature supports local file paths and remote URLs, integrates with the LLM orchestration flow (initially for images), and enforces security, performance, and reliability constraints without introducing heavy asset management.

Target users: developers configuring Compozy projects via YAML who need to reference images (now), with future extensibility to video, audio, PDF, generic files, and raw URLs.

## Goals

- Increase configuration usability and consistency by replacing `image_url/images` with a single `attachments` model across Tasks, Agents, and Actions.
- Achieve measurable adoption: ≥90% of official examples and docs use `attachments` within 2 releases after GA.
- Reliability: ≤2% resolver error rate across supported types in CI test suite; zero known path‑traversal vulnerabilities.
- Performance (local image path): p95 resolve time <100ms on typical project assets (≤5 MB), streaming without loading whole files into memory.
- Security posture: 100% of per‑type allowlist checks enforced on detected MIME type (not hints); all temp files cleaned up in success/error paths.

## User Stories

- As a developer, I can declare `attachments` in Task/Agent/Action YAML to include images from URLs or local paths without learning multiple ad‑hoc fields.
- As a developer, I can rely on Compozy to reject unsupported MIME types or oversized downloads with clear error messages.
- As a developer, I can update examples and docs to use the same model and see identical behavior across scopes (task ∪ agent ∪ action).
- As a maintainer, I can extend support to new attachment types without breaking existing configs.

## Core Features

- Unified `attachments` configuration that is available at Task, Agent, and Action scopes, merged deterministically at execution time.
- Polymorphic attachment types: image (MVP), video, audio, pdf, file, url (extensible).
- Per‑type resolvers enforce allowlists, size/timeouts, redirects, and MIME detection; shared FS/HTTP helpers handle streaming and CWD protection.
- Orchestrator integration (Phase 1): images only → `ImageURLPart` for URL sources; `BinaryPart` for local files.
- Global configuration for limits and allowlists (e.g., max download size, timeouts, allowed MIME per type).

### Template Compatibility (pkg/tplengine)

Attachments MUST support Go template expressions (via `pkg/tplengine`) so values can be resolved against the live workflow context at Task, Agent, and Action scopes. This enables dynamic URLs/paths and metadata derived from inputs or prior task outputs.

- Template‑enabled fields: `url`, `path`, `urls[]`, `paths[]`, `name`, `mime`, and string fields in `meta`.
- Context variables: `.workflow.id`, `.workflow.exec_id`, `.workflow.input.*`, `.input.*`, `.env.*`, `.agents.*`, `.tools.*`, `.trigger.*`, `.tasks.<task_id>.output.*` (plural `.tasks`).
- Two‑phase resolution: normalize with deferral of `.tasks.*` when not yet available; resolve fully at execution time when prior tasks have produced outputs.

Examples:

```yaml
attachments:
  - type: image
    url: "{{ .workflow.input.image_url }}" # from workflow input
  - type: image
    path: "{{ .tasks.generate-image.output.path }}" # from previous task output
```

Functional requirements (numbered):

1. The system MUST accept `attachments` at Task, Agent, and Action scopes and compute an effective merged list at runtime (task → agent → action order; last metadata wins).
2. The system MUST support `image` attachments from both `url`/`path` (single) and `urls`/`paths` (multiple) in MVP; additional types are defined but may be no‑op for LLM integration initially.
3. The system MUST expand pluralized sources (`paths`, `urls`) into individual attachments during normalization, with glob pattern support for local files.
4. The system MUST enforce MIME allowlists per attachment type using detected MIME; user‑provided `mime` is treated only as a hint.
5. The system MUST stream local files and remote downloads to temp files, never loading entire large files into memory.
6. The system MUST prevent path traversal by resolving relative paths against CWD and rejecting paths that escape the project root.
7. The system MUST remove legacy `image_url/images` inputs across orchestrator and examples.
8. The system MUST expose errors with actionable messages (timeouts, size exceeded, disallowed MIME, redirect loop).
9. The system MUST document provider expectations for `BinaryPart` and `ImageURLPart` support.

## User Experience

- YAML-first ergonomics: a single `attachments` array with explicit `type` and source (`url` or `path`), plus optional `name`, `mime`, `meta`.
- **Enhanced DX**: Support for pluralized sources (`paths`, `urls`) to handle multiple files of the same type efficiently, including glob patterns for local files.
- CLI diagnostics: `compozy config show` and related commands display `attachments.*` global limits and effective values.
- Accessibility: documentation examples include descriptive `name` guidance to aid downstream labeling and logging.

## High-Level Technical Constraints

- LLM provider compatibility: adapters must accept `BinaryPart` for image files; URL images mapped to `ImageURLPart`.
- Performance: streaming I/O for files and downloads; explicit per‑request timeouts and redirect caps.
- Security: MIME allowlists per type, strict path normalization/verification under CWD, temp file cleanup with `defer`.
- No heavy asset manager, CDN, or caching layer in MVP.

## Non-Goals (Out of Scope)

- Rich media processing (resizing, OCR, transcription, ffprobe metadata) in MVP.
- Centralized asset caching, deduplication service, or CDN distribution.
- UI for browsing/attaching files; this is a configuration‑level feature.

## Phased Rollout Plan

- MVP (GA): Polymorphic model implemented; image attachments fully supported end‑to‑end (URL → ImageURLPart; path → BinaryPart). Legacy image inputs removed. Docs/examples migrated.
- Phase 2: Enforce per‑type policies for pdf/audio/video/file and expose non‑image attachment availability for future use (not necessarily mapped to LLM parts yet). Add richer diagnostics.
- Phase 3: Optional media enhancements (e.g., pdf text extraction, basic ffprobe metadata) and expanded adapter support as needed.

## Success Metrics

- Adoption: ≥90% of official examples reworked to `attachments` within 2 releases; new examples use `attachments` exclusively.
- Quality: ≤2% resolver failure rate in CI; 0 unresolved temp files in tests.
- Performance: p95 local image `path` resolve <100ms on reference assets; p95 remote image download <3s within size limits.
- Security: 100% tests for MIME allowlists and path traversal protection; zero known security issues post‑release.

## Risks and Mitigations

- Provider mismatch for `BinaryPart`: Document provider support; fallback guidance to `ImageURLPart` when needed; maintain adapter tests.
- Configuration complexity: Provide clear schema, examples, and validation errors; keep PRD scope focused on WHAT/WHY.
- Legacy migration risk: Breaking changes by removing `image_url/images`; provide migration notes and example diffs.
- Performance/regression risk: Enforce streaming and timeouts; add tests for large files and cancellation.

## Open Questions

- Do any supported LLM adapters lack `BinaryPart` support today? If so, document provider‑specific constraints.
- What are sensible default timeouts and size limits per type for production vs. dev?
- Should `url` attachments without fetch be exposed to tools or future features in Phase 2?

## Appendix

- Tech Spec: `tasks/prd-attachments/_techspec.md`
- Template: `tasks/docs/_prd-template.md`
