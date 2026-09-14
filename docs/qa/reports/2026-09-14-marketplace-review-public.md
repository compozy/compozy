# QA Run Report — 2026-09-14 — Marketplace review public journeys

- **Scope:** public CLI/HTTP follow-up for the accepted PR636 review corrections; one review round remains the only review round.
- **Cadence tier:** targeted.
- **Build:** initially `0f77945a70e3f081296ccd772b93189357b6d45e`, latest replay `71fb1466557c68605a1a2191009dcae82f0aa1f9`; isolated daemon compiled for these journeys. Delivery gates run in GitHub CI.
- **Environment:** fresh bootstrap lab; local authored packages and synthetic credentials. No operator workspace or credentials are mutated.
- **Started:** 2026-09-14T08:08:45.342453+00:00; **Status:** targeted public walks complete; final delivery CI pending.

## Personas

Bruno is an operator maintaining separate workspaces and profiles through CLI and documented HTTP endpoints. His intent is to keep installed tools available while changing sources, development packages and profile names.

## Flows in Scope

The bounded retests extend existing charters for Marketplace source management, plugin development, skill discovery, MCP authorization and profile lifecycle. Existing screenshots remain historical visual evidence. Current-head CI owns full Web/Desktop E2E and the injected database-failure, encrypted-storage and subprocess integration checks.

## Session Matrix & Results

| # | Charter | Scenario | Persona | Tour | Status | Issue | Fix commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | CH-marketplace-public-ownership | ET-api-marketplace-sources | Bruno | Data | Pass | | |
| 2 | CH-marketplace-public-ownership | ET-agent-plugin-dev-reload | Bruno | Data | Fixed | BUG-20260914-agent-plugin-dev-rejected | 685af5f3c |
| 3 | CH-marketplace-public-ownership | RT-agent-hot-discovery-skill-isolation | Bruno | Data | Pass | | |
| 4 | CH-marketplace-public-ownership | ET-profile-cli-lifecycle | Bruno | Data | Fixed | BUG-20260914-profile-rename-mcp-reference | c131f5764 |
| 5 | CH-marketplace-public-ownership | ET-cli-mcp-authorize | Bruno | Data | Fixed | BUG-20260914-mcp-secret-replacement-owner; BUG-20260914-mcp-invalid-definition-repair | 2422a5793; 71fb14665 |

These verdicts cover the targeted PR636 public retests described below. They do not replace the broader historical scenario verdicts or claim a full OAuth/provider run. Standard/Codex/Cursor manifest variants, stale persisted resource reconstruction, failure injection and encrypted OAuth record resolution retain their separate owning-suite evidence; current-head CI must confirm those lanes.

## Session Debriefs

### Source maintenance — Bruno — 08:13–08:17 UTC

Created the local `studio` source through CLI and read the same row over HTTP. Duplicate registration, an invalid toggle payload, protected-source removal and an unsupported source scheme returned their structured rejections (CLI duplicate exit 2; HTTP 400/403/422). The documented config.toml retained exact bytes and its comment; the separately saved 11-minute refresh TTL survived. Disabled, re-enabled and removed `studio`; the installed `desk-notes` package retained its version, enabled state and exact provenance. Fresh CLI/HTTP reads after daemon restart confirmed both retained installation and removed source. Evidence: steps 012–044.

The public walk covers operator-reachable rejection paths. Injected database transaction failures remain verified by the real SQLite CI suite, not attributed to this walk. The initial `/api/v1` probe was a runner URL assumption corrected against the documented `/api` route before the source walk. CLI `config set marketplace.ttl` is unsupported; the documented Settings HTTP endpoint performed the edit. No product defect was filed for either setup detour.


### Plugin development — Bruno — 08:25 UTC

Validation accepted the Claude package, but CLI development rejected it before linking (steps 055–056). The leg ended and BUG-20260914-agent-plugin-dev-rejected was filed. Repair and a fresh public re-walk remain pending.

### Development recovery — Bruno — 08:29–08:30 UTC

After 685af5f3c, CLI dev linked the Claude package from inside the design workspace, reload changed only that instance from 1.0.0 to 1.1.0, and HTTP confirmed the same version. Restart retained both the dev generation and the other workspace's published 1.0.0 installation. Removing the dev link restored the published installation in design. Steps 069–085 verify BUG-20260914-agent-plugin-dev-rejected; the additional hook-placement leg remains pending.

### Profile credentials — Bruno — 08:31–08:32 UTC

