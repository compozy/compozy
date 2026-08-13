# Plan Mode Launch & Acceptance (Claude Code & Codex)

Mechanics to run a worker through an interactive planning phase before a
hands-off implementation: launch the TUI in plan mode, submit the delegation
packet, watch and answer clarifying questions, review the plan, accept it, and
supervise hands-off until the worker reports done. Plan-first is opt-in — the
parent skill's activation gate (explicit user ask or `--plan-mode`) decides
whether this flow runs at all. The parent `herdr-orchestration` SKILL.md
carries the transport verbs and the workers-are-TUIs rule — both TUIs here get
their own named tab and launch through `herdr agent start`, never a headless
runner. All commands are `rtk`-prefixed. Scope everything to the caller
workspace; never change focus.

## Contents

- Shared rules (both TUIs)
- Claude Code: launch in plan mode
- Claude Code: accept the plan
- Codex: launch and enter Plan mode
- Codex: accept the plan
- Watch plan mode & answer questions
- Hands-off monitoring & completion

## Shared rules (both TUIs)

- TUI strings (status lines, acceptance menus, option labels) are
  build-specific. Verify them with `agent read` on first use against the
  installed version (`rtk claude --version` / `rtk codex --version`). If a
  newer build renames a mode or menu, adapt the keys/labels before relying on
  the flow rather than failing mid-loop.
- Submit inside a running TUI per the parent skill's *Sending prompts to
  running TUIs* rule: `agent prompt` submits text plus Enter atomically; a
  trailing `\n` alone never submits.
- Detect menus and questions two ways, semantic first: `agent wait <name>
  --until blocked` fires when the worker stops for input;
  `pane wait-output <pane_id> --match "<menu title>"` pins the exact menu. On
  either, `agent read <name> --source visible` before acting.
- Plan mode is interactive. The worker may ask clarifying questions before
  presenting a plan — answer them with the task context. Do not answer
  questions it did not ask.
- Review the plan before accepting: correctness, scope against the delegation
  packet's file claims, and no out-of-scope edits. Read with
  `agent read <name> --source recent-unwrapped --lines 200` if the plan is
  longer than one screen.
- **After acceptance, do not interrupt.** From the moment the plan is accepted
  until the worker reports done, only observe via read-only `agent read` and
  status waits. Send input only to answer a direct blocking question the
  worker surfaces or to handle a stop condition.

## Claude Code: launch in plan mode

`claude` has a first-class flag: `--permission-mode plan`. It composes with
`--dangerously-skip-permissions` (which stays mandatory for workers), so the
worker plans first and runs unattended after acceptance.

Create the named tab, then pass the packet as launch argv — the session starts
planning immediately and skips the TUI-ready race entirely:

```bash
rtk herdr tab create --workspace "$HERDR_WORKSPACE_ID" --cwd "$PWD" \
  --label "opus: fix loops (plan)" --no-focus

rtk herdr agent start fix-loops --kind claude --pane <root_pane_id> -- \
  --dangerously-skip-permissions --permission-mode plan --model opus \
  "<packet, single block>"
```

To launch without an initial prompt, drop the packet argument and submit with
`agent prompt` once `agent start` returns.

Confirm plan mode via `agent read <name> --source visible`: the input-box
status area must indicate plan mode (current builds show `plan mode on`). If a
session is already running outside plan mode, **shift+tab** cycles the
permission modes (default → accept edits → plan → bypass) — press and re-check
until plan mode shows:

```bash
rtk herdr agent send-keys fix-loops shift+tab
```

## Claude Code: accept the plan

When the worker finishes planning it presents an acceptance menu (current
builds title it `Would you like to proceed?`), typically:

```
> 1. Yes, and auto-accept edits
  2. Yes, and manually approve edits
  3. No, keep planning
```

Catch it with `rtk herdr pane wait-output <pane_id> --match "Would you like to
proceed?"` (or the `blocked` status wait). Review the plan first, then select
the **auto-accept** option — workers run unattended, and manual approval would
stall the tab waiting for per-edit confirmations. Press Enter while the
auto-accept option is highlighted (move with `agent send-keys <name> down` /
`up` if another option is selected):

```bash
rtk herdr agent send-keys fix-loops enter
```

Confirm via `agent read` that plan mode is off and the worker is working. To
reject and keep planning, select the "keep planning" option and send feedback
via `agent prompt`.

## Codex: launch and enter Plan mode

Codex has no CLI flag for plan mode — it is entered inside the TUI with
**shift+tab**. Because of that, do **not** pass the packet as launch argv (the
session would start working in the default mode before the switch). Launch
bare into its named tab, enter Plan mode, then submit the packet:

```bash
rtk herdr tab create --workspace "$HERDR_WORKSPACE_ID" --cwd "$PWD" \
  --label "gpt-5.5: audit auth (plan)" --no-focus

rtk herdr agent start audit-auth --kind codex --pane <root_pane_id> -- \
  --yolo -m gpt-5.5
```

`agent start` returns only once the TUI is ready for input. Then enter Plan
mode:

```bash
rtk herdr agent send-keys audit-auth shift+tab
```

`agent read` again — the bottom status line must read **`Plan mode`**. If it
does not, send shift+tab again and re-check (the key cycles modes). Do not
submit the packet until `Plan mode` is confirmed. Then submit:

```bash
rtk herdr agent prompt audit-auth "<packet, single block>" --wait --timeout 300000
```

## Codex: accept the plan

When the worker presents the menu:

```
Implement this plan?
> 1. Yes, implement this plan          Switch to Default and start coding.
  2. Yes, clear context and implement  Fresh thread.
  3. No, stay in Plan mode             Continue planning with the model.
```

Catch it with `rtk herdr pane wait-output <pane_id> --match "Implement this
plan?"` (or the `blocked` status wait). Review the plan first, then select
option **1** (keeps the investigation context) by pressing Enter while it is
highlighted:

```bash
rtk herdr agent send-keys audit-auth enter
```

Confirm the status line no longer shows `Plan mode` and the worker is
`Working`. To reject, select option 3 and send feedback via `agent prompt`.

## Watch plan mode & answer questions

Block on `agent wait <name> --until blocked` in 3–5 minute check-in intervals
while the worker investigates (per the parent skill's *Track workers* rule —
the timeout is a check-in, never a deadline); on expiry, `agent read <name>
--source visible` and re-enter the wait. If the worker asks a clarifying
question, answer it via `agent prompt` using the delegation-packet context.
Update the state registry: `planning` while investigating, `plan-review` when
the acceptance menu appears, `plan-accepted` once accepted.

## Hands-off monitoring & completion

After acceptance, only observe. Block on the terminal states in 3–5 minute
check-in intervals — implementation, validation, and any QA regularly span
many intervals, and the worker is never cut off because a wait expired:

```bash
rtk herdr agent wait fix-loops --until done --timeout 300000
rtk herdr agent wait fix-loops --until blocked --timeout 300000
```

On expiry, read the screen and re-enter the wait. On `blocked`, read the
screen and answer only a direct blocking question; on a stop condition (errors
it cannot resolve, out-of-scope request), stop and report. When the worker
reports done, verify its claims per the parent skill: re-open cited files,
re-run the key commands yourself, and review the diff before accepting the
patch. Record the disposition, then retire the worker — close its tab per the
parent skill's Retire workers section.
