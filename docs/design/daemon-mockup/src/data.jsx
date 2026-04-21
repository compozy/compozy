/* global React */
// Fake data for Compozy daemon prototype. All strings in the terse developer-first voice.

const PROVIDERS = {
  claude:   { id: "claude",   name: "Claude Code", logo: "assets/providers/claude-code.svg", model: "Sonnet 4.5" },
  codex:    { id: "codex",    name: "Codex",       logo: "assets/providers/codex.svg",       model: "gpt-5-codex" },
  gemini:   { id: "gemini",   name: "Gemini",      logo: "assets/providers/gemini.svg",      model: "2.5 Pro" },
  opencode: { id: "opencode", name: "OpenCode",    logo: "assets/providers/opencode.svg",    model: "v1.3" },
  ollama:   { id: "ollama",   name: "Ollama",      logo: "assets/providers/ollama.svg",      model: "llama3.1:70b" },
};

const WORKFLOWS = [
  {
    name: "user-auth",
    title: "User authentication & session management",
    status: "running", // running | paused | done | failed | archived
    phase: "exec",
    phases_done: ["prd", "techspec", "tasks"],
    repo: "compozy-code",
    branch: "feat/user-auth",
    provider: "claude",
    tasks: { total: 12, done: 7, running: 2, failed: 1, pending: 2 },
    created: "3d ago",
    updated: "42s ago",
    reviews: 1,
    pending_reviews: 4,
    owner: "Rui",
  },
  {
    name: "multi-repo",
    title: "Ship multi-repo rollout runbook",
    status: "paused",
    phase: "tasks",
    phases_done: ["prd", "techspec"],
    repo: "compozy-code",
    branch: "chore/rollout",
    provider: "codex",
    tasks: { total: 8, done: 3, running: 0, failed: 0, pending: 5 },
    created: "5d ago",
    updated: "yesterday",
    reviews: 0,
    pending_reviews: 0,
    owner: "Rui",
  },
  {
    name: "manifest-v2",
    title: "Provider binary manifest validation",
    status: "running",
    phase: "review",
    phases_done: ["prd", "techspec", "tasks", "exec"],
    repo: "provider-sdk",
    branch: "feat/manifest-v2",
    provider: "claude",
    tasks: { total: 6, done: 6, running: 0, failed: 0, pending: 0 },
    created: "1w ago",
    updated: "12m ago",
    reviews: 2,
    pending_reviews: 9,
    owner: "Rui",
  },
  {
    name: "stream-chunks",
    title: "Stream chunked plan updates from orchestrator",
    status: "done",
    phase: "review",
    phases_done: ["prd", "techspec", "tasks", "exec", "review"],
    repo: "go-orchestrator",
    branch: "feat/stream-chunks",
    provider: "claude",
    tasks: { total: 9, done: 9, running: 0, failed: 0, pending: 0 },
    created: "2w ago",
    updated: "4d ago",
    reviews: 3,
    pending_reviews: 0,
    owner: "Rui",
  },
  {
    name: "skill-banner",
    title: "Skill runtime: recommended updates banner",
    status: "failed",
    phase: "exec",
    phases_done: ["prd", "techspec", "tasks"],
    repo: "skill-runtime",
    branch: "feat/skill-banner",
    provider: "gemini",
    tasks: { total: 5, done: 2, running: 0, failed: 1, pending: 2 },
    created: "6d ago",
    updated: "2d ago",
    reviews: 0,
    pending_reviews: 0,
    owner: "Rui",
  },
  {
    name: "warm-toast",
    title: "Warm stone toast variant — align with badge palette",
    status: "archived",
    phase: "review",
    phases_done: ["prd", "techspec", "tasks", "exec", "review"],
    repo: "compozy-code",
    branch: "chore/warm-toast",
    provider: "codex",
    tasks: { total: 4, done: 4, running: 0, failed: 0, pending: 0 },
    created: "3w ago",
    updated: "2w ago",
    reviews: 1,
    pending_reviews: 0,
    owner: "Rui",
  },
];

