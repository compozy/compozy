# Copy System: CompozyOS

One product language across CompozyOS marketing, documentation, runtime UI, CLI help, release copy, package metadata, OpenGraph metadata, examples, and launch material.

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

CompozyOS is the system around the agent, already built: it keeps AI agents working continuously, without the scripts, cron jobs, and glue code people otherwise assemble and maintain.

### Short Pitch

Anyone can prompt an agent. Making agents work continuously is still an engineering project: loops, triggers, cron, memory, permissions, approvals, observability, and the glue scripts that hold them together. CompozyOS turns that entire agent stack into one product. It runs the agent CLIs people already use (Claude Code, OpenClaw, and Hermes) and ships the operating layer around them already built: durable sessions, Loops, triggers, memory, permissions, approvals, automation, and supervision through web, CLI, HTTP/SSE, UDS, and tools. Compozy Network adds agent-to-agent coordination inside the same system.

### Product Category

Use `operating system for AI agents` (or `agent operating system`) as the category descriptor and **CompozyOS** whenever naming the product. Use `compozy` only for the command and its technical identifier family. The category is a supporting label, never the headline claim: the promise is the system around the agent delivered already built, and the category explains what kind of product delivers it. The OS claim rests on the assembled, integrated system, not on a desktop metaphor or a feature count.

### Hero Lock

> **The system around the agent, already built.**
>
> One complete environment to create, automate, and supervise agent work, without scripts, plugin chains, or orchestration frameworks.

Use the headline and subhead together, verbatim, on the landing hero, with `An operating system for AI agents` as a small category label near them. The retired hero ("The only true OS for AI agents" and its OS-test definition) must not reappear on any evergreen surface: it led with architecture and category purity, which are mechanisms, never the promise. Dated launch posts that already shipped it stay as history.

### Primary Promise

People get advanced, continuous agent work (loops, scheduled automation, memory, permissions, approvals, supervision) without assembling or maintaining the system around the agent. The work stays durable and inspectable, and agents can manage the same runtime through structured surfaces.

### Differentiator Ladder

Lead with compression, then prove it through the connected parts:

1. **Already built:** loops, triggers, memory, permissions, approvals, automation, supervision, and the OS shell arrive as one product. The claim is that there is nothing to assemble; a feature count is not the claim.
2. **Runs the agents people already use:** ACP-compatible agent CLIs (Claude Code, OpenClaw, and Hermes) plug in as drivers. CompozyOS is not another boxed agent competing with them.
3. **One runtime, one state model:** execution, tasks, loops, memory, permissions, automation, coordination, and the shell stay connected because they are core objects of the same local-first runtime, not plugins. One Go binary and SQLite-backed daemon keep the work durable, resumable, and inspectable. This is why the system does not feel stitched together; it is the mechanism behind the promise, never the headline.
4. **Built to be built on:** extensions, hooks, skills, capabilities, bridges, SDKs, MCP, and native tools plug into daemon-owned registries and public contracts.
5. **Shared control, bounded autonomy:** web, CLI, HTTP/SSE, UDS, and tools expose the same runtime state to people and agents; approvals, claim tokens, leases, safe spawn, and coordinator handoff keep autonomous work observable and recoverable.
6. **Compozy Network:** sessions can become peers, exchange typed envelopes, and return receipts inside the same runtime that owns their work, state, permissions, and memory.

### What CompozyOS Is Not

Use the glossary as the authority. In public copy, keep these boundaries clear:

- CompozyOS is not another agent or assistant. It runs the ACP-compatible agent CLIs people already use; agent neutrality is a feature line, never the headline promise.
- CompozyOS is not a desktop shell placed over an agent CLI. The shell is one surface over a daemon-owned operating system.
- CompozyOS is not a workflow engine. Capabilities are interpretive, not deterministic programs.
- CompozyOS is not a federation protocol. Compozy Network is a self-contained agent coordination layer, not an organization-level trust system.
- CompozyOS is not an MCP replacement. MCP integrates into CompozyOS.
- CompozyOS is not an A2A replacement. Compozy Network and A2A can coexist.
- CompozyOS does not compete on owning a wire protocol. It competes on the integrated runtime, extension surface, observability, and depth of coordination.

## 3. Message Architecture

### Primary Narrative

The system around the agent, already built.

Anyone can prompt an agent; making agents work continuously is still an engineering project. The advanced techniques (reliable loops, triggers, scheduled automation, memory, permissions, approvals) stay with the few who can assemble the system around the agent. CompozyOS lowers that floor: it ships the entire agent stack as one product, so the operating expertise comes built in. The parts already work together: a task can start a session, permissions bound it, memory follows the workspace, people can see and steer it, and another agent can continue the work without rebuilding the context by hand.

