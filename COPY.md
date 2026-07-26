# Copy System: AGH

One product language across AGH marketing, documentation, runtime UI, CLI help, release copy, package metadata, OpenGraph metadata, examples, and launch material.

`COPY.md` is the verbal counterpart to `DESIGN.md`.

- `DESIGN.md` governs visual grammar: colors, type, layout, depth, motion, iconography, and visual content rules.
- `COPY.md` governs product language: positioning, claims, proof, vocabulary, voice, CTA patterns, public documentation prose, release language, and microcopy.

If the two files overlap, use this split: `DESIGN.md` decides how the surface looks; `COPY.md` decides what the surface is allowed to say.

## 1. Purpose & Source Hierarchy

Use `COPY.md` before changing any public or product-facing text:

- marketing landing copy in `packages/site/components/landing/`
- blog, launch posts, changelog, and metadata in `packages/site/content/`
- runtime and protocol docs in `packages/site/content/runtime/` and `packages/site/content/protocol/`
- OpenGraph, SEO, site config, social snippets, and package descriptions
- web UI labels, headings, empty states, errors, onboarding text, settings text, and toasts
- CLI help and generated docs source text
- README, SDK, example, extension, and marketplace copy
- release notes and public PR descriptions

Canonical sources, in order:

1. **Runtime truth:** implemented code, generated API/CLI references, tests, release artifacts, and `make verify` evidence.
2. **Product vocabulary:** `docs/_memory/glossary.md`.
3. **Standing engineering posture:** `docs/_memory/standing_directives.md`.
4. **Visual grammar:** `DESIGN.md`.
5. **Current public surfaces:** `packages/site/`, `web/`, SDK packages, and generated references.
6. **Planning evidence:** `.compozy/tasks/*`, `.codex/plans/*`, and `.compozy/tasks/site-copy/analysis/*`, only when their claims still match current runtime truth.

Runtime truth beats copy preference. Generated API/CLI references beat paraphrase. The glossary beats older RFCs, old task artifacts, and stale public copy.

## 2. Positioning Snapshot

### Canonical One-Liner

AGH is a local-first agent operating system: one daemon for durable agent sessions, one shared surface for humans and agents, and one open network for agent-to-agent coordination.

### Short Pitch

AGH runs the agent CLIs teams already use as durable, inspectable sessions. It keeps work attached to a workspace, exposes the same state through CLI, HTTP/SSE, UDS, and web UI, and ships `agh-network/v0` so agents can discover peers, delegate work, exchange capabilities, and close the loop with receipts.

### Product Category

Use `agent operating system` as the category descriptor when needed. Do not make it the only hero idea. The sharper public hook is the open-workplace promise:

> An open workplace for AI agents.

### Primary Promise

Real agent work should be durable, observable, agent-manageable, and able to cross the boundary of one terminal tab.

### Differentiator Ladder

Lead with the highest-leverage, most public differentiators:

1. **AGH Network:** active sessions can become peers, discover each other, exchange typed envelopes, and return receipts.
2. **Local-first durable runtime:** one Go binary, background daemon, SQLite-backed state, durable sessions, event history, and resumable work.
3. **Agent-manageable surfaces:** CLI, HTTP/SSE, UDS, and tools expose structured controls agents can call.
4. **Autonomy kernel:** task runs, claim tokens, leases, safe spawn, and coordinator handoff make multi-agent work observable and bounded.
5. **Tool registry and extensibility:** native Go tools, MCP, extensions, hooks, skills, capabilities, bridges, and policies through one daemon-owned runtime.
6. **Memory and consolidation:** typed memory, workspace/global scopes, operation history, and gated consolidation behavior.

### What AGH Is Not

Use the glossary as the authority. In public copy, keep these boundaries clear:

- AGH is not a workflow engine. Capabilities are interpretive, not deterministic programs.
- AGH is not a federation protocol. AGH Network is a self-contained agent coordination layer, not an organization-level trust system.
- AGH is not an MCP replacement. MCP integrates into AGH.
- AGH is not an A2A replacement. AGH Network and A2A can coexist.
- AGH does not compete on owning the wire protocol. AGH competes on runtime, SDK, observability, DX, and integration depth.