const TASKS_USERAUTH = [
  { id: "task_01", title: "Add users table migration with indexed email", status: "done",        domain: "db",       effort: "S", files: 3, duration: "4m 12s", started: "3d ago", provider: "claude" },
  { id: "task_02", title: "Wire password hashing with argon2id defaults", status: "done",        domain: "backend",  effort: "M", files: 4, duration: "6m 40s", started: "3d ago", provider: "claude" },
  { id: "task_03", title: "Issue session tokens using paseto v4", status: "done",                domain: "backend",  effort: "M", files: 5, duration: "9m 04s", started: "3d ago", provider: "claude" },
  { id: "task_04", title: "POST /auth/login handler with rate limit", status: "done",            domain: "api",      effort: "M", files: 3, duration: "5m 22s", started: "2d ago", provider: "claude" },
  { id: "task_05", title: "POST /auth/logout handler revokes session", status: "done",           domain: "api",      effort: "S", files: 2, duration: "2m 45s", started: "2d ago", provider: "claude" },
  { id: "task_06", title: "Middleware: resolve session from cookie + header", status: "done",    domain: "api",      effort: "M", files: 4, duration: "7m 01s", started: "2d ago", provider: "claude" },
  { id: "task_07", title: "CSRF double-submit cookie on mutating routes", status: "done",        domain: "api",      effort: "L", files: 6, duration: "12m 19s",started: "2d ago", provider: "claude" },
  { id: "task_08", title: "Bruteforce detection + exponential backoff", status: "running",       domain: "backend",  effort: "L", files: 5, duration: "running · 3m 40s", started: "just now", provider: "claude" },
  { id: "task_09", title: "Email verification flow — send + confirm", status: "running",         domain: "api",      effort: "L", files: 7, duration: "running · 1m 12s", started: "1m ago",   provider: "claude" },
  { id: "task_10", title: "Password reset via one-time token", status: "failed",                 domain: "backend",  effort: "M", files: 4, duration: "timeout @ 10m", started: "15m ago",  provider: "claude", error: "Activity timeout — agent produced no diff" },
  { id: "task_11", title: "Login UI — forms + error handling", status: "pending",                domain: "frontend", effort: "M", files: 0, duration: "—", started: "—", provider: null },
  { id: "task_12", title: "End-to-end Playwright test suite", status: "pending",                 domain: "test",     effort: "L", files: 0, duration: "—", started: "—", provider: null },
];

const RUNS = [
  {
    id: "run_2a9f",
    workflow: "user-auth",
    provider: "claude",
    started: "12m ago",
    duration: "12m 14s · running",
    status: "running",
    jobs_total: 12, jobs_done: 7, jobs_running: 2, jobs_failed: 1, jobs_pending: 2,
    model: "Sonnet 4.5", reasoning: "medium", concurrent: 2, batch: 3,
    tokens_in: "284.2k", tokens_out: "38.7k",
    command: "compozy start --name user-auth --ide claude --concurrent 2 --batch-size 3",
  },
  {
    id: "run_18b3",
    workflow: "manifest-v2",
    provider: "claude",
    started: "2h ago",
    duration: "38m 04s",
    status: "done",
    jobs_total: 6, jobs_done: 6, jobs_running: 0, jobs_failed: 0, jobs_pending: 0,
    model: "Sonnet 4.5", reasoning: "medium", concurrent: 1, batch: 1,
    tokens_in: "142.1k", tokens_out: "17.4k",
    command: "compozy start --name manifest-v2 --ide claude",
  },
  {
    id: "run_3c2e",
    workflow: "skill-banner",
    provider: "gemini",
    started: "2d ago",
    duration: "22m 48s",
    status: "failed",
    jobs_total: 5, jobs_done: 2, jobs_running: 0, jobs_failed: 1, jobs_pending: 2,
    model: "2.5 Pro", reasoning: "medium", concurrent: 1, batch: 1,
    tokens_in: "88.3k", tokens_out: "9.2k",
    command: "compozy start --name skill-banner --ide gemini",
  },
  {
    id: "run_5d1a",
    workflow: "multi-repo",
    provider: "codex",
    started: "yesterday",
    duration: "14m 22s",
    status: "paused",
    jobs_total: 8, jobs_done: 3, jobs_running: 0, jobs_failed: 0, jobs_pending: 5,
    model: "gpt-5-codex", reasoning: "high", concurrent: 1, batch: 1,
    tokens_in: "61.0k", tokens_out: "6.8k",
    command: "compozy start --name multi-repo --ide codex --reasoning-effort high",
  },
  {
    id: "run_11a8",
    workflow: "stream-chunks",
    provider: "claude",
    started: "4d ago",
    duration: "1h 12m",
    status: "done",
    jobs_total: 9, jobs_done: 9, jobs_running: 0, jobs_failed: 0, jobs_pending: 0,
    model: "Sonnet 4.5", reasoning: "medium", concurrent: 2, batch: 2,
    tokens_in: "312.5k", tokens_out: "44.1k",
    command: "compozy start --name stream-chunks --ide claude --concurrent 2",
  },
];

