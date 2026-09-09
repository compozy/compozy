# herdr-bridge

A [CompozyOS](https://compozy.com) extension that makes your agents show up as
agent rows in [herdr](https://herdr.dev).

herdr tracks agents by reading terminal panes. CompozyOS agents run headless in
a daemon, so herdr cannot see them. This extension bridges the two: it
subscribes to CompozyOS hook events and pushes agent state into herdr's socket
API, giving each agent a row that turns `working` / `idle` / `blocked` in real
time — plus a pane tailing that agent's log, colorized.

```
w1:pX  compozy  working  loop goal reviewer main   $cz_agent=reviewer
```

## Requirements

- CompozyOS `>= 0.3.0-beta.1`
- herdr running (the extension talks to `~/.config/herdr/herdr.sock`)
- `python3` (stdlib only — no dependencies)

If herdr is not running, hooks remain fail-open and existing row mappings are
preserved. Unexpected errors are recorded in the bridge log.

## Install

```bash
compozy extension install AlexandreAkao/herdr-bridge-compozy --allow-unverified --yes
```

The flags are required: community-tier extensions carry no registry-verified
checksum, so the daemon refuses a plain install with
`Extension checksum is unverified`.

Or from a local clone:

```bash
compozy extension install ./herdr-bridge-compozy --allow-unverified --yes
```

To update an existing marketplace installation:

```bash
compozy extension update herdr-bridge --allow-unverified --yes
```

## What you get

One row per agent name, not per session — loops spin up dozens of short-lived
sessions of the same agent, and a row per session would be unreadable. The row
consolidates every live session of that agent: `blocked` wins over `working`
wins over `idle`, so a session ending never clears a sibling that is still
working.

When the last session ends, the bridge closes its pane and removes the row
from its map. An idle session stays open between turns, and another open
session of the same agent keeps the shared pane alive. Repeated stop events
do not recreate a closed pane; a new session opens a new one.

Each row owns a herdr tab running:

```
python3 tail.py <workspace_id>/<agent_name>
```

`tail.py` reads original session events from the local Compozy daemon and
renders them with `colorize.py`. Unlike log summaries, original message text
retains spaces, line breaks, indentation, and content beyond 240 characters.
Printable fragments retain their whitespace; changing session or turn starts a new
line. Terminal control characters are escaped, while tabs and newlines remain intact.

The reader discovers sessions from the bridge map, scoped to the row's
workspace. It initially shows the latest 100 events per session, then polls
once per second using sequence cursors. Reconnection resumes after the last
rendered event. Tool output and infrastructure noise stay filtered. Rows whose hook
omits the workspace resolve each session through the daemon session-owner API
before reading its workspace-scoped events; `no-ws` is never sent as a workspace ID.

After updating, restart existing bridge viewers with
`python3 ~/.compozy/extensions/herdr-bridge/bridge.py --refresh`.
New panes use the updated reader automatically.

## Commands

| Command | What it does |
| --- | --- |
| `bridge.py --status` | Shows the map, prunes dead panes, reconciles loop rows against the daemon |
| `bridge.py --watch-loops` | Monitors loop status until no loop rows remain; normally started by the hook drainer |
| `bridge.py --refresh` | Restarts the tail in existing panes without closing tabs |
| `bridge.py --reset` | Closes every tab the bridge opened and clears the map |

The installed copy lives in `~/.compozy/extensions/herdr-bridge/`.

## Hooks

Every hook runs `hook.sh`, a short-lived shell shim that spools the payload and
returns; `bridge.py --drain` then processes the spool in timestamp order in
the background. Ordering preserves RFC3339 nanoseconds and timezone offsets.
The spool directory is private (0700), and new payloads are owner-only (0600). The daemon dispatches an extension's hooks serially and drops
the queue when the run ends, so the hook entry point has to be faster than the
events arrive.

### Agent rows

| Event | Row becomes |
| --- | --- |
| `session.post_create` | `idle` |
| `turn.start` | `working` |
| `turn.end` | `idle` |
| `permission.request`, `permission.denied`, `task.needs_attention` | `blocked` |
| `permission.resolved` | `working` |
| `session.attention.changed` | `blocked` / `idle`, from the `class` field — see below |
| `session.post_stop`, `agent.stopped`, `agent.crashed` | session leaves the row; the pane closes when the last session ends |

Only `user` and `system` sessions get a row. The daemon's own internals
(`spawned` memory extractors, `dream` curators) are filtered out.

### Loop rows

A loop is not an agent — it is a run that spawns agent sessions, and its events
carry `loop_run_id` / `loop_name` / `generation`, never `agent_name`. So each
loop gets its own row, keyed `loop/<workspace>/<loop_name>`, whose pane follows
`compozy loop events <id> --follow --workspace <workspace_id>`.

| Event | Mode | Row becomes |
| --- | --- | --- |
| `loop.started` | async | `working`, `$cz_run` |
| `loop.generation.pre` | **sync** | `working`, `$cz_gen` |
| `coordinator.decision` | **sync** | `working`, `$cz_node = review.0:loop_action` |
| `loop.generation.post`, `loop.gate.post` | async | `working`, `$cz_gen` |
| `loop.node.terminal` | async | `$cz_node = review.0:succeeded` |
| `loop.terminal` with `status: blocked` | async | **`blocked`** — and it stays until you act |
| `loop.terminal` with `done` / `no-op` / `failed` / `exhausted` / `stalled` / `canceled` | async | pane closes when the last run ends |

The two **sync** hooks are the reliable ones: the daemon waits for them. The
async ones are canceled whenever the emitting step's context ends — on a
zero-agent loop that finishes in 200 ms, nearly all of them; on a real loop,
mostly `loop.terminal`, which fires as the run's context closes.

After draining the spool, one detached process monitors loop rows through the
Compozy daemon's local briefing API, with five seconds between passes. This
recovers missing terminal hooks without needing another event or a manual
status check. It stops when no loop rows remain. A file lock keeps only one
monitor active, and daemon queries do not hold the map lock used by hooks.

Only confirmed terminal outcomes close panes. Queued, watching, paused,
approval-waiting, and blocked runs remain visible. Unknown statuses or an
unavailable daemon leave the pane intact; failed closes are retried.

For rows left open before upgrading, `bridge.py --status` also runs this
reconciliation once. Normal hook delivery starts the automatic monitor.

### Rows are per agent, and they self-heal

The map key is `(workspace_id, agent_name)`, so the same agent running in two
workspaces gets two rows instead of fighting over one.

A row's state is consolidated from every live session of that agent, so a
session ending never clears a sibling that is still working. Sessions with no
event for 30 minutes are dropped from that calculation: a lost `turn.end` — a
crash, a restarted daemon — would otherwise pin the row at `working` forever,
because a hook only runs when there is an event.

This timeout only affects the displayed activity. Open sessions remain tracked
until a terminal event arrives, so a quiet sibling does not lose its pane.
If closing the pane fails, the map entry stays available for a subsequent stop
event to retry.

`--status` prunes only confirmed missing panes. Unavailable sockets and RPC
errors preserve mappings so subsequent hooks do not create duplicate tabs.
State and spool files live under `${XDG_STATE_HOME:-$HOME/.local/state}/herdr-bridge`.

### Attention, and payloads not yet observed

`session.attention.changed` carries `from` / `to` (the session's activity) and
`class` — *why* it wants you. Observed at runtime: `none` (nothing) and
`finished` (it ended; informational). The daemon also knows `clarify`, the live
question behind `compozy session clarify`. Anything outside the benign set marks
the row `blocked` and logs the class, so an unknown reason errs toward being
visible rather than silent.

Unknown attention payload shapes are logged without changing row state.

## Development validation

From the repository root:

```bash
python3 -B -m unittest discover -s catalog/packages/herdr-bridge/tests -v
go run ./cmd/compozy-catalog package ./catalog/packages/herdr-bridge ./catalog/artifacts/herdr-bridge-v0.3.3.tar.gz
go run ./cmd/compozy-catalog digest ./catalog/artifacts/herdr-bridge-v0.3.3.tar.gz
go run ./cmd/compozy-catalog validate ./catalog
```

Update the catalog digest after packaging. Validation installs the local artifact
through the production installer; the Python suite covers runtime regressions.

## License

MIT
