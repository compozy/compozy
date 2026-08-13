# Hermes & OpenClaw Hype Templates → Compozy Adaptations

**Date**: 2026-05-28
**Source**: X recent search (last ~7 days, the standard-tier API ceiling)
**Voice**: drafted under `x-pedro-voice` (Pedro Nauck). Re-run the voice gate (`Workflow A`) before publishing.

## Honesty note (replaces a worse first draft)

The first version of this file invented features. It claimed an "compozy 0.0.5" release, a "background task panel with scroll position restore", Bitwarden Secrets Manager support, Krea 2 built-in, and specific PR-log items that don't exist. None of those are in the codebase. The latest tag is `v0.0.4` and main has 0 commits past it at the time of writing.

This rewrite anchors every Compozy-side claim on a code reference. Adaptations that would require a feature Compozy doesn't ship today are marked **TEMPLATE BLOCKED** with what would have to land first. Adaptations that hold up against the current code are marked **SHIPPABLE TODAY**.

## Verified Compozy surface area (used as the only source of truth below)

| Primitive | Where | Notes |
|---|---|---|
| Hero hook | `COPY.md § 2 — Positioning Snapshot` | "An open workplace for AI agents." |
| One-liner | `COPY.md § 2` | "Compozy is a local-first agent operating system: one daemon for durable agent sessions, one control surface for humans and agents, and one open network for agent-to-agent coordination." |
| Latest tag | `git tag` | `v0.0.4`, 0 commits since on `main`. |
| Hosted ACP agents (featured trio) | `packages/site/components/landing/hero.tsx:7` | `["Claude Code", "OpenClaw", "Hermes"]` |
| Providers wired | `internal/config/provider.go` | claude, anthropic, openrouter, xai (grok), qwen, openai, blackbox, codex, mistral, groq, hermes, openclaw, opencode, gemini |
| Workspaces | `internal/workspace/workspace.go:36–77` | RootDir, AdditionalDirs, Name, DefaultAgent, SandboxRef; RuntimeResolver with Resolve/ResolveOrRegister |
| `workspace_id` propagation | `internal/network/envelope.go:200, 260` | Required field on network envelopes; ConversationRef validation enforces it |
| MCP integration | `internal/mcp/hosted.go:29–60+` | `compozy-hosted-tools` session-scoped MCP service, nonce-authed bind, tool invocation via canonical ToolID registry |
| Autonomy kernel | `internal/task/lease_manager.go:14–53` | `ClaimNextRun` atomic claim with pre/post hooks, lease TTL, `HeartbeatRunLease` |
| Hook taxonomy | `internal/hooks/events.go:6–143` | 16 families, 60+ events |
| Peer card | `internal/network/envelope.go:347–355` | PeerID, DisplayName, ProfilesSupported, Capabilities, ArtifactsSupported, TrustModesSupported, Ext |
| `compozy-network/v0` | `internal/network/envelope.go:12` | 6 message kinds: greet, whois, say, capability, receipt, trace |
| Skills vs Extensions | `internal/registry/types.go:8–11, 38–46` | PackageType enum: `skill` vs `extension`; bundles not a separate registry type |
| CLI verbs (real) | `internal/cli/*.go` | agent, workspace, automation, network, catalog, mcp, hooks, bundle, claim/heartbeat/complete, plus more |

Anything below that cites a primitive not on this table is fabricated. If you spot one, treat it as a bug in this file.

---

## The 20

### 1. release-bullets — flagship feature drop

**Source** (@NousResearch, 2026-05-27 · 2,092 ❤ · 141 🔁 · 34 quotes · 887k impressions · ID 2059638198075109769)
URL: https://x.com/NousResearch/status/2059638198075109769

> Hermes Agent now has a built-in MCP Catalog

**Hype mechanic**: bare claim, no setup, the attached video does the heavy lifting.

**TEMPLATE BLOCKED**: Compozy integrates MCP via `compozy-hosted-tools` (`internal/mcp/hosted.go`) but there is no user-facing **catalog** UI verified. Don't claim catalog parity until a discoverable catalog ships.

