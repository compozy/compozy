# Plan Mode Launch & Acceptance (Claude Code & Codex)

Mechanics to run a worker through an interactive planning phase before a
hands-off implementation: launch the TUI in plan mode, submit the delegation
packet, watch and answer clarifying questions, review the plan, accept it, and
supervise hands-off until the worker reports done. Load `cmux`,
`cmux-workspace`, and the parent `cmux-orchestration` SKILL.md for the
underlying pane verbs. All commands are `rtk`-prefixed. Scope everything to the
caller workspace; never change focus.

## Contents

- When to require plan-first
- Shared rules (both TUIs)
- Claude Code: launch in plan mode
- Claude Code: accept the plan
- Codex: launch and enter Plan mode
- Codex: accept the plan
- Watch plan mode & answer questions
- Hands-off monitoring & completion

## When to require plan-first

Require plan mode when the slice needs investigation before edits: root-cause
fixes, multi-file or cross-package changes, unfamiliar code, or anything where
the controller wants to review intent before the worker touches files. Skip
plan mode for trivially bounded slices (evidence gathering, read-only
research, single-file mechanical edits) — the extra round trip only adds
latency there.

## Shared rules (both TUIs)

- TUI strings (status lines, acceptance menus, option labels) are
  build-specific. Verify them with `read-screen` on first use against the
  installed version (`rtk claude --version` / `rtk codex --version`). If a
  newer build renames a mode or menu, adapt the keys/labels before relying on
  the flow rather than failing mid-loop.
- Inside a running TUI a trailing `\n` is a soft newline and does NOT submit.
  Send the packet as text, then `send-key enter`, then confirm with
  `read-screen` (input line empty + working indicator active). Treat an
  unconfirmed send as not delivered.
- Plan mode is interactive. The worker may ask clarifying questions before
  presenting a plan — watch for them and answer with the task context. Do not
  answer questions it did not ask.
- Review the plan before accepting: correctness, scope against the delegation
  packet's file claims, and no out-of-scope edits. Scroll the transcript if
  the plan is longer than one screen.
- **After acceptance, do not interrupt.** From the moment the plan is accepted
  until the worker reports done, only observe via read-only `read-screen`.
  Send input only to answer a direct blocking question the worker surfaces or
  to handle a stop condition.

## Claude Code: launch in plan mode

`claude` has a first-class flag: `--permission-mode plan`. It composes with
`--dangerously-skip-permissions` (which stays mandatory for workers), so the
worker plans first and runs unattended after acceptance.

Prefer passing the packet as the CLI argument at launch — the session starts
planning immediately and you skip the TUI-ready race entirely:

```bash
rtk cmux send --surface surface:<claude> \
  "rtk claude --dangerously-skip-permissions --permission-mode plan --model opus \"<packet, single block>\"\n"
```

To launch without an initial prompt, drop the packet argument, wait for the
TUI, and submit per the shared rules (send text → `send-key enter` → confirm).

Confirm plan mode via `read-screen`: the input-box status area must indicate
plan mode (current builds show `plan mode on`). If a session is already
running outside plan mode, **shift+tab** cycles the permission modes
(default → accept edits → plan → bypass) — press and re-check until plan mode
shows:

```bash
rtk cmux send-key --surface surface:<claude> shift+tab
```

## Claude Code: accept the plan

When the worker finishes planning it presents an acceptance menu (current
builds title it `Would you like to proceed?`), typically:

```
> 1. Yes, and auto-accept edits
  2. Yes, and manually approve edits
  3. No, keep planning
```

Review the plan first, then select the **auto-accept** option — workers run
unattended, and manual approval would stall the pane waiting for per-edit
confirmations. Press Enter while the auto-accept option is highlighted (use
arrow keys via `send-key down`/`send-key up` if another option is selected):

```bash
rtk cmux send-key --surface surface:<claude> enter
```

Confirm via `read-screen` that plan mode is off and the worker is working. To
reject and keep planning, select the "keep planning" option and send feedback
as a follow-up prompt.

## Codex: launch and enter Plan mode

Codex has no CLI flag for plan mode — it is entered inside the TUI with
**shift+tab**. Because of that, do **not** pass the packet as a CLI argument
(the session would start working in the default mode before you can switch).
Launch bare, enter Plan mode, then submit the packet.

The fresh pane is a plain shell, so a trailing `\n` submits the launch line:

```bash
rtk cmux send --surface surface:<codex> "rtk codex --yolo -m gpt-5.5 -C \"$PWD\"\n"
```

Wait ~6s, then `read-screen` to confirm the TUI is up (banner shows the model
+ permissions + the input box). Then enter Plan mode:

```bash
rtk cmux send-key --surface surface:<codex> shift+tab
```

`read-screen` again — the bottom status line must read **`Plan mode`**. If it
does not, send shift+tab again and re-check (the key cycles modes). Do not
submit the packet until `Plan mode` is confirmed.

Submit the packet per the shared rules:

```bash
rtk cmux send --surface surface:<codex> "<packet, single block>"
rtk cmux send-key --surface surface:<codex> enter
rtk cmux read-screen --surface surface:<codex>   # input line empty + working indicator active
```

## Codex: accept the plan

When the worker presents the menu:

```
Implement this plan?
> 1. Yes, implement this plan          Switch to Default and start coding.
  2. Yes, clear context and implement  Fresh thread.
  3. No, stay in Plan mode             Continue planning with the model.
```

Review the plan first, then select option **1** (keeps the investigation
context) by pressing Enter while it is highlighted:

```bash
rtk cmux send-key --surface surface:<codex> enter
```

Confirm the status line no longer shows `Plan mode` and the worker is
`Working`. To reject, select option 3 and send feedback as a follow-up prompt.

## Watch plan mode & answer questions

Poll the worker surface read-only at calm intervals (`read-screen`, tail the
last ~30 lines) while the status line still shows plan mode. If the worker
asks a clarifying question, answer it the same way (send text →
`send-key enter`) using the delegation-packet context. Update the state
registry: `planning` while investigating, `plan-review` when the acceptance
menu appears, `plan-accepted` once accepted.

## Hands-off monitoring & completion

After acceptance, only observe. Poll read-only at unhurried intervals —
implementation, validation, and any QA each take minutes. Watch for:

- a direct blocking question (answer it),
- a stop condition (errors it cannot resolve, out-of-scope request),
- the final completion report (worker idle, summary printed).

Do not send input for anything else. When the worker reports done, verify its
claims per the parent skill: re-open cited files, re-run the key commands
yourself, and review the diff before accepting the patch.
