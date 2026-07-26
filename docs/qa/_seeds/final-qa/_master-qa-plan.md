---
name: final-qa-master-plan
description: Authoritative pre-release QA plan for AGH. Defines philosophy, execution order, evidence requirements, gate criteria, fixture/bootstrap, real-LLM inventory, forbidden-needle rollup, cross-module failure matrix, decision points. Index of all 283 numbered scenarios across 15 module child plans.
type: qa-master-plan
status: planning-complete · ready-for-execution
authoring_run: 2026-05-02
applies_to: AGH at HEAD; no production users; greenfield zero-legacy posture
language_policy: artifacts-en · conversation-brpt
---

# AGH Final QA — Master Plan

> **Single mission.** Prove that every load-bearing AGH behavior — runtime, contracts, autonomy, memory, skills, tools, extensions, automation, network, API+CLI, web, docs, observability — works end-to-end against real LLMs and real agents, end-to-end through CLI / HTTP / UDS, with every claim cited by `file:line` and every assertion backed by captured evidence. No glorified integration tests; no mocks pretending to be live; no controls in the UI that the daemon doesn't actually implement.

## Table of contents

1. Operating model (the philosophy this pass adopts)
2. Module map and authoritative scope boundaries
3. Real-LLM scenario inventory (all 283, by provider class)
4. Edge-case rollup (the things that break products silently)
5. DX cliffs rollup (the things that confuse users)
6. Cross-module failure matrix (link to `14-cross-cutting.md` §12)
7. Forbidden-needle list (mechanically verified)
8. Bootstrap, fixtures, isolation, credential broker
9. Execution order (dependency-correct, parallelizable lanes)
10. Decision points still open (BLOCK live lane until resolved)
11. Gate criteria (what "release-ready" means)
12. Reporting contract (artifact tree the executor produces)
13. Standing directive coverage proof (which SD each child upholds)
14. Lessons-learned coverage proof (which `L-*` each child covers)
15. Out of scope and intentional omissions

---

## 1. Operating model

The QA philosophy AGH is adopting is the synthesis of the openclaw QA framework (`_references/openclaw-qa-patterns.md`) and the hermes test discipline (`_references/hermes-qa-patterns.md`), translated to AGH's Go runtime + ACP subprocess + SQLite persistence + HTTP/SSE/UDS surfaces.

### 1.1 Core tenets

1. **Behavior first.** Every scenario asserts an observable product behavior, not the shape of an internal call. Operators run scenarios; they don't read mocks.
2. **Real-LLM where it matters.** Spawning Claude Code, OpenClaw, or Hermes via ACP and watching the JSON-RPC + SSE evidence is the canonical proof for any interactive surface. `mock-acp` is permitted only where determinism (claim-race, async-bridge, codec) is the actual property under test.
3. **Tri-state liveness.** Every scenario declares `live: true | false | conditional`. Live runs that need credentials use a pooled credential broker (openclaw pattern, AGH-local SQLite-backed implementation per `08-extensions-bridges.md` §6).
4. **Evidence is the artifact.** Every scenario lists what to capture: log path, db query, SSE stream snapshot, HAR (web), screenshots (web), goroutine snapshot (where goleak is asserted). Without evidence, "passed" is a claim, not a proof.
5. **`file:line` citations are mandatory.** Every behavioral claim in this plan and its children cites the implementation it's proving. If the citation rots, the citation rots in the plan, not in a hidden test.
6. **Hermetic by default, but never against the provider contract.** Every run uses an isolated `AGH_HOME`, daemon port, UDS socket, and Web proxy target. Bound-secret, brokered, and explicitly isolated-home lanes use isolated `PROVIDER_HOME` / `PROVIDER_CODEX_HOME`; `native_cli` lanes with `home_policy=operator` preserve the operator `HOME` / native login state unless the scenario explicitly validates isolated provider-home behavior. The bootstrap manifest is the source of truth (see §8).
7. **Truthful UI.** UI elements that the daemon doesn't actually serve must NOT render. The web QA includes a positive audit (`UI-19`) and the cross-cutting plan elevates this into a build-rejecting gate (`XCT-14`).
8. **Greenfield posture.** No backward-compatibility shims. No "soft" assertions. Failure means we delete the offending code or the offending test, not both.
9. **Two-touch rule.** If a child surfaces the same defect twice during execution, the third encounter triggers a TechSpec, not a third patch.
10. **No partial-surface completions.** Any QA-surfaced contract bug must be closed end-to-end (contract → HTTP → UDS → CLI → docs) in one commit; no transport-only fixes.

