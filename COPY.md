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

Anyone can prompt an agent. Making agents work continuously is still an engineering project: loops, triggers, cron, memory, permissions, approvals, observability, and the glue scripts that hold them together. CompozyOS turns that entire agent stack into one product. It runs the agent CLIs people already use (Claude Code, OpenClaw, and Hermes) and ships the operating layer around them already built: durable sessions, Loops, triggers, memory, permissions, approvals, automation, and supervision through web, CLI, HTTP/SSE, UDS, and tools.

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
4. **Built to be built on:** extensions, hooks, skills, capabilities, SDKs, MCP, and native tools plug into daemon-owned registries and public contracts.
5. **Shared control, bounded autonomy:** web, CLI, HTTP/SSE, UDS, and tools expose the same runtime state to people and agents; approvals, claim tokens, leases, safe spawn, and coordinator handoff keep autonomous work observable and recoverable.

### What CompozyOS Is Not

Use the glossary as the authority. In public copy, keep these boundaries clear:

- CompozyOS is not another agent or assistant. It runs the ACP-compatible agent CLIs people already use; agent neutrality is a feature line, never the headline promise.
- CompozyOS is not a desktop shell placed over an agent CLI. The shell is one surface over a daemon-owned operating system.
- CompozyOS is not a workflow engine. Capabilities are interpretive, not deterministic programs.
- CompozyOS is not an MCP replacement. MCP integrates into CompozyOS.
- CompozyOS does not compete on owning a wire protocol. It competes on the integrated runtime, extension surface, observability, and depth of coordination.

## 3. Message Architecture

### Primary Narrative

The system around the agent, already built.

Anyone can prompt an agent; making agents work continuously is still an engineering project. The advanced techniques (reliable loops, triggers, scheduled automation, memory, permissions, approvals) stay with the few who can assemble the system around the agent. CompozyOS lowers that floor: it ships the entire agent stack as one product, so the operating expertise comes built in. The parts already work together: a task can start a session, permissions bound it, memory follows the workspace, people can see and steer it, and another agent can continue the work without rebuilding the context by hand.

The enemy in public copy is the DIY agent stack (agent CLI + loops + triggers + cron and webhooks + memory + permissions + approvals + observability + glue scripts), never a named rival. Architecture ("one runtime, one state model") explains why the system holds; it never leads.

### Secondary Narrative

Built to be built on.

CompozyOS exposes extensions, hooks, skills, capabilities, SDKs, MCP, native tools, and structured control surfaces as parts of the operating system. Agents do not only run on the system; they can operate it through the same contracts people use.

### Proof Pillars

Every major copy surface should draw from one or more proof pillars.

| Pillar            | Claim Shape                                                                             | Proof to Prefer                                                                                                |
| ----------------- | --------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| Assembled System  | The whole agent stack ships in one product; there is nothing to assemble.               | An install-to-first-Loop journey that needs no outside tooling; the built-in surfaces in generated references. |
| Integrated System | Work, state, policy, memory, automation, coordination, and the OS shell stay connected. | A real task/session/loop journey that crosses those surfaces without duplicate state.                          |
| Durable Runtime   | Sessions survive beyond one terminal interaction and remain inspectable.                | Session CLI, event databases, SSE, UDS/HTTP parity, web session views.                                         |
| Shared Control    | People and agents operate the same daemon-owned state through structured surfaces.      | CLI `-o json`, HTTP/UDS endpoints, native tools, hosted MCP projection, truthful web views.                    |
| Bounded Autonomy  | Work ownership is token-fenced, leased, observable, and recoverable.                    | Task claim, heartbeat, complete/fail/release, coordinator state, safe spawn.                                   |
| Extensibility     | Public contracts let the operating system grow without bypassing runtime ownership.     | Host API, hooks, extensions, skills, capability catalog, SDKs, and tool registry.                              |
| Memory            | Memory is typed, scoped, file-backed, and inspectable.                                  | `compozy memory` commands, memory taxonomy, operation history, health.                                         |

### Feature Priority by Surface

- **Homepage:** the compression promise first (advanced agent work, no stack to assemble), built-in capability proof second (create, automate, supervise), agent neutrality and extensibility third. Architecture appears only as a why-it-holds caption.
- **Runtime docs:** the reader's problem first, architecture second.
- **Web UI:** truthful state and the person's next action first, marketing language last.
- **CLI help:** exact verb behavior first, product narrative only when it clarifies intent.
- **Changelog:** merged behavior and breaking changes first, no aspirational roadmap.
- **Blog/launch:** narrative is allowed, but every concrete claim still needs evidence.