// Log samples for a running job
const SAMPLE_LOG = [
  { t: "12:04:11", lv: "info",  msg: "[compozy] dispatching task_08 → claude / Sonnet 4.5 / medium" },
  { t: "12:04:11", lv: "debug", msg: "[prompt] workflow=user-auth task=task_08 tokens.in≈18.4k files.allowed=[packages/api, packages/auth]" },
  { t: "12:04:12", lv: "info",  msg: "[claude] opened session · worktree=feat/user-auth · skills=[cy-execute-task, cy-workflow-memory, cy-final-verify]" },
  { t: "12:04:13", lv: "claude",msg: "Reading memory/task_08.md and memory/MEMORY.md to resume context from previous run…" },
  { t: "12:04:15", lv: "claude",msg: "Planning: I'll introduce a PenaltyBox that tracks failed attempts per IP+email, with exponential backoff starting at 1s up to 10m." },
  { t: "12:04:16", lv: "tool",  msg: "→ shell: rg -n \"login\" packages/api/src --type go" },
  { t: "12:04:16", lv: "tool",  msg: "← ok · 14 matches in 4 files" },
  { t: "12:04:18", lv: "claude",msg: "Opening packages/auth/src/penalty.go · adding PenaltyBox type with sync.Map + atomic counter." },
  { t: "12:04:20", lv: "tool",  msg: "→ edit: packages/auth/src/penalty.go" },
  { t: "12:04:21", lv: "tool",  msg: "← +87 −0" },
  { t: "12:04:23", lv: "tool",  msg: "→ edit: packages/api/src/handlers/login.go" },
  { t: "12:04:23", lv: "tool",  msg: "← +14 −2" },
  { t: "12:04:25", lv: "claude",msg: "Writing unit tests for penalty window, threshold, and decay." },
  { t: "12:04:27", lv: "tool",  msg: "→ edit: packages/auth/src/penalty_test.go" },
  { t: "12:04:28", lv: "tool",  msg: "← +112 −0" },
  { t: "12:04:30", lv: "tool",  msg: "→ shell: go test ./packages/auth/..." },
  { t: "12:04:33", lv: "tool",  msg: "← ok · PASS ./packages/auth (3 tests)" },
  { t: "12:04:34", lv: "claude",msg: "Checking acceptance criteria in task_08.md: ✓ per-identity counter, ✓ exponential backoff, ✓ decay, ✓ metric hook. Updating memory." },
  { t: "12:04:35", lv: "info",  msg: "[memory] patching memory/task_08.md (+38 lines)" },
  { t: "12:04:36", lv: "info",  msg: "[memory] patching memory/MEMORY.md (+6 lines, promoting: decay policy, metric naming)" },
  { t: "12:04:37", lv: "info",  msg: "[compozy] verifying evidence via cy-final-verify…" },
];

const MEMORY_SHARED = `# Workflow memory — user-auth

## Decisions
- **Session format**: PASETO v4 (local), 32-byte symmetric key, rotated per env.
- **Password hashing**: argon2id with params time=3, mem=64 MiB, parallelism=2.
- **Rate limit strategy**: sliding window 60s bucket at API gateway, plus per-identity PenaltyBox at auth layer (decided in task_08).
- **CSRF**: double-submit cookie only on mutating routes; SameSite=lax + Secure on session cookie.

## Patterns discovered
- Every handler in \`packages/api/src/handlers\` uses the \`WithCtx()\` middleware — penalty middleware must be registered before it to short-circuit cleanly.
- Integration tests seed fixtures via \`test/fixtures/auth/*.go\`; add a \`users.go\` fixture before touching login tests.

## Open risks
- Email verification relies on a transactional SMTP provider we haven't picked yet — leaving the send step behind a \`Mailer\` interface so task_09 can stub.
- PenaltyBox is in-memory; if we run >1 API replica this needs Redis. Noted for post-MVP.

## Handoffs
- task_09 inherits the Mailer interface shape from task_08.
- task_11 (UI) will need the new \`{field, message}\` error envelope introduced in task_04 — consume it in the form renderer.
`;

