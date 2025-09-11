## Compozy Attachments: Polymorphic, Extensible Architecture

### File Structure (to be created/updated)

```
engine/
  attachment/
    config.go                 # Attachment interfaces, type-specific structs, Config, validation helpers
    resolver.go               # Resolver interfaces and Resolved handle
    resolver_factory.go       # Chooses resolver based on Attachment.Type()
    context_normalization.go  # Template context + two-phase normalization (tplengine); build eval ctx, defer .tasks.*, per-field templating
    resolver_image.go         # Image-specific resolution (url/path → URL/Binary); handles ImageAttachment
    resolver_pdf.go           # PDF-specific policies (mime allowlist, size/timeouts); handles PDFAttachment
    resolver_audio.go         # Audio-specific policies; handles AudioAttachment
    resolver_video.go         # Video-specific policies; handles VideoAttachment
    resolver_url.go           # Raw URL attachment handling (no file fetch); handles URLAttachment
    resolver_fs.go            # Shared: Local filesystem resolution (CWD-aware)
    resolver_http.go          # Shared: HTTP(S) fetch with limits/timeouts/redirects
    mimedetect.go             # MIME detection (DetectContentType + mimetype fallback)
    normalize.go              # Structural expansion only: paths/urls → individual attachments with glob support (no templating here)
    merge.go                  # Effective attachments merge (task ∪ agent ∪ action); polymorphic handling
    to_llm_parts.go           # ToContentParts: attachments → llmadapter.ContentPart (images)
    config_test.go            # Unit tests: validation, normalization for all Attachment types
    resolver_image_test.go    # Unit tests: image resolver (limits, timeouts) with ImageAttachment
    resolver_pdf_test.go      # Unit tests: pdf resolver with PDFAttachment
    resolver_audio_test.go    # Unit tests: audio resolver with AudioAttachment
    resolver_video_test.go    # Unit tests: video resolver with VideoAttachment
    resolver_url_test.go      # Unit tests: url resolver with URLAttachment
    normalize_test.go         # Unit tests: paths/urls expansion, glob patterns, metadata inheritance
    merge_test.go             # Unit tests: de-duplication and ordering across polymorphic attachments

engine/agent/
  config.go                   # Embed attachment.Config in Agent Config
  action_config.go            # Embed attachment.Config in Action Config

engine/task/
  config.go                   # Embed attachment.Config in BaseConfig
  uc/exec_task.go             # Compute effective attachments and pass to orchestrator

engine/llm/
  orchestrator.go             # Build messages from attachments (remove image_url/images)
  adapter/interface.go        # (No structural change; ensure BinaryPart support)

pkg/config/
  definition/schema.go        # Register attachments.* global properties (limits, allowlists)
  config.go                   # Add typed fields under a new Attachments section or existing
  provider.go                 # Defaults mapping (duration/string formatting if needed)
  env_mappings.go             # Ensure env tags are mapped
  loader.go                   # Cross-field validation if any

cli/helpers/
  flag_categories.go          # Categorize new CLI flags for attachments.*

cli/cmd/config/
  config.go                   # Flatten attachments.* for diagnostics/show
```

### Relevant and Dependent Files (current project)

- `engine/llm/orchestrator.go`: define `Request`, build messages; remove legacy image fields; integrate attachments → parts
- `engine/llm/adapter/interface.go`: `ContentPart` (`ImageURLPart`, `BinaryPart`) used by ToContentParts
- `engine/task/config.go`: `BaseConfig`; add `attachment.Config` embedding and CWD usage
- `engine/task/uc/exec_task.go`: execution path to compute effective attachments and pass into LLM request builder
- `engine/agent/config.go`: Agent `Config`; add `attachment.Config` and propagate CWD to actions
- `engine/agent/action_config.go`: Action `Config`; add `attachment.Config`; validation hooks
- `pkg/logger/*`: `logger.FromContext(ctx)` usage in resolvers
- `pkg/config/definition/schema.go`: register global fields for `attachments.*` per @global-config.mdc
- `pkg/config/config.go`: typed fields for `attachments.*`, validations, and builders
- `pkg/config/provider.go`: defaults mapping and stringification of durations
- `cli/helpers/flag_categories.go`: categorize CLI flags for `attachments.*`
- `cli/cmd/config/config.go`: diagnostics and flatteners for `attachments.*`
- `engine/core/*`: `PathCWD`, `AsMapDefault/FromMapDefault`, used for CWD and map conversions
- `examples/pokemon-img/*`: update to use attachments (remove image_url/images)
- `docs/content/docs/*`: update examples and provider notes (BinaryPart support)