Settings wrote synthetic manual MCP client secrets independently in user, profile and workspace scopes. A shared Vault ref remained independently owned. The CLI profile rename moved the owned Vault row and left shared/user/workspace rows intact, but MCP config retained the old profile ref and public reads failed. Filed BUG-20260914-profile-rename-mcp-reference; the leg ended before delete. The workspace scope selector writes the base workspace sidecar even with a profile query; it does not author workspace-profile files. Extension Settings overrides reject auth changes, so extension OAuth setup must use its documented auth acquisition path rather than treating overrides as an authoring surface.

### Profile rename recovery — Bruno — 08:43 UTC

Fresh profile writing contained an owned MCP client secret. The preview counted both its Vault row and configured reference. Rename to editorial succeeded; HTTP Settings returned 200, CLI auth status returned needs_login without an ownership error, and Vault metadata exposed only the new owner ref. Steps 111–118 prove the configured-reference repair. No OAuth token was acquired, so token_present=false is expected and is not an authentication-completion claim.

### Credential replacement — Bruno — 08:44 UTC

An attempted foreign-profile reference replacement returned 200 instead of rejecting the invalid owner at write time. Auth begin then failed, and the former owned secret was absent. Filed BUG-20260914-mcp-secret-replacement-owner and ended the leg; public replay follows the repair. The earlier expectation of immediate write rejection was validated by the observed destructive replacement, rather than treating a metadata-only check as proof of authorization safety.

### Credential replacement recovery and deletion — Bruno — 08:50 UTC

A fresh publishing profile began with a working owned client-secret binding. All five foreign/daemon-managed ref replacements returned HTTP 400; its sidecar bytes and Vault metadata stayed intact. After restart, independent Settings and CLI auth-status reads succeeded. Delete preview and CLI apply returned identical removed counts, including one exclusive credential; subsequent Vault metadata retained user, workspace, other-profile and shared refs. Steps 126–141 verify BUG-20260914-mcp-secret-replacement-owner. Metadata presence does not claim token exchange or plaintext decryption; real encrypted resolution remains owning-suite evidence.

Attempting to repair the already-invalid editorial profile from the earlier failed walk returned 500 (step 125), so the re-walk started with the fresh publishing profile, as required by the replay protocol. That separate repair failure was subsequently fixed and replayed in steps 145–153, as recorded below.

### Invalid-definition repair follow-up

The step125 failure is a separate confirmed defect, BUG-20260914-mcp-invalid-definition-repair: pre-mutation existence detection probes old auth state and aborts before repair. The owning Settings coordinator now resolves only the exact definition. Focused race suites passed. Steps145–153 verified repair through HTTP/CLI, including restart, on71fb14665. Shared/environment ref handling and shared metadata retention also passed in155–161. The preceding CI exposed two formatting/function-length issues, repaired in the same follow-up, and a session stop-recovery database identity failure that did not reproduce in ten focused race repetitions; no regression cause was established.

### Retained skill and local sibling — Bruno — 09:04–09:06 UTC

Prepared representative historical ClawHub installation files using the current documented sidecar shape; this is not a fresh ClawHub download and no persisted resource rows were injected. After restart, public skill list/view/info preserved the original body and provenance. Resource list retained the local sibling MCP and manual MCPs, while excluding the ClawHub embedded MCP. Operator tool inventory also excluded the retired server. Steps169–181. The public mcp_status command does not expose the skill-owned canary as a configured Settings row, so these observations prove publication/retirement, not a successful callable MCP session. The stale-row and callable reconstruction assertions remain attributed to the real SQLite daemon integration suite.

### Hook authoring and placement precondition — Bruno — 09:07–09:09 UTC

Authored a hook audit extension against the public Go SDK, built it through extension build, and installed the resulting generation in design/default. The scoped hook is present in the public hook inventory. Operator tool invoke succeeds but does not dispatch session notifier hooks (no authored audit output in any scope); it cannot substitute for a session-origin trigger. Steps184–194. Execution/isolation and dev override transitions were subsequently verified in steps 206–277 below. The dev-version extension init command could not locate its source checkout from the isolated lab; the standalone authored SDK project built successfully with an explicit local SDK module replacement.

Historical CI for `71fb14665`: Catalog validation `34825774952` and React Doctor `34825774988` passed; main CI exposed six style findings, fixed in `06459353a`. The preceding2422 shard7 failure was TestSharedSessionStopOperation/Should_recover_active_stop_after_manager_restart_at_event_persistence (pinned database identity mutated). Ten focused race repetitions passed in4.962s; the production and test paths are unchanged against main. No assertion or database guard was weakened; current-head CI remains required.

