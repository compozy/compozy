Goal (incl. success criteria):

- Decompose the approved `daemon-web-ui` TechSpec into validated task files under `.compozy/tasks/daemon-web-ui/`, following the `cy-create-tasks` workflow and mirroring the daemon feature's final QA tail shape.
- Success means: load task type/config context, read the TechSpec and ADRs, explore the codebase for implementation boundaries, present a dependency-safe task breakdown for approval, then generate `_tasks.md` plus `task_NN.md` files and validate them with `compozy tasks validate --name daemon-web-ui`.

Constraints/Assumptions:

- Must follow `cy-create-tasks`; the skill requires interactive approval of the proposed task breakdown before writing task files.
- `.compozy/config.toml` is absent in this repository, so task `type` values fall back to the built-in defaults: `frontend`, `backend`, `docs`, `test`, `infra`, `refactor`, `chore`, `bugfix`.
- `daemon-web-ui` currently has `_techspec.md` and ADRs, but no `_prd.md`; task derivation therefore comes from the TechSpec and ADRs.
- User asked to include the same two final tail tasks pattern used in `.compozy/tasks/daemon` for `$qa-report` and `$qa-execution`.
- User previously allowed multiple subagents for exploration only; all edits remain in the main agent.

Key decisions:

- Keep the feature slug/path as `.compozy/tasks/daemon-web-ui/`.
- Derive tasks from the approved TechSpec build order, but split large steps into smaller vertical slices to avoid mega-tasks.
- Mirror the daemon feature's terminal QA pattern:
  - penultimate `docs` task for `/qa-report`
  - final `test` task for `/qa-execution`
- Treat frontend foundation, backend contract/read models, browser security/context, static serving, route-domain slices, Storybook/MSW, verify/CI, and Playwright as separate implementation lanes before the QA pair.

State:

- Task-context loading is complete.
- Existing daemon task pattern and QA tail pattern are understood.
- Codebase exploration for task boundaries is complete enough to propose the breakdown.
- Task breakdown was approved.
- Task catalog files are written.
- Task validation and repository verification passed.

Done:

- Read repository instructions from `AGENTS.md` / `CLAUDE.md`.
- Read `.agents/skills/cy-create-techspec/SKILL.md`.
- Read `.agents/skills/brainstorming/SKILL.md`.
- Scanned existing `.codex/ledger/*-MEMORY-*.md` files for cross-agent awareness.
- Read prior daemon context from `.codex/ledger/2026-04-17-MEMORY-daemon-architecture.md`, `.codex/ledger/2026-04-17-MEMORY-agh-comparison.md`, `.codex/ledger/2026-04-18-MEMORY-daemon-command-surface.md`, and `.codex/ledger/2026-04-20-MEMORY-daemon-improvs-techspec.md`.
- Read the existing daemon TechSpec and ADRs under `.compozy/tasks/daemon/`.
- Spawned explorer agents to inspect: current looper daemon architecture, AGH frontend/embedding structure, and the local daemon mockup.
- Confirmed current looper daemon seams:
  - `internal/daemon/host.go` starts persistence, run manager, UDS, and localhost HTTP
  - `internal/api/core/routes.go` already exposes the daemon REST surface under `/api`
  - `internal/api/httpapi` is the natural host for a bundled SPA because it already owns the localhost Gin listener
  - current daemon HTTP serving is API-only; there is no static asset / SPA fallback yet
- Confirmed AGH frontend structure and serving model:
  - root Bun workspace with `web/` + `packages/ui/` + other packages
  - `web/` uses React 19, Vite 8, Vitest 4, TanStack Router, TanStack Query, Tailwind v4, shadcn (`base-nova`), Zustand, Zod, and `openapi-fetch`
  - `packages/ui/` is the shared UI kit consumed by `web/`
  - built assets are embedded from `web/dist` via `web/embed.go`
  - `internal/api/httpapi/static.go` serves exact assets and falls back to `index.html` for SPA routes while bypassing `/api`
- Confirmed the daemon mockup information architecture:
  - shell with sidebar + top command bar + route-driven content
  - pages: dashboard, workflows, runs, run detail, tasks, task detail, reviews, review detail, spec, memory index, memory detail
  - primary entities: daemon, workflows, runs, tasks, reviews, ADR/spec documents, memory notebooks, provider metadata
  - the mockup is operator-console oriented, not docs-site oriented