This document proposes a unified attachments model usable across tasks, agents, and actions. It replaces ad‑hoc image inputs (e.g., `image_url`, `images`) with a consistent, type‑safe configuration that supports local and remote assets and cleanly integrates with the LLM orchestrator. The model uses a polymorphic Attachment interface with type-specific structs to reduce coupling, enable explicit validation per type, and allow resolvers to operate on tailored data without type assertions or shared fields.

#### Template Compatibility (pkg/tplengine)

Attachments MUST be fully compatible with the project template engine (`pkg/tplengine`) in all scopes (Task, Agent, Action) so they normalize against the active workflow context. All relevant string fields MUST support Go template expressions (with Sprig) evaluated using the same context model used throughout Compozy.

- Template‑enabled fields: `type`, `url`, `path`, `urls[]`, `paths[]`, `name`, `mime`, and any string values inside `meta`.
- Available context keys (typical):
  - `.workflow.id`, `.workflow.exec_id`, `.workflow.input.*`
  - `.input.*` (task‑local inputs), `.env.*`, `.agents.*`, `.tools.*`, `.trigger.*`
  - `.tasks.<task_id>.output.*` for chaining from prior tasks (note plural `.tasks`)
- Hyphenated identifiers (e.g., task ids like `process-image`) are supported by the template engine via automatic conversion to `index` form — no special authoring required in YAML.

Two‑phase resolution is REQUIRED to preserve runtime references:

1. Normalization phase: When computing the effective attachments list and expanding `paths`/`urls`, evaluate template strings with a filter that DEFERS unresolved runtime references (specifically `.tasks.*`). Any string containing `.tasks.` that cannot be resolved MUST be kept as‑is for later evaluation.
2. Execution phase: Immediately before resolving an attachment (e.g., downloading a URL or opening a file), re‑evaluate any remaining templates using the full runtime context including completed task outputs.

If template evaluation fails (e.g., missing keys) during normalization for non‑deferred expressions, surface clear, actionable errors. All template outputs STILL pass through path normalization, allowlists, and size/time constraints.

Examples (templated):

```yaml
attachments:
  # From workflow input (URL source)
  - type: image
    url: "{{ .workflow.input.image_url }}"
    name: "source:{{ .workflow.id }}"

  # From a previous task output (path source)
  - type: image
    path: "{{ .tasks.generate-image.output.path }}"
    name: "generated:{{ .workflow.exec_id }}"

  # Plural sources with templated segments
  - type: image
    paths:
      - "{{ .input.assets_dir }}/cover.png"
      - "{{ .input.assets_dir }}/thumbnails/*.jpg"
```

### Goals

- Generic attachments across the platform: image, video, audio, pdf, file, url
- Support both local filesystem paths and remote URLs
- Clean composition into existing configs without overengineering
- Progressive enhancement: minimal LLM integration now (images), extensible later
- Explicit type-specific structs for better type safety, validation, and resolver implementation

### Non‑Goals (initial phase)

- No heavy asset manager (caching/dedupe/CDN)
- No deep media probing/processing (only basic MIME detection)

---

### 1) Domain Model (engine/attachment)

Core types (polymorphic):

