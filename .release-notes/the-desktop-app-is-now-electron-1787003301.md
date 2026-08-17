---
title: The desktop app is now Electron
type: breaking
---

The Tauri/Rust desktop host is replaced by an Electron shell with a narrow preload boundary, and updating became one durable daemon-owned operation exposed identically through the CLI, HTTP, UDS, Settings, and the menubar. The app provisions the bundled daemon from an empty home, or attaches to a compatible daemon that is already running without taking ownership of it, keeping single-instance focus, deep links, safe navigation boundaries, page zoom, window geometry recovery, diagnostics, logs, and the owned-versus-attached quit contract. (#424)

- Runtime and App are separate update tracks with operation progress, holder-aware blocked state, staged-next-launch state, apply and cancel actions, and truthful absence when a track is unsupported.
- A keyboard-accessible menubar indicator appears only when an update is actionable and navigates to Settings. The renderer holds no desktop-only update authority, and the SPA behaves the same in a browser and in the app.
- Desktop artifacts are planned, inventoried, and channel-checked as one release authority, with notarization and signing input checks and packaged smokes provisioned from empty isolated homes. On macOS the ZIP and on Linux the AppImage are the updater artifacts; DMG and DEB are install artifacts only.

Migration notes: this is a hard cut with no compatibility bridge. The Tauri runtime, commands, permissions, capabilities, fixtures, generated bindings, Cargo dependencies, build configuration, scripts, config keys, docs, and tests are deleted rather than deprecated. Install the app from the artifacts published with this release. The installed-app update walk from one beta to the next was not verified for this build, so the App track must be proven by a release owner across a fresh beta pair before it is treated as delivered.
