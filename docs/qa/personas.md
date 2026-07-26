# Personas

Project personas for AGH QA. Derived from the seed catalog (`.agents/skills/qa-report/references/personas.md`) and grounded in AGH's real audience: people who run agent work — technical or not: founders, product people, writers, analysts, and developers — autonomous agents that manage that work through structured surfaces, and the humans who evaluate and approve the results. PRODUCT.md Users is the register authority: design and test for the least technical person who still owns real work. Personas are durable instance data — update when the audience changes, not per cycle. The `Persona Affected:` field in bug reports and the `persona` field in scenario files use each persona's `name`.

Four persona families share this tree: the **Loops** surface (Bruno / Lea / Marina / Ada / Sol), **Bridge operations** (Tessa / Maya / Omar, with Ada driving structured surfaces), the **Session experience** — the ACP agent conversation/transcript thread (Théo / Nia / Rafa, with Ada driving it headless and Sol/Marina as accessibility/mobile lenses) — and **Marketplace & acquisition** (Bruno as the mid-session acquirer, Ada as the agent plane, plus Vera / Iris below for the administrative and remote-operator roles the Marketplace program introduced). A persona is defined by its goal on a surface, not just its archetype: the same human can wear a different operating role on each surface. Cora (below) is the cross-surface plain-language lens — the roster's least technical person, re-walking hero journeys on every family's surface.

> **Mobile & accessibility coverage.** A dedicated mobile persona is not maintained because AGH's primary surface is a desktop web SPA + CLI; mobile is covered as a device *lens* on Marina (the read/approve surfaces are the realistic phone use — approving a merge gate, or glancing at a running session, between meetings). The **loop visual editor canvas is explicitly desktop-only** (DAG canvas, drag, inspector) — mobile is a recorded skip for that surface, not a gap. Accessibility is a first-class persona (Sol), whose lens extends over the redesigned session thread (live SSE announcements, status never color-only, reduced-motion streaming pulse — see J-13/CH-020).

---

## Bruno — Delivery Builder (primary)

```yaml
persona:
  name: Bruno
  base: Power User
  goal: "Run software-delivery daily to drive already-authored tasks to a verified, reviewed, merged finish — and trust it stopped for the right reason."
  device: desktop
  network: wifi-fast
  modality: mouse-keyboard
  locale: en-US
  patience_seconds: 20
```

- **Who:** the developer who replaced `compozy tasks run` with the `software-delivery` Loop. Runs Loops many times a day, keeps the run page and CLI open side by side, knows the overrides and the ceilings.
- **What they reveal:** false `done` on an exhausted/stalled run (the trust-killer), meter drift, speed regressions in the run form, pause/resume/stop that lies about state, configure/fork friction, override clamps that don't hold, and partial task/automation catalogs presented as complete.
- **Owns journeys:** J-01 arrive-and-use, J-02 dry-run, J-04 pause/resume, J-05 configure, J-06 fork-and-edit, J-08 watch-and-maintain, J-10 converse-and-decide, J-24 triage-work-at-scale, and J-diagnose-task-session-health. **Goal:** J-26 controls, J-27 editor, and J-28 context/budgets.

## Lea — First-time Adopter

```yaml
persona:
  name: Lea
  base: New User
  goal: "Evaluate whether Loops replaces my manual orchestration — run a default dev-cycle Loop once and decide if it's worth adopting."
  device: laptop
  network: wifi-fast
  modality: mouse-keyboard
  locale: en-US
  patience_seconds: 60
```

- **Who:** a Compozy user meeting Loops for the first time. Arrives at the catalog, expects arrive-and-use to be **no harder than Compozy today** — if running a default-enrolled `dev-cycle` Loop is one step harder, she abandons and the design failed (use-cases §2, PRD Time-to-value).
- **What they reveal:** onboarding friction on the catalog → run-form → run path, unclear primary action, confusing input form, the "what will this cost / how do I stop it" first-run anxiety, empty states.
- **Owns journeys:** J-01 arrive-and-use, J-02 dry-run; **J-26 first conversational Goal/replace/draft**.

## Marina — Reviewer / Evaluator