const MEMORY_TASK = `# task_08 — Bruteforce detection + exponential backoff

## Objective snapshot
Introduce per-identity rate limiting with exponential backoff and a decay window.
Acceptance: counter resets after success; 1s → 10m backoff; metric hook for observability.

## Files touched
- \`packages/auth/src/penalty.go\` — new PenaltyBox (sync.Map-backed)
- \`packages/auth/src/penalty_test.go\` — unit coverage for window/threshold/decay
- \`packages/api/src/handlers/login.go\` — wired PenaltyBox.Check before credential compare

## Errors hit
- Initial pass used a naked \`map[string]int\` → data race under concurrent logins.
  Fix: \`sync.Map\` + \`atomic.Int64\` for the counter.
- \`go test -race\` flagged the decay goroutine leaking on process shutdown.
  Fix: accept \`context.Context\` in \`NewPenaltyBox\` and exit on Done.

## Ready for next run
- PenaltyBox interface is stable; task_09 (email verification) can reuse it to rate-limit the /auth/verify endpoint.
- Consider promoting \`decay policy\` to shared memory → DONE.
`;

const SPEC_USERAUTH = {
  workflow: "user-auth",
  prd: `# PRD — user-auth

## Problem
Our dashboard is unauthenticated. Session hijacking risk is real now that customer data lives behind the gateway.
We need primary-auth that is **boring, recoverable, and self-hostable** — no external auth provider, no magic.

## Goals
1. Users can sign up, log in, log out, and recover their password.
2. Session tokens are cryptographically sealed and revocable.
3. All auth endpoints are rate-limited per-identity and globally.

## Non-goals
- SSO (SAML, OIDC) — planned post-v1.
- Social login.
- Multi-factor beyond TOTP (no WebAuthn in v1).

## User stories
- **As a new user**, I can sign up with email + password and land in the product.
- **As a returning user**, I can log in and my session survives refresh but not a server restart from key rotation.
- **As an operator**, I can revoke a session server-side immediately.

## Success metrics
- p99 login latency < 200ms at 50 RPS steady state.
- 0 CVE-category bugs at launch (external pentest pass).
- Password reset funnel ≥ 80% completion rate.
`,
  techspec: `# TechSpec — user-auth

## Architecture
Authentication lives in a new \`packages/auth\` module, consumed by \`packages/api\`.
Session tokens are **PASETO v4 local** — symmetric encryption, 32-byte key stored in KMS and rotated per-env.
Password hashing uses **argon2id** (\`time=3, memory=64 MiB, parallelism=2\`) — calibrated for ≤ 60ms on target hardware.

## Data model
\`\`\`sql
CREATE TABLE users (
  id            UUID PRIMARY KEY,
  email         CITEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  email_verified_at TIMESTAMPTZ,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX users_email_idx ON users (email);

CREATE TABLE sessions (
  id         UUID PRIMARY KEY,
  user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ
);
\`\`\`

## API surface
- \`POST /auth/signup { email, password }\` → 201 + session cookie.
- \`POST /auth/login { email, password }\` → 200 + session cookie.
- \`POST /auth/logout\` → 204 (revokes server-side).
- \`POST /auth/reset/request { email }\` → 202 always (no enumeration).
- \`POST /auth/reset/confirm { token, password }\` → 204.

## Security
- Double-submit CSRF cookie on mutating routes.
- PenaltyBox per identity (5 fails → 1s backoff, doubling to 10m; decay on success).
- Password minimum: 12 chars, class-weighted zxcvbn ≥ 3.
`,
  adrs: [
    { n: "001", title: "Use PASETO over JWT", decision: "accepted", date: "3d ago", summary: "JWT's algorithm confusion CVEs and optional-signature quirks are a constant footgun. PASETO v4 local gives us symmetric, versioned, no-negotiation tokens." },
    { n: "002", title: "Rate-limit at two layers", decision: "accepted", date: "3d ago", summary: "Global per-IP at gateway (cheap, DoS-grade) + per-identity PenaltyBox at handler (precise, per-account). Defense in depth without adding Redis yet." },
    { n: "003", title: "No WebAuthn in v1", decision: "deferred", date: "3d ago", summary: "Low customer demand now, meaningful UX/DX surface. Revisit in Q3 when the web push/passkey ecosystem stabilizes." },
  ],
};

const REVIEW_ROUNDS = [
  {
    n: 1, workflow: "manifest-v2", provider: "coderabbit", pr: 312, fetched: "2d ago",
    issues: { total: 14, open: 4, fixed: 8, invalid: 2 },
  },
  {
    n: 2, workflow: "manifest-v2", provider: "coderabbit", pr: 312, fetched: "4h ago",
    issues: { total: 9, open: 5, fixed: 2, invalid: 2 },
  },
];