## 4. Audience & Surface Intent

| Audience                                              | Reader Job                                                                                                        | Proof They Need                                                                                                                 | CTA Style                                                            |
| ----------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------- |
| Developers and technical operators running agent work | Get advanced, continuous agent work (loops, automation, approvals) without assembling the stack around the agent. | A clear install path, a first Loop or automation that works, visible session state; commands and event history one step deeper. | `Install the runtime`, `Start the daemon`, `Open the runtime docs`.  |
| Agent/runtime developers                              | Understand extension points and daemon contracts.                                                                 | APIs, SDKs, tool registry, hooks, capabilities, generated references.                                                           | `Build an extension`, `Read the Host API`, `View the tool registry`. |
| Contributors                                          | Work safely in the repo and preserve product semantics.                                                           | Glossary, AGENTS/CLAUDE instructions, tests, task specs.                                                                        | `Read the contributor path`, `Run the verification gate`.            |
| Evaluators                                            | Decide whether CompozyOS is different from local CLIs, harnesses, MCP, A2A, and workflow engines.                 | Sharp positioning, named constraints, honest maturity, sourced comparison.                                                      | `Compare the runtime`, `See what ships today`.                       |

Today's product experience is technical: terminal install, local daemon, CLI, `config.toml`. Split the claim in two and keep the halves apart.

- **Register is present tense.** Every end-user surface reads plainly today: everyday words carry the claim, and the exact mechanism stays one step away (a linked reference, expandable detail, or secondary text), not in the first sentence. This is an obligation on the surface, met now.
- **Audience is future-framed.** Present-tense audience claims stay within developers and technical operators. People who don't write code are not served by the current workflow, because install, setup, and configuration still require a terminal. That path is served when it ships.

A plainly written surface is never evidence that the workflow reaches a broader audience. Write plainly now; claim the audience later.

## 5. Voice & Editorial Rules

CompozyOS copy is people-first, plain-spoken, and calm-confident. It writes plainly for the people who run agent work today: developers and technical operators. Plain language is an obligation on every end-user surface now, and it is still not an audience claim — writing plainly does not mean the current workflow serves people who don't write code. Everyday words carry the claim; the mechanism stays one step away as proof, never as an entry fee.

### Voice

- Direct, specific, and grounded in shipped behavior.
- Calm, not cute.
- Plain, not vague.
- Confident, not inflated.
- Person-first: speak to the person whose work the agents are doing, never to an abstract "user," and never through protocol jargon. Today that person is a developer or technical operator; keep the language plain anyway.
- Product-led: CompozyOS is usually the subject.

### Style Rules

- Prefer nouns and mechanisms over adjectives.
- Prefer short sentences when making claims.
- Lead with outcomes, then mechanism, then proof.
- Prefer the everyday verb over the runtime noun on first contact: agents keep working after the tab closes; the session mechanism follows for readers who want it.
- **Define on first use.** On end-user surfaces a runtime term arrives in a fixed shape: plain label, one clause of gloss, canonical term one step deeper (tooltip, detail view, inspector, or linked reference). Reference docs may assume the term. This is a rule, not a preference — a runtime noun with no gloss and no deeper canonical term does not ship.
- Use second person in docs and how-to copy when it helps the reader act.
- **Second person is the default in UI microcopy, onboarding, empty states, and approvals.** These surfaces are talking to the person whose work is running; `you` is the plain choice there, not a sales device.
- Use `you` sparingly in marketing. It should sharpen the reader's job, not turn every line into sales copy. The sparing rule is marketing-scoped and does not reach the UI surfaces above.
- Do not use `we` or `our` in marketing body copy. Use the product as the subject: `CompozyOS does...`, `The runtime keeps...`.
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

> Agents should not stop working when a session window closes. CompozyOS keeps them in durable sessions — with saved history, resumable state, and the same view from CLI, API, and web.

## 6. Vocabulary & Naming

The glossary is authoritative. This section lists the terms most likely to appear in public copy.

### Product Names