## 3. Message Architecture

### Primary Narrative

An open workplace for AI agents.

This is the strongest first-contact story. AGH is not just another local agent runner. It gives durable agent sessions a place to find peers, share capabilities, and close work with receipts on agh-network/v0.

### Secondary Narrative

Local-first runtime for real agent work.

AGH keeps agent sessions durable, replayable where supported by the event model, observable, resumable, and controllable through the same surfaces humans and agents use.

### Network Mode Naming

Use **Local** and **Live** in product copy. Local is the default and creates no Network participation.
Live is explicit, channel-scoped, and finitely bounded. Configuration can make Live available, but
availability never opts an execution in. In code and structured payloads, preserve the canonical
`local` and `live` values and the stored `network-participation/v1` version atom.

### Proof Pillars

Every major copy surface should draw from one or more proof pillars.

| Pillar                   | Claim Shape                                                                       | Proof to Prefer                                                                                 |
| ------------------------ | --------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------- |
| AGH Network              | Explicitly Live agents exchange typed envelopes and collect receipts.             | Local/Live controls, `agh network` commands, message kinds, commit-first delivery, audit trail. |
| Durable Runtime          | Sessions survive beyond one terminal interaction and remain inspectable.          | Session CLI, event DBs, SSE, UDS/HTTP parity, web session views.                                |
| Agent-Manageable Control | Agents operate AGH through structured surfaces, not hidden UI-only paths.         | CLI `-o json`, HTTP/UDS endpoints, tool registry, hosted MCP projection.                        |
| Autonomy Kernel          | Work ownership is token-fenced, leased, and recoverable.                          | Task claim, heartbeat, complete/fail/release, coordinator state, safe spawn.                    |
| Tool Registry            | One canonical tool surface spans native tools, MCP, and extensions.               | `agh tool list/search/info/invoke`, policy decisions, toolsets.                                 |
| Memory                   | Memory is typed, scoped, file-backed, and inspectable.                            | `agh memory` commands, memory taxonomy, operation history, health.                              |
| Extensibility            | Extensions, hooks, skills, bridges, capabilities, and SDKs plug into the runtime. | Host API, hook dispatch, capability catalog, bridge adapters, SDK docs.                         |

### Feature Priority by Surface

- **Homepage:** AGH Network first, then runtime proof, then install path.
- **Runtime docs:** the reader's problem first, architecture second.
- **Protocol docs:** AGH Network value and adoption path first, wire mechanics second.
- **Web UI:** truthful state and the person's next action first, marketing language last.
- **CLI help:** exact verb behavior first, product narrative only when it clarifies intent.
- **Changelog:** merged behavior and breaking changes first, no aspirational roadmap.
- **Blog/launch:** narrative is allowed, but every concrete claim still needs evidence.

## 4. Audience & Surface Intent

| Audience                  | Reader Job                                                                                  | Proof They Need                                                                                               | CTA Style                                                            |
| ------------------------- | ------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| People running agent work | Start, watch, guide, resume, and finish agent work — technical or not.                      | A clear install path, visible session state, plain-language docs; commands and event history one step deeper. | `Install the runtime`, `Start the daemon`, `Open the runtime docs`.  |
| Agent/runtime developers  | Understand extension points and daemon contracts.                                           | APIs, SDKs, tool registry, hooks, capabilities, generated references.                                         | `Build an extension`, `Read the Host API`, `View the tool registry`. |
| Protocol implementers     | Implement or inspect `agh-network/v0` outside AGH.                                          | Envelope shape, message kinds, trust model, conformance guidance.                                             | `Read the agh-network/v0 spec`, `Send a minimal message`.            |
| Contributors              | Work safely in the repo and preserve product semantics.                                     | Glossary, AGENTS/CLAUDE instructions, tests, task specs.                                                      | `Read the contributor path`, `Run the verification gate`.            |
| Evaluators                | Decide whether AGH is different from local CLIs, harnesses, MCP, A2A, and workflow engines. | Sharp positioning, named constraints, honest maturity, sourced comparison.                                    | `Compare the runtime`, `See what ships today`.                       |

