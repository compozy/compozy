# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 0.4.1 - 2026-10-25

### Bug Fixes

- Strip AppImage runtime state from child environments (#954)
- Align catalog artifacts and expose version incompatibility (#957)
- Recover Claude usage from renewed native credentials (#959)
- Load and cancel session monitors outside the default profile (#961)
- Recover half-removed worktree checkouts instead of failing with 500 (#964)
- Prevent contention in GlobalDB integration upgrades (#965)
- Keep authored line breaks and full composer links in user messages (#968)
- Preserve worktree locks and finish interrupted removal cleanup
- Preserve authored inline code spacing
- Honor app reduced motion in content transitions
- Preserve skeleton delay under reduced motion
- Synchronize ACP connection startup and align tray coverage
- Keep the settings loading rail out of the live navigation

### Features

- Add Kanban card actions, board group/sort, and per-window session order (#963)
- Add content-shaped loading states and React view transitions (#972)
- Finish loading states, window fallbacks and transition fixes for every route (#973)

### Performance Improvements

- Suspend closed palette topology updates (#960)
- Avoid palette context updates for window geometry (#971)
- Defer loading skeletons until restored frames commit

### Testing

- Align catalog fixtures and runtime readiness with current contracts
- Synchronize clarify keepalive log capture
- Retry transient desktop fixture removal failures
- Seed Kanban data before opening the browser page

## 0.4.0 - 2026-10-24

### Bug Fixes

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
- Daemon path resolution
- Website font style
- Harden Loop runtime and graph execution (#438)
- Use candidate version in release dry-run
- Coalesce redundant ACP tool updates (#442)
- Resolve agent runtime recovery regressions (#447)
- Preserve run-agent session lifecycle (#446)
- Use migration timeout for tail replay
- Recover ACP sessions after provider disconnects (#454)
- Close profiles regressions after PR 457 (#459)
- Make sessions dock contextual (#468)
- Classify settled spawned TTL cleanup correctly (#469)G
- Restore reliable session message copy (#460)
- Add runtime provider tooltips (#464)
- Close windows when sessions are deleted (#465)
- Route command palette entities directly (#467)
- Preserve worktree binding during reconciliation (#456)
- Harden profile-scoped runtime and web flows (#481)
- Keep inactive loop routes out of task projection (#483)
- Harden Loop recovery and expose execution truth (#492)
- Publish npm packages with trusted identity (#495)
- Accept setup-node OIDC placeholder
- Preserve web assets in release module
- Remove npm token from release recovery
- Allow extension loops to call owned tools (#503)
- Honor selected orchestrated implementer (#502)
- Externalize oversized loop action results (#510)
- Force loop cancellation (#509)
- Enable prompts for managed sessions (#517)
- Preserve sparse fan-out roster rows (#518)
- Scope Loop extension tools to worktrees (#519)
- Preserve profile-scoped extension Agent skills (#516)
- Prevent session cancel and clear races (#523)
- Restore beta 22 release notes and changelog
- Package site search catalogs
- Zoom windows without covering others and keep the layout stream alive (#525)
- Honor route skips within a planning pass (#529)
- Scope lifecycle loop actions (#531)
- Preserve profile scope in loop implementers (#527)
- Stabilize base CI integration (#536)
- Fence terminal generation outputs and stabilize e2e contracts (#547)
- Preserve ACP options in loop-managed session profiles (#542)
- Preserve unresolved review findings (#543)
- Mark skipped terminal loop nodes as not taken (#544)
- Sanitize runtime settings when changing providers (#546)
- Ship the AppImage with the static runtime (#548)
- Preserve provider failures before output validation (#545)
- Terminal operating system (#552)
- Stabilize loop recovery and simplify run inspection (#554)
- Keep pending activity visible and repair CI synchronization
- Prevent stale terminal catalog reads from replacing live state
- Honor profile in operator tool catalog (#551)
- Tolerate unmatched terms in memory recall (#569)
- Scope startup tool guidance to session roles (#572)
- Preserve migrated state during update recovery (#574)
- Unify spec-cycle task completion semantics (#575)
- Preserve provider command routing for spawned sessions (#576)
- Preserve prior session profiles and bound metadata warnings (#578)
- Preserve native model reasoning capabilities (#579)
- Make memory opt-in and preserve pressure compaction (#580)
- Resolve post-merge shutdown and verification failures
- Keep loop authoring editable during validation
- Preserve profile scope across loop run views (#583)
- Preserve profile ownership in loop and memory runtimes (#582)
- Reserve shutdown time for required cleanup (#581)
- Retain the Global desktop across workspace catalog changes
- Repair release integration contracts and lifecycle cleanup
- Stabilize release lifecycle races and integration fixtures
- Respect startup and shutdown lifecycle contexts
- Preserve interrupted session identity and bridge progress bursts
- Preserve cancellation delivery and pending activity deadlines
- Load knowledge memories without enabling automation (#590)
- Allow resizing maximized internal windows (#589)
- Delegate bounded native tools to orchestrated workers (#591)
- Emit profile scope from memory extraction (#592)
- Recover layout settings saves and expose apply results (#596)
- Confirm terminal termination when closing windows (#597)
- Reduce excessive CPU in connected sessions (#599)
- Bound session summaries and preserve complete details (#600)
- Reconcile Goal context and session lifecycle (#601)
- Preserve terminal creation scope and accessible title actions
- Preserve exact window titles and wait for reasoning controls
- Deduplicate delivered skill context and gate input-only roles (#607)
- Verify private Tailscale endpoints through tsnet (#608)
- Report effective dream role in memory health (#609)
- Group reasoning and tools in session work segments (#610)
- Add durable notification acknowledgement (#611)
- Complete notification review and main integration repairs
- Keep unread title counts current in background tabs
- Resolve notification integration Go lint findings
- Preserve escalation acknowledgement across neutral events
- Restore finished notifications and timeline disclosure behavior
- Synchronize attention delivery and durable loop state
- Recover stopped session deletion and queue polling (#614)
- Hide internal Goal prompts from the operator queue (#618)
- Recover owned task work after supervised silence stops (#621)
- Repair agent runtime and Goal workflows found in QA (#624)
- Recognize structured Claude authentication status (#632)
- Discover and bind models in provider overlays (#633)
- Preserve zsh startup directory semantics (#630)
- Preserve nested raw terminal input visibility (#631)
- Preserve unicode terminal prompt redraws (#634)
- Complete overlay admission and integration checks
- Render cursor edits in terminal quote lines
- Resolve terminal cwd from the bound session worktree (#641)
- Repair release integration and catalog compatibility
- Stabilize release integration under concurrent startup
- Goal delete session
- Reopen private Gateway files safely on Windows (#646)
- Report checkpoint role availability from compaction configuration (#645)
- Recover marketplace sources and render catalog logos under CSP (#648)
- Repair marketplace CI fixtures and evidence capture
- Cancel unbound goals before deleting sessions (#647)
- Preserve profile scope when deleting sessions (#652)
- Discover Claude reasoning options for each exact model (#658)
- Bootstrap pinned Bun with npm for Vercel builds
- Honor workspace default in session launcher (#668)
- Fixing clarify tool call timeout with keep-alive pings (#661)
- Preserve declared awaited run-loop terminal output (#672)
- Preserve whitespace in session transcripts (#670)
- Wake creator on child turn completion (#685)
- Resolve reported runtime, catalog and delivery failures (#686)
- Restore release nightly coverage and package extension runtimes
- Preserve loop results under SQLite contention (#690)
- Repair profile isolation, runtime recovery, and loop results (#691)
- Skills nested
- Upgrade dependencies and repair CI compatibility (#692)
- Repair release validation and desktop restoration (#694)
- Preserve profile recovery guidance and repair browser validation (#695)
- Preserve SQLite commit outcomes during cancellation (#696)
- Preserve profile entry across query reconciliation (#697)
- Apply the chosen runtime to continued sessions and keep continue/fork top-level (#702)
- Keep live model catalogs current without releases (#701)
- Classify teardown survivors by final process liveness (#713)
- Fold transcript entries incrementally instead of replaying each turn (#715)
- Resolve Windows runtime failures and desktop context menus (#725)
- Reduce session startup and attention overhead (#731)
- Bound scheduled claim attempts before contention backoff
- Allocate run-agent binding epochs for reviewed actions (#735)
- Keep session image previews dismissible (#744)
- Preserve goal origin identity across provider changes (#749)
- Preserve HTTPS origins through the Tailscale gateway (#743)
- Make bundled spec-cycle workflows self-contained (#741)
- Preserve daemon-owned Loop action claims (#748)
- Keep session lists stable with created and name sorts (#752)
- Attribute model catalog refresh failures to providers (#745)
- Drain queued inputs before Goal continuation (#755)
- Render session message cards for Claude and Codex sends (#757)
- Hide daemon-owned verification sessions (#759)
- Keep runtime models clickable during provider failures (#768)
- Tolerate transient session stream reconnects (#769)
- Stop false Goal attention and cut daemon idle cost (#770)
- Keep background subagents running and show them in the composer (#771)
- Keep terminals running across workspace switches (#776)
- Cut idle UI polling and attention bell CPU cost (#779)
- Preserve profile ownership in session actions and subagent rosters (#785)
- Stop warning about and stopping idle sessions (#787)
- Make the idle-session setting drive idle runtime release (#789)
- Deliver session messages to busy targets with operator-first queue, batching, hop model and explicit replies (#780)
- Address merge-train leftovers from #787 #789 #780 #786 (#793)
- Retry pending steer settlement without locking status
- Advance role fallback when the provider refuses an accepted turn (#662)
- Inherit the caller profile for agent-created sessions and keep one global profile selection (#796)
- Align catalog stream test with global profile selection
- Resolve merged PR review and CI regressions
- Align codegen boundaries and runtime test contracts
- Repair release validation and session shutdown races
- Prevent WAL maintenance from blocking shutdown history
- Group completed tool calls during live Goal turns (#807)
- Auto-enqueue dependents after direct run completion (#806)
- Keep transcripts loadable when a tool call crosses a Goal turn (#803)
- Discover releases promptly and expose manual update checks (#808)
- Allow steering active sessions with attachments (#809)
- Reject blank queued-input promotion as a client error
- Retire session windows after list deletion
- Keep first prompts with skills admitted across registry reloads and never drop a refused draft
- Keep hosted tool results schema-valid when they exceed the result budget
- Retain desktop shutdown teardown diagnostics
- Move runtime selector model list issues behind a header warning mark
- Standardize composer toolbar controls and drop the steer/queue badges
- Recover WAL owner checks and storage-failed session stops
- Allow safe cleanup of worktrees with gone upstreams
- Stabilize transcript file summaries during subagent work
- Retain session runtime metadata and aggregate profile folders
- Close the product renderer after shell cleanup
- Scope the session sidebar state to each session window (#844)
- Fit both bundled tool manuals in the skills-disabled startup budget (#849)
- Resolve review findings left open on #842, #844, and #845 (#848)
- Keep the routing coordinator within the 500-line source limit (#850)
- Render Mermaid, enable desktop notifications, and declare session env (#855)
- Stabilize live transcripts and model-aware delegation (#856)
- Avoid interpreting request identities as provider failures (#857)
- Recover releases from Intel Apple networking failures
- Confirm bulk worktree removal and recognize landed patches (#863)
- Bring runtime and attachments to the empty desktop composer (#865)
- Report actionable errors for invalid workspace config (#866)
- Consume first-message handoffs when attachments are removed
- Preserve session message formatting and editing (#874)
- Stop stale session and terminal retry loops (#875)
- Replace newer desktop runtimes that reject the app (#878)
- Keep terminal journal databases outside project directories (#877)
- Scope session monitor routes to the profile and never drop monitor refreshes
- Re-read open monitor lists when the session catalog stream reconnects
- Keep stale provider usage resolutions from restoring disabled targets
- Prevent session deletion from deadlocking the daemon (#880)
- Allow session derivation after root deletion (#884)
- Stage desktop updates before installer shutdown (#886)
- Default new automations to the host time zone with a searchable picker (#889)
- Render supervised terminal calls as compact tool rows (#893)
- Keep skill_view source a string and expose command_source for command_id reads (#892)
- Address terminal and session release review findings
- Let session transcript messages use the full pane width on large screens
- Resolve session agent catalogs in their bound profile
- Keep an assistant reply as one prose block when subagent work streams between its deltas
- Collapse provider usage rows, overlay scrollbars, name-only workspace identity, accent All projects globe
- Keep session list group collapse across session selection
- Resolve session organization CI lint and formatting failures
- Preserve session lineage and profile boundaries during reparenting
- Preserve profile folds and honor reduced motion
- Enable accessible nesting across session list views
- Own group folder interaction state in a store
- Preserve missing-root recovery during lineage refresh
- Preserve standalone agent slash commands
- Keep a long transcript at its end while it settles on open
- Require live bindings for curated model selection
- Authenticate Windows hosted MCP peers
- Preserve model negotiation errors after discovery failure
- Keep transcript following through held-pointer layout clamps
- Reconcile provider Fast state and accept ultra reasoning
- Repair delegation and runtime recovery from dogfood evidence (#915)
- Preserve steered transcript continuity (#926)
- Preserve message directives and format links (#925)
- Retain unsent composer attachments across sessions (#928)
- Complete session branch and attachment lifecycle behavior
- Validate bridge desktop smoke against its update feed
- Refresh Claude usage after account ownership changes (#931)
- Revalidate configuration before reusing workspace snapshots
- Format workspace snapshot configuration checks
- Align stable release dry-run verification
- Resume interrupted sessions after daemon restart (#942)
- Isolate window surfaces from geometry updates (#943)
- Recover stops after runtime identity release (#948)
- Reveal native subagent work from session lists (#949)
- Keep async subagent navigation outside compiler scope
- Preserve recovered stop reasons and native reveal state
- Preserve current branch and destination in session board actions
- Reveal settled native subagents by card identity
- Wrap current branch queries within source line limits
- Stop waiting on completed subagent delegations
- Collect delivery facts independently of PR display cache

### Features

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
- Add Batuta to marketplace (#432)
- Complete Loop graph engineering and typed inputs (#427)
- Simplify the interface for everyday users (#440)
- Deliver the command palette operating surface (#441)
- Make loop runs legible and task lists calm (#452)
- Complete production demo seed (#453)
- Profiles — the who-is-working dimension (#457)
- Combine settings update actions (#461)
- Replace environment path with worktree icon (#463)
- Group session tool calls (#466)
- Restore agent-manageable Goal orchestration (#470)
- Support conjunctive runtime routing (#475)
- Expand profile identity customization (#484)
- Absorb ecosystem skill folders and expose skills back (#488)
- Merge spec-cycle task delivery loops (#491)
- Rebuild ACP runtime catalogs (#498)
- Support child Loop config overrides (#494)
- Integrated terminal — runtime-owned shells for people and agents (#490)
- Live steer and truthful stop for sessions (#555)
- Manageable session queue, truthful live view, and legible transcripts (#557)
- Add herdr bridge to community catalog (#560)
- Bulk actions for the sessions list (#619)
- Add built-in open-design extension (#625)
- Show session context and confirmed prompt deliveries (#635)
- Unify marketplace catalog and plugin sources (#636)
- Add release video workspace and production skill
- Give rendered Markdown a readable visual hierarchy (#660)
- Add safe bulk worktree cleanup to workspace lists (#656)
- Add provider-neutral ACP full-access preference (#674)
- Calmer, plain-language web UI for everyday users (#683)
- Continue and fork sessions across agents, with fallback account routes (#684)
- Shell rail v2 with light and dark themes (#687)
- Show nested child runs and a distinct awaiting-child state (#708)
- Merge Jobs and Triggers into one Automations area (#709)
- Remove the memory feature family and adopt agent-native session compaction (#710)
- Delegate tasks to subagents with live cards, durable wakes, and operator surfaces (#711)
- Render session tool calls as verb + object rows on a work rail (#716)
- Attribute agent session messages, wake senders with replies, and isolate subagents in worktrees (#722)
- One-click CompozyOS update from the menubar (#733)
- Show the runtime provider mark on session rows (#758)
- Add session folders, manual drag order, and session context menus (#786)
- Give session titles their own line in the session list (#790)
- Adding antigravity acp (#773)
- Add app SDKs and browser application tokens (#794)
- Pick the default provider, model and reasoning from Settings → General
- Clean up missing worktrees in bulk and opt into local branch deletion
- Open new sessions in the existing Session window
- Add @ mentions for files, folders, and sessions in the session composer (#842)
- Make the session window's sessions rail resizable (#845)
- Add opencode v2 support (#811)
- Let open-design prototype with the project's component library (#867)
- Wake agent sessions natively when background work finishes (#860)
- Mark workspaces with working sessions in the switchers (#869)
- Remove discovered worktrees without adopting them (#870)
- Show and cancel session monitors from the session list and composer (#876)
- Add provider usage tray, usage app and daemon collectors (#871)
- Move selected sessions into folders by drag or menu (#881)
- Bind spawned workers to worktrees and inherit parent permissions (#885)
- Work across all projects and in a User scope without a project (#887)
- Bring session list organization to All projects and All profiles and allow nesting sessions
- Archive a running session in one action
- Show total active session work time (#924)
- Unify child sessions and subagent indicators (#927)
- Prepare the first stable v0.4.0 release (#918)
- Add managed worktree teardown commands (#937)
- Link session pull requests, add waiting badge and sessions Kanban (#944)

### Miscellaneous Tasks

- _(mise)_ Pin Go 1.26.6 (#433)

### Performance Improvements

- Delegate full gates to pull request CI (#476)
- Reduce test and development feedback overhead (#556)
- Avoid rendering unchanged windows during layout updates
- Reduce SQLite contention and query costs (#723)
- Reduce desktop startup and live transcript overhead (#784)
- Cut daemon idle cost and keep the host responsive under agent load (#797)
- Reduce concurrent session query and projection overhead (#795)
- Share hosted MCP projections across equivalent peers (#938)
- Make session inventory reads cheap and side-effect free (#940)
- Reduce idle skill watcher and workspace resolver scans (#936)

### Refactoring

- State management with xstate-store (#268)
- Site improvements (#277)
- Replace bundles with extension kits (#291)
- Modernize Go runtime packages (#293)
- Use geist instead of inter (#334)
- Add global workspace toggle (#368)
- Unify PRD and TechSpec into a single spec pipeline (#397)
- Quiet the Context rail and sessions row for everyday use (#649)
- Retire Network, managed Sandbox, and Bridges (#681)
- Complete Modern Go adoption across module boundaries (#706)
- Remove 12 long-tail builtin providers behind a one-release compatibility shim (#778)
- Group session catalog exports with board surface

### Testing

- Guard ACP initialize protocol version (#318)
- Align nightly runtime fixtures
- Align dead session recovery coverage
- Preserve loop claim tokens in daemon fixtures (#418)
- Fix cases failing
- Fix failing tests
- Stabilize Windows PTY read readiness
- Drain Windows PTY startup output
- Enable memory for the knowledge browser journey
- Keep noncooperative ACP prompts alive after cancellation
- Select the desktop workspace before awaiting its client
- Wait for connected idle Skills prompt readiness
- Supply valid origins in acknowledgement fixtures
- Label unscoped attention story notifications
- Retain explicit global scope in attention fixtures
- Create escalation fixtures through their lifecycle
- Project escalation events in acknowledgement fixtures
- Persist canonical escalation status in overview fixtures
- Await completed automation run before daemon cleanup
- Wait for terminal command output before detaching
- Refresh credential assertions context after migration
- Cover inactive loop routes with automatic approval (#703)
- Cut duplicated and misplaced tests across the pyramid (#707)
- Wait for subagent card navigation readiness (#753)
- Control popover exit animation time
- Synchronize migration and startup verification
- Apply formatter to blank promotion regression
- Assert bounded caller reads in native policy regression
- Assert specific workspacedb validation errors
- Assert desktop runtime retry before and after replacement
- Align UDS integration runtime with user scope
- Align HTTP integration runtime with user scope
- Open session creation through the catalog menu
- Align restart fixtures with explicit restart admission
- Include pull request columns in CLI round-trip contract
- Attach successor approval to the named wake turn
- Separate migration verification from upgrade budget

## 0.2.15 - 2026-07-17

### Bug Fixes

- Recover stalled and wedged multi-runs (#230)
- Share parallel task status enum (#241)
- Surface progress and bound the reviews-fix daemon start (#236)
- Package cy-qa-workflow as a module and make host.tasks.create v2-aware (#234)
- Correct Kiro CLI ACP model handling (#226)
- Isolate sync tests and clarify ignore checks (#248)
- Isolate task artifacts and add complexity runtime defaults (#250)

### Features

- Cy-capture-decisions — skill-only extension for durable decision capture (#237)

## 0.2.14 - 2026-07-15

### Bug Fixes

- Acp integratoin

## 0.2.13 - 2026-07-10

### Bug Fixes

- Codex acp

## 0.2.12 - 2026-07-10

### Bug Fixes

- Parallel tasks (#231)

## 0.2.11 - 2026-07-03

### Bug Fixes

- Parallel execution (#217)
- Specifying the model on ACP (#215)
- Worktree management (#223)
- Restore run TUI elapsed timer across retry, failure, cancel, and remote paths (#221)

### Features

- Agentic runs (#212)
- Simplify repo-level default setup overrides (#90)
- Support COMPOZY_HOME env override for home directory (#216)

## 0.2.10 - 2026-06-18

### Bug Fixes

- Reviews watch bug

### Features

- Worktree-backed parallel multi-run for tasks run --multiple (#200)
- Add Devin CLI agent support (#204)

### Refactoring

- Tui redesign (#201)

## 0.2.9 - 2026-06-14

### Bug Fixes

- Record ACP token usage and adapt to acp-go-sdk v0.13.5 (#198)

## 0.2.8 - 2026-06-12

### Bug Fixes

- Keep multi-run task timers ticking (#179)
- Treat model auto as runtime default (#181)
- Pin claude model via ANTHROPIC_MODEL instead of unsupported session/set_model (#187)
- Restart stale daemon when CLI and daemon versions mismatch (#191)
- Surface ACP session setup errors in job logs and fail runs fast (#192)
- Reviews watch (#196)

### Features

- Warn when tasks run starts beside active runs in other workspaces (#190)

## 0.2.6 - 2026-05-27

### Bug Fixes

- Add Windows daemon support (#163)

### Features

- Add multi-task run support (#162)

## 0.2.5 - 2026-05-25

### Bug Fixes

- Homebrew formula
- Emit one task slug per compozy completion candidate (#159)
- Run managed upgrade commands (#158)

### Features

- Add zsh task completion plugin docs and script (#149)
- Add kiro-cli as supported ACP execution runtime (#160)
- Discover task files recursively in nested subdirectories (#153)

### Testing

- Internal test fix

## 0.2.4 - 2026-05-14

### Bug Fixes

- Codex acp integration (#151)

## 0.2.3 - 2026-05-09

### Bug Fixes

- Cwd path

## 0.2.2 - 2026-05-09

### Bug Fixes

- Workspace register (#140)
- Workspace discover path
- Prevent false task completion via prompt kickoff + worktree diff-check (#144) (#145)

### Features

- Add qa extension (#138)

## 0.2.1 - 2026-05-01

### Bug Fixes

- Binary release

## 0.2.0 - 2026-05-01

### Bug Fixes

- Daemons adjustments (#116)
- Harden runtime activity and version handling (#127)
- Release adjustments (#131)
- Infer task type during migrate (#129)
- Watch adjustments
- Lint errors

### Features

- Add optional sound notifications on run lifecycle events (#96)
- Global config defaults (#106)
- Add per task prop selection (#109)
- Migrate to daemon (#112)
- **BREAKING:** migrate to daemon (#112)
- Daemon web UI (#122)
- Web ui polish (#125)
- Review watch (#133)

### Refactoring

- Daemon improvs (#121)

### Testing

- Release config

## 0.1.12 - 2026-04-14

### Bug Fixes

- Execution order
- Fetch reviews parsing

### Features

- Add shared layout package for run artifact filenames (#95)

### Testing

- Fix suite

## 0.1.11 - 2026-04-14

### Features

- Agents spec (#78)
- Add extensability (#80)
- Add compozy skill
- Extension improvements (#83)
- Migrate core extension (#93)

## 0.1.10 - 2026-04-10

### Bug Fixes

- Stop rewriting all _meta.md files when listing workflows (#73)

### Features

- Kernel refactoring (#68)

### Refactoring

- Improve packages (#70)
- Add nitpicks for coderabbit (#75)

## 0.1.9 - 2026-04-06

### Bug Fixes

- Close issue #61 (#63)
- Fail for unsupported --add-dir (#66)

### Features

- Exec command (#60)

## 0.1.8 - 2026-04-05

### Features

- Add GitHub Copilot CLI as ACP runtime (#57)

### Refactoring

- Rename idea-factory artifacts from issue to idea (#56)

## 0.1.7 - 2026-04-05

### Bug Fixes

- Review round

### Features

- _(build)_ Add AUR support and automation via GoReleaser (#49)

### Refactoring

- Tool calls (#48)
- Task artifacts changes (#52)

## 0.1.6 - 2026-04-04

### Bug Fixes

- Improve failures

## 0.1.5 - 2026-04-03

### Bug Fixes

- Check skills shift before run
- Acp permission

### Features

- Add config.toml (#40)

## 0.1.4 - 2026-04-03

### Bug Fixes

- Failed tool call crash
- Skills frontmatter

### Features

- Add cy-idea-factory skill and improve planning skills DX (#35)

## 0.1.3 - 2026-04-03

### Features

- _(repo)_ Add archive command
- Use acp instead of stream raw json (#34)

## 0.1.2 - 2026-04-02

### Bug Fixes

- _(repo)_ Close tui when finish
- Correct opencode run flags and add stdin support (#25)

## 0.1.1 - 2026-04-02

### Bug Fixes

- _(repo)_ Automatic completion

## 0.1.0 - 2026-04-01

### Bug Fixes

- _(repo)_ Release
- _(repo)_ Color bugs

### Features

- _(repo)_ Add build and release
- _(repo)_ Add adr support
- _(repo)_ Add fetch reviews
- _(repo)_ Add review-round skill
- _(repo)_ Add setup command
- _(repo)_ Add _meta.md for tasks
- Main structure

### Refactoring

- _(repo)_ Improve commands
- _(repo)_ Remove not needed flags
- _(repo)_ Remove PR as required for fix-reviews
- _(repo)_ Improve setup command
- _(repo)_ Remove prd- tasks folder prefix
- _(repo)_ Many improvements
- _(repo)_ Add cy prefix for skills and memory system

[0.4.1]: https://github.com/compozy/compozy/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/compozy/compozy/compare/v0.2.15...v0.4.0
[0.2.15]: https://github.com/compozy/compozy/compare/v0.2.14...v0.2.15
[0.2.14]: https://github.com/compozy/compozy/compare/v0.2.13...v0.2.14
[0.2.13]: https://github.com/compozy/compozy/compare/v0.2.12...v0.2.13
[0.2.12]: https://github.com/compozy/compozy/compare/v0.2.11...v0.2.12
[0.2.11]: https://github.com/compozy/compozy/compare/v0.2.10...v0.2.11
[0.2.10]: https://github.com/compozy/compozy/compare/v0.2.9...v0.2.10
[0.2.9]: https://github.com/compozy/compozy/compare/v0.2.8...v0.2.9
[0.2.8]: https://github.com/compozy/compozy/compare/v0.2.7...v0.2.8
[0.2.6]: https://github.com/compozy/compozy/compare/v0.2.5...v0.2.6
[0.2.5]: https://github.com/compozy/compozy/compare/v0.2.4...v0.2.5
[0.2.4]: https://github.com/compozy/compozy/compare/v0.2.3...v0.2.4
[0.2.3]: https://github.com/compozy/compozy/compare/v0.2.2...v0.2.3
[0.2.2]: https://github.com/compozy/compozy/compare/v0.2.1...v0.2.2
[0.2.1]: https://github.com/compozy/compozy/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/compozy/compozy/compare/v0.1.12...v0.2.0
[0.1.12]: https://github.com/compozy/compozy/compare/v0.1.11...v0.1.12
[0.1.11]: https://github.com/compozy/compozy/compare/v0.1.10...v0.1.11
[0.1.10]: https://github.com/compozy/compozy/compare/v0.1.9...v0.1.10
[0.1.9]: https://github.com/compozy/compozy/compare/v0.1.8...v0.1.9
[0.1.8]: https://github.com/compozy/compozy/compare/v0.1.7...v0.1.8
[0.1.7]: https://github.com/compozy/compozy/compare/v0.1.6...v0.1.7
[0.1.6]: https://github.com/compozy/compozy/compare/v0.1.5...v0.1.6
[0.1.5]: https://github.com/compozy/compozy/compare/v0.1.4...v0.1.5
[0.1.4]: https://github.com/compozy/compozy/compare/v0.1.3...v0.1.4
[0.1.3]: https://github.com/compozy/compozy/compare/v0.1.2...v0.1.3
[0.1.2]: https://github.com/compozy/compozy/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/compozy/compozy/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/compozy/compozy/releases/tag/v0.1.0

---

_Generated by [git-cliff](https://git-cliff.org)_