The enemy in public copy is the DIY agent stack (agent CLI + loops + triggers + cron and webhooks + memory + permissions + approvals + observability + glue scripts), never a named rival. Architecture ("one runtime, one state model") explains why the system holds; it never leads.

### Secondary Narrative

Built to be built on.

CompozyOS exposes extensions, hooks, skills, capabilities, bridges, SDKs, MCP, native tools, and structured control surfaces as parts of the operating system. Agents do not only run on the system; they can operate it through the same contracts people use.

### Network Mode Naming

Use **Local** and **Live** in product copy. Local is the default and creates no Network participation.
Live is explicit, channel-scoped, and finitely bounded. Configuration can make Live available, but
availability never opts an execution in. In code and structured payloads, preserve the canonical
`local` and `live` values and the stored `network-participation/v1` version atom.

### Proof Pillars

Every major copy surface should draw from one or more proof pillars.

| Pillar            | Claim Shape                                                                             | Proof to Prefer                                                                                                |
| ----------------- | --------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| Assembled System  | The whole agent stack ships in one product; there is nothing to assemble.               | An install-to-first-Loop journey that needs no outside tooling; the built-in surfaces in generated references. |
| Integrated System | Work, state, policy, memory, automation, coordination, and the OS shell stay connected. | A real task/session/loop journey that crosses those surfaces without duplicate state.                          |
| Durable Runtime   | Sessions survive beyond one terminal interaction and remain inspectable.                | Session CLI, event databases, SSE, UDS/HTTP parity, web session views.                                         |
| Shared Control    | People and agents operate the same daemon-owned state through structured surfaces.      | CLI `-o json`, HTTP/UDS endpoints, native tools, hosted MCP projection, truthful web views.                    |
| Bounded Autonomy  | Work ownership is token-fenced, leased, observable, and recoverable.                    | Task claim, heartbeat, complete/fail/release, coordinator state, safe spawn.                                   |
| Extensibility     | Public contracts let the operating system grow without bypassing runtime ownership.     | Host API, hooks, extensions, skills, capability catalog, bridge adapters, SDKs, and tool registry.             |
| Memory            | Memory is typed, scoped, file-backed, and inspectable.                                  | `compozy memory` commands, memory taxonomy, operation history, health.                                         |
| Compozy Network   | Explicitly Live agents exchange typed envelopes and collect receipts.                   | Local/Live controls, `compozy network` commands, message kinds, commit-first delivery, audit trail.            |

### Feature Priority by Surface

- **Homepage:** the compression promise first (advanced agent work, no stack to assemble), built-in capability proof second (create, automate, supervise), agent neutrality and extensibility third, and Compozy Network as the unique subsystem within that system. Architecture appears only as a why-it-holds caption.
- **Runtime docs:** the reader's problem first, architecture second.
- **Protocol docs:** Compozy Network value and adoption path first, wire mechanics second; never promote the subsystem into the whole product category.
- **Web UI:** truthful state and the person's next action first, marketing language last.
- **CLI help:** exact verb behavior first, product narrative only when it clarifies intent.
- **Changelog:** merged behavior and breaking changes first, no aspirational roadmap.
- **Blog/launch:** narrative is allowed, but every concrete claim still needs evidence.

## 4. Audience & Surface Intent

| Audience                                              | Reader Job                                                                                                        | Proof They Need                                                                                                                 | CTA Style                                                            |
| ----------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| Developers and technical operators running agent work | Get advanced, continuous agent work (loops, automation, approvals) without assembling the stack around the agent. | A clear install path, a first Loop or automation that works, visible session state; commands and event history one step deeper. | `Install the runtime`, `Start the daemon`, `Open the runtime docs`.  |
| Agent/runtime developers                              | Understand extension points and daemon contracts.                                                                 | APIs, SDKs, tool registry, hooks, capabilities, generated references.                                                           | `Build an extension`, `Read the Host API`, `View the tool registry`. |
| Protocol implementers                                 | Implement or inspect `compozy-network/v0` outside CompozyOS.                                                      | Envelope shape, message kinds, trust model, conformance guidance.                                                               | `Read the compozy-network/v0 spec`, `Send a minimal message`.        |
| Contributors                                          | Work safely in the repo and preserve product semantics.                                                           | Glossary, AGENTS/CLAUDE instructions, tests, task specs.                                                                        | `Read the contributor path`, `Run the verification gate`.            |
| Evaluators                                            | Decide whether CompozyOS is different from local CLIs, harnesses, MCP, A2A, and workflow engines.                 | Sharp positioning, named constraints, honest maturity, sourced comparison.                                                      | `Compare the runtime`, `See what ships today`.                       |