The primary audience spans non-technical and technical people. Default public prose to plain language; keep the exact mechanism one step away — a linked reference, expandable detail, or secondary text — not in the first sentence.

## 5. Voice & Editorial Rules

AGH copy is people-first, plain-spoken, and calm-confident. It writes for anyone who runs agent work — technical or not. Everyday words carry the claim; the mechanism stays one step away as proof, never as an entry fee.

### Voice

- Direct, specific, and grounded in shipped behavior.
- Calm, not cute.
- Plain, not vague.
- Confident, not inflated.
- Person-first: speak to the person whose work the agents are doing — founder, writer, analyst, or engineer — never to an abstract "user," and never through protocol jargon.
- Product-led: AGH, AGH Runtime, and AGH Network are usually the subject.

### Style Rules

- Prefer nouns and mechanisms over adjectives.
- Prefer short sentences when making claims.
- Lead with outcomes, then mechanism, then proof.
- Prefer the everyday verb over the runtime noun on first contact: agents keep working after the tab closes; the session mechanism follows for readers who want it.
- Define a runtime term at first use on end-user surfaces; reference docs may assume it.
- Use second person in docs and how-to copy when it helps the reader act.
- Use `you` sparingly in marketing. It should sharpen the reader's job, not turn every line into sales copy.
- Do not use `we` or `our` in marketing body copy. Use the product as the subject: `AGH does...`, `AGH Network gives...`, `The runtime keeps...`.
- No emoji, exclamation marks, or hype punctuation.
- No fake urgency.
- No fabricated testimonials, logos, stats, benchmarks, or maturity claims.
- Sentence case for headings and labels unless the UI component or design system requires uppercase mono metadata.

### Copy Rhythm

Good AGH copy often has this shape:

1. Name the reader's problem.
2. State the product capability.
3. Prove it with a runtime mechanism, command, protocol object, or artifact.

Example:

> Agents should not stop working when a terminal tab closes. AGH keeps them in durable sessions — with saved history, resumable state, and the same view from CLI, API, and web.

## 6. Vocabulary & Naming

The glossary is authoritative. This section lists the terms most likely to appear in public copy.

### Product Names

| Term                   | Use                                                                                                                                       |
| ---------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| `AGH`                  | The complete product: runtime, daemon, site, docs, CLI, UI, extensibility, and network implementation.                                    |
| `AGH Runtime`          | The local daemon and runtime surface: sessions, memory, skills, workspaces, automation, bridges, tools, hooks, extensions, observability. |
| `AGH Network`          | The agent-to-agent coordination layer and public network concept.                                                                         |
| `agh-network/v0`       | The protocol/version name. Use lowercase, mono in UI/docs when possible.                                                                  |
| `AGH Network Protocol` | Spec/reference contexts only. Prefer `AGH Network` in marketing and narrative copy.                                                       |

### Canonical Example Trio

When public copy needs to name 2–3 specific agent CLIs as examples (hero subhead, runtime intros, installation prerequisites, blog narrative, project overviews), use this trio in this order:

> Claude Code, OpenClaw, and Hermes.

Why this trio: these are the most recognizable ACP-compatible CLIs in the current AGH ecosystem. Older copy used Claude Code, Codex, Gemini CLI, or Pi as the canonical examples. Replace those inline lists with the trio above unless the surrounding sentence has a specific reason to name a different driver (for example, a CLI-specific command example or a comparison to a named runtime).

The full enumeration of supported drivers lives in `packages/site/components/landing/provider-data.ts` (`SUPPORTED_AGENT_PROVIDERS`). When public copy needs the total count, derive it from `SUPPORTED_AGENT_COUNT` instead of hardcoding a number.

### Runtime Terms

