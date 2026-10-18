# Runtime Overview Illustrations — Tweets

One tweet per runtime overview illustration in `packages/site/public/images/runtime/`. Each post pairs with its `*-overview-storyboard-*.png` poster and links the reader to the matching docs section under `/runtime/core/<topic>`.

Voice: Pedro on X — informational opening (what Compozy does differently at a product level), then a tight mechanism paragraph with concrete file paths, CLI verbs, and exact numbers. Each opener has a different shape (imperative, declarative, gerund, numeric subject, product name, etc.) so the 14-post series doesn't read as one template.

---

## 1. Runtime

Underneath every Compozy session is the agent CLI binary you already trust. Claude Code, OpenClaw, and Hermes run unchanged, but every run becomes a durable unit with a stable id, replayable history, workspace-scoped memory, and one control surface across CLI, HTTP/SSE, and the web UI.

one local daemon under everything. start a session in the terminal, resume it from the web UI, inspect its events over HTTP. when the tab closes, the work doesn't.

---

## 2. `agents-overview-storyboard-v1.png` — Agents

Agents on Compozy have a different standard, they are self-contained folders instead of one single markdown file. They have SOUL.md, HEARTBEAT.md, AGENT.md and couple more features.

in Compozy, AGENT.md declares identity, provider, permissions, tools, mcp. discovery is first-wins: workspace > additional roots > global. one file, one ACP subprocess, fully supervised until stop.

---

## 3. `sessions-overview-storyboard-v1.png` — Sessions

Forget chat tabs. Every Compozy session is a stable id that survives crashes, supports attach + replay + audit, and owns its own SQLite events.db while running, plus a content-addressed jsonl ledger written after stop.

lifecycle is strict: starting → active → stopping → stopped. resume reattaches to the same id, no new subprocess, no rebuilt history. permission gates fire mid-flight (`deny-all`, `approve-reads`, `approve-all`), not just at start.

---

## 4. `workspaces-overview-storyboard-v1.png` — Workspaces

Move your project folder to a new machine and your workspace identity follows. Compozy stores a ULID inside `.compozy/workspace.toml`, and sessions, memory, skills, and MCP overlays all key on that ULID, not on the absolute path.

config layers strictly: global config.toml → workspace config.toml → AGENT.md → SKILL.md → mcp.json. additional roots stack for monorepos but only the primary root contributes config. registered root always wins over cwd.

---

## 5. `memory-overview-storyboard-v1.png` — Memory

No vector database. Compozy memory is just curated markdown plus FTS5 indexes, with three scopes (global, workspace, agent) and strict precedence. Every write routes through one controller WAL with idempotency keys, so crashes replay deterministically and you can `cat` or `git diff` the whole thing.

dreaming runs only after 4 gates pass: time (24h default), sessions (3 default), lock (one per workspace), signal (≥5 unpromoted + ≥0.75 score). agents use `compozy__memory_propose`, never raw writes. snapshots are frozen at session start; new writes show up next session.

---

## 6. `skills-overview-storyboard-v1.png` — Skills

A skill in Compozy is a directory with SKILL.md plus optional resources and an optional MCP sidecar, not a prompt block you paste into the system message. The daemon scans them across 6 precedence tiers and injects only a compact catalog at session start.

precedence: bundled → marketplace → user → additional root → workspace → agent-local. agents pull the full body on demand via `compozy__skill_view`. each catalog entry is just a name + 200-char description, so your prompt stays sane with 30+ skills installed.

---

## 7. `automation-overview-storyboard-v1.png` — Automation

Three doors, one dispatcher. Compozy automation gives you cron/interval/one-time schedules, runtime-event triggers, and signed webhooks, and every fire becomes a run that looks identical to a manual session from the control surface, with the same audit trail and the same event store.

triggers react to runtime events like `session.stopped`, `memory.consolidated`, `ext.*`. webhooks sign HMAC-SHA256 inside a 5min freshness window and dedup by delivery id. retries use exponential backoff. fire limits default to 12/hr. pre-fire hooks can cancel or mutate the prompt.

---

## 8. `autonomy-overview-storyboard-v1.png` — Autonomy

Creating a task records intent only. Nothing claims work, nothing spawns. Execution starts when somebody publishes, starts, or approves it, and from that point autonomous flows use the exact same tasks, runs, sessions, hooks as manual work.

one workspace coordinator per workspace. token-fenced leases for claim, heartbeat, complete, fail, release. the task service owns execution state. safe-spawn caps depth at 1 and children at 5, permissions must be a subset.

---

## 10. `hooks-overview-storyboard-v1.png` — Hooks

Each hook in Compozy binds exactly one named runtime event to exactly one executor, with deterministic ordering by source (native → config → agent → skill), then priority, then name. Sync hooks can mutate the payload; async hooks observe outcomes. Typed lifecycle dispatch, not a generic event bus.

executor receives the event on stdin as JSON, returns a JSON patch on stdout, 8KiB cap, 5s default timeout. patches apply in order. outcomes (applied / denied / failed / skipped / dropped) land on the session event stream. memory v2 plugs in via `session.message_persisted` and `session.post_stop`.

---

## 11. Extensions