### 1.2 What this pass deliberately is NOT

- It is **not** a benchmarking pass. Benchmarks live in `extreme-software-optimization` and frontier-harness work; performance regressions are flagged but not tuned here.
- It is **not** a security audit replacement. `security-review` skill remains the canonical security workflow; this pass exercises the *runtime invariants* that security review proves are *correctly invariants in code*. The two complement.
- It is **not** an SDK conformance test for third-party agent runtimes (a separate AGH Network conformance suite per RFC 003/004 covers that).

### 1.3 Why two reference frameworks (and not just one)

- **openclaw** gives us the scenario anatomy, qa-channel, qa-coverage CLI, frontier vs regression separation, and the credential broker shape.
- **hermes** gives us the python-tested-discipline translated to Go: build-tag lanes, hermetic env, packaging-drift tests, atomic-replace-symlinks, OAuth round-trip with mocked transport, cache-isolation tests, source-text invariants, and the `tests/run_interrupt_test.py`-style "explicitly NOT a test runner test" pattern.

The two together give AGH a balanced execution model: openclaw-style scenarios where the unit of value is the operator journey, hermes-style hermetic Go-tag lanes where the unit of value is correctness under concurrency, signals, OS quirks, and crash recovery.

---

## 2. Module map and scope boundaries

| #  | Module | Owns | Does NOT own (cross-references) |
|----|--------|------|-------------------------------|
| 01 | daemon-boot | composition root, lock, boot pipeline, shutdown ordering, Goose schema streams, signals, subprocess lifetime, diagnostics + heartbeat | session lifecycle (→ 03), task claim semantics (→ 04), event coverage matrix (→ 15) |
| 02 | config-settings | TOML parse + merge + overlay, agent-def parsing, vault redaction, workspace resolver, frontmatter | settings projection over SSE (→ 11), session-snapshot config (→ 03) |
| 03 | acp-sessions | ACP client + JSON-RPC, session manager + state machine, transcripts, replay assembly, agentidentity, situation surface | task_run claim (→ 04), event ledger (→ 15), web rendering (→ 12) |
| 04 | autonomy-kernel | task_runs, ClaimNextRun, mechanical scheduler, hooks dispatch, coordinator, lease/sweep | session lifecycle (→ 03), automation/cron triggers (→ 09), tool deny/narrow (→ 07) |
| 05 | memory-soul | memory store + recall + provenance, consolidation cascade, agent soul, lifecycle hooks invocation order | hooks taxonomy + dispatch (→ 04), workspace scoping (→ 02) |
| 06 | skills-capabilities | skills catalog + loader + VerifyContent, registry, situation, resources projector | hook dispatch (→ 04), extension manifest (→ 08), tool resolution (→ 07) |
| 07 | tools-sandbox | tool registry + dispatch, toolruntime interrupts, sandbox profiles, MCP sidecars, path-security | hook deny/narrow (→ 04), skill provenance (→ 06) |
| 08 | extensions-bridges | extension manifest + install + host API, bundle activation, bridges (Slack/Telegram), bridge SDK | skills VerifyContent on extension load (→ 06), workspace scoping (→ 02) |
| 09 | automation-cron | cron expressions, webhook ingress, scheduled triggers, durable scheduler state, DST/timezone | task_run dispatch (→ 04), session spawn lineage (→ 03) |
| 10 | network-identity | Local/Live participation, durable conversations, bounded wakes, usage, identity authorization | task-run wake substrate (→ 04), public-surface parity (→ 11/12) |
| 11 | api-cli-parity | contract types, BaseHandlers, HTTP, UDS, CLI cobra, SSE replay, codegen | every endpoint's underlying domain (→ 01..10) |
| 12 | web-ui | React 19 SPA, TanStack Router, assistant-ui, Playwright e2e, accessibility, COPY.md adherence | API contract (→ 11), settings projection (→ 02) |
| 13 | docs-site | Fumadocs static export, MDX pipeline, OpenAPI doc rendering, CLI reference rendering, COPY.md/DESIGN.md adherence | OpenAPI source (→ 11), CLI source (→ 11) |
| 14 | cross-cutting | ≥3-module integration scenarios, cross-module failure matrix | composes children — owns nothing alone |
| 15 | observability | canonical event taxonomy + coverage matrix, claim_token grep gate, durable-append invariant, log discipline | event sources in every module (→ 01..14) |