```yaml
persona:
  name: Marina
  base: Casual User
  goal: "Watch a Loop's live progress, judge whether the work actually completed and was verified, and approve the merge gate — often from my phone between meetings."
  device: phone-large
  network: 4g
  modality: touch
  locale: en-US
  patience_seconds: 40
```

- **Who:** the team lead / evaluator (PRD secondary persona). Doesn't author Loops; she scans the global **Runs** "Awaiting you" queue, opens a run, reads the contract + outcome, and approves or requests changes. Frequently mobile — the approval gate is the realistic touch surface.
- **What they reveal:** truthful-outcome trust (is a waiting run shown as waiting, not done?), approval routing correctness, mobile layout of the run page / approval card / Runs KPIs, discoverability of the "needs a look" queue, start-binding attach flow.
- **Owns journeys:** J-03 observe-and-approve, J-09 automation-start-bindings, J-08 watch-and-maintain (evaluator view); **J-27 mobile Goal observation/discovery**.

## Ada — Autonomous Agent (structured surfaces)

```yaml
persona:
  name: Ada
  base: Power User
  goal: "Discover a Loop, supply its declared inputs, run it, and monitor it to a terminal outcome entirely through structured tool output — no human, no web UI."
  device: desktop
  network: wifi-fast
  modality: native-tool  # non-human ACP actor; deliberate extension of the seed enum (mouse-keyboard|touch|screen-reader|keyboard-only|voice)
  locale: en-US
  patience_seconds: 5
```

- **Who:** an ACP agent (PRD primary persona "Autonomous agent") driving Loops via `agh__loop_*` native tools over CLI/HTTP/UDS. **Ada is a non-human actor** — QA role-plays her to verify AGH's agent-manageability premise: every web action has a structured equivalent, output is deterministic, and the capability gates hold. Zero patience for ambiguous or non-parseable output.
- **What they reveal:** CLI↔HTTP↔UDS↔native-tool parity gaps, status values that don't map 1:1 to the 11-state enum, coercion in structured output, the approve capability gate (an agent must not approve its own gate), `Unavailable(ReasonDependencyMissing)` contracts before the service is ready, non-deterministic `ReasonCode`s`. **On the session surface** (session-improvements program): bounded REST tail/older pages, stable pagination cursors, cold bounded snapshots, fenced reconnect via `after_sequence` + `epoch`/`generation`, explicit reset reasons, empty-delta cursor advancement, keep-alive cadence, byte-identical `frames=raw` follow, and list/detail/status lifecycle parity through spawn→background→stop→restart. **On bridges:** strict JSON setup, HTTP/UDS parity, explicit skipped checks, deterministic exit codes, and a complete setup with no TTY or browser.
- **Owns journeys:** J-07 agent-operated-run (Loops); **J-15 operate-session-via-cli-api** and **J-diagnose-task-session-health** (session experience — the Automation Agent role in `_qa.md` §2 maps to Ada; not a new persona); **J-connect-bridge-provider** and **J-diagnose-repair-bridge** (structured bridge operation). **Goal:** J-29 structured operation and recovery.

## Sol — Accessibility-Reliant User

```yaml
persona:
  name: Sol
  base: Accessibility-Reliant
  goal: "Operate Loops without a mouse — run, observe, approve, and configure — using a keyboard and a screen reader, on equal terms."
  device: desktop
  network: wifi-fast
  modality: screen-reader
  locale: en-US
  patience_seconds: 45
```

- **Who:** a person who relies on VoiceOver/NVDA and keyboard-only interaction. AGH's truthful-UI rule ("color carries state") is an accessibility risk if state is signalled by **color alone** — Sol is the leash that keeps status legible without sight.
- **What they reveal:** status pills that are color-only (the 11 states must be announced/labelled, not just tinted), focus traps and escape in the Configure sheet + approval dialog, reduced-motion honored on the running/watching pulse, keyboard reachability of the editor canvas and its inspector, unannounced live SSE updates (dynamic content), missing labels on auto-generated input fields.
- **Owns journeys:** cross-cutting a11y lens on J-03 observe-and-approve and J-05 configure (see CH-011); also informs J-01 run-form and J-06 editor; **model-selector:** a11y lens on J-17 start-a-session-through-the-unified-runtime-selector (see CH-034). **On the session surface:** cross-cutting a11y lens on J-13 follow-a-live-run (see CH-020) — live SSE updates must be announced, the 11 lifecycle states legible without color, the streaming/working pulse gated by reduced-motion, and the redesigned tool rows/composer keyboard-reachable. **On Goal:** J-27 chip/timeline/editor accessibility (CH-040).