| Term        | Use                                                                                                                                                                                                                                               |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `CompozyOS` | The public product name in prose, UI, package descriptions, calls to action, and formal category language. It names the complete system: runtime, daemon, work model, memory, automation, permissions, OS shell, extensibility, and coordination. |
| `compozy`   | The CLI command and technical identifier family. Keep the binary, `COMPOZY_*` environment variables, module path, `@compozy/*` packages, formula, sockets, config paths, and `compozy__*` tool IDs unchanged.                                     |

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
- **The second enemy**, for people who never assemble a stack: work redone by hand, agents that forget what already happened, and results nobody can verify. Name it in plain outcomes — never as a persona, never as a named rival, and never as a claim that those people are served today.
- `Batteries included.`: the label and headline form of the completeness claim. In prose use `comes built in`, `already built`, or `nothing to assemble`. Never `full-feature`.
- `simple` / `easy`: only with a measured metric behind them; until then use `assembled`, `complete`, `built in`.
- `wedge`: banned in all public copy; say `developers first` or `we start with`.
- `one workspace` / `all your agents in one place`: banned as promise framing. Agent neutrality is a feature line ("runs the agents you already use"), never the headline.

### Runtime Terms

- `ACP`: Agent Client Protocol — the standard CompozyOS uses to talk to agent CLIs. Expand it on first use in end-user prose; reference and protocol docs may use the acronym directly.
- `daemon`: the local background runtime process.
- `control surface`: a human/agent-operable surface — CLI, HTTP/SSE, UDS, or web UI — over the same daemon state.
- `session`: a durable managed agent run. Prefer `session` over `chat`.
- `event ledger`: durable event history. Use only when the implementation exposes the relevant event trail.
- `workspace`: project root and scoped runtime context.
- `tool registry`: daemon-owned tool identity, policy, discovery, and execution.
- `toolset`: grouped exposure or policy set for tools.
- `hook`: typed lifecycle dispatch. Do not call hooks a generic event bus.
- `extension`: package that can provide resources, capabilities, and Host API actions.

### Session Continue & Fork Terms

- **Continue with another agent…**: the menu and marker action that starts a new session for another agent (or runtime, or declared route) with this session's conversation carried over. The source stays unchanged. The child reads "Continued from {agent}" (status pill) and "Continued from {source title}" (transcript divider).
- `fork`: a **conversation fork** — a second session with the same agent and the conversation up to a point ("Fork session…", "Fork from here", "Forked from {title}"). Unqualified `fork` in session copy means this; always qualify the others: "fork into a worktree", "Loop fork".
- **Restart in a new session**: the dead-runtime recovery action (an empty child in the same workspace). Never call it a fork.
- `handoff`: reserved for Network; never a label for continue or fork. The API's `next_action: "handoff"` is a wire value — the UI says "Continue this session with another agent or route."
- Never in this feature's copy: `branch` (git, worktrees, Loops), `chat`, "Handoff from X", a "Badge".

### Surface Aliases

Some canonical nouns are precise in the runtime and opaque on an end-user surface. A surface alias lets the UI use the plain word without renaming anything. **canonical values stay in code, payloads, CLI, API, and reference docs; the alias is a UI label only, never a rename.**

Three rules bind every row:

- The canonical noun stays reachable one step deeper — tooltip, detail view, or inspector.
- An alias never appears in code, wire payloads, CLI verbs, config keys, or generated references.
- An alias must clear the reservations in `docs/_memory/glossary.md` before it lands. This table is mirrored there under Surface Names; the two must be edited together.