**SHIPPABLE TODAY** (different angle, same primitive):
> mcp inside compozy is workspace-scoped, not agent-scoped.
>
> register the server once, every hosted agent in that workspace can call its tools through `compozy-hosted-tools`. one nonce per session, no key juggling per agent.

**Voice check**: format=`two-part`. No em-dash, no banned opener, 0 emojis, no rule-of-three.

---

### 2. release-bullets — integration drop with operator setup

**Source** (organic, 2026-05-28 · ID 2059790511184171446 · the operator-recipe template downstreams reuse)
URL: https://x.com/i/web/status/2059790511184171446

> Hermes just added Qwen 3.7 Max.
> Here's the setup:
> → Run hermes update.
> → Open your terminal.
> → Type hermes model.
> → Select OpenRouter.

**Hype mechanic**: feature claim + 4-step terminal recipe pasteable in under a minute.

**SHIPPABLE TODAY** — OpenRouter and Qwen are real providers in `internal/config/provider.go`. The CLI shape needs to be sanity-checked against the actual `compozy agent` subcommands before posting (run `compozy agent --help` and adjust):

> compozy hosts qwen 3.7 max through openrouter today.
>
> rough setup (verify against `compozy agent --help`):
> → install compozy
> → register your openrouter key
> → add the agent into your workspace pointed at qwen-3.7-max
>
> same workspace, same memory, new brain.

**Voice check**: format=`release-bullets`. Hedging on the exact CLI is honest — replace the bullets with the real verbs before shipping.

---

### 3. release-bullets — provider built-in announcement

**Source** (@NousResearch, 2026-05-27 · 572 ❤ · 38 🔁 · ID 2059730199344816407)
URL: https://x.com/NousResearch/status/2059730199344816407

> Krea is now built in to Hermes Agent as an image generation API provider…

**TEMPLATE BLOCKED**: Krea is not in the Compozy codebase. `grep -ri krea` returns no matches. Don't fake parity.

**Reframe options if you want to post in this slot**:
- Wait until an image-gen provider is actually wired, then use this template verbatim against the real one.
- Or pivot to a verified provider that's already there (xAI / Grok image, if/when that lands in Compozy; or a model that just got added to `provider.go`). Check `internal/config/provider.go` first.

---

### 4. release-bullets — partner secrets integration

**Source** (@NousResearch, 2026-05-22 · 1,508 ❤ · 92 🔁 · 27 quotes · ID 2057879204490883278)
URL: https://x.com/NousResearch/status/2057879204490883278

> Hermes Agent now supports the @Bitwarden Secrets Manager

**TEMPLATE BLOCKED**: Bitwarden is not in the Compozy codebase. Don't fake the co-brand.

**Reframe**: Compozy does have `internal/providerauth` and `bound_secret` / `native_cli` / `none` auth modes per `internal/config/provider.go`. If you want a secrets-management tweet, write it about that existing surface instead of a vendor partner:

> compozy splits provider auth into three modes per provider: native_cli, bound_secret, none.
>
> claude code keeps its operator login. brokered providers get scoped secrets. nothing leaks across workspaces.

**SHIPPABLE TODAY** for the reframe above (still verify the three mode names against current `internal/config/provider.go` since they may have drifted).

---

### 5. short-reaction — community reframe joke

**Source** (@NousResearch reply, 2026-05-26 · 1,548 ❤ · 26 🔁 · ID 2059376554153566514)
URL: https://x.com/NousResearch/status/2059376554153566514

> But how many Hermes Agents?

**Hype mechanic**: 5-word reply that flips a user-count question into an agent-count question.

**SHIPPABLE TODAY** (reply / quote-reaction, no feature claim attached):
> right but how many of them are agents 🤯

**Voice check**: format=`short-reaction`. 1 emoji from approved set. No feature claim, so no fabrication risk.

---

### 6. single-block — community event drum-up

**Source** (@NousResearch, 2026-05-25 · 820 ❤ · 56 🔁 · ID 2058968426110922938)
URL: https://x.com/NousResearch/status/2058968426110922938

> Join the team on Wednesday for another Hermes Agent Jam!