```go
package attachment

import "context"

// Type discriminator
type Type string     // "image" | "video" | "audio" | "pdf" | "file" | "url"
type Source string   // "url" | "path"

// Common interface for all attachments
type Attachment interface {
    Type() Type
    Name() string
    Meta() map[string]any
    // Resolution delegates to per-type resolver via factory
    Resolve(ctx context.Context, cwd *core.PathCWD) (Resolved, error)
}

// Resolved is a transport-agnostic handle
type Resolved interface {
    AsURL() (string, bool)
    AsFilePath() (string, bool)
    Open() (io.ReadCloser, error)
    MIME() string
    Cleanup()
}

// Base shared fields embedded by concrete types
type baseAttachment struct {
    name string
    mime string // optional user-provided hint; detection is authoritative
    meta map[string]any
}

// Concrete types
type ImageAttachment struct {
    baseAttachment
    Source Source
    URL    string   // required when Source=="url"
    Path   string   // required when Source=="path"
    URLs   []string // multiple URLs (alternative to URL)
    Paths  []string // multiple paths with glob support (alternative to Path)
    // Future-extensible: e.g., MaxWidth/MaxHeight
}
func (a *ImageAttachment) Type() Type { return "image" }
func (a *ImageAttachment) Name() string { return a.name }
func (a *ImageAttachment) Meta() map[string]any { return a.meta }

type PDFAttachment struct {
    baseAttachment
    Source   Source
    URL      string   // single URL
    Path     string   // single path
    URLs     []string // multiple URLs (alternative to URL)
    Paths    []string // multiple paths with glob support (alternative to Path)
    MaxPages *int     // example future field
}
func (a *PDFAttachment) Type() Type { return "pdf" }
func (a *PDFAttachment) Name() string { return a.name }
func (a *PDFAttachment) Meta() map[string]any { return a.meta }

type AudioAttachment struct {
    baseAttachment
    Source Source
    URL    string   // single URL
    Path   string   // single path
    URLs   []string // multiple URLs (alternative to URL)
    Paths  []string // multiple paths with glob support (alternative to Path)
}
func (a *AudioAttachment) Type() Type { return "audio" }
func (a *AudioAttachment) Name() string { return a.name }
func (a *AudioAttachment) Meta() map[string]any { return a.meta }

type VideoAttachment struct {
    baseAttachment
    Source Source
    URL    string   // single URL
    Path   string   // single path
    URLs   []string // multiple URLs (alternative to URL)
    Paths  []string // multiple paths with glob support (alternative to Path)
}
func (a *VideoAttachment) Type() Type { return "video" }
func (a *VideoAttachment) Name() string { return a.name }
func (a *VideoAttachment) Meta() map[string]any { return a.meta }

type URLAttachment struct {
    baseAttachment
    URL string // raw URL; no file fetch
}
func (a *URLAttachment) Type() Type { return "url" }
func (a *URLAttachment) Name() string { return a.name }
func (a *URLAttachment) Meta() map[string]any { return a.meta }

type FileAttachment struct {
    baseAttachment
    Path string // generic binary/file by path
}
func (a *FileAttachment) Type() Type { return "file" }
func (a *FileAttachment) Name() string { return a.name }
func (a *FileAttachment) Meta() map[string]any { return a.meta }

// Config holds a slice of polymorphic attachments
type Config struct {
    Attachments []Attachment `json:"attachments,omitempty" yaml:"attachments,omitempty"`
}
```

Polymorphic unmarshal & validation:

- Implement custom `UnmarshalYAML`/`UnmarshalJSON` to read a `type` discriminator and instantiate the proper concrete struct.
- Normalize aliases at load (e.g., `document` → `pdf`).
- For types that support multiple sources (image/pdf/audio/video): enforce exactly one of URL, Path, URLs, or Paths via type-specific validation.
- `mime` is an optional user hint; resolvers always detect MIME and the detected value is authoritative.
- Expansion/normalization step: after unmarshaling, expand `URLs`/`Paths` fields into individual attachment instances (with glob support for `Paths`).

Resolvers & factory:

- `resolver_factory.go` selects the resolver by `Attachment.Type()` and invokes the typed resolver.
- Per-type resolvers (`resolver_image.go`, etc.) accept the concrete attachment (no type assertions) and apply type-specific policies (allowlist, size/timeouts).
- Shared helpers: `resolver_http.go` (limits/redirects, streaming to temp); `resolver_fs.go` (CWD-aware path resolution and traversal prevention).

MIME detection:

- Use `net/http.DetectContentType` on the first 512 bytes; fallback to `github.com/gabriel-vasile/mimetype` for broader coverage.
- Treat user-provided `mime` as a hint only; validation is performed against the detected MIME type.

Rationale: explicit, testable types improve safety and evolvability while keeping the surface small and streaming-first.

---

### 2) Composition (Config embedding)

Embed `attachment.Config` into existing configs to keep a single, consistent shape:

```go
// example
type AgentConfig struct {
    attachment.Config
}

type TaskBaseConfig struct {
    attachment.Config
}

type ActionConfig struct {
    attachment.Config
}
```

Availability rules:

- Task.attachments: available to all agents/actions inside the task
- Agent.attachments: available to the agent (before system instructions) and all its actions
- Action.attachments: available only for that action

Merging (at execution time):
`effective = task.attachments ∪ agent.attachments ∪ action.attachments`

- De‑duplication key: `Type + normalized URL` OR `Type + canonical absolute Path` (do not include Name).
- Order preserved: task → agent → action. For duplicate resources, later metadata (Name/Meta) overwrites earlier.

---

### 3) Orchestrator Integration (Phase 1: images)

Change: remove ad‑hoc `image_url/images` handling in `engine/llm/orchestrator.go` and build parts exclusively from attachments.

Proposed minimal change:

- Add helper: `attachment.ToContentParts(ctx, items []attachment.Attachment) []llmadapter.ContentPart`
  - ImageAttachment + URL → `llmadapter.ImageURLPart`
  - ImageAttachment + Path → `llmadapter.BinaryPart` with detected `image/*`