- `daemon`: the local background runtime process.
- `control surface`: a human/agent-operable surface — CLI, HTTP/SSE, UDS, or web UI — over the same daemon state.
- `session`: a durable managed agent run. Prefer `session` over `chat`.
- `event ledger`: durable event history. Use only when the implementation exposes the relevant event trail.
- `workspace`: project root and scoped runtime context.
- `tool registry`: daemon-owned tool identity, policy, discovery, and execution.
- `toolset`: grouped exposure or policy set for tools.
- `hook`: typed lifecycle dispatch. Do not call hooks a generic event bus.
- `extension`: package that can provide resources, capabilities, and Host API actions.
- `bridge`: external messaging/platform adapter. Do not use `channel` for Slack/Discord/etc. adapters.
- `channel`: AGH Network namespace or coordination channel, not a generic adapter.

### Agent Artifact Terms

- `capability`: the canonical term for reusable agent artifacts advertised or transferred between peers.
- `skill`: local procedural instruction loaded by AGH.
- `AGENT.md`: single-agent definition format.
- `AGENTS.md`: project-level agent instruction file.

Forbidden synonyms for `capability` in current behavior:

- `recipe`
- `workflow`
- `procedure`
- `playbook`

Use those words only when discussing external systems or historical migration context, and make that context explicit.

### Autonomy Terms

- `task run`: durable work record.
- `claim token`: ownership token for a claimed run. Never expose raw tokens in public examples.
- `claim_token_hash`: safe public form.
- `lease`: bounded ownership interval.
- `safe spawn`: daemon-managed child-session creation with TTL, caps, and permission narrowing.
- `coordinator`: managed AGH session that orchestrates coordinated work.

### OS Shell Terms

The web UI presents as a desktop environment. These terms are runtime-true — each names a real surface or projection, never an aspirational one. A `workspace` remains the project/runtime scope; each workspace owns one or more persistent visual `desktops`.

- `workspace`: the project root and scoped runtime context. The Workspaces surface switches runtime scope; it does not manage visual desktops.
- `desktop`: one persistent virtual arrangement inside a workspace. It owns its tiled groups and floating-window order. Switching desktops does not switch runtime scope.
- `window`: a frame hosting one app's durably resumed route subtree; the views inside are the same views the routes render. A window belongs to exactly one desktop and may be tiled, stacked, or floating.
- `tiled group`: one non-overlapping arrangement tree inside a desktop. A desktop may contain multiple tiled groups alongside floating windows.
- `desktop pager`: the minimal lower-left horizontal dot control for switching desktops, aligned with the Dock centerline. Full create, rename, reorder, transfer, and delete actions live in Desktops Overview.
- `dock`: the bottom strip of app launchers, with running/minimized indicators and badges bound to runtime projections.
- `menubar`: the top bar — workspace trigger, app menus, the approvals bell, the ⌘K palette, Settings.
- `window manager`: the daemon-authoritative, workspace-scoped topology and command surface for desktops and windows. Browser focus and the active desktop are client-local projections. This presentation data never contains agent `memory`.

### Burned-Out Marketing Phrases

Avoid these unless quoting another source:

- `AI-powered`
- `revolutionary`
- `game-changing`
- `next-generation`
- `supercharge`
- `unleash`
- `seamless`
- `effortless`
- `10x`
- `cutting-edge`
- `state-of-the-art`
- `magical`
- `build the future`
- `empower your developers`
- `production-ready` without concrete evidence

## 7. Claim Standards

Truthful copy beats plausible copy.

Do not turn roadmap, mockups, Paper artboards, desired architecture, old specs, or aspirational comments into present-tense product claims.

### Maturity Labels

Use these internally when drafting. Public copy can include the label when it clarifies risk.