- Read AGH static serving references in `internal/api/httpapi/static.go` and route registration that wires `NoRoute` to the SPA fallback.
- Resolved the technical clarification sequence:
  - v1 scope = operational + rich read
  - browser integration = daemon-only REST/SSE
  - production serving = same daemon HTTP listener, SPA at `/`, API at `/api`, embedded bundle
  - web contract = OpenAPI-generated typed client
  - repo topology = mirror only `web/` + `packages/ui/`
  - validation bar = Vitest + typed-client tests + Playwright against embedded assets + full Storybook/MSW coverage
- Created ADRs under `.compozy/tasks/daemon-web-ui/adrs/`:
  - `adr-001.md` runtime frontend topology
  - `adr-002.md` single-listener embedded SPA serving
  - `adr-003.md` daemon-only OpenAPI REST/SSE contract
  - `adr-004.md` v1 operational + rich read scope
  - `adr-005.md` full frontend verification bar with Storybook and MSW
- Ran the requested external review before approval:
  - `./bin/compozy exec --ide claude --model opus --reasoning-effort xhigh --format json --add-dir /Users/pedronauck/Dev/compozy/looper --add-dir /Users/pedronauck/dev/compozy/agh --prompt-file .codex/tmp/daemon-web-ui-techspec-review.md`
  - run id: `exec-20260421-005915-000000000`
  - verdict: `approve_with_changes`
- Captured the highest-signal review corrections to incorporate into the draft:
  - do not silently replace current root-scoped `/api/tasks|reviews|runs|sync|exec` routes with a new `/workspaces/:id/workflows/...` hierarchy without an explicit migration or compatibility posture
  - make active-workspace resolution, persistence, empty-state, and switch behavior first-class instead of an inference
  - define the SSE streaming contract explicitly, including cursor/reconnect/heartbeat semantics
  - add browser-listener security design for Host/Origin validation and CSRF on mutation endpoints
  - pin OpenAPI authoring/generation/check workflow, build bootstrap order, and `web/dist` placeholder/bootstrap behavior
  - tighten route topology to AGH-style `_app` layout routes
  - define document read caching/invalidation and memory-file identifier rules
- Saved the final TechSpec to `.compozy/tasks/daemon-web-ui/_techspec.md`.
- Ran the required repository verification after the final save:
  - command: `make verify`
  - result: PASS
  - highlights:
    - `Formatting completed successfully`
    - `0 issues.`
    - `DONE 2438 tests, 1 skipped in 41.192s`
    - `All verification checks passed`
- Read `.agents/skills/cy-create-tasks/SKILL.md`.
- Read `.agents/skills/cy-create-tasks/references/task-template.md`.
- Read `.agents/skills/cy-create-tasks/references/task-context-schema.md`.
- Confirmed `.compozy/config.toml` is absent and task types must use built-in defaults.
- Read `.compozy/tasks/daemon-web-ui/_techspec.md` headings, API/testing/build-order sections, and all `daemon-web-ui` ADRs.
- Read `.compozy/tasks/daemon/_tasks.md`, `task_18.md`, and `task_19.md` to mirror the QA tail pattern.
- Read supporting ledgers: `web-ui-architecture`, `agh-frontend-map`, `daemon-task-slices`, and `daemon-qa-tasks`.
- Inspected current repo seams relevant to task slicing:
  - root `package.json`, `turbo.json`, `tsconfig*.json`, and `Makefile`
  - `.github/workflows/ci.yml`
  - `internal/api/httpapi/{server.go,routes.go}`
  - `internal/api/core/{routes.go,interfaces.go,handlers.go}`
  - `internal/daemon/{task_transport_service.go,workspace_transport_service.go,review_exec_transport_service.go,sync_transport_service.go}`
- Spawned explorer agents for:
  - backend task boundaries
  - frontend/runtime task boundaries
  - QA tail task pattern
- Collected high-signal exploration findings:
  - backend must be split across static/embed serving, browser-only security/workspace context, projection/document query layer, daemon transport read models, additive API/OpenAPI handlers, and SSE compatibility
  - root Bun/Turbo tooling already exists, but workspaces currently cover only `sdk/*`, and `Makefile` / CI remain Go-centric
  - `daemon-web-ui` should end with a `qa-report` planning task and a `qa-execution` validation task rooted at `.compozy/tasks/daemon-web-ui/qa/`