The "Does NOT own" column is enforceable by `mage Boundaries` on the Go side and by code-search on the test side: a scenario asserting behavior outside its declared module without a `composes:` field is rejected at planning time.

---

## 3. Real-LLM scenario inventory

All 283 scenarios are categorized below by their `provider:` declaration. Operator MUST run each lane with the matching driver.

### 3.1 Live Claude Code (real `claude-opus-4-8` / `claude-sonnet-5` via ACP)

Approximately 180 scenarios. Examples (read each child for the complete list):

- **Daemon-boot live**: DB-04 (kill -9 mid-prompt + reclaim), DB-14 (detached lifetime via real curl abort)
- **Config live**: CFG-04..05 (hot-apply vs restart-required end-to-end), CFG-08 (multi-workspace isolation with real Claude in workspace A reading B's memory and failing)
- **ACP/Sessions live**: ACP-01 (Claude Code multi-tool prompt), ACP-04 (Last-Event-ID reconnect), ACP-05 (detached cancel proof), ACP-08 (lineage via spawn), ACP-09 (replay equivalence), ACP-19 (daemon-restart persistence)
- **Autonomy live**: AUT-09 (coordinator bootstrap dispatching to real Claude), AUT-12 (real lineage), AUT-15 (real-LLM end-to-end with cron overlap), AUT-18 (claim_token redaction sweep on real run)
- **Memory live**: MEM-01 (3-session feedback memory + agent uses it in turn 1 of session 4), MEM-08 (agent soul write-update-delete), MEM-12 (cross-workspace isolation), MEM-13 (stale-memory verification), MEM-16 (consolidation diff)
- **Skills live**: SKL-01 (bundled skill activates on prompt), SKL-09 (provenance in transcript), SKL-16 (`agh-design` skill citations)
- **Tools live**: TOL-01 (read/write/run roundtrip), TOL-02 (interrupt mid-execution), TOL-08 (secret redaction), TOL-17 (multi-step refactor)
- **Extensions live**: EXT-01 (install at runtime), EXT-15 (multi-tool real-LLM conversation)
- **Automation live**: CRN-01 (one-minute ping over 5 minutes with real Claude), CRN-12 (concurrent 50 cron jobs to real LLM), CRN-15 (real-LLM end-to-end with child spawn)
- **API/CLI live**: API-02 (CLI prompt parity), API-03 (UDS prompt parity), API-04 (HTTP prompt parity), API-18 (compose CLI + HTTP + tail)
- **Web live**: UI-02..05 (real chat through SPA), UI-09 (lineage tree), UI-12 (a11y on streaming), UI-15..17 (DX-cliff catches), UI-20 (Playwright real-LLM regression)
- **Cross-cutting live**: XCT-02..05, XCT-08, XCT-13 (≥3-module live integration paths)
- **Observability live**: OBS-02 (correlation key completeness), OBS-19 (10-minute conversation, 30 tool calls)

### 3.2 Live OpenClaw (`live: conditional` — opt in via credential)

Selected parity scenarios where openclaw must drive the same path as Claude Code: ACP-02, EXT-15 (Slack ↔ openclaw), NET-12 (daemon-authoritative identity with openclaw).

### 3.3 Live Hermes (`live: conditional`)

ACP-03 (parity proof) and a small number of cron + memory scenarios where hermes is on the critical path.

### 3.4 Live Slack/Telegram bridges (`live: conditional`, broker-pooled)

EXT-06, EXT-07, EXT-09, XCT-09. The credential broker contract is in `08-extensions-bridges.md` §6.

### 3.5 Mock-ACP (legitimate determinism)

Reserved for: claim races (AUT-01), async-panic containment (AUT-17), structural skill scenarios (SKL-06 static, SKL-07 trace, SKL-14 codec, SKL-17 fake clock, SKL-19 fixture, SKL-20 log-only), high-write-rate stress (MEM-17), lock-race determinism (MEM-05). These are the only legitimate mock paths.

### 3.6 No-ACP daemon-only

Daemon boot, lock, migration, signal handling, lifecycle hooks, observability internals, and parity-matrix audits don't need an ACP agent. Scenarios DB-01..03, DB-06..13, DB-15, MEM-09..10 (lifecycle hooks), OBS-01, OBS-09..14, API-01 (parity-matrix audit), API-07..09 (codegen drift), DOC-01..22 (docs build), and a subset of NET-* run without an LLM.

---

## 4. Edge-case rollup

The full edge-case inventory is per-child. The cross-cutting view to keep in mind during execution:

| Class | Examples (where to read) |
|-------|--------------------------|
| OS quirks | macOS `/private/var/folders` symlink canonicalization (SKL-08, TOL-07), Windows process-group fallback (DB-05), full disk during migration (DB-07 §5), fsync failure (DB-07 §5), EBUSY socket creation (DB §5) |
| Time | DST fall-back / spring-forward (CRN-10/11), timezone discipline (CRN-09), clock skew on event seq (OBS-16), heartbeat time-jump (DB §5) |
| Filesystem | null-byte path (TOL-04), URL-encoded traversal (TOL-05), Unicode normalization (TOL-06), symlink escape (TOL-07, SKL-07), atomic-replace-symlinks (hermes pattern, applied to skill+extension copies) |
| Encoding | UTF-8 BOM (CFG §5), trailing-newline missing (CFG §5), embedded tabs in TOML (CFG §5), embedded newlines in secrets (CFG §5), MDX with backtick-in-fence (DOC §5) |
| Concurrency | claim race (AUT-01), lock race (MEM-05), concurrent prompts on session (ACP-15), concurrent tool dispatch (TOL-16), concurrent cron triggers (CRN-12), concurrent config writes (CFG §5) |
| Crash recovery | kill -9 mid-prompt (DB-04), kill -9 between append + broadcast (OBS-03), kill -9 mid-cron-fire (CRN-05), kill -9 before lease expire (AUT-08) |
| Authentication | webhook unsigned (CRN-07), webhook replay (CRN §5), proof-stripped peer (NET-02), invalid proof (NET-03), unknown-key peer (NET-04), revoked bridge token (EXT-17), spoofed AGH_SESSION_ID (ACP-10) |
| Resources | very large prompt output >1MB (ACP-16), 10k events/s (OBS-16), 1k memory writes/s (MEM-17), oversize webhook (CRN-17), 1000 inbound bridge messages/min (EXT-16), 10k peer-card requests/min (NET-17) |
| Path security | full pattern-set: TOL-04..07, SKL-07..08, EXT-04, plus the load-time scan invariant SKL-04 |
| Daemon downgrade / upgrade | multi-binary upgrade (DB-10), version mismatch (ACP-17), cross-version peers (NET-09) |
| Schema | schema reopen (DB-09), `-wal`/`-shm` recovery (DB-08), append-only invariant (OBS-13) |

Operator MUST run §4 edge-case scenarios on Linux + macOS Apple-Silicon at minimum; Windows scenarios are gated behind cross-build (`GOOS=windows GOARCH=amd64 go build`) and a single Windows runner pass for DB-05.

---

## 5. DX-cliffs rollup

Per child, but the QA gate must catch at least the following classes:

| Class | Where it bites users | Catch in this plan |
|-------|---------------------|--------------------|
| Error message lacks context | "validation failed" without key path | CFG §7, DB §7 |
| Status output lies | `agh status` says healthy with stuck subprocess | DB-11, DB-12 |
| Raw secrets in logs/output | OPENAI_API_KEY, claim_token, vault values | TOL-08, OBS-04, OBS-05, CFG §7 |
| Truthful UI break | a control whose backend doesn't ship | UI-19, XCT-14 |
| Vocabulary drift | `recipe` / `workflow` / `procedure` / `playbook` for current AGH behavior | UI-18 (web COPY scrape), DOC-14 (MDX COPY scrape), site-copy task §1 |
| Help text rot | CLI help disagrees with cobra | DOC-05 (CLI reference render) + DOC-13 (bun gate) |
| Codegen drift | contract change without `make codegen` | API-07, API-08, XCT-15 |
| Restart-required missing flag | UI does not surface `restart_required` | UI-07, CFG-05 |
| Hot-install hot-vs-cold ambiguity | new skill not visible to live session | SKL-11 (open question §10) |
| Bridge auth-revoked silent | revoked token → silent retry storm | EXT-17 |
| `agh extension info` vs `status` | mismatch CLI verb in docs | DOC-05 + EXT §1 (canonical: `status`) |

---

## 6. Cross-module failure matrix

Authoritative version is `14-cross-cutting.md` §12 (15×15 grid keyed by misbehaving-module rows × trusting-module columns). High-coupling cells the QA executor MUST audit visually:

- daemon × session (orphan PIDs / orphan goroutines): DB-13 + ACP-12
- task × hooks (hook bypass attempt against lease primitive): AUT-06
- session × observe (missing correlation keys): OBS-02
- skills × extension (load-time VerifyContent): SKL-04 + EXT-12
- automation × task (cron-fired claim): CRN-13
- network × identity (proof-stripping): NET-02
- web × api-contract (truthful UI): UI-19 + XCT-14
- memory × hooks (lifecycle hook order): MEM-09
- config × settings (hot-apply vs restart-required misclassification): CFG-04 + CFG-05
- codegen × everything (drift): API-07 + API-08 + XCT-15

---

## 7. Forbidden-needle list (mechanically verified)

Every captured artifact (logs, SSE streams, transcripts, db rows queried, web HAR, web DOM, screenshots metadata, error payloads, settings views, channel messages) is grep'd against this list at the end of each child run. Any hit is a ship-blocker.

### 7.1 Always forbidden (regex)

- `agh_claim_[A-Za-z0-9_-]+` — raw claim_token (the canonical non-negotiable)
- `OPENAI_API_KEY=sk-[A-Za-z0-9]+` — provider API key
- `ANTHROPIC_API_KEY=sk-ant-[A-Za-z0-9]+` — Anthropic API key
- `xoxb-[0-9]+-[0-9]+-[0-9]+-[a-f0-9]+` — Slack bot token
- `xoxp-[0-9]+-[0-9]+-[0-9]+-[a-f0-9]+` — Slack user token
- `[0-9]+:AAH[A-Za-z0-9_-]+` — Telegram bot token shape (rough)
- `password\s*=\s*['"][^'"]+['"]` — literal password assignment
- `secret\s*=\s*['"][^'"]+['"]` — literal secret assignment
- `bearer\s+[A-Za-z0-9._-]+` (case-insensitive) — bearer token
- `-----BEGIN (RSA |EC |OPENSSH |DSA )?PRIVATE KEY-----` — leaked private key

### 7.2 Vocabulary forbidden (case-insensitive in user-facing artifacts only)

For describing CURRENT AGH artifacts (capabilities), the words `recipe`, `workflow`, `procedure`, `playbook` are forbidden. Allowed only when discussing OTHER agent ecosystems or comparing.

### 7.3 Test-only fixture string (must match the seeded fixture, not absent)

- `agh_claim_FAKE_QA_*` — the deliberately-seeded redaction test needle. ACP-18, AUT-16, OBS-04 each plant this needle and assert its absence in every output channel.
- `AGHQA-FAKE-SECRET-9c4e1a` — the deliberately-seeded vault redaction test needle (CFG-10).

The needle's *absence* is the proof; its *presence in any output* is a ship-blocker.

---

## 8. Bootstrap, fixtures, isolation, credential broker

### 8.1 The bootstrap manifest

Every QA run starts with `agh-qa-bootstrap`. It produces a `bootstrap-manifest.json` and `bootstrap.env` containing:

- `AGH_HOME` — unique directory under `.tmp/qa/<run-id>/agh-home`
- `AGH_DAEMON_PORT` — free port allocated for the run
- `AGH_UDS_SOCKET` — unique socket path
- `AGH_TMUX_SOCKET` — unique tmux-bridge socket path (bridge tests)
- `AGH_WEB_API_PROXY_TARGET` — derived from above for isolated Web QA
- `PROVIDER_HOME` — isolated provider state root for bound-secret, brokered, and explicitly isolated-home lanes
- `PROVIDER_CODEX_HOME` — isolated Codex root when the lane actually uses Codex-specific auth/config
- Pooled provider credentials (Slack/Telegram/etc.) leased via the broker (§8.4)

A fresh manifest per pass by default. A previous manifest is reused only when continuing the same active QA session/loop (per the deterministic-bootstrap directive).

### 8.2 Worktree isolation (parallel runs)

Concurrent runs MUST allocate isolated `AGH_HOME` + ports + sockets per the parallel-QA rule. The `agh-worktree-isolation` skill is the canonical helper. Default port use is forbidden when concurrency is signaled.

### 8.3 Provider-home isolation

Provider-backed live scenarios follow each provider's auth contract. Bound-secret, brokered, and explicitly isolated-home lanes point at `PROVIDER_HOME` and `PROVIDER_CODEX_HOME` derived from the manifest. `native_cli` lanes with `home_policy=operator` preserve the operator `HOME` / native login state unless the scenario explicitly validates isolated provider-home behavior.

### 8.4 Credential broker (openclaw pattern, AGH-local)

For live Slack/Telegram/Network scenarios that need credentials:

- **Storage**: SQLite under `.compozy/qa-broker.db` (AGH-local; not Convex; we do not pull a Convex dependency for QA).
- **Lease**: `qa-broker lease --capability slack-bot` returns a token + lease TTL. Returns `unavailable` if pool empty (scenario auto-skips with `live: conditional → live: skipped`).
- **Release**: lease is auto-released on scenario success; explicit release on failure to avoid lock-out.
- **Rotation**: revoked tokens are removed from pool at the broker, not by the scenario.
- **Default budget**: pool sized so one parallel-3 lane can run end-to-end.

The broker contract is detailed in `08-extensions-bridges.md` §6.

### 8.5 Test fixtures (workspaces, configs, seeds)

A `fixtures/` tree under `.tmp/qa/<run-id>/`:

- `workspaces/wsA/`, `workspaces/wsB/` — distinct workspaces for isolation tests
- `agents/{claude-code,openclaw,hermes}.toml` — agent definitions
- `skills/critical-bad/`, `skills/warning-bad/`, `skills/good/`, `skills/symlink-escape/` — skill rejection fixtures
- `extensions/{slack,telegram,malicious}/` — extension manifests
- `seeds/cron/{one-minute,single-run,past-at,dst-fallback}.toml`
- `seeds/secrets.env` — populated by broker; never committed; gitignored
- `seeds/needles.txt` — the forbidden-needle planting list (claim_token, secret, password)

### 8.6 Captured evidence layout

Per scenario, under `.tmp/qa/<run-id>/evidence/<child-id>/<scenario-id>/`:

- `daemon.log` — slog stderr capture
- `events.jsonl` — SSE/event stream from `/api/events?after_seq=0`
- `db.sql` — `sqlite3 events.db .dump` snapshots before+after
- `transcript.json` — assembled transcript
- `goroutines.txt` — pre/post `goroutine` dump where goleak asserted
- `har.json` (web) — Playwright HAR export
- `screenshots/` (web) — PNG dump
- `needles.report.json` — grep result for forbidden-needle list (MUST be empty)

---

## 9. Execution order

The plan is deliberately ordered so earlier children unblock later children (boot → contracts → workflows → integration → cross-cutting).

### Lane A — Foundations (sequential, ~hours)

1. `01-daemon-boot.md` (DB-01..15)
2. `02-config-settings.md` (CFG-01..16)
3. `15-observability.md` (OBS-01..20)

These must pass before any other lane is meaningful.

### Lane B — Agent-runtime parity (parallel after A)

4. `03-acp-sessions.md` (ACP-01..19)
5. `04-autonomy-kernel.md` (AUT-01..18)
6. `11-api-cli-parity.md` (API-01..18)

These three exercise the same runtime surface from different angles. Run in parallel via worktree isolation.

### Lane C — Knowledge + capability surface (parallel after A, fold into B's events)

7. `05-memory-soul.md` (MEM-01..19)
8. `06-skills-capabilities.md` (SKL-01..20)
9. `07-tools-sandbox.md` (TOL-01..17)

### Lane D — Extension and integration (parallel after C)

10. `08-extensions-bridges.md` (EXT-01..20) — bridges live: conditional
11. `09-automation-cron.md` (CRN-01..22) — DST scenarios pinned to fixed clock
12. `10-network-identity.md` (NET-01..21) — Local/Live, commit-first delivery, bounds, identity, isolation

### Lane E — Surface adherence (parallel after B+C)

13. `12-web-ui.md` (UI-01..20) — Playwright; real Claude Code daemon-side
14. `13-docs-site.md` (DOC-01..22) — static export; bun gate

### Lane F — Cross-cutting validation (sequential after all above)

15. `14-cross-cutting.md` (XCT-01..16) — composes upstream scenarios

### Lane G — Final audits (after F)

- Forbidden-needle rollup grep across `.tmp/qa/<run-id>/evidence/**` (§7)
- Coverage matrix verification (`15-observability.md` §11) — zero red rows
- Cross-module failure matrix walk (`14-cross-cutting.md` §12)
- SD-005 audit: at least one `live: true` scenario per module (XCT-16)

The grand-total wall time estimate (single executor, single host, no broker stalls): ~6–8 hours; parallel-3 lanes (A → B||C → D||E → F → G): ~3–4 hours.

---

## 10. Decision points still open

These came out of the research. The operator MUST pick a side before the live lane runs, then the chosen child gets a one-line update.

1. **Skill hot-install hot-vs-cold rule** — does a newly-installed skill become visible to an existing in-flight session, or only to subsequent sessions? Asked by SKL-11. Codepath: `internal/skills/store_runtime.go` cache TTL.
2. **Per-agent recall filtering** — `internal/memory/catalog.search` does not filter by `agent_name` today; the canonical "agent" memory scope is realized as a per-agent SOUL directory. MEM coverage gap §3 documents the gap; decide whether to (a) add filtering, (b) treat soul as the only agent-scope mechanism, or (c) ship as-is and document.
3. **Default scope for memory write** — RFC says `memory.scope` is per-agent; today no global config-level fallback. MEM §3 flags. Choose default.
4. **Webhook past-fire rule** — `at` triggers with `time <= now`: log-and-skip (current per `schedule.go:525-535`) vs immediate-fire vs reject. CRN-08 needs an explicit decision; current implementation is log-and-skip.
5. **DST spring-forward** — cron at 02:30 in spring-forward window: skipped or rolled forward? CRN-11 gates on the answer.
6. **v0 ↔ v1 network negotiation** — clean version-mismatch error vs negotiation handshake? NET-09 gates.
7. **Spawn depth cap** — `DefaultSpawnMaxDepth = 1` today (`internal/session/spawn.go:17-18`). The OBS-17 deep-lineage scenario was tuned to depth 1. Decide: keep at 1 (deny depth>1 with typed event), raise to 5/6, or make per-agent-overridable.
8. **`agh extension info` vs `agh extension status`** — canonical command name. Today the implementation has `status` (`internal/cli/extension.go:220`). EXT-09 picks `status` and recommends docs sync.
9. **Observability matrix red rows** — four canonical event names not 100% pinned by code-grep (memory write, health status change, ACP fresh-start fallback, bridge auth failure). OBS-01 produces `coverage_matrix.json`; the operator must close the four flags in the same commit that lands the live-lane fix.

Until these are decided, the affected scenarios run with `expected: TBD-decision-N` placeholders that the executor renders as "BLOCKED — decision N pending."

---

## 11. Gate criteria

A QA pass is **release-ready** only when ALL of the following are true:

1. **All 283 scenarios** ran with their declared provider class and produced an evidence directory.
2. **Forbidden-needle rollup**: zero hits across all evidence (§7).
3. **Coverage matrix** (`15-observability.md` §11): zero red rows.
4. **Cross-module failure matrix** (`14-cross-cutting.md` §12): every coupling-debt cell either green or filed as a known issue.
5. **SD-005 audit** (XCT-16): at least one `live: true` scenario per module ran successfully against a real LLM.
6. **`make verify`** is green at the QAd commit (codegen-check + bun-lint + bun-typecheck + bun-test + web-build + fmt + lint + test + build + boundaries).
7. **`make test-e2e-runtime`** + **`make test-e2e-web`** green at the QAd commit.
8. **Decision points (§10)**: zero unresolved.
9. **Two-touch rule**: no defect was patched twice during execution; if so, a TechSpec was opened and is referenced.
10. **Worktree isolation**: every parallel run produced a clean shutdown with no orphan PIDs / orphan sockets / orphan goroutines.

---

## 12. Reporting contract (executor produces)

The executor is `qa-execution`. It walks each child file and produces:

```
.compozy/tasks/final-qa/_runs/<run-id>/
├── manifest.json           # bootstrap manifest, run metadata, decisions snapshot
├── summary.md              # run-level summary, per-lane status
├── evidence/
│   └── <child-id>/<scenario-id>/  # see §8.6
├── needles.report.json     # roll-up of all needle scans
├── coverage-matrix.json    # snapshot from OBS-01
└── failures.md             # any scenario that failed, with link to evidence
```

The executor MUST NOT write outside `_runs/<run-id>/` during a run. The plan tree (`_master-qa-plan.md`, `_children/*.md`, `_references/*.md`, `README.md`) is read-only during execution.

---

## 13. Standing directive coverage

This plan satisfies the following standing directives (`docs/_memory/standing_directives.md`):

| SD | Requirement | Where this plan honors it |
|----|-------------|--------------------------|
| SD-001 | long-running session supervision | DB-13, ACP-12 (goleak), DB-04/08 (crash recovery) |
| SD-002 | greenfield-delete | XCT-12 (zero-legacy invariant), §1.1 tenet 8 |
| SD-003 | BR-PT/EN convention | this plan in EN; conversation in BR-PT |
| SD-004 | multi-LLM pipeline default | acknowledged; QA executor uses real Claude / openclaw / hermes only |
| SD-005 | real-scenario QA | §3 (180+ live scenarios), XCT-16 (audit) |
| SD-006 | forensic-first bug fixes | §1.1 tenet 5 + every child cites file:line |
| SD-007 | truthful UI > plausible UI | UI-19, XCT-14, §5 |
| SD-008 | composition-root discipline | DB-01 + integration §6 ↔ daemon-only composition |
| SD-009 | detached lifetime | DB-14, ACP-05 |
| SD-010 | extensible-and-agent-manageable design | API-01 (parity matrix), every child has an "agent path" sub-section |
| SD-011 | (per repo doc) | covered by §1 + §11 gate |

---

## 14. Lessons-learned coverage

Spot-check (the plan does not duplicate `docs/_memory/lessons/L-*.md` content; it references):

- **L-001 concurrency / API class** — covered by AUT-01, AUT-08, MEM-05, ACP-12, ACP-15, TOL-16, CRN-12
- **L-005 testing discipline** — every child has an "Existing Test Coverage Map" + "suspicious mocks" call-out (cf. DB §2.9)
- **L-006 autonomy architecture** — AUT-* + the authority-exclusivity audit AUT-14
- **L-007 persistence** — DB-07/08, OBS-13, schema reopen DB-09
- **L-009 spec authoring** — out of scope for QA but `cy-spec-preflight` is the source of authoring discipline

---

## 15. Out of scope and intentional omissions

- Performance benchmarks (frontier-harness work; QA flags regressions, doesn't tune)
- Third-party AGH Network conformance suite (separate test target per RFC 003/004)
- Marketplace integration tests (no real marketplace surface yet beyond local registry)
- Extension marketplace publishing flow (out of v0 scope)
- IDE integrations beyond CLI/HTTP/UDS surfaces
- Long-term durability (months of `runtime.db` accumulation) — annual scenario, not pre-release

---

## Appendix A — Scenario count by child (sanity check)

| # | Child | Mandatory | Optional | Total | Plan lines |
|---|-------|-----------|----------|-------|-----------|
| 01 | daemon-boot | 15 | 0 | 15 | 1,081 |
| 02 | config-settings | 16 | 0 | 16 | 937 |
| 03 | acp-sessions | 19 | 0 | 19 | 772 |
| 04 | autonomy-kernel | 16 | 2 | 18 | 1,131 |
| 05 | memory-soul | 17 | 2 | 19 | 1,509 |
| 06 | skills-capabilities | 18 | 2 | 20 | 1,344 |
| 07 | tools-sandbox | 17 | 0 | 17 | 1,100 |
| 08 | extensions-bridges | 18 | 2 | 20 | 1,291 |
| 09 | automation-cron | 20 | 2 | 22 | 1,367 |
| 10 | network-identity | 19 | 2 | 21 | 1,366 |
| 11 | api-cli-parity | 18 | 0 | 18 | 1,007 |
| 12 | web-ui | 20 | 0 | 20 | 1,273 |
| 13 | docs-site | 22 | 0 | 22 | 1,185 |
| 14 | cross-cutting | 16 | 0 | 16 | 1,285 |
| 15 | observability | 18 | 2 | 20 | 1,420 |
|   | **Totals** | **269** | **14** | **283** | **17,068** |

References add 1,542 lines. Master plan + README add the connective tissue. Plan tree total ≈ 19.5k lines, all evidence-first, all cited.

---

## Appendix B — How to extend this plan

When a new feature lands:

1. Write its TechSpec per `cy-spec-preflight` + `cy-create-techspec`.
2. Generate tasks per `cy-create-tasks` + `cy-tasks-tail-qa-pair` (the tail QA pair is the seed of this plan's next child).
3. Fold the new child into `_children/` with the next number; update the README table; update §2 module map; add scenarios to §3 if they're live-LLM; add forbidden needles to §7 if they introduce new secret classes.
4. Re-run §11 gate. The plan is a living artifact, not a one-shot.

---

End of master plan.