Today's product experience is technical: terminal install, local daemon, CLI, `config.toml`. Present-tense claims target developers and technical operators; broader operators are vision and stay future-framed. Public prose still defaults to plain language: everyday words carry the claim, and the exact mechanism stays one step away (a linked reference, expandable detail, or secondary text), not in the first sentence.

## 5. Voice & Editorial Rules

CompozyOS copy is people-first, plain-spoken, and calm-confident. It writes plainly for the people who run agent work today: developers and technical operators. Plain language is a courtesy, not an audience claim. Everyday words carry the claim; the mechanism stays one step away as proof, never as an entry fee.

### Voice

- Direct, specific, and grounded in shipped behavior.
- Calm, not cute.
- Plain, not vague.
- Confident, not inflated.
- Person-first: speak to the person whose work the agents are doing, never to an abstract "user," and never through protocol jargon. Today that person is a developer or technical operator; keep the language plain anyway.
- Product-led: CompozyOS and Compozy Network are usually the subject.

### Style Rules

- Prefer nouns and mechanisms over adjectives.
- Prefer short sentences when making claims.
- Lead with outcomes, then mechanism, then proof.
- Prefer the everyday verb over the runtime noun on first contact: agents keep working after the tab closes; the session mechanism follows for readers who want it.
- Define a runtime term at first use on end-user surfaces; reference docs may assume it.
- Use second person in docs and how-to copy when it helps the reader act.
- Use `you` sparingly in marketing. It should sharpen the reader's job, not turn every line into sales copy.
- Do not use `we` or `our` in marketing body copy. Use the product as the subject: `CompozyOS does...`, `Compozy Network gives...`, `The runtime keeps...`.
- No emoji, exclamation marks, or hype punctuation.
- No fake urgency.
- No fabricated testimonials, logos, stats, benchmarks, or maturity claims.
- Sentence case for headings and labels unless the UI component or design system requires uppercase mono metadata.

### Copy Rhythm

Good CompozyOS copy often has this shape:

1. Name the reader's problem.
2. State the product capability.
3. Prove it with a runtime mechanism, command, protocol object, or artifact.

Example:

> Agents should not stop working when a terminal tab closes. CompozyOS keeps them in durable sessions — with saved history, resumable state, and the same view from CLI, API, and web.

## 6. Vocabulary & Naming

The glossary is authoritative. This section lists the terms most likely to appear in public copy.

### Product Names

| Term                 | Use                                                                                                                                                                                                                                               |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `CompozyOS`          | The public product name in prose, UI, package descriptions, calls to action, and formal category language. It names the complete system: runtime, daemon, work model, memory, automation, permissions, OS shell, extensibility, and coordination. |
| `compozy`            | The CLI command and technical identifier family. Keep the binary, `COMPOZY_*` environment variables, module path, `@compozy/*` packages, formula, sockets, config paths, and `compozy__*` tool IDs unchanged.                                     |
| `Compozy Network`    | The agent-to-agent coordination subsystem and public network concept. It is part of CompozyOS, not the product category.                                                                                                                          |
| `compozy-network/v0` | The protocol/version name. Use lowercase and monospace in UI/docs when possible.                                                                                                                                                                  |

### Canonical Example Trio

When public copy needs to name 2–3 specific agent CLIs as examples (hero subhead, runtime intros, installation prerequisites, blog narrative, project overviews), use this trio in this order:

> Claude Code, OpenClaw, and Hermes.

Why this trio: these are the most recognizable ACP-compatible CLIs in the current CompozyOS ecosystem. Older copy used Claude Code, Codex, Gemini CLI, or Pi as the canonical examples. Replace those inline lists with the trio above unless the surrounding sentence has a specific reason to name a different driver (for example, a CLI-specific command example or a comparison to a named runtime).

The full enumeration of supported drivers lives in `packages/site/components/landing/provider-data.ts` (`SUPPORTED_AGENT_PROVIDERS`). When public copy needs the total count, derive it from `SUPPORTED_AGENT_COUNT` instead of hardcoding a number.

### Factory Vocabulary

"Software factory" / "agent factory" name the workload people build on top of CompozyOS — a standing, mostly-autonomous pipeline that turns outside signals into shipped software, with humans at chosen gates. Rules:

- Factory describes what runs **on** CompozyOS, never the product itself. Approved bridge line: "The OS your agent factory runs on." CTA form: "Build your factory on it."
- Never adopt `Factory` as a CompozyOS product or feature name. The noun is occupied in this space (Factory.ai claims the "software factory" category; Mastra ships "Mastra Factory").
- Never write `AI factory`. That phrase is NVIDIA's and means a datacenter that manufactures intelligence, not a software process.
- Keep the anti-cage framing: an OS gives agents general capability with safe boundaries; a factory built on it is not a rigid assembly line. (Context: Garry Tan's "Foxconn factories for your agents" critique, 2026-06.)

### Positioning Vocabulary

- `the agent stack` / `the DIY agent stack`: the pile a user otherwise assembles and maintains around an agent CLI (loops, triggers, cron and webhooks, memory, permissions, approvals, observability, glue scripts). This is the canonical problem name in marketing copy; the enemy is the stack, never a named rival.
- `Batteries included.`: the label and headline form of the completeness claim. In prose use `comes built in`, `already built`, or `nothing to assemble`. Never `full-feature`.
- `simple` / `easy`: only with a measured metric behind them; until then use `assembled`, `complete`, `built in`.
- `wedge`: banned in all public copy; say `developers first` or `we start with`.
- `one workspace` / `all your agents in one place`: banned as promise framing. Agent neutrality is a feature line ("runs the agents you already use"), never the headline.

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
- `channel`: Compozy Network namespace or coordination channel, not a generic adapter.

### Agent Artifact Terms

- `capability`: the canonical term for reusable agent artifacts advertised or transferred between peers.
- `skill`: local procedural instruction loaded by CompozyOS.
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
- `coordinator`: managed CompozyOS session that orchestrates coordinated work.

### OS Shell Terms

The web UI presents as a desktop environment. These terms are runtime-true — each names a real surface or projection, never an aspirational one. A `workspace` remains the project/runtime scope; each workspace owns one or more persistent visual `desktops`.

- `workspace`: the project root and scoped runtime context. The Workspaces surface switches runtime scope; it does not manage visual desktops.
- `desktop`: one persistent virtual arrangement inside a workspace. It owns its tiled groups and floating-window order. Switching desktops does not switch runtime scope.
- `window`: a frame hosting one app's durably resumed route subtree; the views inside are the same views the routes render. A window belongs to exactly one desktop and may be tiled, stacked, or floating.
- `tiled group`: one non-overlapping arrangement tree inside a desktop. A desktop may contain multiple tiled groups alongside floating windows.
- `desktop pager`: the minimal lower-left horizontal dot control for switching desktops, aligned with the Dock centerline. Full create, rename, reorder, transfer, and delete actions live in Desktops Overview.
- `dock`: the bottom strip of app launchers, with running/minimized indicators and badges bound to runtime projections.
- `menubar`: the top bar — CompozyOS mark, Global scope globe, workspace trigger, app menus, the approvals bell, the ⌘K palette, Settings. The globe sits between the mark and the chip and is the only owner of Global vs workspace destination. Chip identity is the project name when scoped down, or **Global** (`~`) when Global scope is on.
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
- `wedge`
- `full-feature`

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
- Message-kind count must match `compozy-network/v0`.
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

- the Hero Lock in §2 verbatim: headline, subhead, and the small category label.
- the compression promise first, then proof that loops, triggers, memory, permissions, approvals, automation, and supervision come built in (create, automate, supervise).
- agent neutrality as a feature line: CompozyOS runs the agent CLIs people already use (Claude Code, OpenClaw, and Hermes).
- extensibility as the next criterion: show how extensions, hooks, skills, SDKs, and tools participate in the same runtime.
- Compozy Network as the unique subsystem inside the complete system, not as the whole-product lead.
- architecture only as a why-it-holds caption ("one runtime, one state model"), never a section lead.
- install path as primary conversion.
- concrete signal cards only when the numbers are current.

Avoid:

- leading with ACP, JSON-RPC, stdio, UDS, SQLite, delivery internals, or package names.
- leading with architecture, OS purity, or category tests.
- centralization framing ("one workspace", "all your agents in one place") as the promise.
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

Goal: help implementers understand `compozy-network/v0` without adopting CompozyOS internals.

Use:

- `Compozy Network` for the concept.
- `compozy-network/v0` for protocol/version.
- message kinds, envelope behavior, trust profile, conformance, and examples.

Avoid:

- implying CompozyOS ownership is required to implement the protocol.
- confusing MCP, A2A, and Compozy Network roles.

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

Formula (never a feature roll — the claim is what it keeps doing for the person and the pain it removes):

```text
CompozyOS is <the promise as a noun phrase>: <what it keeps doing for the person, in plain verbs>, without <the concrete pain it removes>.
```

Approved:

```text
CompozyOS is the system around the agent, already built: it keeps AI agents working continuously, without the scripts, cron jobs, and glue code people otherwise assemble and maintain.
```

### Hero

Use the Hero Lock in §2 verbatim across the landing and launch surfaces. Keep the subhead adjacent
to the headline, keep the category label small, and never let architecture, centralization, or
Compozy Network stand in for the promise.

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
Description: Sessions discover peers, send typed envelopes, and close work with receipts through compozy-network/v0.
```

Weak:

```text
Eyebrow: Innovation
Title: Seamless agent collaboration
Description: CompozyOS unlocks the future of autonomous teamwork.
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
- `Read the compozy-network/v0 spec`
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

### Strong CompozyOS Copy

```text
Real commands, not docs-ware.
```

Why it works: short, specific, dry, and tied to a command surface.

```text
No Docker. No Postgres. compozy daemon start.
```

Why it works: concrete local-first proof.

```text
Orca, Paperclip, Smithers, Hermes, OpenClaw, Synara, and T3 each prove demand for part of the system. CompozyOS ships those parts in one product, batteries included.
```

Why it works: names the comparison set and frames the difference as compression rather than an unsupported ranking. Comparison surfaces may name systems; the narrative enemy stays the DIY stack.

### Weak CompozyOS Copy

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

- the retired hero resurfacing anywhere: "The only true OS for AI agents", the OS-test definition, "an agent operating system for real work", "a local-first operating system for agent work", or "gives agent work a durable place to live" in metadata, OG images, docs intros, or package descriptions.
- architecture climbing into headline slots: "one runtime, one state model" or OS-purity tests as a promise instead of a why-it-holds caption.
- centralization framing returning: "one workspace", "all your agents in one place".
- `Compozy Network` or "runtime with a network built in" phrasing standing in for the integrated CompozyOS category.
- `capability` vs old `recipe`, `workflow`, `procedure`, or `playbook` language.
- runtime behavior that moved from planned to shipped or from spec to deleted.
- present-tense copy drifting past the current audience: non-technical operators are vision, not a shipped claim.
- people-first language drifting back toward control-room personas: the runtime term is `control surface`.

## 11. Agent Prompt Guide

Use these as task-local prompts after reading the target files.

### Rewrite a Homepage Hero

```text
Use COPY.md and DESIGN.md. Use the §2 Hero Lock verbatim; do not invent or relock the headline or subhead. Lead with the compression promise, prove the built-ins (create, automate, supervise) with current runtime evidence, keep architecture as a why-it-holds caption, and keep Compozy Network in its subsystem role. Primary CTA installs or starts the runtime; the secondary CTA points to the strongest supporting proof.
```

### Write a Docs Intro

```text
Use COPY.md, docs/_memory/glossary.md, and the generated CLI/API reference for this surface. Start with the reader's task, then the CompozyOS surface used to complete it, then constraints. Do not paraphrase generated flags or endpoints if a generated reference exists.
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
- The copy uses `CompozyOS`, `compozy`, `Compozy Network`, and `compozy-network/v0` correctly.
- Glossary terms are applied, especially `capability`, `skill`, `bridge`, `channel`, `AGENT.md`, and `AGENTS.md`.
- Inline example lists of agent CLIs use the canonical trio (Claude Code, OpenClaw, and Hermes) unless a CLI-specific reason exists.
- ACP driver/agent counts in public copy are derived from `PROVIDERS.length`, not a hardcoded number.
- Claim maturity is clear.
- The promise leads: the system around the agent, already built. Architecture ("one runtime, one state model", OS-purity tests) appears only as a why-it-holds mechanism, never in a headline slot.
- No `wedge`, no `full-feature`, and no `simple`/`easy` claims without a metric behind them.
- No centralization promise (`one workspace`, `all your agents in one place`).
- Present-tense audience claims stay within developers and technical operators; broader operators are future-framed.
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
- Compozy Network protocol naming changes
- the canonical example trio of agent CLIs needs to change
- generated CLI/API surfaces change in a way that affects public docs or examples
- a review finds repeated copy drift across surfaces

Do not use `COPY.md` as a dumping ground for campaign-specific copy. Put dated campaign drafts, competitor research, and one-off launch material in task or analysis artifacts, then distill only durable rules back into this file.