- Presented a 16-task breakdown and received explicit user approval (`ok`).
- Wrote `.compozy/tasks/daemon-web-ui/_tasks.md` plus `task_01.md` through `task_16.md`.
- Incorporated final frontend exploration references for OpenAPI client structure, `packages/ui`, Storybook/MSW, and Playwright-related file anchors.
- Ran task validation:
  - command: `./bin/compozy tasks validate --name daemon-web-ui`
  - result: PASS
  - output: `all tasks valid (16 scanned)`
- Ran repository verification after the task-file changes:
  - command: `make verify`
  - result: PASS
  - highlights:
    - `Formatting completed successfully`
    - `0 issues.`
    - `DONE 2438 tests, 1 skipped in 20.806s`
    - `All verification checks passed`

Now:

- Finalization complete; prepare the close-out response with created task files and fresh verification evidence.

Next:

- No further action unless the user asks for task execution or refinements to the task catalog.

Open questions (UNCONFIRMED if needed):

- UNCONFIRMED: `daemon-web-ui` is the assumed feature slug/path unless the user wants a different name before save.

Working set (files/ids/commands):

- `.codex/ledger/2026-04-20-MEMORY-daemon-web-ui.md`
- `.agents/skills/{cy-create-techspec/SKILL.md,brainstorming/SKILL.md,cy-create-tasks/SKILL.md}`
- `.agents/skills/cy-final-verify/SKILL.md`
- `.agents/skills/cy-create-tasks/references/{task-template.md,task-context-schema.md}`
- `.compozy/tasks/daemon/_techspec.md`
- `.compozy/tasks/daemon/_tasks.md`
- `.compozy/tasks/daemon/{task_18.md,task_19.md}`
- `.compozy/tasks/daemon-web-ui/_tasks.md`
- `.compozy/tasks/daemon-web-ui/task_01.md`
- `.compozy/tasks/daemon-web-ui/task_02.md`
- `.compozy/tasks/daemon-web-ui/task_03.md`
- `.compozy/tasks/daemon-web-ui/task_04.md`
- `.compozy/tasks/daemon-web-ui/task_05.md`
- `.compozy/tasks/daemon-web-ui/task_06.md`
- `.compozy/tasks/daemon-web-ui/task_07.md`
- `.compozy/tasks/daemon-web-ui/task_08.md`
- `.compozy/tasks/daemon-web-ui/task_09.md`
- `.compozy/tasks/daemon-web-ui/task_10.md`
- `.compozy/tasks/daemon-web-ui/task_11.md`
- `.compozy/tasks/daemon-web-ui/task_12.md`
- `.compozy/tasks/daemon-web-ui/task_13.md`
- `.compozy/tasks/daemon-web-ui/task_14.md`
- `.compozy/tasks/daemon-web-ui/task_15.md`
- `.compozy/tasks/daemon-web-ui/task_16.md`
- `.compozy/tasks/daemon-web-ui/_techspec.md`
- `.compozy/tasks/daemon-web-ui/adrs/{adr-001.md,adr-002.md,adr-003.md,adr-004.md,adr-005.md}`
- `.compozy/tasks/daemon/adrs/adr-001.md`
- `.compozy/tasks/daemon/adrs/adr-002.md`
- `.compozy/tasks/daemon/adrs/adr-003.md`
- `.compozy/tasks/daemon/adrs/adr-004.md`
- `docs/design/daemon-mockup/`
- `/Users/pedronauck/dev/compozy/agh`
- `/Users/pedronauck/dev/compozy/agh/web/`
- `/Users/pedronauck/dev/compozy/agh/packages/ui/`
- `/Users/pedronauck/dev/compozy/agh/internal/api/httpapi/static.go`
- `internal/daemon/{host.go,boot.go,service.go,run_manager.go}`
- `internal/api/{core,httpapi,udsapi}/`
- `internal/config/home.go`
- Explorer agents: `019dad7c-48d2-7cf3-beef-52025d0564cd`, `019dad7c-498a-7d11-bc1a-54b6c3a4ef7a`, `019dad7c-499a-77b2-9749-cb12131b7257`
