# Unicode prompt redraw verification

Scope: issue #629, the terminal security filter and its model-facing read path.

## Contract and cause

The output filter classified the `0x9d` continuation byte inside U+276F (`e2 9d af`)
as a standalone C1 OSC introducer. A colored prompt was truncated after `e2`, and
subsequent single-character redraws remained buffered as an unterminated control.
The ASCII control prompt delivered each character. The canonical filter regression
fails on the original implementation for exactly this controlled pair.

The same byte-based scanning also mistook UTF-8 continuation bytes for DCS starts
and string terminators. Both stream and model-facing filtering now scan rune
boundaries. Stream parsing retains only an incomplete UTF-8 suffix between reads,
flushes it at EOF, and keeps existing control-size bounds and fail-closed behavior.
The downstream VT UTF-8 carry, display-width tables, ANSI handling, and ACK flow
control remain unchanged.

The browser's xterm input handler decodes UTF-8 to code points before parsing C1
controls. Therefore U+0090/U+009D/U+009C must still be recognized when UTF-8 encoded;
only continuation bytes inside other characters are ignored. The canonical suite
covers both raw and UTF-8 encoded controls, including split discard terminators.

This evidence establishes a filtering defect. It does not validate the reporter's
display-width hypothesis, clipboard observation, or distinguish every condition
under which the original plain-output experiment succeeded.

## Verification matrix

| Journey / owning suite | Result | Evidence |
| --- | --- | --- |
| Filter preservation across every tested chunk size; input/output, raw C1, ANSI, oversized controls, EOF and model reads | Pass | `CGO_ENABLED=1 go test -race ./internal/terminal/... -count=1`; original regression fails, corrected suite passes |
| Fresh real zsh, ASCII versus U+276F, shell integration disabled and daemon restarted; separate browser keys before Enter | Pending | Canonical browser `terminal.spec.ts`, E2E-020 |
| CLI quote parity, reconnect, backspace and subsequent command execution | Pending | E2E-020 |
| Required local gates and current-head PR CI | Pending | Recorded at delivery |

The browser fixtures allocate isolated daemon/operator homes, workspaces and sockets,
and dispose their processes after each case. No operator daemon is restarted.
Linux and macOS results must be reported separately; a passing macOS replay alone
does not establish Linux parity. Other terminal issues (#626 and #628) are excluded.

Delivery override: the operator explicitly moved all final gates and browser/runtime
validation to existing GitHub Actions workflows. The in-flight local gate was canceled;
the checks recorded above completed before that override. No local gate completion or
local browser replay is claimed. Per-command `HUSKY=0` skips gate hooks for delivery
under this authorization without changing hook configuration.
