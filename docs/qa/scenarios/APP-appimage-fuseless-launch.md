---
id: APP-appimage-fuseless-launch
area: APP
title: Launch the Linux AppImage on a host without libfuse2
persona: Lea
journey: J-desktop-first-run
expected: The released x64 AppImage launches on a distribution that ships only FUSE 3 and has no libfuse2 package installed, mounting and opening the app without `--appimage-extract-and-run`; the artifact keeps its AppImage type 2 magic so the desktop still resolves `application/vnd.appimage`, and the electron-updater feed keeps its block map and `latest-linux.yml`.
entry_points: CompozyOS-<version>-linux-x64.AppImage on a FUSE 3 only host
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/reports/2026-09-03-appimage-static-runtime.md
last_report: docs/qa/reports/2026-09-03-appimage-static-runtime.md
overlaps: APP-install-first-run-provision; APP-app-auto-update
---

qa-impact: 2026-09-03 the desktop build pins `toolsets.appimage` to the static AppImage runtime
(AppImage/type2-runtime 20251108) instead of the electron-builder legacy toolset, which linked
against libfuse2 dynamically. Hosts that ship only FUSE 3 — current Arch, Ubuntu 24.04 and later,
recent Fedora — failed at mount with "AppImages require FUSE to run" before this change.

2026-09-03 walk: passed on Arch Linux with `fuse3` installed, `fuse2` absent, and only
`/usr/bin/fusermount3` present. The packaged AppImage mounted and opened the app twice with no
extraction fallback, the embedded runtime is `static-pie linked` with no libfuse2 dependency, and
bytes 8-10 still carry the `41 49 02` type 2 magic so the desktop resolves
`application/vnd.appimage`. The electron-updater feed kept its block map and `latest-linux.yml`; the
update walk itself stays with APP-app-auto-update.
