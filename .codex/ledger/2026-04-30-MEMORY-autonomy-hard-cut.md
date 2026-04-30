Goal (incl. success criteria):

- Execute tools-refac Task 09: session-bound autonomy tools and raw claim-token hard cut across Go contracts, CLI/HTTP/UDS, OpenAPI, web generated types/fixtures, docs, and tests.
- Success requires no raw `claim_token` on AGH-owned public surfaces, dedicated `agh__autonomy` tools, clean `make verify`, tracking updates, and one local commit if complete.

Constraints/Assumptions:

- Follow root/user AGENTS restrictions: no destructive git commands without explicit permission.
- Must use workflow memory, cy-execute-task, golang/test skills, and cy-final-verify.
- Existing worktree already has broad Task 09-related modifications; treat them as user/agent work and do not revert.
- Current task memory file was stale from a prior QA task; update it for this run.

Key decisions:

- UNCONFIRMED until review: existing uncommitted changes may already implement most Task 09 surfaces.

State:

- Grounding in PRD, ADRs, repo instructions, skills, and existing diff before editing code.

Done:

- Read shared workflow memory and current task memory.
- Read cy-workflow-memory, cy-execute-task, golang-pro, no-workarounds, testing-anti-patterns, and systematic-debugging skill instructions.
- Read root/internal/web/site guidance and Task 09/ADR-003/ADR-005 summary.

Now:

- Inspect relevant TechSpec sections and existing uncommitted autonomy diff for gaps.

Next:

- Build checklist, capture pre-change signal, run focused tests/review, fix only confirmed gaps, regenerate if needed, then run final verification.

Open questions (UNCONFIRMED if needed):

- Whether existing uncommitted changes are complete and verified.
- Whether Task 09 tracking status is inconsistent because header says completed while subtasks/master remain pending.

Working set (files/ids/commands):

- PRD dir: `/Users/pedronauck/Dev/compozy/agh/.compozy/tasks/tools-refac`
- Workflow memory: `.compozy/tasks/tools-refac/memory/MEMORY.md`, `.compozy/tasks/tools-refac/memory/task_09.md`
- Expected surfaces: `internal/task`, `internal/api`, `internal/cli`, `internal/tools`, `internal/daemon`, `openapi/agh.json`, `web/src/generated/agh-openapi.d.ts`, `web/src/systems/tasks`, `packages/site/content/runtime`