| Canonical                                                           | UI surface alias                                  | Notes                                                                                                                                                                                        |
| ------------------------------------------------------------------- | ------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `daemon`                                                            | "CompozyOS" / "CompozyOS is running"              | Never "daemon" in end-user UI. The word stays canonical in this file, the glossary, and runtime docs.                                                                                        |
| `workspace`                                                         | "project"                                         | Alias may NOT be "environment" — reserved for process-level variables and operating-system context.                                                                                          |
| `event ledger`                                                      | "history"                                         | Use only where the implementation actually exposes the event trail.                                                                                                                          |
| `tool registry` / `toolset`                                         | "what agents are allowed to do"                   | A descriptive gloss, not a label swap. The registry keeps its name in every other surface.                                                                                                   |
| `control surface`                                                   | — drop from UI                                    | Internal vocabulary. It stays the runtime term in specs and docs, and never reaches an end-user label.                                                                                       |
| `capability`                                                        | keep + define on first use                        | Wire identity `(peer_id, capability_id)` is unchanged. Forbidden synonyms (`recipe`, `procedure`, `playbook`) stay forbidden everywhere.                                                     |
| `session`                                                           | keep + gloss on first use                         | Already everyday English; it earns a gloss, not an alias.                                                                                                                                    |
| `terminal`                                                          | "Terminal"                                        | First-class product surface. Never label it "console" or "shell pane".                                                                                                                       |
| `Loop`                                                              | — pending owner decision                          | Do not alias. `workflow` is no longer a forbidden synonym for `capability`, but the historical "workflow" positioning is still warned off in the glossary. No alias until the owner decides. |
| `Jobs` / `Triggers` (dock titles)                                   | — pending owner decision                          | Do not rename. Dock titles stay as they are until the owner decides.                                                                                                                         |
| settings group `Operator`                                           | "Personal"                                        | Group label only.                                                                                                                                                                            |
| settings section `Observability`                                    | "Diagnostics"                                     | Section label only.                                                                                                                                                                          |
| settings section `Attention`                                        | "Notifications"                                   | Section label only.                                                                                                                                                                          |
| settings section `Gateway`                                          | "Remote access"                                   | Section label only.                                                                                                                                                                          |
| settings group `workspace`                                          | "Basics"                                          | Group label only.                                                                                                                                                                            |
| settings group `runtime`                                            | "Agents"                                          | Group label only.                                                                                                                                                                            |
| settings group `system`                                             | "Advanced"                                        | Group label only.                                                                                                                                                                            |
| Loop `generation`                                                   | "round"                                           | One iteration of a Loop run. Wire, CLI, and payloads keep `generation`.                                                                                                                      |
| Loop step `quarantined`                                             | "set aside"                                       | A step removed from scheduling after repeated failures; the UI verb is "Retry" (wire: requeue).                                                                                              |
| fork (built-in Loop)                                                | "Copy and edit"                                   | UI verb for forking a built-in Loop into the project.                                                                                                                                        |
| memory `dream`                                                      | "tidy up"                                         | Memory consolidation. `dream` stays in API, CLI, and config keys.                                                                                                                            |
| extension dev overlay                                               | "local development copy"                          | Menu verb "Unlink local copy".                                                                                                                                                               |
| session status tokens (`waiting-for-input`, `hung`, `unhealthy`, …) | "Needs your answer", "Stuck", "Having trouble", … | Display words only; the token stays on `data-badge`/aria. Color only states that need the user.                                                                                              |

`Roles`, `Hooks`, and `Extensions` keep their names — they are glossary terms that already read plainly.

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
- `desktop pager`: the minimal dot control in the menubar tray for switching desktops; an orange dot marks an off-screen desktop that needs you. The All desktops button beside it opens Desktops Overview, where full create, rename, reorder, transfer, and delete actions live.
- `dock`: the rail of app launchers along the left edge (a bottom tab bar in compact presentation), with running/minimized indicators and badges bound to runtime projections. Its foot holds the profile switcher, the light/dark toggle, and Settings. User-facing copy says "dock", not "rail".
- `menubar`: the top bar — CompozyOS mark (its menu holds Settings), Global scope globe, workspace trigger, app menus, then the tray: desktop pager, All desktops, the approvals bell, the ⌘K palette. The globe sits between the mark and the chip and is the only owner of Global vs workspace destination. Chip identity is the project name when scoped down, or **Global** (`~`) when Global scope is on.
- `window manager`: the daemon-authoritative, workspace-scoped topology and command surface for desktops and windows. Browser focus and the active desktop are client-local projections. This presentation data never contains agent `memory`.
- `theme`: the light or dark color scheme. Settings › Appearance offers **Light**, **Dark**, and **System** (help: "System follows your computer's light or dark setting."); Dark is the default. The dock-foot toggle reads "Switch to light mode" / "Switch to dark mode" (tooltip "Light mode" / "Dark mode"). Say "light" and "dark", never "night mode" or "day mode".
- Empty desktop: "{desktop name} is empty" + "Open an app from the dock, or press ⌘K to open anything." + one primary **New session**. It is a card on the desk, never a modal and never a blocking wall.
- Window arrangements (Window › Arrange): **Main and stack**, **Columns**, **Grid**, **Balance sizes**; **Move window to** sends the focused window to another desktop by name. Use "desktop", never "space" or "workspace", for these destinations.

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
- architecture only as a why-it-holds caption ("one runtime, one state model"), never a section lead.
- install path as primary conversion.
- concrete signal cards only when the numbers are current.

Avoid:

- leading with ACP, JSON-RPC, stdio, UDS, SQLite, delivery internals, or package names.
- leading with architecture, OS purity, or category tests.
- centralization framing ("one workspace", "all your agents in one place") as the promise.
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

