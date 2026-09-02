# BUG-20260902-private-input-shell-leak: Private passphrase appears in terminal history

- **Status:** fixed
- **Impact (user-side):** Trust-Damage
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Marina
- **Journey Step:** J-supervise-agent-terminal, hidden input handoff
- **Scenarios:** ET-terminal-agent-handoff-input; ET-terminal-redaction-boundaries
- **Found:** 2026-09-02 · **Report:** docs/qa/reports/2026-09-01-integrated-terminal-review-r2.md

## Summary

Marina entered a passphrase through the protected terminal input card, but the idle shell displayed
and executed it as a command. The passphrase then remained visible in the terminal and readable from
the durable journal and quote surfaces.

## Reproduction

- **Charter:** CH-terminal-lease-fencing-takeover · **Tour:** Interrupt Tour
- **Environment:** desktop / wifi-fast / en-US / isolated local lab

1. Start a fresh managed `general` session and ask it to open a project terminal, run a readiness
   command, then request a short private passphrase without including the value in its reply.
2. Wait for the protected redacted input card to appear while the terminal is at its ordinary shell
   prompt.
3. Enter a recognisable passphrase through the protected card and submit it.
4. Read the terminal screen, `terminal journal`, and `terminal quote` through their public surfaces.

**Expected:** The runtime refuses to create or answer a redacted request unless the foreground
program is already hiding input; no submitted bytes reach an ordinary shell prompt.
**Actual:** The shell displays and executes the passphrase, and the public journal and quote retain it.

## Evidence

- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/screenshots/marina-private-passphrase-redacted-request.png`
- `/Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-review-r2-20260902-020216-937662-lab/qa-artifacts/qa/screenshots/marina-private-passphrase-terminal-leak.png`
- Session `sess-fa980bf046c5cdbc`, terminal `term-55c8821372eb`, request
  `input-1c8378c78a22b64a`; public journal and quote reads contain the submitted passphrase.

## Fix

- **Root cause:** `WriteRedacted` temporarily disabled terminal echo around a write instead of
  requiring the foreground process to have already entered a hidden-input mode. Fish's line editor
  rendered and submitted the bytes itself, outside the kernel echo flag the runtime changed.
- **Fix commit:** pending remediation batch
- **Regression test:** `internal/terminal/manager_test.go` and `internal/terminal/pty/pty_test.go`

## Verification

- **Retested:** 2026-09-02 in fresh session `sess-0dd350aef0908b61` and recording handoff terminal
  `term-26e57f60c14c`.
- **Result:** Passed. Visible-input requests failed closed; hidden answers and declines completed with
  length-only markers. The raw values were absent from screen, quote, journal, recording, spill
  artifact, daemon log, and the isolated runtime tree.