**SHIPPABLE TODAY** only if Compozy actually runs an open-hour ritual. If there isn't one yet, this is a **TEMPLATE BLOCKED** that needs the ritual to exist first.

**Draft (assuming the ritual exists or you're starting one)**:
> compozy open hour wednesday 6pm BRT.
>
> bring your workspace, your weirdest hook setup, whatever you're dogfooding. discord link in bio.

**Voice check**: only ship after the discord and the recurring slot are real.

---

### 7. two-part — live-event headcount flex

**Source** (@Teknium, 2026-05-27 · 196 ❤ · 14 🔁 · ID 2059701141483782334)
URL: https://x.com/Teknium/status/2059701141483782334

> 384 people in the Hermes Agent Jam Session in our discord, happening now!

**TEMPLATE BLOCKED** until Compozy has both the recurring jam **and** a real headcount + a real demo to point to. Don't pre-fake either.

**When ready**:
> {real headcount} people in the compozy open hour right now.
>
> someone's demoing parallel openclaw runs routing through one workspace. another is showing an mcp server bound to a session. come watch.

(Only post if the demos actually happened.)

---

### 8. question — founder ops register

**Source** (@steipete, 2026-05-26 · 339 ❤ · 11 🔁 · ID 2059421603268608302)
URL: https://x.com/steipete/status/2059421603268608302

> What do people use for SSO/SCIM/Endpoint Security in 2026.

**SHIPPABLE TODAY** — peer cards are real (`internal/network/envelope.go:347–355`) and carry identity-equivalent fields, so the audit question is grounded:

> honest question for folks running agent fleets in 2026.
>
> what's your audit story when a peer card gets compromised? compozy peer cards carry PeerID + Capabilities + TrustModesSupported, so revocation has surface area, but the operator playbook still feels open. open to wrong answers.

**Voice check**: format=`question`. No fabricated feature, just a real ops question rooted in the verified envelope schema.

---

### 9. build-in-public-story — extract & open-source

**Source** (@steipete, 2026-05-26 · 134 ❤ · 4 🔁 · ID 2059423344961671290)
URL: https://x.com/steipete/status/2059423344961671290

> Also extracted our image-logic into a separate library… Rastermill - Portable image processing for Node agents.

**TEMPLATE BLOCKED**: there is no public "workspace router" or similar extracted Go module from Compozy today. Don't claim an extraction that hasn't happened. Use this template only after a real extraction lands.

---

### 10. release-bullets — manifesto + product reveal

**Source** (Pancake intro, 2026-05-27 · 242 ❤ · 43 🔁 · 62 quotes · ID 2059678950436282539)
URL: https://x.com/i/web/status/2059678950436282539

> Autonomous companies will become a reality before the end of 2026. Introducing Pancake: the OpenClaw cofounder that makes…

**Hype mechanic**: time-bound prediction + accessibility manifesto + product reveal.

**SHIPPABLE TODAY** (anchor every claim on a verified primitive):
> teams of agents running on one laptop, coordinating over `compozy-network/v0`, closing work with receipts.
>
> compozy ships the runtime piece. workspaces hold the state. ClaimNextRun moves work between agents. one go binary, no infrastructure to babysit. start with a fleet of two.

**Voice check**: format=`technical-insight`. Cites `compozy-network/v0`, workspaces, ClaimNextRun — all verified. No em-dash. 0 emojis.

---

### 11. build-in-public-story — flex with concrete result

**Source** (2026-05-26 · 161 ❤ · 87 🔁 · ID 2059088882210418691)
URL: https://x.com/i/web/status/2059088882210418691

> I made $3,500 in 4 hours By Cloning a Pro Trader Strategy Using Claude Code & Hermes Agent.

**TEMPLATE BLOCKED** unless Pedro actually has this story to tell. Don't fabricate income claims or weekend builds.

**Pattern note** for when there's a real story: lead with the outcome number, name the agents inside the Compozy workspace, end with cost or time saved. Only ship when the project actually exists.

---

### 12. build-in-public-story — concrete SEO claim

**Source** (2026-05-27 · 12 ❤ · 5 🔁 · ID 2059435076841341335)
URL: https://x.com/i/web/status/2059435076841341335

> Rank #1 on Google with Claude + Hermes Agent OS.

**TEMPLATE BLOCKED** same reason as #11 — don't claim a personal outcome that hasn't happened.

---

### 13. build-in-public-story — weekend voice agent

**Source** (2026-05-27 · 13 ❤ · 1 🔁 · ID 2059538021192716443)
URL: https://x.com/i/web/status/2059538021192716443

> I built a live AI voice agent with OpenClaw 🤯

**TEMPLATE BLOCKED** until you've actually wired a voice loop through Compozy. Don't pre-claim the build.

---

### 14. single-block — uptime brag

**Source** (@Railpushagent, 2026-05-27 · 26 🔁 chain visible in our search · ID 2059804805447237919)
URL: https://x.com/Railpushagent/status/2059804805447237919

> saturday afternoon. openclaw still on duty. no weekend rate. no overtime. $5/mo means $5/mo. 168 hours of it.

**Hype mechanic**: lowercase rhythm + cost anchor + math punchline (168 hours = a full week).

**SHIPPABLE TODAY** — Compozy is a daemon designed for durable sessions per `COPY.md § 2`, so "still grinding" is honest:

> sunday night. compozy daemon still grinding in the background. one go binary, sqlite-backed session state, my laptop, 168 hours of it if i want.

**Voice check**: format=`single-block`. Lowercase. No em-dash. ⚠️ "one go binary, sqlite-backed session state, my laptop" — 3 comma-separated items, borderline rule-of-three. Tighten to two clauses before shipping: "one go binary on my laptop. sqlite holds the state. 168 hours of it if i want."

---

### 15. comparison-take — model test write-up

**Source** (organic, 2026-05-28 · 1 ❤ · ID 2059806859184288183)
URL: https://x.com/i/web/status/2059806859184288183

> Tried Grok 4.3 on Hermes and OpenClaw. Hit-or-miss on both.

**SHIPPABLE TODAY only if you actually run the test**. Grok and OpenClaw are both real surfaces (xai provider in `provider.go`, OpenClaw in the featured ACP trio), so the experiment is feasible — but don't post the conclusion before running it.

**Draft to use after you run the actual test**:
> ran grok 4.3 inside compozy against both claude code and openclaw, same workspace.
>
> {real observation here about tool-use adoption}. {real observation about latency / planner behavior}. early days, still useable.

**Voice check**: keep "useable" if it survives `pedro-persona.md § Grammar quirks whitelist`.

---

### 16. comparison-take — outage dueling

**Source** (2026-05-27 · 38 ❤ · 3 🔁 · ID 2059613342864470175)
URL: https://x.com/i/web/status/2059613342864470175

> Hermes devs confirmed it was an OpenAI issue… OpenClaw still dead. Hermes takes the lead by a hair.

**TEMPLATE BLOCKED**: don't post about competitor outages you didn't witness. If a real outage hits and the Compozy workspace genuinely keeps routing tasks across surviving agents, that's a great tweet — but only then.

**Future-shippable shape (anchor on `ClaimNextRun` + hooks)**:
> {outage} hit today. {agent} died on auth. compozy workspace kept routing claims to the surviving agents.
>
> the autonomy kernel doesn't care which host crashes. ClaimNextRun just hands the next run to whoever is alive.

(Only post after a real event.)

---

### 17. question — operator status poll

**Source** (@Teknium, 2026-05-27 · 53 ❤ · 1 🔁 · ID 2059761768436772898)
URL: https://x.com/Teknium/status/2059761768436772898

> Okay if you are on the latest Hermes Agent update and use OpenAI OAuth, how is it doing now? Reliably working yet?

**SHIPPABLE TODAY** as long as the version is current and the surface you ask about is real. v0.0.4 is the right version. Pick a real surface to ask about:

> okay folks on compozy v0.0.4 running claude code as a hosted agent: are the lease heartbeats holding under long-running task runs?
>
> want signal before i open a fix. drop your `compozy task` output if you've seen drops.

**Voice check**: format=`question`. Version is correct. `lease heartbeats` and `ClaimNextRun` lease TTL are real per `internal/task/lease_manager.go`. Verify `compozy task` is the actual CLI noun (it might be `compozy automation` or `compozy runs` — check `internal/cli/automation.go`) before posting.

---

### 18. release-bullets — version digest

**Source** (organic OpenClaw release recap, 2026-05-28 · ID 2059803535076663529)
URL: https://x.com/i/web/status/2059803535076663529

> OpenClaw 2026.4.22 has been released. 163 changes. Highlights: …

**TEMPLATE BLOCKED for a fake release**. The last Compozy tag is `v0.0.4` with 0 commits since on `main`. Don't ship a release digest until a real `v0.0.5` tag exists.

**When ready, use this honest version-digest shape**:
> compozy v0.0.{N} shipped. {real commit count} commits since v0.0.4.
>
> highlights:
> • {actual change 1, with PR link}
> • {actual change 2, with PR link}
> • {actual change 3, with PR link}
>
> {real upgrade note}.

---

### 19. technical-insight — framework take

**Source** (organic, 2026-05-28 · ID 2059804459841245521)
URL: https://x.com/i/web/status/2059804459841245521

> The LLM is the planner. It is not the workflow engine.

**Hype mechanic**: aphoristic reframe. Quotable.

**SHIPPABLE TODAY** — every primitive cited below is in the verified table:

> the LLM picks the next step. the runtime owns the durability.
>
> compozy is the runtime piece. workspaces hold state. ClaimNextRun moves work. hooks observe everything. compozy-network/v0 lets sessions become peers.

**Voice check**: format=`technical-insight`. ⚠️ "workspaces hold state. ClaimNextRun moves work. hooks observe everything." — 3 short parallel sentences, rule-of-three risk. Tighten before shipping. Suggested cut: "workspaces hold state. ClaimNextRun moves work between agents. hooks observe the whole thing."

---

### 20. build-in-public-story — daily PR log

**Source** (2026-05-26 · 10 ❤ · ID 2059364631546654905)
URL: https://x.com/i/web/status/2059364631546654905

> shipped 4 PRs for @NousResearch Hermes Agent today:
> 1/ web_crawl was fully implemented but never registered as a callable tool. now it is.

**TEMPLATE BLOCKED for fake PRs**. The previous version of this file made up 5 specific PRs that don't exist. Use this template **only with PRs you actually shipped**, with real PR links.

**Shape to follow when ready**:
> shipped {N} PRs into compozy main today:
>
> 1/ {actual PR title + link} — {one-line about what the bug was}
> 2/ {actual PR title + link} — {one-line}
> …

---

## Cross-cutting voice notes

- **Run Workflow A** (`x-pedro-voice` § Workflow A) per draft. The self-review I applied while writing is not the deterministic gate.
- **Opener cooldown**: posts that start `compozy + verb` ("compozy hosts qwen…", "compozy splits provider auth…", "compozy open hour…") share an opener fingerprint shape — space them out by 48h.
- **Truthful UI rule (from `CLAUDE.md § Design System`)**: don't render features the runtime doesn't support. Every adaptation above either cites the verified primitive table or is marked TEMPLATE BLOCKED.
- **Comparison tone**: drafts 14, 15, 17 mention OpenClaw / Hermes / Claude Code by name. Tone stays comparative and respectful — these are featured hosts inside Compozy, not punching bags.

## Open follow-ups

- **30-day window**: the X v2 standard endpoint caps at ~7 days and blocks `min_faves`. A real "last month" sweep needs either Pro tier or `xdk` with full-archive access.
- **Repeatable weekly competitor sweep**: not wired today. If you want one, the cleanest path is a new `--query` flag on `x-trend-research` writing to `social-media/_trends/competitors/YYYY-MM-DD/`. Easy add if you say the word.
- **Pre-publish CLI verb check**: a handful of drafts reference CLI verbs (`compozy agent`, `compozy task`, `compozy workspace`) without confirming the exact spelling. Before posting any draft that includes a command, run `compozy --help` and replace placeholders with real verbs.