Internal window controls use `Zoom window` to maximize and `Restore window` while
zoomed to return to the saved placement and size. These labels describe internal
CompozyOS windows, not native browser or application fullscreen. Resizing a zoomed
window adopts the resized geometry instead of restoring its saved placement.

Use:

- current state, next action, and consequence.
- labels that map 1:1 to a backend noun, using the §6 surface alias where one is defined; the canonical noun stays reachable (tooltip, detail, inspector).
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
to the headline, keep the category label small, and never let architecture, centralization, stand in for the promise.

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
Eyebrow: Tasks
Title: Hand off work with evidence
Description: Task runs retain ownership, review decisions, and the evidence needed for the next step.
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
- `Open the runtime docs`
- `Create a session`
- `Send a message`
- `Build an extension`
- `Inspect events`

On plain-register surfaces (first-run, onboarding, empty states, approvals, notifications) prefer the CTA that names the outcome in everyday words:

- `Open CompozyOS`
- `See what's running`
- `Approve this step`
- `Stop this run`

Avoid:

- `Get Started` when a specific action exists. The ban is conditional: if a more specific true action exists, name it; if nothing more specific is true yet, `Get Started` is allowed.
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
- `capability` vs old `recipe`, `workflow`, `procedure`, or `playbook` language.
- runtime behavior that moved from planned to shipped or from spec to deleted.
- register and audience collapsing into one claim: plain register is required on every end-user surface today, while serving people who don't write code is still vision. A plainly written surface is not evidence the workflow reaches them, and a technical workflow is not permission to write a surface obscurely.
- a surface alias escaping the UI: an alias appearing in code, payloads, CLI verbs, config keys, or generated references is a rename, and renames do not happen through §6.
- people-first language drifting back toward control-room personas: the runtime term is `control surface`.

## 11. Agent Prompt Guide

Use these as task-local prompts after reading the target files.

### Rewrite a Homepage Hero

```text
Use COPY.md and DESIGN.md. Use the §2 Hero Lock verbatim; do not invent or relock the headline or subhead. Lead with the compression promise, prove the built-ins (create, automate, supervise) with current runtime evidence, keep architecture as a why-it-holds caption. Primary CTA installs or starts the runtime; the secondary CTA points to the strongest supporting proof.
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
Map every label 1:1 to a backend noun, using the COPY.md §6 surface alias where one is defined; keep the canonical noun reachable one step deeper (tooltip, detail, inspector). State what is true, what action is available, and what happens next. Do not imply a metric, control, or repair path exists unless the runtime exposes it.
```

### Review Public Copy

```text
Check runtime truth, glossary vocabulary, claim maturity, CTA specificity, forbidden phrases, stale counts, and metadata drift. If a claim cannot be traced to code, docs, tests, generated references, or a release artifact, narrow or remove it.
```

## 12. Review Checklist & Maintenance

Before shipping copy or product-facing text, verify:

- Runtime truth is checked against current code, generated references, docs, tests, or release artifacts.
- The copy uses `CompozyOS` and `compozy` correctly.
- Glossary terms are applied, especially `capability`, `skill`, `AGENT.md`, and `AGENTS.md`.
- Inline example lists of agent CLIs use the canonical trio (Claude Code, OpenClaw, and Hermes) unless a CLI-specific reason exists.
- ACP driver/agent counts in public copy are derived from `PROVIDERS.length`, not a hardcoded number.
- Claim maturity is clear.
- The promise leads: the system around the agent, already built. Architecture ("one runtime, one state model", OS-purity tests) appears only as a why-it-holds mechanism, never in a headline slot.
- No `wedge`, no `full-feature`, and no `simple`/`easy` claims without a metric behind them.
- No centralization promise (`one workspace`, `all your agents in one place`).
- Register and audience are checked separately: every end-user surface reads plainly today, and present-tense audience claims stay within developers and technical operators — people who don't write code stay future-framed.
- Surface aliases match the §6 table, clear the glossary reservations, and appear as UI labels only; the canonical noun is still reachable one step deeper.
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
- the canonical example trio of agent CLIs needs to change
- generated CLI/API surfaces change in a way that affects public docs or examples
- a review finds repeated copy drift across surfaces

Do not use `COPY.md` as a dumping ground for campaign-specific copy. Put dated campaign drafts, competitor research, and one-off launch material in task or analysis artifacts, then distill only durable rules back into this file.
