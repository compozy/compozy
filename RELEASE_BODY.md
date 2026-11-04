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

### Release Notes

#### Features

##### Approve a check-in script as soon as it exists

A new or edited check-in script used to ask for approval only after a check had already tried to run it. On a new or paused agent, that check never came, so the script stayed at "Not checked yet" with nothing to approve.

- **Approve right away.** The Heartbeat page shows a "needs your approval" notice for each waiting script as soon as it exists, with its size, when it last changed, and an Open script action in the desktop app.
- **Approve from the bell.** The menubar bell lists scripts waiting for approval, and you can approve them from there.
- **Paused agents.** On a paused agent, an approved script runs as soon as you resume it.

Approval is still operator-only and tied to the script's exact contents.

##### Change a bundled agent's permissions without copying it

You no longer need to copy a bundled or extension agent's whole `AGENT.md` just to run it with different permissions or a different model. Override only the fields you need in `config.toml`:

```toml
[agents.overrides.orchestrator]
permissions = "approve-all"
# provider, model, reasoning_effort and speed are optional
```

The override applies to whichever definition of that agent is in effect: workspace, global, extension or built-in. Everything you don't set, including the prompt, appearance, tools, skills and MCP servers, keeps following the original, so package updates still reach you. Agents can't change this setting themselves.

#### Fixes

##### The compozy command works after installing the macOS app

Installing CompozyOS from the DMG ran the runtime but left no `compozy` command in your terminal. Only sessions inside the app could find it.

On first launch, the macOS app now links `compozy` into `/usr/local/bin`, or into `~/.local/bin` when that folder is already on your shell's PATH. If it needs administrator rights, it asks first. If the link is missing, use **Install compozy Command…** in the CompozyOS menu to add it. Existing commands are never overwritten. App updates keep the command working without reinstalling, and `compozy --version` now works too.

##### Smaller fixes

- **Claude check-ins work again.** Hidden check-in triage on Claude always-on agents no longer fails with `provider did not confirm config option "model"`. Configuration rejections are now reported as such, instead of counting toward the automatic pause.
- **Faster Agents pages.** The Agents team and agent profiles load their activity in a few grouped reads instead of one scan per member and workspace. Pages that took 2–3 seconds now open without the CPU spike.
- **Bundled agents keep their avatars.** Bundled always-on agents such as the Orchestrator show their generated avatar again instead of a generic icon.
- **Security update.** The web UI uses Next.js 16.4.5, the November 2026 security release.

##### Long data upgrades show progress instead of looking stuck

The first start after upgrading could spend several minutes moving telemetry out of a large database. During that time the desktop app sat on "Preparing CompozyOS" with no sign of progress, and quitting it made the upgrade look failed.

- **Visible progress.** The desktop boot screen now shows "Upgrading data…" with a progress bar, and `compozy daemon bootstrap` reports "Upgrading CompozyOS data (N%)", while that one-time move runs.
- **No false failures.** If the desktop app starts while a CompozyOS runtime is still booting, it now waits for that runtime instead of trying five more launches and reporting a failure.

#### Highlights

##### PR badges no longer use up your GitHub quota

CompozyOS shows a pull request badge on every session and worktree. To keep those badges current, the daemon polled GitHub with your own `gh` login, and that polling alone could exhaust the 5,000 requests/hour that you, `gh` and your agents share. Once the quota ran out, `gh` commands and agents started failing with rate-limit errors.

- **Only live worktrees refresh.** Removed, dismissed and missing worktrees keep showing their last known PR without calling GitHub.
- **Fewer checks for quiet branches.** A branch with no PR is rechecked after 10 minutes, then less often (up to an hour) while it still has none. A merged or closed PR is rechecked every 30 minutes. Open PRs still refresh every minute.
- **No more refresh loops.** A refresh that changes nothing no longer triggers another refresh, and repository capability lookups are cached.