---

# Everyday Delegation persona (cross-surface)

Added 2026-07-22 with the people-first register (PRODUCT.md Users): AGH's primary audience includes non-technical people who delegate real work to agents. Cora is the roster's least technical person — the plain-language leash every hero surface must survive.

## Cora — Non-technical Founder (delegates work)

```yaml
persona:
  name: Cora
  base: Casual User
  goal: "Hand real work to agents from the web desktop — start it, see what is running, what needs me, and what finished — and act on it without ever opening a terminal or learning runtime vocabulary."
  device: laptop
  network: wifi-fast
  modality: mouse-keyboard
  locale: en-US
  patience_seconds: 30
```

- **Who:** a solo founder doing marketing, research, and ops work who runs agents weekly through the web UI only. No CLI, no protocol knowledge, no appetite for either. She judges every screen by one test: can she answer "what is running, what needs me, what finished, what did it produce" in seconds, in her own words.
- **What they reveal:** runtime jargon or raw enums/ids as primary text, states that require protocol knowledge to interpret, controls with unclear consequences, error copy that names internals without a next step, approval prompts that assume terminal context, signal-color noise that reads as alarm on a healthy screen, empty states that don't say what to do next, and any hero flow where the plain-language read fails while the expert read works.
- **Owns journeys:** cross-cutting plain-language lens on the hero journeys — J-01 arrive-and-use, J-03 observe-and-approve, J-10 converse-and-decide, J-11 return-to-running-session, and J-12 open-session-fast. Charters pair her with the owning persona's mission re-run in plain-language mode; she never needs a journey of her own to block a release.

---

# Bridge Operations personas

Personas for the Hermes bridge-parity cycle. These roles cover the user who connects a provider,
the teammate who experiences agent work inside a channel, and the operator who keeps several
providers healthy. Ada above owns the non-human structured-output lane. Security is applied as a
tour lens across these personas, not modeled as a separate user.

## Tessa — First-time Bridge Operator

```yaml
persona:
  name: Tessa
  base: New User
  goal: "Connect one external provider to an AGH workspace and see the first real agent response without having to infer a missing dashboard or CLI step."
  device: laptop
  network: wifi-fast
  modality: mouse-keyboard
  locale: en-US
  patience_seconds: 90
```

- **Who:** a developer or operations generalist connecting Slack, WhatsApp, Telegram, Discord,
  Teams, Google Chat, GitHub, or Linear for the first time. She can follow provider-console
  instructions but does not know AGH's bridge lifecycle yet.
- **What they reveal:** time-to-first-message friction, wizard false-accepts, missing credential
  provenance, unclear public-to-local webhook mapping, non-actionable verification, and Web
  checklist state that looks complete before the daemon can prove it.
- **Owns journeys:** J-connect-bridge-provider, J-diagnose-repair-bridge, and J-complete-web-bridge-setup.

## Maya — Channel Teammate

```yaml
persona:
  name: Maya
  base: Casual User
  goal: "Ask the team agent for help, understand what it is doing without channel spam, and correct or contextualize my request in the same thread."
  device: laptop
  network: wifi-slow
  modality: mouse-keyboard
  locale: en-US
  patience_seconds: 30
```

- **Who:** a teammate who never administers the bridge. She encounters AGH only through a channel,
  issue, or agent-session thread and judges it by routing, readable progress, and the final answer.
- **What they reveal:** progress spam, rate-limit stalls, typing that never clears, cross-thread or
  cross-user misrouting, progress chrome in transcript history, edits ignored as new messages, and
  reply context attributed to the wrong author.
- **Owns journeys:** J-watch-agent-work-channel and J-edit-reply-context.

## Omar — Bridge Fleet Operator

```yaml
persona:
  name: Omar
  base: Power User
  goal: "Keep an eight-provider bridge fleet predictable through long replies, provider limits, credential failures, and daemon restarts."
  device: desktop
  network: wifi-fast
  modality: mouse-keyboard
  locale: en-US
  patience_seconds: 20
```