### Hook placement execution — Bruno — 09:12–09:16 UTC

The SDK package uses session.post_create so empty public sessions produce real hook dispatch without invoking a model. Workspace-only installation ran once in design/default, zero times in operations/default and design/editorial. Global installation ran once in both default-profile workspaces. A development generation ran exactly once with the development label in design and left the installed label in operations. Reload to a hookless generation suppressed the inherited hook only in design; restart preserved suppression. Unlink restored one global hook; profile disable suppressed it in both workspaces. Each checkpoint compares the authored append-only audit file with public hooks runs. Steps206–277. Sessions remained runtime-unbound and were stopped through their owning profile; no provider prompt or provider initialization occurred.

Two setup detours are explicit: a same-name local install in a second scope is rejected by the existing public acquisition path, so the public walk switched from workspace-only to global installation before applying the dev overlay; simultaneous attachment and detach permutations remain the owning real SQLite integration's evidence. Reload requires the source directory when invoked outside that source, and session stop requires the selected non-default profile. These did not require product changes. Tool.pre_call dispatch itself remains covered by the unchanged canonical session-notifier integration rather than these session.post_create observations.

### Four-cell profile credential matrix — Bruno — 09:18 UTC

Public Vault writes prepared20 synthetic credential references: manual and extension-owned cells in profile and workspace-profile scopes, with access/refresh/configured-client/DCR/registration-access suffixes. These are standalone Vault rows, not injected OAuth token/registration records. Ordinary profile and selected workspace-profile sidecars referenced their own configured-client values. Rename cred-source to cred-result rewrote all20 refs and both configured definitions; CLI auth status and HTTP Settings succeeded in both scopes. Delete returned exactly the preview's removed object, removed all20 owned refs, and retained the four pre-existing user/workspace/other-profile/shared refs. Steps279–312. Actual encrypted OAuth record relocation and decryption remain the real Vault/SQLite lifecycle suite's proof.

### Hot skill discovery — Bruno — 09:19 UTC

Changing only the local sibling's MCP sidecar while the daemon ran replaced its published resource within approximately2.2s. The retired ClawHub MCP stayed absent, and the manual MCP resource remained. Public skill view/info retained the original body and provenance; independent hashes show all retained installation files byte-identical. Steps313–322. Callable MCP session reconstruction and stale-row fail-closed behavior remain separately attributed to the daemon integration suite; no public callable-tool claim is inferred from resource metadata.

## What Was Fixed

Review corrections are already implemented through `0f77945a7`; this run verifies their public effects. Four defects found in the public walk were repaired in 685af5f3c, c131f5764, 2422a5793 and 71fb14665, with owning race checks and public replays recorded in this report. The single review round was not repeated.

## Paper Cuts

See the recorded divergences and recovery observations above.

## Runtime Errors Observed

See the recorded divergences and recovery observations above.

## Verification and Evidence

The bootstrap manifest and completed session state are tracked in `.deep-review/pr-636-luna-round-1/public-qa-state.json`. Local public transcripts are stored under `docs/qa/evidence/2026-09-14-marketplace-review-public/` (323 numbered receipts, excluded from Git). Database-failure injection and direct ciphertext checks remain integration evidence, not public-walk evidence.

## Decisions for a Human

None identified during preparation.

## Final Status

Targeted public QA is complete: two passed rows and three fixed rows, covering four repaired defects (one Data-Loss and three Blocks-Completion). All four defects have owning race checks and public recovery evidence. The broader historical OAuth JSON-parity failure is not reclassified by this scoped run. This report distinguishes the public observations from reused owning integration evidence throughout.

The strict evidence audit passed at 09:22:46 UTC with 323 log entries, zero blockers and zero warnings after the final report and explicit CI-only delivery policy were recorded. This is evidence-structure validation, not proof of green CI. Manifest-owned teardown completed at 09:22:47 UTC with `clean: true` and `survivors: []`; the daemon required a targeted signal sweep after graceful stop. Receipts: `qa/qa-audit-report.json` and `qa/teardown.json` under `/Users/pedronauck/dev/qa-labs/compozy-marketplace-review-public-20260914-080721-168683-lab/qa-artifacts/`. The lab is closed. Final-head CI remains required before delivery. Heavy local gates are waived by the user's explicit CI-only instruction. No final delivery claim is made until the final committed head passes every required CI lane.