- In `buildMessages`, compute `effective attachments` (task+agent+action) and append parts from `ToContentParts`.

Future phases (optional):

- Map `pdf|file|url` to textual context or tool inputs
- Add video/audio metadata (duration, dimensions) via `ffprobe` (exec) or MP4/MKV parsers (e.g., `abema/go-mp4`).

---

### 4) YAML Configuration

Examples:

Agent‑level:

```yaml
agents:
  - id: image-classifier
    attachments:
      # Single attachments (existing syntax)
      - type: image
        url: https://foo.com/1.png
      - type: document # alias of pdf (normalized to pdf)
        url: https://baz.com/spec.pdf

      # Multiple attachments with enhanced DX
      - type: image
        paths:
          - ./assets/*.png
          - ./screenshots/latest.jpg
        meta:
          source: "local-assets"

      - type: video
        urls:
          - https://bar.com/demo1.mp4
          - https://bar.com/demo2.mp4

      - type: url # raw URL without file
        url: https://example.com
    instructions: |
      You are an image classifier.
    actions: [...]
```

Task‑level (applies to all agents/actions of the task):

```yaml
tasks:
  - id: classify
    type: basic
    attachments:
      # Single file
      - type: image
        path: ./assets/input.png

      # Multiple files with glob pattern
      - type: image
        paths:
          - ./batch/*.jpg
          - ./batch/*.png
    agent: { id: image-classifier }
    action: recognize
```

Action‑only:

```yaml
agents:
  - id: doc-analyzer
    instructions: |
      Analyze documents.
    actions:
      - id: summarize
        attachments:
          # Single document
          - type: pdf
            url: https://example.com/spec.pdf

          # Multiple documents
          - type: pdf
            urls:
              - https://example.com/spec1.pdf
              - https://example.com/spec2.pdf
            name: "Specification Bundle"
        prompt: "Summarize the attached documents"
```

Validation rules:

- `type` ∈ { image, video, audio, pdf, file, url, document }
- For types supporting multiple sources: exactly one of `url`, `path`, `urls`, or `paths` must be present (Image/Audio/Video/PDF).
- `url`-only: URLAttachment; `path`-only: FileAttachment.
- `paths` supports glob patterns (e.g., `*.png`, `assets/**/*.jpg`) resolved against CWD.
- optional: `name`, `mime`, `meta` (freeform). `mime` is a hint and never overrides detected MIME.
- Metadata (`name`, `meta`) from pluralized sources applies to all expanded items.

---

### 5) Library Choices (via Perplexity research)