- **Who:** the operator responsible for steady-state bridge reliability. He uses CLI/HTTP health,
  metrics, fake-provider labs, and provider consoles to distinguish AGH failures from upstream
  policy or credential failures.
- **What they reveal:** broken markdown or UTF-16 boundaries, missing or duplicate chunks, silent
  half-answers after restart, non-durable metrics, secrets in progress previews, redirect/SSRF
  mistakes, and check results that mutate lifecycle state.
- **Owns journeys:** J-diagnose-repair-bridge, J-deliver-long-formatted-reply, and J-recover-mid-turn-restart.

---

# Session Experience personas

Personas for the session-improvements program (the ACP agent conversation/transcript thread — a distinct surface from Loops). Introduced from `_qa.md` §2 and grounded in AGH's session audience: people running long-lived background agent sessions, first-time viewers judging AGH in ten seconds, reviewers auditing finished transcripts, and headless agents driving sessions over CLI/HTTP/UDS. The **Automation Agent (CLI/API)** role is Ada above (extended), not a new entry — do not duplicate.

## Théo — Returning Session User (session hero)

```yaml
persona:
  name: Théo
  base: Power User
  goal: "Return to a long-lived background agent session — via tab restore, session list, or permalink — and immediately see my persisted conversation, current and truthful, with the live run resuming. One blank thread on return is trust damage."
  device: desktop
  network: wifi-fast
  modality: mouse-keyboard
  locale: en-US
  patience_seconds: 15
```