One Compozy extension ships a whole bundle: skills, agents, hooks, MCP servers, and subprocess services together under one manifest, one enable/disable lifecycle, and one trust contract. No piecemeal package management.

three trust tiers: official (registry-verified, default-allowed), community (verified checksum), unverified (blocked without `--allow-unverified --yes`). subprocess code calls the daemon via JSON-RPC, gated by `[security.capabilities]`. marketplace extensions are constrained to a read-oriented capability ceiling.

---

## 13. `configuration-overview-storyboard-v1.png` — Configuration

Five files cover every Compozy configuration decision, with strict precedence between them. Global `~/.compozy/config.toml` → workspace `.compozy/config.toml` → `AGENT.md` → `SKILL.md` → `mcp.json`. TOML merges field-level; JSON sidecars replace whole objects. No more sprawl across yaml and random .env files.

secrets live in Vault as write-only `vault:<namespace>/...` refs, namespace-scoped (providers, automation, sessions/<id>). only 3 settings live-reload; everything else flags `restart-required`. `compozy config apply-history` tells you what's pending vs live, with `next_action` per change.

---

## 14. `operations-overview-storyboard-v1.png` — Operations

Two SQLite databases hold every Compozy operational truth: a global `compozy.db` catalog for sessions, workspaces, and automation, plus a per-session `events.db` for ordered event streams. Both ship with -wal/-shm companions that have to be backed up together.

three-signal triage clears most fires before going deeper: `compozy status`, `compozy doctor`, `$COMPOZY_HOME/logs/compozy.log`. corruption auto-recovery moves the bad DB to `.corrupt.<timestamp>` and retries fresh. lock-file singleton prevents two daemons stomping each other.

---

## Posting notes

- Attach the matching PNG from `packages/site/public/images/runtime/` to each tweet.
- Suggested doc link to append in the reply tweet (one per post):
  - 1 → `https://compozy.com/runtime`
  - 2 → `https://compozy.com/runtime/core/agents`
  - 3 → `https://compozy.com/runtime/core/sessions`
  - 4 → `https://compozy.com/runtime/core/workspaces`
  - 5 → `https://compozy.com/runtime/core/memory`
  - 6 → `https://compozy.com/runtime/core/skills`
  - 7 → `https://compozy.com/runtime/core/automation`
  - 8 → `https://compozy.com/runtime/core/autonomy`
  - 10 → `https://compozy.com/runtime/core/hooks`
  - 11 → `https://compozy.com/runtime/core/extensions`
  - 13 → `https://compozy.com/runtime/core/configuration`
  - 14 → `https://compozy.com/runtime/core/operations`
- These are long-form X posts (require X Premium for >280 chars). Add the doc link inline at the bottom or as a reply.

## Opener shape varieties

To avoid the "X on Compozy is/are…" template, each tweet uses a different opener shape:

| # | Topic | Opener shape | First words |
|---|---|---|---|
| 1 | Runtime | adverbial preposition | "Underneath every Compozy session is…" |
| 2 | Agents | noun + verb (user-authored) | "Agents on Compozy have a different standard…" |
| 3 | Sessions | imperative + declarative | "Forget chat tabs. Every Compozy session is…" |
| 4 | Workspaces | imperative action | "Move your project folder to a new machine…" |
| 5 | Memory | declarative fragment | "No vector database. Compozy memory is…" |
| 6 | Skills | indefinite article subject | "A skill in Compozy is a directory…" |
| 7 | Automation | numeric subject | "Three doors, one dispatcher." |
| 8 | Autonomy | gerund subject | "Creating a task records intent only…" |
| 10 | Hooks | distributive ("each") | "Each hook in Compozy binds exactly one…" |
| 11 | Extensions | numeral subject | "One Compozy extension ships a whole bundle…" |
| 13 | Configuration | numeric subject (different) | "Five files cover every Compozy configuration decision…" |
| 14 | Operations | numeric subject (different) | "Two SQLite databases hold every Compozy operational truth…" |

## Voice + humanizer audit

- **Two-paragraph shape** matching T2: sentence-case positioning paragraph (what Compozy does differently at the product level), blank line, lowercase mechanism paragraph (file paths, CLI verbs, exact numbers, algorithms).
- **Informational lift**: every positioning paragraph names a concrete distinction (markdown over vector db, identity over directory, additive over parallel, one dispatcher over three pipelines, typed lifecycle over event bus, one install over piecemeal) instead of being a take.
- **Mechanism density**: 3-6 hard runtime facts per tweet. Substance from the 14-subagent exploration: SOUL.md/HEARTBEAT.md alongside AGENT.md, 4 dreaming gates with exact thresholds, 6-tier skill precedence chain, lock-file singleton, .corrupt.<timestamp> recovery, `[security.capabilities]` Host API gate, `next_action` in apply-history, `compozy__memory_propose` agent-write path, frozen-snapshot memory semantics, only-3-settings live-reload.
- **Anti-patterns**: zero em dashes, zero AI vocab, zero hedging, zero banlist openers, no sycophancy/signposting/false-ranges/negative-parallelism overuse.
- **Format**: technical-insight, two-part structure. Char counts run ~300-520 (long-form X, requires Premium).
- **Topic palette**: all High-tier (founder talking about Compozy, agent orchestration, dogfooding).
