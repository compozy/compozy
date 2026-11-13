# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## 0.6.3 - 2026-11-13

### Bug Fixes

- Download the recorded desktop release when the update feed is unreachable (#1643)
- Keep continued child sessions in their parent's session tree (#1647)
- Read the Claude Keychain login past a stale macOS credential file (#1648)
- Promote synthetic wakes with live ACP defaults (#1650)
- Count only closing PR references as issue-triage coverage

### Features

- Add a Move under parent picker for session organization (#1646)
- Group CompozyOS menus into submenus with List and Kanban parity (#1652)

### Performance Improvements

- Reuse the session writer's transcript encode buffer across appends (#1634)
- Read only triage rows in the periodic triage session sweep (#1636)
- Reload skill watcher config only after its files change (#1627)
- Share hosted MCP projection discovery across session streams (#1629)

### Testing

- Isolate the telemetry benchmark receiver process (#1639)

## 0.6.2 - 2026-11-13

### Bug Fixes

- Recover orphaned automation runs and expose cancellation (#1613)
- Stop dropping mid-pass message promotions and race-scale flaky Go test waits (#1615)
- Restore v0.6.0 config upgrades and make daemon boot and restart bounded (#1621)
- Bound filtered logs stream polls by the ledger head (#1625)

### Features

- Optional OpenTelemetry trace and metric export (#1552)
- Make the update button prominent with an animated brand border (#1623)

### Performance Improvements

- Read a session's instance binding by key instead of a catalog list (#1632)

### Testing

- Wait for obsolete attention wake settlement

## 0.6.1 - 2026-11-12

### Bug Fixes

- Point the Atlassian extension at the v2 MCP endpoint (#1551)
- Keep issue triage running when a gh search flag is rejected
- Encode triage search queries and define retryable gh errors
- Make issue intake failures reach maestro
- Preserve requested reasoning during Goal rotation (#1555)
- Align Loop ACP provenance with applied options (#1554)
- Keep daemon booting with undefined disabled providers (#1556)
- Isolate native tool state and enrich retired task results (#1558)
- Honor recurring catch-up policies after host sleep (#1569)
- Keep loaded transcript messages stable when a live entry rolls pages (#1571)
- Retire stale terminal windows and open agent terminals on demand (#1570)
- Keep skill documents inline without duplicate result content (#1575)
- Recover heartbeat cadence after runtime configuration repairs (#1584)
- Preserve exact Claude models during session recovery (#1560)
- Watch explicitly selected fork pull requests (#1578)
- Resolve current heartbeat policy for manual wakes (#1579)
- Reconcile Goal ACP provenance and prove cleanup delivery (#1574)
- Retire pre-view Loop pin translation in v0.6.1 (#1590)
- Resume agent instances after daemon listeners start (#1591)
- Accept check-in proposals with surrounding prose (#1589)
- Keep UDS and HTTP harness lifecycle parity with standing context (#1594)
- Reconnect the window-manager stream after a client-side failure (#1583)
- Keep unusable SQLite controls from rejecting compliant heads (#1592)
- Allow terminal exec to target managed worktrees (#1585)
- Group native recovery reads and expose reply watches (#1576)
- Avoid duplicate and stale child early-stop wakes (#1586)
- Validate heartbeat health requests and synchronize daemon boot (#1582)
- Return admitted automation runs before agent completion (#1587)
- Retry terminal attach attempts that stall before ATTACHED (#1609)
- Preserve exact Claude models on fresh startup (#1610)
- Prevent child reply and attention wake starvation (#1604)
- Keep boot window progress, attempts and versions accurate across retries

### Features

- Add bundled interface fonts, text size and letter spacing to CompozyOS Appearance (#1550)
- Reveal model names and page the provider filter rail (#1536)
- Bind operator rules and peer roster into always-on agent context (#1572)
- Fold worktrees under projects in the terminal picker (#1599)
- Redesign the desktop boot window with real upgrade progress (#1600)

### Miscellaneous Tasks

- Route workspace heartbeat triage to claude-haiku-5-5

### Refactoring

- Remove compatibility shims expired in v0.6.1 (#1593)
- Split the desktop boot draft and address its review notes

### Testing

- Cover the composer context toggle and pin process fixture identities (#1577)
- Expect the boot window font-src CSP in E2E-034

## 0.6.0 - 2026-11-11

### Bug Fixes

- Publish User-scope instance reports, notifications and event summaries (#1410)
- Preserve pre-permission assistant prose ordering (#1397)
- Honor the runtime confirmed when continuing a session (#1417)
- Order lifecycle attention baselines with permission publication (#1426)
- Align continuation preflight and advanced default semantics (#1430)
- Keep a reload from restoring All profiles after leaving it (#1443)
- Retain canonical attention baselines through dismissal and shutdown (#1429)
- Stop re-sending static prompt context and count provider costs per turn (#1437)
- Publish committed dismissal when status lookup fails (#1446)
- Refuse monitors that cannot resume Claude native subagents (#1450)
- Settle native parent turns independently of attention badges (#1449)
- Explain hosted terminal calls without an active run (#1452)
- Clarify instance list capacity and missing target errors (#1454)
- Preserve actionable skill catalog resolution failures (#1453)
- Preserve newer native tracking during turn cancellation (#1458)
- Stop pinning the embedded tailscale node to the default-route device (#1459)
- Align batch session actions and bound parallel execution (#1469)
- Cap open child-session lists at six with Show more (#1476)
- Run providers unrestricted under Full access (#1477)
- Bound session family traversal and refresh SQLite statistics (#1466)
- Recover provider sessions after signing in again (#1483)
- Keep file tree and diff path names readable on deep paths (#1482)
- Bound native history and catalog reads (#1484)
- Lead the session head with the list toggle and make the composer context control a toggle (#1490)
- Retain persisted turn identity in native call tracking (#1496)
- Show resumed Claude subagents as running (#1499)
- Share one session batch lock and show progress on every running target (#1498)
- List profile-only providers and mark a session's disabled provider (#1503)
- Return compact records from native tools by default (#1515)
- **BREAKING:** return compact records from native tools by default (#1515)
- Keep daemon startup and instance pause available when a report fails to publish (#1492)
- Reclaim oversized telemetry WAL allocation (#1501)
- Preserve startup Stop ownership and Loop runtime provenance (#1500)
- Restore main CI after the v0.6.0 release merges (#1527)
- Start full access sessions without an advertised mode option (#1525)
- Refresh hosted MCP catalogs and reduce native schema boot overhead (#1537)

### Features

- Seed a realistic always-on team in the demo seed (#1415)
- Let operators disable providers they don't use (#1372)
- Adopt the v1.2 brand identity, add the brand manual and redesign compozy.com (#1408)
- Answer provider interactive questions through ACP elicitation (#1478)
- Group the session list by project by default (#1488)

### Performance Improvements

- Stop unchanged workspace-catalog polls from re-rendering the shell (#1457)

### Testing

- Isolate migration-tail timeout from fixture admission (#1423)
- Cover permission dispatch with a full event channel (#1427)
- Assert pending permission blocker on dismissal retry (#1455)
- Serve every agent-editors request so MSW never opens a real socket (#1456)
- Strengthen routing, file reveal and automation page coverage (#1495)
- Finish followed sessions before daemon teardown (#1497)

## 0.5.5 - 2026-11-09

### Bug Fixes

- Keep SQLite statement journals in memory during write transactions (#1367)
- Hide automation-run sessions from the session list (#1366)
- Make TestSetProcessGroupNiceness deterministic and document the setpriority fork race (#1373)
- Keep session owner snapshots valid while the writer appends WAL frames (#1371)
- Let triage cleanup wait for the full stop ladder before archiving (#1365)
- Keep internal daemon handoff variables out of session and terminal environments (#1364)
- Prevent source executable contention and isolate cooldown fixtures (#1375)
- Keep the Files identity in its window head and route transcript paths to Files tabs (#1378)
- Return main CI to green after the 2026-11-08 merges (#1380)
- Apply Linux launch niceness before exec (#1386)
- Preserve prompt completion across concurrent steering events (#1376)
- Move core WAL restart synchronization to maintenance (#1377)
- Settle automation fires after agent startup failures (#1389)
- Preserve explicitly empty environments through process launch (#1390)
- Wait for archive responses before retiring session windows (#1392)
- Name automation and always-on sessions by origin and keep relative times live (#1391)
- Clear acknowledged always-on agent attention (#1395)
- Retain lifecycle ownership across startup and shutdown races (#1394)
- Recover acknowledged attention without blocking live sessions (#1396)
- Retain startup recovery through process and storage failures (#1398)
- Preserve attention publication ordering and lifetimes (#1401)
- Preserve failed startup event sequences and cleanup ownership (#1404)
- Bind approvals to run identity and explain terminal denials (#1412)

### Features

- Browse the CompozyOS marketplace by category with one-step Add and a curated plugin catalog (#1360)
- Let always-on check-ins call the agent's tools and auto-accept declared proposals (#1385)

### Testing

- Repair check-in proposal pick runtime fixtures (#1406)

## 0.5.4 - 2026-11-07

### Bug Fixes

- Retain oversized tool results in User sessions (#1322)
- Allow profile moves with stopped subagent descendants (#1321)
- Compare SQLite commit pauses against paired storage tails (#1325)
- Accept fenced proactive triage replies (#1319)
- Preserve managed Goal pause ownership until coordinator settlement (#1329)
- Read proactive check events in heartbeat status (#1332)
- Hold MCP server form errors until a field is touched (#1338)
- Settle expired permissions and stopped session interactions (#1335)
- Keep the CompozyOS desktop window open when Settings restarts the daemon (#1339)
- Compact always-on agent cards and hide deleted agents (#1341)
- Keep dispatched children responsive after definition pauses (#1333)
- Defer oversized invoked skills to skill_view instead of failing dispatch (#1330)
- Refresh MCP tools in open sessions (#1342)
- Preserve permission recovery reports and terminal refusals (#1347)

### Features

- Route new issues through an unattended issue vetter
- Add a desktop download page and header download CTA to the site
- Add daily CompozyOS daemon log triage agent (#1353)
- Add project file explorer, file viewer and session Changes panel (#1351)

### Refactoring

- Rename the workspace always-on agents by their pipeline role

### Testing

- Observe paused child monitor settlement before receipts (#1349)

## 0.5.3 - 2026-11-06

### Bug Fixes

- Show a pending runtime switch on session rows (#1294)
- Allow profile moves with archived descendants (#1299)
- Wake pull request monitors when merge conflicts appear (#1289)
- Keep code_implementer working until its item is done
- Hide session chrome that doesn't apply to always-on main sessions (#1304)
- Defer fallback steering until active tools finish (#1296)
- Enable sign-in for bare OAuth MCP remotes (#1308)
- Let agents attach MCP servers (#1309)
- Align OAuth browser journeys with sign-in action (#1310)
- Keep native CLI sign-in verdicts truthful (#1278)
- Load skill resources in User sessions (#1312)
- Back off provider usage 429s, collect each account once, and re-verify native sign-in after restart (#1307)
- Keep CompozyOS usage aliases on their own account (#1280)

### Features

- Remove retired agent kind compatibility inputs (#1295)
- Add a Marketplace MCPs view with a Plugins | MCPs toggle (#1306)

### Performance Improvements

- Batch forge lookups and refresh displayed branches on demand (#1291)

### Testing

- Give the post-upgrade phase of the destructive-cut reopen test its own budget

## 0.5.2 - 2026-11-04

### Bug Fixes

- Keep generated avatars for bundled always-on agents (#1260)
- Bump next to 16.4.5 (November 2026 security release)
- Stop CompozyOS PR display polling from exhausting the GitHub quota (#1268)
- Attach CompozyOS desktop bootstrap to a live, still-booting daemon (#1255)
- Expose CompozyOS CLI after macOS desktop installation (#1261)
- Remember cross-workspace read consent for always-on agents (#1274)
- Allow script source approval before the first check-in (#1262)
- Show CompozyOS data upgrade progress during coordinated restarts (#1256)
- Make SQLite contention acceptance measure untraced work (#1285)
- Confirm Claude triage models and report configuration rejections (#1276)

### Features

- Override bundled agent permissions without forking its prompt (#1273)

### Performance Improvements

- Bound agent activity reads and batch team pulses (#1288)

### Testing

- Fix loop editor, agent select, inspector and context menu story debt (#1277)

## 0.5.1 - 2026-11-04

### Bug Fixes

- Let the mouse wheel scroll the transcript over Mermaid diagrams (#1176)
- Apply the normal removal policy when a worktree's base ref was deleted (#1177)
- Stop reporting a lost wake-up when a session cancels its own monitor (#1190)
- Keep session_prompt output schema in sync with its receipt (#1188)
- Open a profile-scoped agent in its own profile from All profiles (#1189)
- Close another profile's session windows when switching into a profile (#1197)
- Show N linked PRs instead of one PR chip plus +N in the session header (#1207)
- Offer Add stays-on agent from the session list toolbar (#1187)
- Pick a stays-on agent's model inline on its home (#1201)
- Stop stays-on agents asking for every CompozyOS tool call (#1206)
- Allow comparison and logic builtins in CompozyOS Loop templates (#1213)
- Require approval for destructive session profile moves
- Compare Loop template numbers exactly
- Apply review follow-ups to profile-switch window sweep (#1221)
- Start the CompozyOS daemon when a profile is archived (#1214)
- Save agent avatar and color changes and never fail an agent read on appearance (#1208)
- Reject unsupported agent avatar backgrounds and clarify validation
- Resume a desk's interrupted profile-switch sweep
- Align generated clients and validation after main merges
- Cover profile-switch rollback during owner reads
- Preserve newer session attention when marking it seen
- Enforce session profile on workspace routes (#1218)
- Scope the command palette client directory to the caller's profile (#1217)
- Keep document-wide streams and queued clicks out of the HTTP/1.1 pool (#1212)
- Validate window manager bindings in the client profile
- Declutter the session list rows and give built-in agents icons (#1215)
- Preserve scoped session row identity and accessible runtime details
- Bound CI change detection to compared commits
- Match subagent navigation against accessible runtime labels
- Recover an interrupted move to another profile after a daemon restart (#1222)
- Let agent duplicate drop an unsupported saved appearance (#1225)
- Keep session row actions working after a profile rename (#1228)
- Close another profile's terminal, agent and automation windows on a profile switch (#1226)
- Sweep a desk again when an owner read failed during a profile switch (follow-up from #1221)
- Preserve provider auth recovery boundaries
- Keep an encoded colon in a file href's name (follow-up from #1223)
- Name gallery runtimes like the live row, no model for provider-only (follow-up from #1215)
- Reset the declutter draft thread segment on direction switch (follow-up from #1215)
- Draw the built-in General icon before the agent catalog loads (follow-up from #1215)
- Apply one all_profiles read contract to terminal, agent-instance and agent reads (#1238)
- Use a non-auth ACP error for rate-limit recovery fixture
- Advertise the model in the desktop open-paths fixture
- Format profile authority regression tests
- Omit HTTP-only SDK examples for UDS profile capture
- Simplify profiling test context error selector
- Select the agent runtime after its creation scope
- Preserve palette catalog writes and refresh User catalogs
- Assert always-on agent creation succeeds
- Preserve agent runtime when changing creation scope
- Exercise runtime retention across agent scope changes
- Isolate skill refresh failures and scope catalog diagnostics
- Classify agent exits during daemon shutdown as interrupted, not crashed (#1245)
- Repair CI diagnostics and scope agent page title assertions
- Satisfy scoped Go lint after catalog performance changes
- Preserve revocation errors on canceled mutation requests
- Resolve native instance creation approval in integration journey
- Keep one page heading in agent setup editors
- Assert unique headings after correcting agent editor hierarchy
- Preserve canceled turns across daemon restart races
- Address PR review feedback on the Check-ins source UI
- Harden heartbeat source polling and SDK compatibility
- Preserve optional heartbeat source fields in extension SDK
- Retain persisted heartbeat hook cancellation reason in regression
- Keep extension runtime within the source size limit
- Register safe file reads in heartbeat source boundaries
- Format source status client contract coverage
- Collect usage for provider aliases and keep live native_cli sign-in verdicts (#1250)
- Reject ambiguous usage routes and edit refreshed provider settings
- Preserve heartbeat contract names while satisfying focused lint
- Persist restart skip before acknowledging shutdown cancellation
- Align source fixtures with profile ownership and post-poll skips
- Verify heartbeat source provide conformance in SDK integration
- Align provider auth contract coverage with probe timestamps
- Keep shutdown cancellation responsive without resuming canceled turns
- Decode instance response envelopes in source catalog coverage
- Allow telemetry for registered workspace identities after deletion
- Bound checkpoint shutdown and reject reduced benchmark throughput
- Preserve overflow proposal details in check-in reports
- Read durable cancellation marker envelopes during restart recovery
- Verify telemetry ownership after workspace re-registration
- Seed historical workspace fixtures before head migrations
- Preserve workspace registration before telemetry migration
- Isolate automation history fixture from recursive stop triggers
- Preserve agent error exports in settings skills mock

### Features

- Move a session to another profile (#1205)
- Mark a session's Needs you as seen (#1210)
- Open transcript file paths in the editor from the CompozyOS desktop app (#1223)
- Wait for a lost provider sign-in and resume the turn (#1229)
- Capture daemon runtime profiles over UDS (#1240)
- Rename the Stays on agent kind to Always-on and align permission and setup labels (#1237)
- Let always-on agents check external sources on each check-in (#1247)

### Performance Improvements

- Reduce repeated skill, catalog and cache work (#1244)
- Gate stream-covered interaction polls on catalog stream status (#1246)
- Isolate telemetry and shorten SQLite write locks (#1241)

### Refactoring

- Import profile owner helpers from the profiles barrel (follow-up from #1215)

### Testing

- Synchronize reply-watch integration waits on runtime edges (#1178)
- Cover a fresh derive that names the matching source profile (follow-up from #1218)
- Assert a profile move carries messages and names its recovery (follow-up from #1205)
- Assert the 8s window-manager command deadline (follow-up from #1212)
- Cover mark-seen profile refusal and retained failure (follow-up from #1210)
- Order two JSON number operands numerically (follow-up from #1213)
- Keep a valid saved appearance in the instance projection (follow-up from #1208)
- Assert move result fields and reach the sessionless move guard (follow-up from #1205)
- Record runtime writes in the owner-profile route matrix (follow-up from #1218)
- Cover the committed move response when enrichment fails (follow-up from #1205)
- Assert the exact TOON moved header and value order (follow-up from #1205)
- Return and store the requested worktree ID on create (follow-up from #1222)
- Resume terminal catalog WebSocket streams from last_event_id (follow-up from #1212)
- Format follow-up tests for the line-length lint
- Resume the aggregate terminal stream with its selection-bound cursor (follow-up from #1212)

## 0.5.0 - 2026-11-03

### Bug Fixes

- Deliver late hosted MCP tools and bound projection discovery (#1135)
- Recover cold npm adapter starts before ACP initialization (#1136)
- Recover superseded extensions and interrupted installs (#1137)
- Preserve shared composer images and recover missing attachments (#1144)
- Strip label prefixes from automatic session titles (#1145)
- Keep QA provider credentials opt-in and remove lab copies on teardown (#1150)
- Run stays-on agents at their configured permission level (#1157)
- Keep allowed permission receipts with their tool (#1158)
- Coalesce monitor wakes outside the operator input queue (#1159)
- Suspend Goal turns while external work is pending (#1151)
- Resolve merge train review findings on main
- Claim subagent wakes parked during a parent resume
- Release Goal waits after wait-mode children and keep wakes in generation
- Clear the remaining main lint and integration failures
- Keep finished automation groups reachable from a grouped search
- Keep the linked work list open after an unlink leaves one item
- Keep the linked work list open when Undo is pressed in its toast
- Use CompozyOS product language and format the skill references
- Close cross-scope review gaps in approve-all session reads (#1173)
- Keep store failures out of the masked profile conflict
- Repair main CI after the stays-on merge
- Install an approved skill before recording the approval
- Use CompozyOS product language in the permissions draft
- Keep scenario-configured model discovery in mock-agent E2E daemons
- Drop the builtin reviewer mode when a provider is routed to another runtime
- Match attributed delegated child prompts in the subagent CLI fixture
- Match the attributed delegated child prompt in the update work fixture
- Keep permission requests lost to a disconnect askable after recovery

### Features

- Let archive and delete include child sessions (#1131)
- Add one View options menu with Collapse all and Kanban lane folds (#1133)
- Add stays-on agents and agent identity
- Reveal subagent model, kind and activity in the composer panel (#1147)
- Let a session monitor wait on other sessions (#1146)
- Add a pull_request session monitor (#1156)
- Simplify spec-cycle to agents and spec authoring (#1152)
- **BREAKING:** simplify spec-cycle to agents and spec authoring (#1152)
- Hide finished automation sessions from the default list (#1163)
- Link worktrees, branches and pull requests to sessions (#1161)
- Let approve-all sessions read sessions across workspaces and profiles (#1171)
- Make GitHub Copilot CLI a first-class provider (#1170)
- Make stays-on agents safe to leave running (#1174)

### Performance Improvements

- Improve responsiveness under concurrent sessions (#1141)

### Refactoring

- Extract the skill approval ledger write

### Testing

- Stop the stale-entry promotion hook from blocking its dispatch
- Align integration expectations with merged Goal and instance contracts
- Expect the authored level after instance context replacement

## 0.4.8 - 2026-11-01

### Bug Fixes

- Keep the accent ink on the menubar Updating pill (#1089)
- Restore the usage tray before the bell and hold its slot while loading (#1090)
- Ellipsize long session titles in session message card headers (#1099)
- Remove the Skip to content link that appeared on window reactivation (#1096)
- Hide empty projects in project grouping behind a Show empty projects option (#1091)
- Authorize explicit cross-workspace session replies (#1103)
- Let operators resume and take over spawned sessions after the parent stops (#1098)
- Decode each Later reason character reference once and only when complete
- Keep shelf card edges inside the horizontal scroller (#1116)
- Move the message trail right and add sidebar tonal hierarchy (#1115)
- Clear the window when its open session is archived (#1111)
- Store only clean text in generated session titles (#1125)
- Make the spawn reaper's parent-stop reap atomic and conditional (#1123)
- Let sessions call OAuth-authenticated extension MCP servers (#1117)
- Address merge train review findings on stops, titles, and MCP lookup
- Close shell-wait guard gaps found in review
- Satisfy gocritic in the PowerShell option check
- Keep the explicit owner wording in the MCP ambiguity error
- Format the MCP ambiguity error for golines
- Keep exit-plan reads from emitting worktree.status_refreshed (#1121)

### Features

- Rich-text reason and date picker in the Move to Later dialog (#1093)
- Group session processes into trees (#1106)
- Hide child sessions on the Kanban board by default (#1124)
- Apply runtime selection changes to the live session immediately (#1108)
- Redirect foreground shell waits to Compozy monitors (#1127)
- Make session monitors durable across daemon restarts and trace their lifecycle (#1129)

### Refactoring

- Replace the in-repo prototype gallery with @compozy/proto (#1092)
- Align @compozy/ui with the bnui contract (#1126)

### Testing

- Cover captured-only AfterStop cleanup and lead-only Stop all for process trees

## 0.4.7 - 2026-10-31

### Bug Fixes

- Preserve clarify line breaks and make the clarification dock collapsible (#1062)
- Support User-owned session lifecycle actions in global scope (#1060)
- Verify Claude hosted MCP readiness before startup succeeds (#1066)
- Show only the title for open terminals, without a running mark (#1073)
- Flag child sessions that stop without delivering and make end-turn guidance conditional (#1069)
- Keep forked and continued sessions visible when hiding child sessions (#1077)
- Reuse stdio MCP server processes across discovery, status, and calls (#1081)
- Resolve MCP OAuth credentials from the declaring scope (#1080)
- Keep failed worktree creations visible and roll them back in order (#1082)
- Verify interrupted worktree checkouts under the lock and by branch (#1084)
- Report the Later count on the active facets read
- Keep the facets Later total optional in the API contract
- Label inherited MCP auth in listings and hold unreadable workspaces

### Features

- Archive a project from the top-bar project submenu (#1061)
- Zoom the Mermaid fullscreen viewer with Ctrl/Cmd+wheel, pinch, and keys at the cursor (#1059)
- Replace the folder browser with a directory picker input and centre onboarding step numbers (#1064)
- Open terminals in a project's managed worktree (#1067)
- Add a Waiting board column and let every Kanban column collapse (#1079)
- Park sessions in Later with an optional reason and return date (#1083)

## 0.4.6 - 2026-10-30

### Bug Fixes

- Make automatic titles lightweight and retry failed attempts
- Return conversation clear after durable reset
- Default terminals to the account login shell and open them from a launcher
- Refresh session rows in every catalog scope when the runtime changes
- Stop requests for unavailable desktop resources (#1038)
- Hand focus between project overlays before hiding their origins (#1040)
- Preserve Claude usage access across Keychain token refreshes (#1041)
- Restore project menubar focus after context menu dismissal
- Keep focused skip link out of desktop layout
- Report the enforced spawn budget to agents (#1048)
- Share terminal approvals through attention summary
- Restore workspace focus after worktree creation dialog

### Features

- Simplify the Fork session dialog and always fork into this window
- Selection-aware session context menu and project context menus (#1032)
- Group projects into folders and add session totals footer (#1047)
- Add proto.tsx prototype gallery that renders real web components (#1043)
- Open Kanban sessions in a resizable drawer over the board (#1055)
- Multi-select sessions and run bulk actions on the Kanban board (#1054)
- Inspect and control processes started by sessions (#1049)

### Testing

- Cover remaining merge train review invariants
- Synchronize detached harness session fixtures
- Record additive attention approval schema contract

## 0.4.5 - 2026-10-29

### Bug Fixes

- Preserve default profile customization across daemon startup (#1001)
- Preserve Linux update executor identity across clock drift (#1004)
- Restore terminal lifecycle under All projects and User (#1009)
- Preserve archive recovery and pause rejected automation replacements
- Unblock missing-root recovery and restore native input contracts
- Preserve picker focus and synchronize subagent navigation checks
- Preserve native tool domain errors and sanitized causes (#1014)
- Recover detached message edits after lost acknowledgment (#1018)
- Isolate missing workspace state from healthy operations (#1016)
- Register SQLite Unicode search before opening connections
- Make Goal controls interrupt work and clarify session lifecycle (#1019)
- Expose CompozyOS native tools in Pi sessions (#1021)

### Features

- Change existing session worktrees in place (#1002)
- Inspect subagents in a resizable activity drawer (#1007)
- Archive projects and add safe removal actions (#1010)

### Performance Improvements

- Accelerate session and app data loading (#1006)

### Testing

- Wait for subagent branch expansion before paging
- Account for uptime in denied settings mutation checks

## 0.4.4 - 2026-10-28

### Bug Fixes

- Preserve active sessions through desktop updates (#991)
- Show truthful subagent status across lists, cards and native tasks (#993)
- Regenerate native tool catalog and approve subagent child_activity schema
- Missing agy acp reasoning effort levels (#858)
- Address subagent status review follow-ups
- Keep the frozen elapsed time on idle subagents
- Split subagent hover preview and expect urgency-ordered branch pages
- Edit session messages directly in the composer (#996)
- Retry refused message edits and keep detached unconfirmed sends

### Features

- Sync Interface and Code fonts across clients (#990)

## 0.4.3 - 2026-10-27

### Bug Fixes

- Prevent duplicate release PRs after release merges (#986)
- Recognize Linux daemon lock owners despite clock drift (#987)
- Bound Mermaid diagrams and add fullscreen zoom (#988)

### Performance Improvements

- Reduce session latency, retained memory and shell render work (#983)

## 0.4.2 - 2026-10-26

### Bug Fixes

- Stop background Claude usage reads from prompting for the keychain (#977)

### Testing

- Allow cold PowerShell startup when reading the Windows grandchild PID

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

[0.6.3]: https://github.com/compozy/compozy/compare/v0.6.2...v0.6.3
[0.6.2]: https://github.com/compozy/compozy/compare/v0.6.1...v0.6.2
[0.6.1]: https://github.com/compozy/compozy/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/compozy/compozy/compare/v0.5.5...v0.6.0
[0.5.5]: https://github.com/compozy/compozy/compare/v0.5.4...v0.5.5
[0.5.4]: https://github.com/compozy/compozy/compare/v0.5.3...v0.5.4
[0.5.3]: https://github.com/compozy/compozy/compare/v0.5.2...v0.5.3
[0.5.2]: https://github.com/compozy/compozy/compare/v0.5.1...v0.5.2
[0.5.1]: https://github.com/compozy/compozy/compare/v0.5.0...v0.5.1
[0.5.0]: https://github.com/compozy/compozy/compare/v0.4.8...v0.5.0
[0.4.8]: https://github.com/compozy/compozy/compare/v0.4.7...v0.4.8
[0.4.7]: https://github.com/compozy/compozy/compare/v0.4.6...v0.4.7
[0.4.6]: https://github.com/compozy/compozy/compare/v0.4.5...v0.4.6
[0.4.5]: https://github.com/compozy/compozy/compare/v0.4.4...v0.4.5
[0.4.4]: https://github.com/compozy/compozy/compare/v0.4.3...v0.4.4
[0.4.3]: https://github.com/compozy/compozy/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/compozy/compozy/compare/v0.4.1...v0.4.2
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