const REVIEW_ISSUES = [
  { id: "issue_001", title: "Missing null guard on manifest.providers[i].capabilities", severity: "high",   status: "open",    domain: "validation", file: "packages/providers/manifest.ts",       line: 128, thread: "coderabbit#aa21", author: "CodeRabbit" },
  { id: "issue_002", title: "Inconsistent error message case — use sentence case",       severity: "low",    status: "fixed",   domain: "copy",       file: "packages/providers/errors.ts",         line:  42, thread: "coderabbit#aa22", author: "CodeRabbit" },
  { id: "issue_003", title: "Potential goroutine leak in ManifestWatcher.Close",         severity: "high",   status: "fixed",   domain: "concurrency",file: "packages/providers/watcher.go",        line: 202, thread: "coderabbit#aa23", author: "CodeRabbit" },
  { id: "issue_004", title: "Schema test doesn't cover empty capabilities array",        severity: "medium", status: "open",    domain: "test",       file: "packages/providers/manifest_test.ts",  line:  18, thread: "coderabbit#aa24", author: "CodeRabbit" },
  { id: "issue_005", title: "Typo: 'recieved' → 'received'",                             severity: "low",    status: "fixed",   domain: "copy",       file: "packages/providers/log.ts",            line:  91, thread: "coderabbit#aa25", author: "CodeRabbit" },
  { id: "issue_006", title: "Prefer errors.Is over == for sentinel compare",             severity: "medium", status: "invalid", domain: "idiom",      file: "packages/providers/watcher.go",        line:  64, thread: "coderabbit#aa26", author: "CodeRabbit" },
  { id: "issue_007", title: "Add context timeout to Validate() default",                 severity: "medium", status: "open",    domain: "api",        file: "packages/providers/validate.go",       line:  12, thread: "coderabbit#aa27", author: "CodeRabbit" },
  { id: "issue_008", title: "Doc comment missing on public Manifest type",               severity: "low",    status: "open",    domain: "docs",       file: "packages/providers/manifest.go",       line:   1, thread: "coderabbit#aa28", author: "CodeRabbit" },
  { id: "issue_009", title: "Return error on unknown capability flag, don't silently drop", severity: "high", status: "open",   domain: "validation", file: "packages/providers/validate.go",       line:  78, thread: "coderabbit#aa29", author: "CodeRabbit" },
];

const DAEMON = {
  status: "healthy",         // healthy | degraded | stopped
  version: "0.1.2",
  pid: 48291,
  uptime: "4d 11h",
  host: "rui-mbp.local",
  api: "http://127.0.0.1:51021",
  cpu: 18,
  mem: "412 MiB",
  mem_pct: 24,
  queue_depth: 7,
  active_runs: 1,
  agents_running: 2,
  tokens_today: { in: "1.24M", out: "184k" },
  tokens_series: [18, 22, 30, 28, 36, 42, 38, 55, 61, 58, 64, 72, 80, 76, 84],
  events: [
    { t: "42s ago",  kind: "task",   msg: "task_08 wrote 3 files · 213 lines added", workflow: "user-auth", status: "running" },
    { t: "1m ago",   kind: "task",   msg: "task_09 dispatched to claude",            workflow: "user-auth", status: "running" },
    { t: "3m ago",   kind: "task",   msg: "task_07 completed in 12m 19s",            workflow: "user-auth", status: "done" },
    { t: "8m ago",   kind: "run",    msg: "run_2a9f progressed — 7/12 tasks done",  workflow: "user-auth", status: "running" },
    { t: "12m ago",  kind: "run",    msg: "run_2a9f started — 12 tasks queued",     workflow: "user-auth", status: "running" },
    { t: "15m ago",  kind: "task",   msg: "task_10 timed out after 10m",            workflow: "user-auth", status: "failed" },
    { t: "2h ago",   kind: "review", msg: "review round 002 fetched — 9 issues",    workflow: "manifest-v2", status: "open" },
    { t: "2h ago",   kind: "run",    msg: "run_18b3 completed · 6/6 tasks · 38m",   workflow: "manifest-v2", status: "done" },
    { t: "yesterday",kind: "archive",msg: "workflow warm-toast archived",            workflow: "warm-toast",  status: "archived" },
  ],
};

Object.assign(window, { PROVIDERS, WORKFLOWS, TASKS_USERAUTH, RUNS, SAMPLE_LOG, MEMORY_SHARED, MEMORY_TASK, SPEC_USERAUTH, REVIEW_ROUNDS, REVIEW_ISSUES, DAEMON });