- **Glob patterns**: `github.com/bmatcuk/doublestar/v4` for recursive `**` pattern support (Go's stdlib `filepath.Glob` lacks `**` support)
- **MIME detection**: stdlib `net/http.DetectContentType` (reliable, magic bytes); fallback `github.com/gabriel-vasile/mimetype` for wider coverage
- **Remote fetch**: stdlib `net/http`
- Optional future:
  - PDF text extraction: `github.com/ledongthuc/pdf`
  - MP4 parsing: `github.com/abema/go-mp4` (dimensions, duration)
  - Rich media probing: `ffprobe` (FFmpeg) via `os/exec`

Notes: keep phase 1 minimal; adopt advanced libs only when needed. The `doublestar` library is essential for user-friendly recursive patterns like `./assets/**/*.png`.

---

### 6) Execution Flow

```
YAML attachments → attachment.Config (embedded) → executor merges scope
   → attachment.Resolver resolves items (URL or path) → LLM orchestrator
   → ToContentParts (images only) → llmadapter.Message.Parts
```

Key considerations:

- Stream local files; avoid loading large binaries in memory
- Always `Cleanup()` temp files
- Respect context deadlines/cancellation in HTTP fetches

---

### 7) Testing Strategy

- Unit tests (engine/attachment): resolver (URL/path), MIME detection, de‑dup merging
- Orchestrator integration: ensure image attachments appear as `Message.Parts`
- Examples: update `examples/pokemon-img` to demonstrate agent/task attachments

---

### 8) Incremental Rollout

1. Implement `engine/attachment` (types, resolver, MIME detection, normalization/expansion)
2. Embed `attachment.Config` into `agent.Config`, `task.BaseConfig`, and `agent.ActionConfig`
3. Add normalization step to expand `paths`/`urls` into individual attachments with glob support
4. Wire orchestrator to build parts from effective attachments (images only)
5. Remove old image input fields and examples entirely (no legacy support)
6. Update docs, examples, and tests to use attachments with enhanced DX syntax

---

### 9) Rationale & Trade‑offs

- Minimal surface area; avoids building a full asset pipeline
- Clean composition and clear availability rules
- Extensible: future support for non‑image consumption without affecting initial scope
- Safe defaults; uses standard libs; easy to mock in tests

---

### 10) Security & Limits

- Max download size: enforce per attachment (bytes); reject over limit
- Timeouts: per‑request download timeout; respect `ctx` cancellation
- Redirects: enforce `max_redirects`; disallow infinite chains
- MIME allowlist by `type` (enforced in per-type resolvers; validation uses detected MIME only):
  - image: `image/*`
  - pdf: `application/pdf`
  - audio: `audio/*`
  - video: `video/*`
  - file/url: configurable allowlist
- Logging: avoid logging full URLs with sensitive query params; redact in structured logs
- Path traversal: resolve against CWD → `filepath.Abs()` → reject if outside CWD root.
- Temp storage: stream to temp files; optional temp dir quota; always call `defer resolved.Cleanup()` immediately after a successful resolve.

Defaults and toggles are defined under Global Configuration (section 14) and applied by per-type resolvers.

---

### 11) Validation Details

- Polymorphic unmarshal: look up `type`, construct the concrete struct, then unmarshal.
- Normalize `type: document` → `type: pdf` during unmarshal.
- Type-specific validation ensures correct source presence (URL vs Path) and disallows invalid combinations.
- CWD normalization: for local `path`, resolve relative to the owning config’s `CWD` and verify it remains within CWD.
- MIME detection is always performed by resolvers; user-provided `mime` is treated as a hint only.

---

### 12) Orchestrator Wiring Details

- Compute effective attachments (task ∪ agent ∪ action) in the task execution path where all scopes are available
- Pass effective attachments into LLM orchestration request construction
- Phase 1: build `llmadapter.ContentPart` only for image attachments
  - ImageAttachment + URL → `ImageURLPart`
  - ImageAttachment + Path → `BinaryPart` with detected `image/*`
- Adapters should accept `BinaryPart`; provider support notes will be documented in Docs

---

### 13) Testing Enhancements

- Per-type resolver unit tests (image/pdf/audio/video/url): size/timeout/redirect enforcement; MIME allowlist
- Config polymorphic unmarshal tests per concrete type (including alias normalization and error paths)
- **Normalization/expansion tests**: `paths`/`urls` expansion with glob patterns, metadata inheritance, order preservation
- Resolver factory selection tests (correct resolver chosen by `Attachment.Type()` and source)
- De‑duplication invariants and merge order (task → agent → action)
- Temp file lifecycle and cleanup on all error paths
- Cancellation propagation (context cancel during download)
- CWD‑relative path resolution and traversal prevention
- **Glob pattern tests**: wildcard expansion, directory traversal prevention, error handling for invalid patterns
- Orchestrator integration: image attachments appear as message parts

---

### 14) Global Configuration (follow @global-config.mdc)

Introduce a set of global properties to control defaults and limits. Implement strictly per @global-config.mdc:

- Proposed paths (examples):
  - `attachments.max_download_size_bytes` (int, default e.g., 10_000_000)
  - `attachments.download_timeout` (duration, default e.g., 30s)
  - `attachments.max_redirects` (int, default e.g., 3)
  - `attachments.allowed_mime_types.image` (string slice)
  - `attachments.allowed_mime_types.audio` (string slice)
  - `attachments.allowed_mime_types.video` (string slice)
  - `attachments.allowed_mime_types.pdf` (string slice)
  - `attachments.temp_dir_quota_bytes` (int, optional)

Implementation checklist:

- Register in `pkg/config/definition/schema.go` (Path, Default, CLIFlag, EnvVar, Type, Help)
- Add to typed structs in `pkg/config/config.go` with full tags (`koanf`, `json`, `yaml`, `mapstructure`, `env`, `validate`)
- Map from registry in the appropriate `build<Section>Config(...)`
- Ensure CLI visibility (`cli/helpers/flag_categories.go`) and diagnostics (`cli/cmd/config/config.go`)
- Redaction where applicable (URLs) and tests under `pkg/config/*_test.go`

---

### Appendix: Schema Sketch

```yaml
attachments:
  type: array
  items:
    type: object
    required: [type]
    properties:
      type:
        type: string
        enum: [image, video, audio, pdf, file, url, document]
      url:
        type: string
        format: uri
      path:
        type: string
      urls:
        type: array
        items:
          type: string
          format: uri
      paths:
        type: array
        items:
          type: string
      name:
        type: string
      mime:
        type: string
      meta:
        type: object
```

Validation note: exactly one of `url`, `path`, `urls`, or `paths` must be present (enforced by struct validation code, not schema alone).
