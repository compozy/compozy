## 0.3.0 - 2026-08-18

### ♻️ Refactoring

- State management with xstate-store (#268)
- Site improvements (#277)
- Replace bundles with extension kits (#291)
- Modernize Go runtime packages (#293)
- Use geist instead of inter (#334)
- Add global workspace toggle (#368)
- Unify PRD and TechSpec into a single spec pipeline (#397)

### 🎉 Features

- Introducing CompozyOS beta
- **BREAKING:** introducing CompozyOS beta
- Add provider-neutral ACP speed control (#267)
- Support grouped skill directories (#270)
- Session transcript calm-surface redesign (#271)
- Add permission-aware cross-workspace access (#275)
- Extensions improvements (#278)
- Select and switch runtime per session prompt (#283)
- Adopt MCP 2026 and expand the curated catalog (#284)
- **BREAKING:** MCP catalogs now require manifest_version 2 and public
  MCP transport no longer accepts SSE.
- Add support for window tabs (#287)
- Adopt feedback semantics for durable Loops (#290)
- Add complete Loop node lifecycle (#305)
- Rewind sessions to durable conversation checkpoints (#310)
- Add session-aware slash skill commands (#311)
- Add reversible session archiving and list actions (#309)
- Replace software-delivery with implement-tasks (#325)
- **BREAKING:** remove software-delivery; use implement-tasks without gate inputs.
- Parent and child sessions (#327)
- Add secure remote gateway access (#331)
- Ship CompozyOS desktop app (#336)
- Add first-class worktree support (#388)
- Close the loops UI visual-contract parity gap (#406)
- Unify zero-inventory empty states for jobs, triggers, and tasks (#409)
- Redesign workspaces overview as command-tab switcher (#410)
- Redesign trigger detail into the When/If/Then rule page (#411)
- Session attachments — paste, drop, and picker to multimodal agents end to end (#412)
- Add Agent Plugins ingestion to extensions (#419)
- Add session attention and orchestration parity (#422)
- Replace Tauri with Electron and unify updates (#424)

### 🐛 Bug Fixes

- Align release and process test contracts
- Workspace params (#266)
- Onboarding workspace selection
- Restore Windows daemon support (PR #163 regression + new fixes) (#274)
- Untested cases from qa (#276)
- Remaining untested qa (#279)
- Durable session messaging (#288)
- Assistant-ui version
- Changelog generation (#292)
- Make busy-session inputs durable (#304)
- Restore durable ACP session continuity (#307)
- Preserve Loop template manifests across hydration (#317)
- Preserve ACP stream disconnect recovery (#319)
- Discover Cursor model catalog before sessions (#320)
- Keep managed skill loading on native seam (#323)
- Loop run bugs (#324)
- Enforce bundled agent and Loop ownership (#326)
- Session native tools and extensions details (#330)
- Judge gate on goal loops
- Restore minimum-age dependencies
- Stabilize release runtime startup
- Start absent SSH daemon
- Make loop goals converge reliably (#335)
- Desktop issues
- Ship the full desktop icon set required by Windows tauri-build
- Pause the Windows desktop lane and ship macOS + Linux only
- Adjust project copy
- Publish staged GitHub release drafts
- Repair release integration contracts
- Harden desktop startup and diagnostics (#343)
- Resolve desktop and session usability issues (#351)
- Recover canceled Loop coordinators on restart (#353)
- Resolve extension-published agents for session command catalogs (#350)
- Eliminate persistent performance bottlenecks (#354)
- Reject automation trigger events no producer emits (#358)
- Read loop watch-events with a stream-global cursor (#356)
- Prevent session timeline render loops (#361)
- Align update artifacts with release policy (#363)
- Canonicalize desktop runtime manifests (#364)
- Validate desktop bundles before release
- Make release publication atomic
- Defer npm publication safely
- Harden release recovery
- Isolate release publishers
- Make release artifacts reproducible
- Align release automation contracts
- Reject invalid agent names (#367)
- Resolve extension agent skills and missing config paths (#372)
- Include extension tools in hosted MCP bootstrap (#373)
- Use conventional release triggers
- Resolve main lint regressions
- Parse nanosecond daemon timestamps
- Validate release workflow tools by type
- Preserve awaited child loop state (#391)
- Use exact Cursor ACP model values (#392)
- Preserve dead session history (#393)
- Validate desktop release smoke wiring
- Validate public CLI version command
- Clarify missing config path errors (#401)
- Guard autonomous memory extractor writes (#396)
- Write $ENV interpolation in pi runtime models.json apiKey (#404)
- Authorize daemon-owned loop effects (#407)
- Retain terminal loop effect results in web (#408)
- Bound desktop runtime health checks (#414)
- Preserve loop goal session lineage (#420)
- Support resource-only extension development (#423)

### 🧪 Testing

- Guard ACP initialize protocol version (#318)
- Align nightly runtime fixtures
- Align dead session recovery coverage
- Preserve loop claim tokens in daemon fixtures (#418)

### Release Notes

#### Breaking Changes

##### Extension kits replace Bundles

The Bundle surface is gone. Extensions are now the single packaging unit, and installing one is inert: it publishes no tools and no resources until you explicitly enable it. Before enabling, you can preview exactly what an extension would publish and inspect what it is publishing right now, and any extension that declares network access must have its network requirement digest confirmed by a person. (#291)

- `compozy extension preview <name>` shows what enabling would publish without changing state; `compozy extension inventory <name>` shows the live published inventory. Agents get the same reads through `compozy__extensions_preview` and `compozy__extensions_inventory`.
- `compozy extension enable|disable|update|install` accept `--confirm-network-requirement <digest>`, so a network-declaring extension cannot start publishing without an explicit confirmation recorded on the install.
- `compozy extension secrets set|bind|list|unset` manages write-only environment bindings that are scoped per workspace and stored as secret references, never as values.
- Marketplace kinds are now exactly `extension`, `mcp`, and `skill`.

Migration notes: the whole `compozy bundle` command group is removed (`catalog`, `preview`, `activate`, `list`, `get`, `deactivate`, `network-settings`), along with the `compozy__bundles_*` native tools, the `compozy__bundles` toolset, the `bundle` marketplace kind, and the Bundle API surfaces. There is no alias — rebuild bundle-shaped setups as extensions and enable them explicitly.

##### Runtime hardening and secret-safe provider login

A broad modernization pass across the Go runtime tightened lifecycle and cleanup ownership, ID allocation, task settlement, filesystem confinement, and streaming framing. Most of it is invisible, but it lands several deliberate cuts that change what operators, scripts, and agents see. (#293)

- `providers.<id>.auth_login_command` is now write-only. You can still set it through `config.toml`, `compozy config set`, or `compozy__config_set`, but no read surface returns it. `compozy config show|list|get|diff`, provider status, doctor, Settings, HTTP, and UDS return a safe `login` descriptor instead: whether it is configured, its source, the executable basename, whether that executable is present, and a recommended action.
- Every per-session event database carries an immutable owner and physical identity. A database that was copied between sessions or workspaces is refused before any migration or mutation — no adoption, no rebinding, no automatic repair. The operator recovery path is documented under "Session event store ownership".
- Markdown files under `<workspace>/knowledge/` are injected as a bounded workspace knowledge snapshot before each accepted turn, including task, task-creator, and Heartbeat wakes. It is prompt context for the turn, not durable memory.
- Installing an extension from Git now requires Git 2.37 or newer and reports `extension_git_version_unsupported` when it is older. Git sources must be HTTPS and resolve to public addresses.

Migration notes: `compozy provider auth login --print-command` is removed. The config key `memory.recall.signals.metrics_enabled` is removed with no alias. The task-notification native tools take `workspace_id` instead of `workspace`, and the old input is not an alias. Notification cursor identity and `delivery_id` are now opaque values that must be echoed back byte for byte.

##### implement-tasks replaces the software-delivery Loop

The bundled dev-cycle Loop now does one job clearly: implement authored task files in dependency order. `software-delivery` is gone and `implement-tasks` takes its place, with a five-node graph — `slug_input → load_tasks → implement → execute_task → collect` — and only three inputs. The old second control layer for review, command verification, and human approval is removed from the bundled Loop; task-level validation, self-review, tracking updates, and optional per-task commits stay inside the implementation agent's own prompt. (#325)

- Inputs are now `slug`, `implementer`, and `auto_commit`. The `review`, `verify`, and `approve` nodes and their edges are deleted, along with the verification contract, stale hash fields, and target-branch handling.
- The separate bundled `review-and-fix` Loop is unchanged, and custom Loops can still declare their own command gates — `verify_command` remains part of the generic Loop DSL.
- The catalog, Loop overview, configuration examples, migration guide, web routes, and the official CompozyOS skill all name `implement-tasks`.

Migration notes: this is a hard cut with no alias. Any config, CLI or API call, automation binding, or documentation link that says `software-delivery` must say `implement-tasks`, and the `target_branch` and `verify_command` inputs must be dropped from `[loops.inputs.*]`.

```toml
# before
[loops.inputs.software-delivery]
target_branch = "main"
verify_command = "make gate"
auto_commit = false

# after
[loops.inputs.implement-tasks]
auto_commit = false
```

##### The desktop app is now Electron

The Tauri/Rust desktop host is replaced by an Electron shell with a narrow preload boundary, and updating became one durable daemon-owned operation exposed identically through the CLI, HTTP, UDS, Settings, and the menubar. The app provisions the bundled daemon from an empty home, or attaches to a compatible daemon that is already running without taking ownership of it, keeping single-instance focus, deep links, safe navigation boundaries, page zoom, window geometry recovery, diagnostics, logs, and the owned-versus-attached quit contract. (#424)

- Runtime and App are separate update tracks with operation progress, holder-aware blocked state, staged-next-launch state, apply and cancel actions, and truthful absence when a track is unsupported.
- A keyboard-accessible menubar indicator appears only when an update is actionable and navigates to Settings. The renderer holds no desktop-only update authority, and the SPA behaves the same in a browser and in the app.
- Desktop artifacts are planned, inventoried, and channel-checked as one release authority, with notarization and signing input checks and packaged smokes provisioned from empty isolated homes. On macOS the ZIP and on Linux the AppImage are the updater artifacts; DMG and DEB are install artifacts only.

Migration notes: this is a hard cut with no compatibility bridge. The Tauri runtime, commands, permissions, capabilities, fixtures, generated bindings, Cargo dependencies, build configuration, scripts, config keys, docs, and tests are deleted rather than deprecated. Install the app from the artifacts published with this release. The installed-app update walk from one beta to the next was not verified for this build, so the App track must be proven by a release owner across a fresh beta pair before it is treated as delivered.

##### The OS Release

CompozyOS v0.3 is a new operating system boundary for agent work. Sessions, tasks, loops, memory,
permissions, automation, the OS shell, and Compozy Network now share one daemon-owned state model.
People can start and inspect that work from the web, CLI, HTTP/SSE, or UDS. Agents can operate the
same runtime through structured tools and extension contracts.

This is a breaking beta. The command, package, environment, storage, API, and tool namespaces move
to CompozyOS, and several v0.2 surfaces have deliberate replacements or removals. Follow the
[v0.3 migration guide](https://compozy.com/runtime/migration/) before replacing an existing install.
The maintained v0.2 line and its collateral remain on `legacy/v0.2`.

Install the beta through the verified hosted installer, `@compozy/cli@beta`, or the explicit
`github.com/compozy/compozy@v0.3.0-beta.1` Go version. The beta channel may change before v0.3.0
stable; production rollouts should pin the version and review each prerelease.

The repository was already MIT licensed. v0.3 corrects stale BSL-1.1 text in distribution metadata;
it does not relicense the code.

#### Features

##### Agent Plugins install as extensions

CompozyOS ingests [Agent Plugins 1.0.0](https://agent-plugins.org/) packages as extensions with no product-specific manifest. A portable plugin contributes skills plus local or remote MCP servers while keeping the existing extension lifecycle, trust, isolation, diagnostics, Marketplace, CLI, HTTP, UDS, native-tool, and Web management surfaces. (#419)

- Manifest discovery and validation are strict: fixed-location skills and MCP configuration, closed schemas, deterministic diagnostics, native-manifest precedence, and safe rejection of unsupported components.
- Portable skills and stdio or streamable-HTTP MCP servers are synthesized into the canonical extension model, with absolute `PLUGIN_ROOT` and `PLUGIN_DATA` expansion, single-token stdio commands, package-root working directories, remote-header bindings, URL policy, and secret redaction.

Migration notes: end-to-end delivery is claimed only for the provider paths proven end to end, Claude Code and Hermes. OpenClaw's current ACP bridge advertises `session_mcp=false`, so CompozyOS fails closed instead of pretending to deliver session MCP servers.

##### Attention: know which session needs you

CompozyOS has one daemon-owned attention model. Pending input, permission requests, finished-unseen sessions, operator presence, notification delivery, and cross-workspace session discovery are runtime state instead of per-surface guesses. Orchestrator agents get structured wait, spawn, stop, approve, clarify-answer, prompt-cancel, and notify controls across native tools, CLI, HTTP, and UDS, so an agent supervising other agents no longer polls a shell or depends on the web UI to act. (#422)

- The global catalog persists canonical attention revisions, pending interactions, seen and settled state, and cursor-stable attention ordering.
- Presence leases, attention summaries and events, sanitized interaction discovery, generalized waits, prompt cancellation, operator notifications, and session wake propagation are available on every transport, with deterministic CLI exit behavior.
- Desktop shortcuts moved to a daemon-owned, configurable keymap, and the command palette gained nested views including an attention-first Sessions view.

##### Bundled Tailscale connectivity extension

Gateway reachability ships with a first-party provider. The `tailscale` extension runs a Tailscale
node inside the CompozyOS process through `tsnet`, against the operator's own account — nothing else
to install, and CompozyOS operates no relay, server, or account on anyone's behalf. The private tier
serves `https://compozy-gateway.<tailnet>.ts.net:8443` on the tailnet; the public tier serves the
same hostname over Tailscale Funnel on 443. (#331)

- Bind the auth key once with `compozy extension secrets set tailscale --env TS_AUTHKEY` (hidden
  input); the value never appears in output, status, or diagnostics.
- The extension declares required Live network participation for `gateway.private` and
  `gateway.public`, so enabling asks for a one-time digest confirmation — and asks again only when
  that declaration changes.
- First activation provisions the HTTPS certificate before the Funnel listener opens, verifies
  public endpoints through authenticated DNS-over-TLS (`gateway.verify.public_dns_resolver`), and
  keeps unverified listeners staged with bounded retries instead of tearing them down.
- Third-party providers implement the same `connectivity.provider` contract from the Go and
  TypeScript SDKs, gated by install-source trust and control-digest re-confirmation on every enable
  and boot.

##### Complete Loop node lifecycle

Loops now have a full declarative failure contract at the node level and precise repair controls at the operator level. Authors classify failures, declare retries with backoff, route errors, absorb them with `allow_fail`, set attempt timeouts and deadlines, emit `on_*` effects, and add durable wait nodes. Operators pause, resume, cancel, kill, or requeue individual nodes and list what is waiting, quarantined, retrying, or asking for attention — all from the CLI, HTTP, UDS, native tools, and MCP, without opening the web UI. (#305)

- `compozy loop cancel` drains a run safely and `compozy loop kill` closes it immediately; `compozy loop node pause|resume|cancel|kill|requeue` repairs a single node, and `compozy loop nodes --state waiting|quarantined|attention|retrying` inventories a run.
- Agents get the same controls through `compozy__loop_cancel`, `compozy__loop_kill`, `compozy__loop_node_pause`, `compozy__loop_node_resume`, `compozy__loop_node_cancel`, `compozy__loop_node_kill`, `compozy__loop_node_requeue`, and `compozy__loop_nodes`; `compozy__loop_status` now reports node lifecycle state.
- Repeated failure on one node quarantines that node instead of terminating the run, so independent lanes keep working, and the failure cause is classified rather than matched from a magic string.
- Long-running Loop-bound sessions are no longer killed on elapsed time. Liveness is judged from real evidence, and prolonged silence raises attention instead of ending the work.
- Defaults are tunable through `loops.defaults.delivery.*`, `loops.defaults.watch.*`, and `loops.breaker.*`, and new blocking lint rules reject invalid routes, impossible timing, malformed effects and waits, and watch sources without a stable identity.

Migration notes: `compozy loop stop` is deleted — the CLI verb, the HTTP route, and the `compozy__loop_stop` native tool. Choose `cancel` or `kill` explicitly. Extension watch sources must now declare `event_key`; a source without a stable event identity is rejected before a run starts.

##### Cursor models come from your account

Cursor used to look curated in CompozyOS but was not truthful to the account that was signed in: a small hand-written list stood in for the real catalog and, worse, acted as an allowlist that rejected valid model ids before Cursor ever saw them. CompozyOS now reads the account catalog from `cursor-agent models` before a session exists, and exact provider model ids are forwarded unchanged. (#320)

- The first catalog read bootstraps Cursor discovery once and persists the outcome; later reads serve the cache, and explicit refresh is the refresh boundary. In the QA account this surfaced 193 real models, including `composer-2.5`.
- Only `id - display name` rows are parsed. Headings, tips, duplicates, and empty output can never become invented models.
- Curated data is metadata again, not membership policy. Sessions and Loops accept ids like `cursor/composer-2.5`, and an unknown _provider_ still fails with a structured `unknown_provider` error.
- `providers.<id>.models.discovery` applies live — no daemon restart. A provider outage records the failure and keeps the rows you already have; disabling discovery clears them and records `disabled`.
- In the web runtime selector, "Use an exact custom model ID…" now opens a dedicated field: empty input cannot be committed, typing turns the action into `Use "<id>"`, Enter and click both commit, and closing returns to normal catalog search.
- Cursor keeps the operator `HOME` its `native_cli` login contract expects.

Migration notes: the curated Cursor allowlist and its session preflight are deleted with no compatibility bridge. If a provider rejects an id, that provider's error is now the authority.

##### First-class Git worktrees

Git worktrees are runtime objects across the daemon, Web desktop, CLI, HTTP and UDS, native tools, extensions, configuration, generated contracts, and documentation: create, adopt, discover, inspect, reconcile, dismiss, and safely remove isolated checkouts. Sessions, Task runs, fan-out workers, and Loop environments bind to an exact worktree without losing the parent workspace's config, skills, agents, or memory. (#388, #410)

- The new worktree domain owns Git capability detection, canonical repository identity, naming and placement, per-repository mutation locking, and durable lifecycle state.
- Creation is a phased `pending → ready` operation with recorded ownership checkpoints, bootstrap copy and setup support, cancel-safe rollback, and boot recovery.
- Adoption verifies the linked checkout, the common Git directory, main-checkout exclusion, and repository identity before registering it. Discovery merges Git-known checkouts with durable records without turning a discovered row into an adopted worktree.
- Removal fences the record as removing, rechecks session activity and Git safety under the repository lock, preserves branches and history, and requires an explicit second step for dirty or unique-unpushed work.
- Finishing work runs through a truthful assisted-exit ladder for commit, push, pull request, merged evidence, and cleanup. Dismissed tombstones release their reserved name, exit actions resolve caller references to canonical record ids, and CLI mutation output keeps worktree identity.

##### Grouped skill directories

CompozyOS now discovers `SKILL.md` definitions at any depth below each skill root, so teams can organize capabilities under folders such as `marketing/content/` without changing frontmatter identity or normal precedence. `compozy skill create <name> --group <relative/path>` now scaffolds grouped workspace skills safely.

##### Feedback semantics for durable Loops

A rejected Loop generation no longer restarts blind. The rejection is carried into the next attempt as context, only the producers responsible for it are re-run, and an opt-in ratchet keeps the best-scoring generation instead of losing it to a later regression. Every generation now records its origin, its parent, the gate verdict, the score, and the blocking issues inside claim-fenced transactions, so the CLI, HTTP, UDS, native tools, SSE, and the web UI all read the same durable run truth. (#290)

- Loop templates can read `previous.*` (including `previous.generation` and `previous.route_causes`) and `best.*` to steer the next attempt from what actually failed.
- Metric gates take a direction — `maximize` or `minimize` — plus a `min_delta` improvement threshold, so a regression is rejected deterministically. Invalid thresholds fail authoring with `metric_min_delta_invalid`.
- `compozy__loop_status` and `compozy__loop_runs` project score, best generation, gate verdict, and generation origin and parentage; run detail, catalog, and recent-runs views render the same fields.
- `compozy extension list` and `compozy extension status` accept `--workspace`, so agents can inspect workspace dev overlays without dropping to raw HTTP.

Migration notes: this is a greenfield hard cut that discards existing Loop run history. The migration clears Loop runs, run events, gate decisions, generation outputs, goal turns and checkpoints, session bindings, and output blobs, along with the task and automation runs that referenced them. Export anything you need before upgrading.

##### Loops UI and empty catalogs match their design contract

Every Loops surface adopts the approved visual contract's locked review decisions, collapse and section grammar, icon budget, and truthfulness rules, rendering only daemon-backed data and adding no helper copy. Jobs, Triggers, and Tasks share one zero-inventory empty state with the same composition, density, and icon grammar. (#406, #409)

- Automation suggestions and task templates are empty-state affordances again: they render only in the unfiltered zero-inventory state instead of sitting above a populated Jobs catalog.
- The three catalogs compose the same components and differ only in icon, title, support line, action, and panel content.

##### Remote gateway: reach your daemon from anywhere

A fresh install is still reachable only from the machine it runs on — and now that is a choice
instead of a limitation. The remote gateway adds three independent, off-by-default switches: a
private overlay that serves the full product to devices you pair over your own Tailscale network, a
public delivery ingress that accepts only signed webhook and bridge callbacks, and consent-gated
public operator access for devices that cannot join the overlay. (#331)

- The daemon never binds a public address. Gateway tier listeners stay on loopback and a
  connectivity provider publishes a verified route to them: an address is advertised only after the
  daemon fetches a one-time challenge through it and gets its own nonce back.
- Reaching an address is never authentication. Devices pair with single-use, five-minute artifacts
  written to private `0600` files, credentials are stored only as hashes, and
  `compozy device revoke` cancels live streams before it returns.
- `compozy gateway status|audit`, `compozy pair`, `compozy device`, and `compozy connect` (HTTPS
  profiles plus zero-exposure SSH) operate everything, with the same state in **Settings → Gateway**
  and the `compozy__gateway` native tool.
- Public delivery verifies CompozyOS's timestamped HMAC contract on every request, with replay
  protection and per-source rate limits. There is no store-and-forward while the daemon is offline —
  senders own retries.

Setup guides live in the new Gateway docs section: https://compozy.com/docs/gateway.

##### Reversible session archiving and list actions

A stopped session can now be archived so it leaves the default catalog without deleting anything. History, events, ledger, and the saved runtime choice stay readable, and unarchiving puts the session back exactly as it was — still stopped, so a normal prompt restarts it. Both session lists gained a row menu with state-aware Stop, Archive, Unarchive, and Delete, a delete confirmation, and a separate section for archived sessions. (#309)

- `compozy session archive <id>` and `compozy session unarchive <id>`, plus `compozy session list --archived` for archived only and `--include-archived` for both. Agents get `compozy__session_archive` and `compozy__session_unarchive`, and extensions get `sessions/archive` and `sessions/unarchive` under `session.write`.
- The catalog contract takes `archive=exclude|only|include` and defaults to `exclude`, with exact filtered totals and cursor fingerprints. Archived sessions are excluded from normal metrics.
- Archiving is stopped-only and idempotent. An archived session stays readable, but prompt, attach, and resume are refused until you unarchive it. Hard delete is unchanged, and archive stays catalog metadata rather than a lifecycle state.
- Bridge providers now wait for HTTP route readiness before serving, which closes a startup race across the bundled Discord, Google Chat, GitHub, Linear, Slack, Teams, Telegram, and WhatsApp runtimes.

Migration notes: existing sessions are unarchived, so nothing changes until you archive something.

##### Rewind a session to an earlier checkpoint

You can now rewind an idle session back to one of your earlier messages instead of starting over. The selected message and everything after it leave the active transcript, the message text comes back as a composer draft, and the session continues under the same session ID with a fresh agent context rebuilt only from the part you kept. Rewind touches the conversation only — it does not undo file edits, tool effects, network activity, saved memory, or anything the provider already did outside CompozyOS — and the discarded events stay archived for audit. (#310)

- `compozy session rewind <session-id>` picks the cut point with `--message-id` and reads the current transcript fences for you; scripts retrying a known request pass `--expected-generation`, `--expected-epoch`, and `--expected-max-sequence` together with the original `--idempotency-key`. Agents get `compozy__session_rewind`.
- Retrying the same rewind with the same idempotency key returns the original result, and the response carries the `draft_text` that goes back into the composer.
- A rewind is refused with a clear conflict — and without cutting the transcript — when the fence is stale, the session is busy, input is queued, an approval is pending, or the session is daemon-managed. It is serialized against clear, delete, repair, resume, and other prompt-producing operations.
- In the web UI, the action appears on your own durable messages, requires an empty composer, confirms the side-effect boundary before it runs, and restores the draft afterward.
- Reads take an `archive` selector: `compozy session events` and `compozy session history` accept `--archive active|archived|all`, and the same selector exists on the HTTP and UDS reads.

Migration notes: `session events` and `session history` now default to `archive=active`. They previously returned archived rows alongside active ones — pass `--archive all` to keep the old behavior.

##### Session attachments: paste, drop, or pick

The session composer accepts images (PNG, JPEG, WebP) and files (PDF, Markdown, plain text) by paste, drag-and-drop, or file picker. Attachments persist before the prompt is accepted, ride the prompt as provider-neutral references, and reach multimodal agents as protocol-conformant ACP content blocks gated by the capabilities that agent negotiated at initialization. Saving a screenshot to disk and describing its path is no longer the workaround. (#412)

- The daemon keeps the agent's prompt capabilities from the initialize handshake instead of discarding them, so unsupported content is refused in place rather than sent to an agent that never advertised it.
- Attachments render durably in the transcript across reload, live streaming, recap, and archive, and they are deleted with their session.
- The capability gate lives inside the composer's attachment strip. Steering a running prompt stays text-only.

##### Session-aware slash commands

Slash commands in the composer are now backed by a single daemon-owned catalog scoped to the exact session, and they work anywhere in a prompt rather than only at the start. Built-in and ACP control commands stay standalone, while a skill command can be dropped inline, repeated, and mixed with the text you already typed; the matched skill's full instructions are injected into that same turn. The same catalog is readable from the CLI, HTTP, UDS, and a native tool, so agents can discover what a session can actually run. (#311)

- `compozy session commands <session-id>` lists the catalog, and agents read it with `compozy__command_list` in the `compozy__catalog` toolset. `compozy__skill_view` accepts a source-qualified `command_id` from that list.
- The catalog groups Built-in, Agent, and Skills at the start of a prompt and shows only effective skills inline. A bare `/skill` resolves the effective winner across bundled, global, additional, workspace, and agent-local sources; extension skills use `/extension-id:skill` and marketplace skills use `/registry-id:skill`.
- What is effective respects global and workspace scope, agent activation and disable lists, runtime disable overlays, enabled and disabled extension resources, workspace and session ownership, and live resource revisions. A `session_commands_changed` stream frame refreshes only the affected session.
- Injection is bounded at 24 KB per skill and 64 KB per turn, and repeating the same skill activates it once. Invocations persist through admission, queueing, replay, transcript storage, and the UI, and queued ones are revalidated against the exact source before dispatch.
- Slash activation is limited to human operator input. Agent-authored prompts and `compozy__session_prompt` keep slash-shaped text literal, and hooks can remove an admitted invocation but never add one.

##### The workspace switcher works like Command-Tab

The fullscreen workspaces overview is rebuilt as a Command-Tab style switcher over the live shell: a glass tile strip, a frosted focus plate, an identity caption, an always-visible vertical worktree menu, a full keyboard model, and a registered `⇧⌘W` shortcut. It switches workspace identity only; window arrangements stay with the Desktops overview. (#410)

Migration notes: the previous 264 px dossier grid, with its member stacks, path footers, and "Enter →" row, is deleted rather than kept behind a flag.

##### Trigger detail is a rule page

A trigger now reads as the rule it is, not as a job inspector with the cron stripped out: a plain-language sentence of what the trigger does, a labeled Enable switch opposite it as the only accent on the page, one When / If / Then card, a single-open Recent-runs accordion, a four-card rail, and an Inspect sheet for runtime internals. (#411)

Migration notes: the shared automation detail panel is jobs-only again, and triggers render through their own component family.

##### Unified docs and Marketplace

`compozy.com` now serves a single `/docs` experience with reworked navigation, breadcrumbs, responsive layouts, generated CLI reference pages, and API references that include Go examples. A new `/marketplace` section lists skills, extensions, MCP entries, bridge providers, and bundled capabilities with search, install commands, and detail pages. (#277)

Migration notes: two CLI verbs were renamed and their old spellings removed — `compozy mcp authorize <server>` is now `compozy mcp auth login <server>`, and `compozy memory extractor list-pending` is now `compozy memory extractor list-failures`. The `compozy network work status` alias was removed in favor of `compozy network work lookup`, and `compozy network send --body` accepts a kind-specific JSON value rather than requiring an object.

##### Window tabs in the OS shell

The OS shell now groups windows into first-class tab frames instead of assuming one window per app. Tabs carry ordered members, an active member, per-tab navigation stacks, pinning, scoped close and reopen, and bounded history. The same topology is exposed through Web, CLI, HTTP, UDS, native tools, streams, hooks, resources, layout profiles, and the bundled CompozyOS skill, so agents operate windows with the same semantics people see. (#287)

- Run multiple instances of the same app, discover their tabs from the dock and the command palette, and drag to group, reorder, or tear out a tab.
- Move, swap, and zoom by frame instead of by single window, and adjust Window Manager behavior directly from Settings.
- `compozy config set window_manager.*` applies through the canonical Settings section endpoint, so a live apply projects only that section and unrelated restart-required drift stays pending and truthful in `compozy status`.

Migration notes: persisted window layouts move to v3 as a hard cut — v2 layout compatibility paths and singleton-window assumptions were removed from the runtime, generated contracts, and layout profiles.

#### Fixes

##### A crashed agent no longer looks like a finished one

When an agent process disconnected mid-answer, the stream simply ended — and everything downstream read that silence as success. A CLI consumer reached end of file and exited zero, `compozy__session_prompt` returned a result, and the only evidence left behind was stderr with no exit code. Streams are now fail-closed: success requires an explicit completion event, and disconnect, terminal error, and process exit stay three distinct outcomes. (#315, #319)

- Chunks already received stay persisted and visible. CompozyOS never synthesizes a completion for them.
- A stream that ends after partial output without a completion event fails the CLI with a clear non-zero exit, and terminal error frames are forwarded before the error is returned so machine-readable diagnostics survive.
- `compozy__session_prompt` classifies a subprocess exit as `tool_backend_failed` with `backend_dead` instead of reporting success; the partial events remain readable in the session transcript.
- Crash evidence now carries the subprocess exit code and, where the operating system exposes it, the terminating signal.
- Fatal cleanup gives the process a bounded grace period to exit on its own before being stopped, so the real exit result is no longer lost to a race with forced teardown.
- CompozyOS does not replay a prompt automatically, because a prompt may already have caused external side effects. Sending the next prompt restarts the agent process and continues the same session and transcript.

Migration notes: crash bundles move to `compozy.session_crash_bundle.v2` with structured `exit_code` and `signal`, with no v1 branch. Any consumer that treated a closed stream as success will now correctly see a failure unless a completion event was sent.

##### Autonomous extraction stays out of curated memory

The autonomous memory extractor could write operational chatter into curated memory, and a generated slug collision could overwrite an unrelated entry. The deterministic scanner now rejects Memory v2 operational identifiers — `memory_propose`, native `compozy__memory_*` tool names, controller event names, and scanner rule IDs — and extractor, provider, and dreaming candidates no longer update an existing memory solely because their generated slug collides. Explicit filename-collision updates from direct CLI or user writes keep working. (#396)

##### Dry-run proves the run you are about to submit

A Loop could validate, dry-run cleanly, and then fail at submission with `executed definition template manifest changed`. The compiler folded default values into the definition it stored, but compiled templates from the definition _before_ those defaults — so a persisted run carried more template keys than its own snapshot, and hydration rightly refused it. Compilation now uses one canonical definition throughout, and dry-run exercises the exact snapshot boundary a real submission uses. (#313, #317)

- Defaults are folded once at the start of compilation and used for linting, contracts, nodes, watch events, graph metadata, and child Loops alike — so omitted child `mode` values no longer appear out of nowhere during hydration.
- A snapshot must load through the production loader before its bytes or digest are returned; one that cannot round-trip can no longer reach storage.
- `compozy loop run --dry-run` and `compozy__loop_run` with `dry: true` run that same check, so a preview can no longer approve a definition that submission would reject.
- A mismatch now names the manifest kind, the exact key, and its source instead of reporting a generic failure.

Migration notes: no storage, API, CLI, or configuration contract changed. Integrity checks were not relaxed — inconsistent definitions are still rejected, just earlier and with a readable reason.

##### Restart a stopped session and keep its runtime

A stopped session used to be a dead end: the UI went read-only and the only way forward was creating a new one. Sending a normal prompt to a stopped session now restarts its agent process, reloads the retained provider history, and continues under the same session ID and transcript. The provider, model, reasoning effort, and speed you picked are stored on the session itself, so they survive a stop and a daemon restart instead of silently reverting to the default. (#307)

- The lifecycle gained a `starting` state, and a normal prompt is the only operation that moves a stopped session back toward execution. `session resume` stays attach-only, and queue, steer, interrupt, and attach do not restart a session.
- `compozy session runtime set <id>` takes `--provider`, `--model`, `--reasoning-effort`, and `--speed`, and `compozy session runtime clear <id>` drops the choice. Both fence on `--expected-revision` and report a conflict on a stale one. Agents get `compozy__session_runtime_set` and `compozy__session_runtime_clear`; extensions get `sessions/runtime/set` and `sessions/runtime/clear` under `session.write`.
- Session reads expose `runtime.selected`, `runtime.effective`, and `runtime.selection_revision`. A prompt resolves its runtime from an explicit snapshot first, then the stored selection, then the current effective values, and an already-queued prompt keeps the snapshot it was accepted with.
- The composer stays enabled for a stopped session, and closing a session window during a live turn no longer breaks the transcript view.

Migration notes: the `Use as Goal` action on settled assistant messages is removed. `/goal` is the single entry point for Goals.

##### Durable inputs for busy sessions

Queue, Steer, and Interrupt are now daemon-owned durable operations instead of client-side intent that could quietly disappear. An input is persisted before it is acknowledged, survives a refresh and a daemon restart, dispatches exactly once in FIFO order, and can be listed, edited, canceled, or promoted to steering by its entry ID from the CLI, HTTP, UDS, native tools, or the extension host. Disruptive changes are fenced against the turn you meant to change, so a stale client cannot interrupt a newer turn. (#304)

- `compozy session prompt` accepts `--queue`, `--interrupt`, and `--steer`; `compozy session input list|edit|steer|cancel` manages pending input by its persisted ID.
- The queue is readable and mutable over HTTP and UDS at `/api/workspaces/{workspace_id}/sessions/{session_id}/prompt/queue`, including per-entry replace, steer, and cancel.
- Agents get `compozy__session_inputs_list`, `compozy__session_input_replace`, `compozy__session_input_cancel`, and `compozy__session_input_promote`.
- The composer clears only after the daemon acknowledges, a failure keeps your draft, and a refresh reconstructs pending input from the daemon. Queued, steered, interrupted, canceled, accepted, and dropped markers no longer render as warnings, and an expected ACP cancellation no longer appears in the transcript as a provider failure.

Migration notes: the dedicated interrupt endpoint is removed — interrupt is now a prompt mode plus a fenced queue operation. The legacy ACP steer handler and the runtime steer source are removed, and the web client no longer mirrors the queue in local state.

##### Durable session messaging

Session prompts no longer duplicate, reorder, or disappear when an optimistic Web message settles, when a client reconnects, or after a cold reload. Every externally authored prompt now carries two durable identities — `message_id` for the rendered message and `idempotency_key` for the command execution — and both survive Web rendering, HTTP/UDS/CLI/native-tool ingress, queueing or steering, ACP dispatch, transcript projection, replay, and reload. (#288)

- Retrying the same prompt across supported transports is at-most-once when the original identities are reused: an exact retry returns the original result with `replayed: true` without re-running hooks or the provider.
- Divergent reuse of an identity returns a typed conflict, and uncertain post-dispatch recovery is reported as indeterminate instead of silently resending.
- Goal retries preserve the original result and the original HTTP status.
- Provider-originated ACP `user_message_chunk` echoes no longer appear as a second authored message, while locally authored steer events are preserved.
- The CLI, the Extension Host, and `compozy__session_prompt` expose the retry identities.

Migration notes: external prompt and steer inputs now require both `message_id` and `idempotency_key`, and Goal prompt responses use the standard wrapped prompt-result envelope.

##### Live changelog and composer fixes

The changelog on `compozy.com` now reads published releases directly from GitHub at request time instead of depending on a bot pushing a generated page back into `main` after every release. Each release gets its own page with rendered Markdown, category sections, evidence, compare links, and downloadable assets, plus an RSS feed at `/changelog/feed.xml`, and releases now appear in site search, the sitemap, and the text feeds that agents read. (#292)

- Typing in the session composer no longer swallows spaces.
- A window-manager WebSocket upgrade that fails for a missing workspace now returns a proper preflight error frame, and the web client refreshes a stale workspace list when it sees that error instead of staying stuck.

Migration notes: the release workflow no longer publishes a site changelog receipt commit, and the generator scripts behind it are removed.

##### Loop runs keep their lineage, permissions, and results

Three Loop defects that broke supervision of long runs are fixed. (#420, #407, #408)

- Loop-owned Goal sessions retain the trusted provenance of the session that started the current or nearest ancestor Loop Run, so session catalogs and the Web group Goal work below its originating session. The relationship stays informational: Goal sessions remain `type=system` with no inherited TTL, auto-stop, spawn budget, or permission narrowing, provenance is derived server-side within the same workspace, and spawn limits still count only contiguous spawned ancestry. (#420)
- Daemon-owned terminal tool effects are authorized correctly. The native policy path treated the synthetic `loop-effect` audit label as an authored workspace agent and failed the lookup before the declared tool could run. The trusted daemon actor kind is now preserved through policy resolution, the label survives for attribution, and workspace policy still denies foreign targets. (#407)
- Terminal effect results stay visible in run details. The Web hook closed its event stream as soon as it received the terminal status, so a later retained or live effect-results frame never reached the run timeline, and reloading repeated the race. Successful and denied effect results now arrive in order while replacement, deactivation, navigation, and unmount keep the normal cleanup path. (#408)

##### Loop runs you can debug from the run page

A Loop run that failed used to be a dead end: every attempt died with "The agent output did not satisfy the action output schema", the node was quarantined, "Open quarantine entry" opened an empty sheet, "Open session" returned 404, the cell task sat in "Queued · attempt 1 of 10" forever, and Usage confidently reported `0 / ~$0.00`. The agent had actually answered correctly every time — the daemon joined streamed text fragments with a newline, which landed inside a JSON string and corrupted a valid reply. That joiner is fixed, and so is every surface that made the failure impossible to read. (#324)

- The agent now sees the authored `output_schema` in its prompt instead of prose that never said "JSON", extraction validates every candidate object newest-first (a quoted `package.json` no longer shadows the real answer), and the failure cause carries the underlying detail instead of one generic sentence.
- Quarantine is routed to the node that actually failed. Parked consumers collapse into a single row — "**execute task is quarantined** — collect, review, verify and approve are parked behind it until it is requeued" — with one button that opens the producer's entry, and `node_attention_flagged` is finally emitted when a run parks.
- Loop cells no longer stall in `ready` after a failed run: quarantine parks them as needs-attention and requeueing clears the park. The misleading `of 10` attempt ceiling is gone, since the Loop owns the retry budget.
- Daemon-claimed runs stop writing placeholder session ids, the real ACP session is bound to the lease under a claim token, and run detail exposes `generations[].outputs[].session_id` — so "Open session" works from the hero and from every node row that has one.
- The task list nests Loop cells under their coordinator with an escalation-first summary ("9 subtasks · 1 needs attention · 2 running") and readable identities like `g2.execute_task` instead of `loop.lo`.
- When a provider reports no tokens, Usage now reads "not reported" and "—" instead of a confident zero; the cost estimate returns only when tokens exist.

Migration notes: adds the `attention_producer_node_id` column to `loop_node_controls` through migration `00055`; run-detail payloads gain `node_controls[].attention_producer_node_id` and `generations[].outputs[].session_id`.

##### Skills load through the native seam inside managed sessions

Managed sessions load installed skills through the native `compozy__skill_view` tool only — including skills that are not listed in the prompt catalog. The earlier attempt to give managed agents a private CLI socket is removed rather than kept as a fallback: provider code runs as the daemon user, so environment values, headers, process ancestry, and file modes cannot tell those requests apart from an operator's. (#314, #323)

- If session policy denies the native tool, the agent reports the skill unavailable instead of shelling out or reading skill files directly.
- Every `compozy skill` verb detects managed-session markers before doing any client, socket, registry, or filesystem work and points the caller at `compozy__skill_list`, `compozy__skill_search`, and `compozy__skill_view`. This is documented as a support guard, not an authorization boundary — same-user code can still clear those markers.
- Hosted-MCP bind windows now start after ACP initialization and immediately before session negotiation. A cold provider launch that takes longer than the bind window no longer expires the tool seam before the agent can use it; a bind attempted before activation still fails closed.

Migration notes: the managed CLI transport is deleted — the socket, `COMPOZY_AGENT_TRANSPORT_SOCKET`, the managed identity headers, and the managed skill API scope. Operator CLI behavior from a normal shell is unchanged.

##### One owner per Loop run, and cancellation that sticks

Loop action runs now have exactly one daemon-owned worker, cancellation survives a restart, and a session that needs CompozyOS tools fails before the provider starts instead of running without them. Fresh CompozyOS homes also start with the bundled `dev-cycle` extension already enabled, while a home that has been booted before keeps whatever you chose. (#321, #322, #326)

- Coordinators and ordinary task-role sessions can no longer activate or bootstrap a run that the dedicated `loop-action` executor already owns.
- When the effective agent or lineage policy requires concrete tools and hosted MCP cannot provide them, session startup fails closed with `ErrHostedMCPUnavailable` before the provider process is launched.
- Loop cancellation is durable: delivery state is persisted, delivery is idempotent, the run advances to draining once acknowledged, and anything still pending is retried from daemon boot and from scheduler sweeps — no restart required to converge.
- Resuming a stopped session discards the stopped ledger projection first and restores it if provider startup or the clear rolls back, so forensic projections stop conflicting and the full history is rematerialized on the next stop.
- Enablement of a bundled extension is a fresh-home default, not an override: generic local and marketplace installs stay disabled by default, and stored state survives restart and update.

##### Pi providers receive the secret, not its variable name

CompozyOS wrote a Pi credential slot's target environment name, such as `ZAI_API_KEY`, straight into the session `models.json` `apiKey` field. Pi reads a bare uppercase value as a literal API key, so the provider received the variable name instead of the secret, the upstream request failed, and the session could finish without an assistant message. The Pi runtime now writes `$ZAI_API_KEY`-style references, which Pi resolves from the secret CompozyOS already injects into the provider process. (#404)

Migration notes: this covers the built-in `pi_acp` bound-secret providers — z.ai, OpenRouter, Moonshot/Kimi, xAI, MiniMax, Mistral, Groq, and Vercel AI Gateway.

##### Resource-only extensions need no toolchain

An extension that ships only declared resources — agents, skills, Loops, automations, layouts — can now use `build`, `dev`, `reload`, and `dev --watch` without installing a Go or TypeScript toolchain. The passive build path validates and publishes those resources without running build or describe subprocesses, and active development links project them into the linked workspace while preserving deterministic generations, atomic reload, and last-good fallback. The Go and TypeScript paths are unchanged, and the resource-only path fails closed. (#423)

##### The desktop stays responsive with a large session catalog

Long-running sessions with a large internal session catalog put the macOS desktop into a request-and-reload feedback loop that made it unusable. The two-second liveness probe now targets a bounded `GET /api/status/identity` surface over HTTP and UDS instead of the full status aggregate, and internal sessions stop inflating the public catalog. (#414)

- Memory-extractor, auto-title, and dream sessions no longer publish wake events to the public session catalog.
- Built-in background agents, including `dreaming-curator`, resolve through effective workspace configuration instead of being reported as missing workspace-authored agents.
- The identity contract ships in OpenAPI and the generated TypeScript types.

#### Highlights

##### Gateway docs: zero to GitHub webhooks

compozy.com gains a dedicated Gateway section written for first-time operators: a ten-minute
quickstart from `gateway.enabled` to a paired phone, a step-by-step "Receive GitHub webhooks"
tutorial verified end to end — including why a native repository webhook cannot sign CompozyOS's
generic trigger contract and the GitHub Actions workflow that can — a Tailscale extension page
covering tailnet prerequisites through clean removal, a remote CLI/SSH/public-access guide, a
devices-audit-teardown runbook, and a plain-language security page. (#331)

Migration notes: `/docs/operations/remote-gateway`, `/docs/operations/gateway-threat-model`, and
`/docs/configuration/gateway` moved into `/docs/gateway/*` as a hard cut — update saved links.

##### MCP catalog, session runtime, and extension management

CompozyOS beta expands how people and agents configure the runtime across MCP, sessions, extensions, workspace boundaries, and the session UI.

- Install, authorize, repair, inspect, and remove curated MCP servers through the CLI, HTTP/UDS APIs, Web, and the official CompozyOS skill. The catalog now uses manifest version 2, the runtime uses the official MCP SDK, and public MCP transport no longer accepts SSE. (#284)
- Choose the provider, model, reasoning effort, and speed for each session prompt, switch runtime within a session, and create sessions before their first prompt. (#283)
- Create, build, validate, develop, distribute, install, and inspect extensions through the daemon, CLI, APIs, native tools, Web, and SDK contracts. Extension manifests now use version 2. (#278)
- Apply existing session permission modes to explicitly targeted cross-workspace agent access, including session-scoped consent where a native-tool prompt is available. (#275)
- Read session transcripts through a calmer timeline with clearer tool results, failures, permissions, clarifications, and goal controls. (#271)
- Run the daemon on Windows with corrected process locking, SQLite paths, detach behavior, process timestamps, and sync-directory handling. (#274)
- Automation jobs can target Loops with workspace inputs and mappings, unresolved tool calls now fail explicitly, and Loop/session recovery paths report clearer state and errors. (#276, #279)

Migration notes: update MCP catalog manifests to version 2 and replace public SSE transport; create a session before submitting its first prompt and runtime selection; update extension manifests to version 2.

## 0.0.9 - 2026-07-04

### ♻️ Refactoring

- Tasks orchestration (#269)

### 🐛 Bug Fixes

- Network optimizations (#263)
- Native tools (#266)
- Align release-gated network integration tests with #263 behavior

### Release Notes

#### Features

##### Network delivery policies, subscriptions, and thread-to-task promotion

CompozyOS network channels gain durable delivery control. Each channel now carries a fan-out policy — deliver to all members, route to a designated coordinator peer, or match by capability (the default) — managed with `compozy network channels create` and `compozy network channels update`. Peers and tasks can subscribe to channels: inspect subscriptions with `compozy network subscriptions` and manage per-task subscriptions with `compozy task subscribe <task-id>`. Executable thread messages can be promoted into durable workspace tasks straight from the CLI with `compozy network promote`, or by an agent through the `compozy__task_promote_from_thread` native tool, and designated sibling task runs can be fanned out as part of a workflow. Network and task views now surface task links, peer and coordination cost, and delivered size and token metrics, while mentions are normalized (trimmed, empties removed) and size and token limits are enforced as non-negative.

##### Task blocking, recovery, and needs-attention triage

Tasks are now first-class to block, triage, and recover. A task can be explicitly blocked and unblocked with `compozy task block <id>` and `compozy task unblock <id>`, and every block is kept as durable history you can inspect with `compozy task blocks <id>`. A new `needs_attention` status surfaces tasks that stalled and need a human or coordinator to step in — task details now carry the blocked reason and the wake creator so it is clear why a task is waiting and who nudged it. Clear the state with `compozy task recover <id>`, recover a specific stalled run with `compozy task run recover <run-id>`, or let an agent do it through the `compozy__task_recover` native tool and the HTTP/UDS API. Task completion now returns the IDs of any tasks it created, and stronger validation rejects invalid created-task references before they are persisted. Claim tokens are redacted across task outputs and hook payloads, and the new `needs_attention_after` setting controls how long a task may wait before it is flagged for attention.

#### Fixes

##### Clearer native tool errors and availability diagnostics

Native and hosted tool calls now fail legibly. Hosted MCP tool calls return richer, structured JSON error details — the tool ID, an error code, and any denial reasons — instead of opaque failures, and advertised tools carry improved metadata and descriptions. When runtime diagnostic data cannot be retrieved, native tool lookups report a clear "unavailable" diagnostic rather than a silent miss. Hosted and session MCP availability handling is safer overall, with clearer informational and warning logs and graceful fallbacks when a feature is disabled. Documentation and the runtime envelope now consistently direct agents to the canonical `compozy__*` tool IDs and the harness-returned tool references.

## 0.0.8 - 2026-06-22

### 🐛 Bug Fixes

- Acp update and general fixes (#256)
- Make docs markdown copy retryable

### 🧪 Testing

- Remove not needed test

## 0.0.7 - 2026-06-04

### ♻️ Refactoring

- Improvements on create modals (#252)

### 🐛 Bug Fixes

- Repair daemon-served e2e flows

## 0.0.6 - 2026-06-03

### 🎉 Features

- Dependency-driven auto-enqueue (opt-in) (#232)

### 🐛 Bug Fixes

- Wake coordinator sessions reliably (#240)
- Enable runtime evidence profiles (#242)
- React doctor fixes (#245)
- Verify marketplace skill installs (#244)
- Persist active workspace and redirect on session/workspace mismatch (#238)
- Safe workspace delete and compozy session remove command (#239)
- Unblock release CI (bootstrapRun complexity, stale gate tests) (#249)

### Release Notes

#### Features

##### Dependency-driven auto-enqueue

Tasks can now opt into dependency-driven auto-enqueue. When a task is marked `auto_enqueue_on_ready`, CompozyOS enqueues its next task run automatically as soon as a blocking dependency completes and the task reconciles to `ready` — so a dependency graph advances without a manual `compozy task enqueue` at each step. The behavior is conservative: only a successful completion satisfies a `blocks` edge, paused dependents are skipped, and the queued-run reservation keeps at most one open run per dependent under concurrent or retried completions. Enqueue happens only after the completion has durably committed and is best-effort — a failed enqueue is logged, never rolled back onto the completing run. It is off by default; enable it per task with `--auto-enqueue-on-ready` on `compozy task create` / `compozy task child create`, toggle it with `compozy task update`, or set the `auto_enqueue_on_ready` field over HTTP/UDS and the extension SDK.

##### Runtime evidence mode for task execution profiles

Task execution profiles can now drive worker startup and opt into runtime evidence mode. A task with no pool owner but a `worker.mode = "select"` profile now starts the selected agent, provider, and model and propagates the profile's required capabilities into its claim command. Setting `runtime.mode = "evidence"` boots that worker with guidance to run browser, simulator, and local-app validation and to capture runtime evidence. CompozyOS elevates the session to auto-approve permissions only when the profile also pins an explicit sandbox reference (`sandbox.mode = "ref"`); otherwise the configured permission policy stays in force, and evidence mode grants no extra task authority. The profile's new `runtime` block is surfaced through the execution-profile CLI, the HTTP/UDS endpoints, and the native task tools.

#### Fixes

##### Coordinator sessions wake reliably on new work

Coordinator sessions now wake reliably when new work arrives. When a task run is enqueued for a workspace that already has a running coordinator session, CompozyOS delivers a synthetic wake to that session — interrupting the agent's current turn if it is idle and waiting — so queued runs are picked up promptly instead of stalling. The same interrupt-if-waiting delivery now applies to harness re-entry and heartbeat wakes, force-retried and force-recovered runs re-trigger a coordinator wake, and heartbeat wakes skipped for non-transient reasons are recorded as dropped rather than silently lost. Wake delivery is de-duplicated per session and run and drained safely across daemon shutdown.

##### Marketplace skill installs are verified end-to-end

Marketplace skill installs are now verified end-to-end. After downloading and writing a skill, CompozyOS confirms the runtime can actually discover it as an enabled marketplace skill with matching provenance, instead of reporting success for an install that would never resolve. Installs that are disabled, shadowed by a higher-precedence skill of the same name, missing provenance, or resolved to a different slug now fail with a clear, terminal error and remediation guidance across `compozy skill install` and the daemon API. Use `compozy skill where <name>` to inspect the winning source before retrying.

##### Safe workspace deletion, plus compozy session remove and compozy open

Workspace deletion is now safe: CompozyOS refuses to delete a workspace while any of its sessions are still active — returning a 409 that names the blocking sessions — and cleans up the workspace's stopped session history transactionally when deletion proceeds. Two agent-manageable CLI commands ship alongside it: `compozy session remove <id>` deletes a single session and its persisted history, and `compozy open` opens the CompozyOS web UI in your default browser. The CLI reference also gains documentation for `compozy open`, `compozy session remove`, and the existing `compozy onboarding` command group.

##### Web UI remembers your active workspace

The web UI now remembers your active workspace across reloads and browser restarts, so a refresh no longer snaps you back to the first workspace. Direct and shared session links resolve a session's owning workspace from its ID and load reliably regardless of which workspace is selected, and the UI redirects to the agent page when an opened session belongs to a different workspace than the active one.

## 0.0.5 - 2026-05-29

### ♻️ Refactoring

- Optimize prompt consumption (#222)
- Orchestration improvements (#230)

### 🎉 Features

- Decouple worker concurrency from coordinator uniqueness (#229)

### 🐛 Bug Fixes

- Handle provider overlay subtables (#228)

## 0.0.4 - 2026-05-27

### 🐛 Bug Fixes

- Default workspace

## 0.0.3 - 2026-05-27

### ♻️ Refactoring

- Memory optimization (#215)

## 0.0.2 - 2026-05-27

### Other Changes

- Lessons learned

### ♻️ Refactoring

- Project structure (#7)
- Kb improvements (#12)
- Rename spaces to channels (#17)
- Add extensions gaps (#21)
- Improve tool calls ui (#22)
- Remove web app header
- Module improvements (#29)
- Memory improvements (#35)
- Storybook for web and ui (#38)
- Enable CompozyOS network by default for new installs (#57)
- Hermes adjustments (#69)
- Badges design (#84)
- Storybook scenario and logos gallery
- Migrate typescript tests (#114)
- Internal go packages (#120)
- Ui patterns (#127)
- Improve e2e tests (#130)
- Ui redesign
- Workspace isolation across runtime surfaces (#145)
- Prod ready applies (#162)
- Tool card ui (#164)
- Alpha on logo
- Prod ready features (#167)
- Thread sheet (#202)

### 🎉 Features

- Implement config foundation packages
- Implement sqlite store package
- Add ACP client package
- Add session lifecycle manager
- Implement observe package
- Add daemon composition root
- Add uds api server
- Implement cli package
- Add http api server
- Add system design
- Add foundation types, schemas, and layout shell for web client
- Add daemon health polling and agent sidebar systems for web client
- Add session system CRUD, streaming core, and session store for web client
- Add chat view, messages, and composer tests for web client
- Add tool cards and renderers for web client
- Add file-backed memory store core
- Scaffold memory session seams
- Add memory dream consolidation service
- Wire memory assembler into daemon
- Add memory api and cli
- New skills system (#1)
- Add workspace entity (#5)
- Add new skill capabilities (#8)
- Web ui v2 (#9)
- Improve hooks system (#10)
- Session resilience (#11)
- Add extensability (#13)
- Add automation (#16)
- Add channels (#14)
- Add network implementation (#15)
- Add network, bridges and automations web pages (#18)
- Ext registry (#20)
- Add core tasks (#19)
- Bridge adapters (#23)
- Add site (#26)
- Add ext refac and sandbox (#25)
- Settings ui (#37)
- Tasks ui (#36)
- Harness improvements (#44)
- Agent capabilities (#49)
- Redesign ui (#48)
- Unify capability (#53)
- Redesign network workspace (#59)
- Add task deletion and split session delete from stop (#58)
- Session provider selection (#60)
- Production grade adjustments (#66)
- Autonomous system (#75)
- Add agent session route (#80)
- Tools registry (#85)
- Agents soul (#88)
- Add network threads (#105)
- Orchestration improvements (#106)
- Memory v2 (#108)
- Agent categories (#113)
- Providers model (#118)
- Add canonical CompozyOS bundled skill (#143)
- Onboarding and improvements (#198)
- Onboarding and improvements (#201)

### 🐛 Bug Fixes

- Review round
- Review rounds
- Resolve memory extensibility review batch
- Embed web into daemon
- Defaults agents
- Acp integration (#4)
- Lint errors
- Prd folder
- Remove orphan web actions and dead surfaces (#55)
- Qa testing and fixes (#73)
- New review rounds (#82)
- Security audit (#90)
- Release qa round (#95)
- Add missing tools (#141)
- New qa round (#147)
- Advanced qa round (#149)
- Homebrew tap
- Final review round (#151)
- Daemon healthy
- Reasoning models (#158)
- Lint errors (#160)
- Review round (#168)
- Release adjustments (#171)
- Stabilize release ci fixtures
- Stabilize release integration gate
- Stabilize release verify gates
- Stabilize release integration flows
- Stabilize release verify gates
- Stabilize main verify shutdown
- Ignore stale acpmock cancel
- Marketplace search focus and filtering (#193)
- Website video
- Workspace command select

### 🧪 Testing

- Add e2e tests (#27)
- Qa rounds (#78)
- Improve test suite (#138)
- Harden daemon-served restart reloads
- Harden daemon-served readiness waits
- Stabilize dashboard focus assertion
- Stabilize release integration gates
- Stabilize release e2e markers
- Stabilize release e2e flows
- Improve suite speed

## 0.0.1 - 2026-05-26

### Other Changes

- Lessons learned

### ♻️ Refactoring

- Project structure (#7)
- Kb improvements (#12)
- Rename spaces to channels (#17)
- Add extensions gaps (#21)
- Improve tool calls ui (#22)
- Remove web app header
- Module improvements (#29)
- Memory improvements (#35)
- Storybook for web and ui (#38)
- Enable CompozyOS network by default for new installs (#57)
- Hermes adjustments (#69)
- Badges design (#84)
- Storybook scenario and logos gallery
- Migrate typescript tests (#114)
- Internal go packages (#120)
- Ui patterns (#127)
- Improve e2e tests (#130)
- Ui redesign
- Workspace isolation across runtime surfaces (#145)
- Prod ready applies (#162)
- Tool card ui (#164)
- Alpha on logo
- Prod ready features (#167)
- Thread sheet (#202)

### 🎉 Features

- Implement config foundation packages
- Implement sqlite store package
- Add ACP client package
- Add session lifecycle manager
- Implement observe package
- Add daemon composition root
- Add uds api server
- Implement cli package
- Add http api server
- Add system design
- Add foundation types, schemas, and layout shell for web client
- Add daemon health polling and agent sidebar systems for web client
- Add session system CRUD, streaming core, and session store for web client
- Add chat view, messages, and composer tests for web client
- Add tool cards and renderers for web client
- Add file-backed memory store core
- Scaffold memory session seams
- Add memory dream consolidation service
- Wire memory assembler into daemon
- Add memory api and cli
- New skills system (#1)
- Add workspace entity (#5)
- Add new skill capabilities (#8)
- Web ui v2 (#9)
- Improve hooks system (#10)
- Session resilience (#11)
- Add extensability (#13)
- Add automation (#16)
- Add channels (#14)
- Add network implementation (#15)
- Add network, bridges and automations web pages (#18)
- Ext registry (#20)
- Add core tasks (#19)
- Bridge adapters (#23)
- Add site (#26)
- Add ext refac and sandbox (#25)
- Settings ui (#37)
- Tasks ui (#36)
- Harness improvements (#44)
- Agent capabilities (#49)
- Redesign ui (#48)
- Unify capability (#53)
- Redesign network workspace (#59)
- Add task deletion and split session delete from stop (#58)
- Session provider selection (#60)
- Production grade adjustments (#66)
- Autonomous system (#75)
- Add agent session route (#80)
- Tools registry (#85)
- Agents soul (#88)
- Add network threads (#105)
- Orchestration improvements (#106)
- Memory v2 (#108)
- Agent categories (#113)
- Providers model (#118)
- Add canonical CompozyOS bundled skill (#143)
- Onboarding and improvements (#198)
- Onboarding and improvements (#201)

### 🐛 Bug Fixes

- Review round
- Review rounds
- Resolve memory extensibility review batch
- Embed web into daemon
- Defaults agents
- Acp integration (#4)
- Lint errors
- Prd folder
- Remove orphan web actions and dead surfaces (#55)
- Qa testing and fixes (#73)
- New review rounds (#82)
- Security audit (#90)
- Release qa round (#95)
- Add missing tools (#141)
- New qa round (#147)
- Advanced qa round (#149)
- Homebrew tap
- Final review round (#151)
- Daemon healthy
- Reasoning models (#158)
- Lint errors (#160)
- Review round (#168)
- Release adjustments (#171)
- Stabilize release ci fixtures
- Stabilize release integration gate
- Stabilize release verify gates
- Stabilize release integration flows
- Stabilize release verify gates
- Stabilize main verify shutdown
- Ignore stale acpmock cancel
- Marketplace search focus and filtering (#193)
- Website video
- Workspace command select

### 🧪 Testing

- Add e2e tests (#27)
- Qa rounds (#78)
- Improve test suite (#138)
- Harden daemon-served restart reloads
- Harden daemon-served readiness waits
- Stabilize dashboard focus assertion
- Stabilize release integration gates
- Stabilize release e2e markers
- Stabilize release e2e flows
