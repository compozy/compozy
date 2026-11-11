## 0.5.6 - 2026-11-11

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

### Release Notes

#### Fixes

##### Full access runs the provider unrestricted

**Full access** (`approve-all`) now runs the provider at its unrestricted level, the level you picked. New Codex sessions start in `agent-full-access`, Claude Code in `bypassPermissions`, Google Antigravity in `yolo`, and GitHub Copilot CLI with `allow_all = on`. Before, Codex under Full access still started in its own Auto review mode unless `permissions.provider_full_access = true` was also set, a key the Settings app never showed. Codex then stopped work such as `git push` or `gh issue create` and asked you to approve each one.

**Auto-review** (`auto`) is unchanged and still selects the provider's reviewer mode. An agent that chooses its own narrower provider mode keeps it, and a session started at a narrower level never inherits the unrestricted mode.

Migration notes: this widens what Codex and Copilot sessions can do when they run at Full access, which is the default level. Unrestricted provider commands run with your user's access inside the CompozyOS workspace and operating-system boundaries. To keep the provider's reviewer, set **Settings → General → Permissions** to **Auto-review**, or set `permissions.mode = "auto"` or an agent's `permissions: auto`. The new mode applies when a session starts or resumes. `permissions.provider_full_access` is deprecated and has no effect: CompozyOS still loads it in v0.5.6 and logs a warning naming `permissions.mode`. Remove it from `config.toml`; v0.5.7 deletes the key and rejects it as unknown.