- **Who:** the developer who keeps several agent sessions running across workspaces, backgrounds the tab for minutes-to-hours, and comes back expecting their work intact. The session-surface counterpart to Bruno. **Hero persona for blank-on-return** (the program's headline bug).
- **What they reveal:** blank/`ThreadEmpty` thread while persisted messages exist (the trust-killer), false `running`/`done`/`stopped` badges, silent permanent blank after a transient transcript 5xx, source-flip races on remount, fenced SSE reconnect gaps, workspace-switch redirects with no explanation, and Network reads/actions that bleed through a lagging active-workspace state.
- **Owns journeys:** J-11 return-to-running-session (hero), J-13 follow-a-live-run, and J-23 return-to-network-work.

## Nia — First-time Session Viewer

```yaml
persona:
  name: Nia
  base: New User
  goal: "Open a session cold — a deep link from a teammate or a first click into the list — and read what the agent did within the first ten seconds, without spinners, double-flashes, or a blank pane."
  device: laptop
  network: wifi-fast
  modality: mouse-keyboard
  locale: en-US
  patience_seconds: 60
```

- **Who:** someone meeting a session for the first time (shared permalink, or first list click). Judges AGH by the open. The session-surface counterpart to Lea; hypersensitive to first-impression friction.
- **What they reveal:** double-spinner flashes, more than one loading phase, unbounded full-history fetches that stall the open, permalink that resolves twice, unclear not-found state for an unknown session id, a cold open that reads as "empty" before it paints.
- **Owns journeys:** J-12 open-session-fast (primary); regression canary on J-11 adjacents (create/attach/approve/workspace, CH-019).

## Vera — Policy Administrator (marketplace & acquisition)

```yaml
persona:
  name: Vera
  base: Power User
  goal: "Own the runtime's acquisition trust posture: keep unverified side-loads blocked by default, flip policy deliberately when a team needs an exception, govern the curated-catalog configuration, and prove a pulled entry actually disappears."
  device: desktop
  network: wifi-fast
  modality: mouse-keyboard
  locale: en-US
  patience_seconds: 30
```

- **Who:** the administrator-operator the Marketplace PRD names as policy owner. She rarely acquires capabilities herself; she configures `Settings › Extensions` (registry, base_url, `allow_unverified`), the `[marketplace.catalog]` keys, and reacts to curation incidents (kill-switch verification). Distinct from Bruno: her surface is administrative settings and config lifecycle, not the acquisition scene.
- **What they reveal:** policy flips that don't apply live, blocked affordances that don't explain themselves or point at the wrong settings page, two-level consent collapsing into one, digest-mismatch installs that leave residue, config validation that accepts garbage or loses the prior good value, settings-split leakage (extension policy resurfacing on the Hooks page), and pulled catalog entries that remain visible past TTL.
- **Owns journeys:** J-extension-policy-admin.

## Iris — Remote Operator (away from the daemon host)

```yaml
persona:
  name: Iris
  base: Power User
  goal: "Operate a daemon that runs on another machine: install and authorize a remote MCP server even though the OAuth callback can never reach my browser, using the copyable link and manual code/redirect paste-back."
  device: laptop
  network: wifi-slow
  modality: mouse-keyboard
  locale: en-US
  patience_seconds: 45
```

- **Who:** the operator whose AGH daemon lives on a homelab box, VM, or remote dev server while her browser and terminal are local. The ADR-011 authorization floor exists for her: the always-copyable authorization URL, the browser-optional flow, and the manual `exchange` completion path (paste a code or the full redirect URL). She also verifies the non-loopback hardening — the auto-callback must refuse to exist on her deployment.
- **What they reveal:** dead or non-copyable authorization links, success toasts before the credential is confirmed present (`authenticated && token_present`), manual paste fields that reject full redirect URLs or echo secrets, expired/superseded PKCE sessions with unhelpful errors, failed exchanges that destroy a previously valid token, and remote flows that silently assume a local browser.
- **Owns journeys:** J-mcp-authorize-repair (manual-completion lane).

## Rafa — Transcript Reviewer

```yaml
persona:
  name: Rafa
  base: Casual User
  goal: "Audit a finished long transcript tool call by tool call — expand groups and turn folds, inspect each tool's structured Input/Output, copy message source, and trust the usage numbers."
  device: desktop
  network: wifi-fast
  modality: mouse-keyboard
  locale: en-US
  patience_seconds: 45
```

- **Who:** the reviewer/auditor who reads the longest finished sessions in the corpus for review or compliance. Cares about the transcript *UI language* rewrite (tasks 25–33, 36–37): grouping, inline inspection, copy affordances, truthful usage. Related to Marina's reviewer archetype but desktop and transcript-deep, not mobile approval.
- **What they reveal:** ungrouped 44px tool-call cards, output hidden behind default-closed chips, missing `+N previous tool calls`/turn folds, lossy or missing inline Input/Output, no copy affordance, a permanently-empty Usage tab presented as real data, false success/danger glyphs, gaps/duplicates when paging older history, and knowledge catalogs that hide old entries or retain ghost headers after interrupted derived synchronization.
- **Owns journeys:** J-14 read-a-finished-transcript (primary) and J-25 browse-and-recover-knowledge.

---

# Runtime Administration persona

Added for the hermes-comparison program (2026-07-19), which introduced a real installation-administrator audience: daemon lifecycle (drain/undrain), doctor and memory observability, the default-on redaction posture, and spend provenance. Vera owns acquisition *policy*; Dora owns the *runtime installation*. The hermes user-story persona "Administrator" maps to Dora; "Operator" maps to Théo (session surface) or Bruno (delivery/automation surface); "Autonomy operator" maps to Bruno; "Managed agent" and "External integrator" map to Ada (structured, non-human lane — the external MCP client is Ada driving through a third-party client instead of native tools).

## Dora — Runtime Administrator

```yaml
persona:
  name: Dora
  base: Power User
  goal: "Keep one AGH installation trustworthy: drain before deploys without killing in-flight work, read truthful doctor/status/memory evidence, keep secrets out of every log and stream, and see real spend — never a fake dollar amount."
  device: desktop
  network: wifi-fast
  modality: mouse-keyboard
  locale: en-US
  patience_seconds: 25
```

- **Who:** the person who owns the daemon: restarts and deploys it, reads `agh status`/`agh doctor`, sets config keys, and answers for the security posture and the bill. Operates mostly through CLI/HTTP/UDS with the Web settings pages as secondary surface.
- **What they reveal:** drains that kill in-flight work or lie about state, doctor items that disagree across HTTP/UDS/CLI, memory reports presented as something they are not, secrets surviving in logs/SSE/event stores, redaction toggles that silently no-op, estimated cost rendered as actual spend, and dead sidecars hammered forever or requiring a restart to recover.
- **Owns journeys:** J-drain-daemon-safely, J-keep-secrets-contained; co-owns J-offer-runnable-capabilities (dead-entity half, with Ada).