| Label         | Meaning                                                                  | Public Claim Style                           |
| ------------- | ------------------------------------------------------------------------ | -------------------------------------------- |
| `shipped`     | Implemented, tested, and visible through public surfaces.                | Present tense.                               |
| `alpha`       | Shipped but intentionally early.                                         | Present tense with alpha context.            |
| `partial`     | Some paths work; others are intentionally incomplete.                    | Narrow claim only.                           |
| `scaffolding` | Framework/gates/types exist, but user-visible execution is not complete. | Do not market as a complete feature.         |
| `planned`     | Spec or roadmap only.                                                    | Future/RFC language only.                    |
| `deprecated`  | Old behavior or term being removed.                                      | Avoid except migration or changelog context. |

### Required Evidence

Before publishing a concrete claim, identify at least one strong source:

- implemented code path
- CLI command or generated CLI reference
- HTTP/UDS/OpenAPI endpoint
- public docs page
- test or QA evidence
- release PR or changelog entry
- runtime screenshot or web UI backed by real data
- protocol spec for protocol behavior

Use stronger evidence for stronger claims. "Only", "first", "complete", "secure", "production", "guaranteed", and numeric claims require especially strong evidence.

### Numbers and Counts

Numbers drift. If public copy uses a number, keep its source and update trigger obvious in the implementation or nearby docs.

Examples:

- Supported agent count must match provider/runtime truth.
- Tool count must match the current registry or release snapshot.
- Message-kind count must match `agh-network/v0`.
- Platform support must distinguish live, alpha, next, and planned.

### Words That Need Care

- `today`: use only for behavior actually available in the current release or current public branch context.
- `shipping`: use only for merged or released behavior.
- `supported`: use only when install/config/runtime docs and tests make support real.
- `live`: use only for working public paths.
- `next`: use only for clearly marked near-term roadmap or staged platform status.
- `open`: specify whether this means source, protocol, extension point, or documentation.
- `secure`: state the mechanism, not the adjective.

## 8. Surface Playbooks

### Homepage / Landing

Goal: make the core difference obvious quickly.

Use:

- hero locked to "An open workplace for AI agents." with subhead locked to "AGH runs the agent CLIs you already use as durable sessions — with memory, autonomy, tools, and automation — connected on agh-network/v0 channels where they find each other, share capabilities, and close work with receipts."
- AGH Network as the differentiator.
- runtime proof immediately after the network claim.
- install path as primary conversion.
- concrete signal cards only when the numbers are current.

Avoid:

- leading with ACP, JSON-RPC, stdio, UDS, SQLite, delivery internals, or package names.
- making runtime and network sound like two unrelated products.
- generic "agent OS" claims without proof.

### Runtime Docs

Goal: help people run and understand their agents; the daemon serves them underneath.

Use:

- problem -> outcome -> command/API reference -> architecture.
- direct second person for procedures.
- generated CLI/API references for exact flags and routes.

Avoid:

- paraphrasing generated references.
- burying user action under implementation internals.
- describing planned features as current behavior.

### Protocol Docs

Goal: help implementers understand `agh-network/v0` without adopting AGH internals.

Use:

- `AGH Network` for the concept.
- `agh-network/v0` for protocol/version.
- message kinds, envelope behavior, trust profile, conformance, and examples.

Avoid:

- implying AGH ownership is required to implement the protocol.
- confusing MCP, A2A, and AGH Network roles.

### Blog / Launch Posts

Goal: explain why the product matters and what ships.

Use:

- narrative openings are allowed.
- concrete "what ships today" sections.
- alpha constraints where relevant.
- direct links to docs or commands.

Avoid:

- launch copy that outruns implementation.
- invented market stats.
- overbroad competitor attacks.

### Changelog / Release Notes

Goal: record real merged work.

Use:

- `added`, `changed`, `fixed`, `breaking` lists from git history and PR descriptions.
- direct behavior descriptions.
- migration steps when required.

Avoid:

- aspirational copy.
- roadmap language.
- claims not tied to merged work.

### Web UI Microcopy

Goal: tell the person what is true and what they can do next.

Use:

- current state, next action, and consequence.
- labels that match backend nouns.
- empty states that explain why no data appears and what to do next.

Avoid:

- UI-only promises.
- controls or metrics the runtime does not model.
- cute empty states.

### CLI Help

Goal: make commands predictable and scriptable.

Use:

- exact nouns and verbs.
- output format guidance when useful.
- examples with safe placeholders.

Avoid:

- marketing slogans.
- raw secrets or raw claim tokens in examples.
- behavior that differs from generated docs.

### OpenGraph / SEO / Package Metadata

Goal: keep compact public summaries aligned.

Use:

- one-liners from this file.
- current positioning.
- no stale feature counts.

Avoid:

- old hero lines after the site narrative changes.
- generic SaaS language.
- protocol jargon without context.

## 9. Copy Patterns

### One-Liner

Formula:

```text
AGH is a <category>: <runtime promise>, <shared-surface promise>, and <network promise>.
```

Approved:

```text
AGH is a local-first agent operating system: one daemon for durable agent sessions, one shared surface for humans and agents, and one open network for agent-to-agent coordination.
```

### Hero

Formula:

```text
Headline: <core outcome>
Subhead: <what AGH is> + <who/what it runs> + <network/runtime proof>
Primary CTA: <install/start action>
Secondary CTA: <network/docs proof path>
```

Approved headline:

```text
An open workplace for AI agents.
```

Approved subhead:

```text
AGH runs the agent CLIs you already use as durable sessions — with memory, autonomy, tools, and automation — connected on agh-network/v0 channels where they find each other, share capabilities, and close work with receipts.
```

### Feature Card

Formula:

```text
Eyebrow: <domain noun>
Title: <verb-forward benefit>
Description: <mechanism + proof in one sentence>
Optional cite: <doc/source path>
```

Good:

```text
Eyebrow: Network
Title: Delegate across peers
Description: Sessions discover peers, send typed envelopes, and close work with receipts through agh-network/v0.
```

Weak:

```text
Eyebrow: Innovation
Title: Seamless agent collaboration
Description: AGH unlocks the future of autonomous teamwork.
```

### Docs Overview

Formula:

```text
This page helps you <reader task>. You will use <surface/command/API> to <outcome>. Before changing <thing>, understand <constraint>.
```

### Release Note

Formula:

```text
Added <public behavior> so <human/agent outcome>. This is available through <CLI/API/UI/docs path>.
```

### UI Empty State

Formula:

```text
No <object> yet. <What creates it>. <Primary action>.
```

Good:

```text
No task runs yet. Publish a task or let a coordinator enqueue work for this workspace.
```

### Error Copy

Formula:

```text
<What failed>. <Why, if known>. <Next safe action>.
```

Avoid blaming the person. Avoid hiding the cause when the runtime knows it.

### CTA Vocabulary

Prefer:

- `Install the runtime`
- `Start the daemon`
- `Read the agh-network/v0 spec`
- `Open the runtime docs`
- `Create a session`
- `View peers`
- `Send a message`
- `Build an extension`
- `Inspect events`

Avoid:

- `Get Started` when a specific action exists
- `Learn More`
- `Submit`
- `Click Here`
- `Unlock`
- `Supercharge`

## 10. Examples & Anti-Patterns

### Strong AGH Copy

```text
Real commands, not docs-ware.
```

Why it works: short, specific, dry, and tied to a command surface.

```text
No Docker. No Postgres. agh daemon start.
```

Why it works: concrete local-first proof.

```text
Every other agent tool stops at the single-runtime boundary.
```

Why it works: names the strategic boundary without inventing a benchmark.

### Weak AGH Copy

```text
An AI-powered platform to supercharge agent workflows.
```

Why it fails: generic SaaS language, no mechanism, no proof, banned terms.

```text
Seamlessly orchestrate limitless autonomous agents.
```

Why it fails: vague adverb, overbroad autonomy claim, no limits or evidence.

```text
The most advanced agent protocol.
```

Why it fails: unsupported ranking, protocol-only framing, no runtime proof.

### Drift Example

If site metadata, OpenGraph images, hero copy, and docs intro use different one-liners, agents should stop and reconcile the copy through this file before adding more variants.

Known drift to watch for:

- "An open workplace for AI agents" as the current hero vs older "Your agents can finally talk to each other.", `AGH Network` as the public hook, or "runtime with a network built in" phrasing.
- `capability` vs old `recipe`, `workflow`, `procedure`, or `playbook` language.
- runtime behavior that moved from planned to shipped or from spec to deleted.
- `operator surface` and operator-first persona language in older copy vs the current people-first register: the runtime term is `control surface`, and the audience is people running agent work (technical or not).

## 11. Agent Prompt Guide

Use these as task-local prompts after reading the target files.

### Rewrite a Homepage Hero

```text
Use COPY.md and DESIGN.md. Keep the hero open-workplace-first. Headline must preserve "An open workplace for AI agents." and subhead must preserve "AGH runs the agent CLIs you already use as durable sessions — with memory, autonomy, tools, and automation — connected on agh-network/v0 channels where they find each other, share capabilities, and close work with receipts." Use only claims backed by current code/docs. Primary CTA installs or starts the runtime. Secondary CTA points to agh-network/v0 docs.
```

### Write a Docs Intro

```text
Use COPY.md, docs/_memory/glossary.md, and the generated CLI/API reference for this surface. Start with the reader's task, then the AGH surface used to complete it, then constraints. Do not paraphrase generated flags or endpoints if a generated reference exists.
```

### Write a Feature Card

```text
Use a domain eyebrow, a verb-forward title, and a one-sentence mechanism. Include proof through a command, route, artifact, or docs path. Avoid "seamless", "powerful", "AI-powered", and unsupported counts.
```

### Write a Changelog Entry

```text
Use only merged work. Group into added/changed/fixed/breaking. State behavior, user impact, and migration notes when needed. Do not include roadmap or launch hype.
```

### Write UI Microcopy

```text
Use backend nouns exactly. State what is true, what action is available, and what happens next. Do not imply a metric, control, or repair path exists unless the runtime exposes it.
```

### Review Public Copy

```text
Check runtime truth, glossary vocabulary, claim maturity, CTA specificity, forbidden phrases, stale counts, and metadata drift. If a claim cannot be traced to code, docs, tests, generated references, or a release artifact, narrow or remove it.
```

## 12. Review Checklist & Maintenance

Before shipping copy or product-facing text, verify:

- Runtime truth is checked against current code, generated references, docs, tests, or release artifacts.
- The copy uses `AGH`, `AGH Runtime`, `AGH Network`, and `agh-network/v0` correctly.
- Glossary terms are applied, especially `capability`, `skill`, `bridge`, `channel`, `AGENT.md`, and `AGENTS.md`.
- Inline example lists of agent CLIs use the canonical trio (Claude Code, OpenClaw, and Hermes) unless a CLI-specific reason exists.
- ACP driver/agent counts in public copy are derived from `PROVIDERS.length`, not a hardcoded number.
- Claim maturity is clear.
- Numbers and counts have a source and update trigger.
- CTAs name a concrete action.
- Marketing body avoids `we` and `our`.
- No emoji, exclamation marks, or banned hype phrases appear.
- Docs do not paraphrase generated API/CLI references where generated references exist.
- UI copy does not invent unsupported controls, states, metrics, or repair paths.
- End-user surfaces (homepage, UI microcopy, onboarding, empty states) read plainly without protocol knowledge; runtime jargon appears only where precision earns it.
- OpenGraph, SEO, package metadata, and social snippets match current positioning.
- `DESIGN.md` remains the visual authority; this file remains the verbal authority.

Update `COPY.md` when:

- product positioning changes
- a public feature moves between planned, partial, alpha, shipped, or deprecated
- canonical vocabulary changes
- homepage hero or product one-liner changes
- AGH Network protocol naming changes
- the canonical example trio of agent CLIs needs to change
- generated CLI/API surfaces change in a way that affects public docs or examples
- a review finds repeated copy drift across surfaces

Do not use `COPY.md` as a dumping ground for campaign-specific copy. Put dated campaign drafts, competitor research, and one-off launch material in task or analysis artifacts, then distill only durable rules back into this file.
