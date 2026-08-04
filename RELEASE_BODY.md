## 0.3.0 - 2026-08-04

### ♻️ Refactoring

- State management with xstate-store (#268)
- Site improvements (#277)
- Replace bundles with extension kits (#291)

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

### Release Notes

#### Breaking Changes

##### The OS Release

Compozy v0.3 is a new operating system boundary for agent work. Sessions, tasks, loops, memory,
permissions, automation, the OS shell, and Compozy Network now share one daemon-owned state model.
People can start and inspect that work from the web, CLI, HTTP/SSE, or UDS. Agents can operate the
same runtime through structured tools and extension contracts.

This is a breaking beta. The command, package, environment, storage, API, and tool namespaces move
to Compozy, and several v0.2 surfaces have deliberate replacements or removals. Follow the
[v0.3 migration guide](https://compozy.com/runtime/migration/) before replacing an existing install.
The maintained v0.2 line and its collateral remain on `legacy/v0.2`.

Install the beta through the verified hosted installer, `@compozy/cli@beta`, or the explicit
`github.com/compozy/compozy@v0.3.0-beta.1` Go version. The beta channel may change before v0.3.0
stable; production rollouts should pin the version and review each prerelease.

The repository was already MIT licensed. v0.3 corrects stale BSL-1.1 text in distribution metadata;
it does not relicense the code.

#### Features

##### Grouped skill directories

Compozy now discovers `SKILL.md` definitions at any depth below each skill root, so teams can organize capabilities under folders such as `marketing/content/` without changing frontmatter identity or normal precedence. `compozy skill create <name> --group <relative/path>` now scaffolds grouped workspace skills safely.

##### Unified docs and Marketplace

`compozy.com` now serves a single `/docs` experience with reworked navigation, breadcrumbs, responsive layouts, generated CLI reference pages, and API references that include Go examples. A new `/marketplace` section lists skills, extensions, MCP entries, bridge providers, and bundled capabilities with search, install commands, and detail pages. (#277)

Migration notes: two CLI verbs were renamed and their old spellings removed — `compozy mcp authorize <server>` is now `compozy mcp auth login <server>`, and `compozy memory extractor list-pending` is now `compozy memory extractor list-failures`. The `compozy network work status` alias was removed in favor of `compozy network work lookup`, and `compozy network send --body` accepts a kind-specific JSON value rather than requiring an object.

##### Window tabs in the OS shell

The OS shell now groups windows into first-class tab frames instead of assuming one window per app. Tabs carry ordered members, an active member, per-tab navigation stacks, pinning, scoped close and reopen, and bounded history. The same topology is exposed through Web, CLI, HTTP, UDS, native tools, streams, hooks, resources, layout profiles, and the bundled Compozy skill, so agents operate windows with the same semantics people see. (#287)

- Run multiple instances of the same app, discover their tabs from the dock and the command palette, and drag to group, reorder, or tear out a tab.
- Move, swap, and zoom by frame instead of by single window, and adjust Window Manager behavior directly from Settings.
- `compozy config set window_manager.*` applies through the canonical Settings section endpoint, so a live apply projects only that section and unrelated restart-required drift stays pending and truthful in `compozy status`.

Migration notes: persisted window layouts move to v3 as a hard cut — v2 layout compatibility paths and singleton-window assumptions were removed from the runtime, generated contracts, and layout profiles.

#### Fixes

##### Durable session messaging

Session prompts no longer duplicate, reorder, or disappear when an optimistic Web message settles, when a client reconnects, or after a cold reload. Every externally authored prompt now carries two durable identities — `message_id` for the rendered message and `idempotency_key` for the command execution — and both survive Web rendering, HTTP/UDS/CLI/native-tool ingress, queueing or steering, ACP dispatch, transcript projection, replay, and reload. (#288)

- Retrying the same prompt across supported transports is at-most-once when the original identities are reused: an exact retry returns the original result with `replayed: true` without re-running hooks or the provider.
- Divergent reuse of an identity returns a typed conflict, and uncertain post-dispatch recovery is reported as indeterminate instead of silently resending.
- Goal retries preserve the original result and the original HTTP status.
- Provider-originated ACP `user_message_chunk` echoes no longer appear as a second authored message, while locally authored steer events are preserved.
- The CLI, the Extension Host, and `compozy__session_prompt` expose the retry identities.

Migration notes: external prompt and steer inputs now require both `message_id` and `idempotency_key`, and Goal prompt responses use the standard wrapped prompt-result envelope.

#### Highlights

##### MCP catalog, session runtime, and extension management

CompozyOS beta expands how people and agents configure the runtime across MCP, sessions, extensions, workspace boundaries, and the session UI.

- Install, authorize, repair, inspect, and remove curated MCP servers through the CLI, HTTP/UDS APIs, Web, and the official Compozy skill. The catalog now uses manifest version 2, the runtime uses the official MCP SDK, and public MCP transport no longer accepts SSE. (#284)
- Choose the provider, model, reasoning effort, and speed for each session prompt, switch runtime within a session, and create sessions before their first prompt. (#283)
- Create, build, validate, develop, distribute, install, and inspect extensions through the daemon, CLI, APIs, native tools, Web, and SDK contracts. Extension manifests now use version 2. (#278)
- Apply existing session permission modes to explicitly targeted cross-workspace agent access, including session-scoped consent where a native-tool prompt is available. (#275)
- Read session transcripts through a calmer timeline with clearer tool results, failures, permissions, clarifications, and goal controls. (#271)
- Run the daemon on Windows with corrected process locking, SQLite paths, detach behavior, process timestamps, and sync-directory handling. (#274)
- Automation jobs can target Loops with workspace inputs and mappings, unresolved tool calls now fail explicitly, and Loop/session recovery paths report clearer state and errors. (#276, #279)

Migration notes: update MCP catalog manifests to version 2 and replace public SSE transport; create a session before submitting its first prompt and runtime selection; update extension manifests to version 2.
