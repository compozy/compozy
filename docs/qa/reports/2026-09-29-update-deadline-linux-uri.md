# Update deadline and Linux URI regression validation

Scope: issues 665 and 666. Evidence is from the working implementation before the enclosing
commit/gate. It does not replace exact-head CI or release artifact smoke.

## Verified

- Complete owning Go suites passed with race detection: `CGO_ENABLED=1 go test -race
  ./internal/update ./internal/cli` (updater 2.064s, CLI 17.602s).
- Linux amd64 and Windows amd64 cross-builds passed for both packages.
- Desktop root Turborepo test/typecheck/lint passed: 139 tests and zero lint warnings/errors.
- An isolated macOS home with the built public CLI canceled a dead-PID `applying` operation before
  its deadline, then `app status` and `update --check` settled expired `applying` operations into
  failed history records without starting a desktop executor. The dev binary's runtime update
  track remained truthfully unsupported; deadline reconciliation still completed.
- An isolated Linux arm64 Docker container ran actual KDE 6 KIO, xdg-utils, desktop-file-validate,
  and kbuildsycoca6. The production registration module generated/registered the current desktop
  entry. KIO launched its executable with a semantically equivalent URL argument; KDE normalizes
  encoded spaces. After deleting the entry, the built Linux CLI still detected the executable
  versioned AppImage path, reported installed/current version, and `app open /settings` launched
  it directly with the exact URI argument. Executable fixtures captured arguments and avoided
  opening a real user desktop. The container was stopped and removed after evidence collection.

The initial minimal Linux container lacked the standard KDE applications menu/session prefix;
KIO could not resolve the application service until plasma-workspace-data and its XDG menu
prefix were provided. No production workaround was introduced for this lab setup. Without
systemd in the container, KDE logged its supported legacy process-launch fallback and still
launched successfully.

## Evidence and remaining scope

Local receipts: `/tmp/compozy-issues-20260929/mac-update-public-proof.log`,
`linux-desktop-public-proof.log`, `linux-desktop-environment.log`, and `linux-desktop-home/`.
Scripts retained beside those receipts make the public-command/KIO proof reproducible.

Linux release smoke already runs on ubuntu-22.04 under Xvfb with transferred AppImage artifacts
and an extraction fallback (`.github/workflows/release.yml`); those historical CI definitions do
not themselves prove the new URI/deleted-shortcut scenario. This lab verifies real Linux/KDE
registration and CLI executable discovery/launch, using an executable fixture at the application
process boundary. A packaged Electron AppImage install/update and a physical KDE session remain
release-artifact QA; they are not claimed by the fixture probe. The cause of the original app's
unexpected exit before handoff remains unobserved.

## Final executable-inspection re-walk

After Go lint remediation, the latest Linux binary repeated the same public/KIO probe with the
AppImage path as a symlink to an executable outside `Applications`. KDE URI dispatch passed,
then desktop-entry deletion followed by public `app status` and `app open /settings` passed.
The new inspection boundary resolved the symlink and verified executable metadata through a
root scoped to the canonical executable directory. No lint suppression was added.

Final receipts: `linux-desktop-final-public-proof.log`, `linux-desktop-final-environment.log`,
`linux-desktop-final-home/`, and `desktop-update-final-source.sha256` in the same receipt folder.
The repeated fixture container was stopped and removed. Focused CLI race checks passed after
these structural edits, including the symlink and missing-executable-permission invariant;
scoped CLI/updater golangci-lint reported `0 issues.` The packaged Electron boundary remains
unclaimed, as in the original probe.
