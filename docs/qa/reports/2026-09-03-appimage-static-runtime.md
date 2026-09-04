# QA Run Report — 2026-09-03 — AppImage static runtime on a FUSE 3 only host

- **Scope:** Linux AppImage launch after pinning `toolsets.appimage` to the static AppImage runtime.
- **Cadence tier:** targeted
- **Build:** local package from `b7667d8f` plus the pending `toolsets.appimage` change · local artifact
  `CompozyOS-0.3.0-linux-x86_64.AppImage` · **SHA-512 (electron-builder feed):**
  `238NMJ4L8okELjJ/FPa9O7eorO21oG/dsDCcCGugcoUC+wJHqeia9BcR52x7/he0CXO+WzX+IyeyBHcJGPD1pA==`
- **Environment:** Arch Linux 7.1.9, `fuse3` 3.18.2 installed, `fuse2` absent, `/usr/bin/fusermount`
  absent and only `/usr/bin/fusermount3` present; emptied operator `~/.compozy`; default socket and
  HTTP port
- **Status:** closed — blocked pending a repository-compliant isolated re-walk

The root builder config expands x64 as `x86_64` for AppImage artifacts. The release config overrides
that local name with `CompozyOS-<version>-linux-x64.AppImage`. This run tested the local artifact; it
did not test a file emitted by the release-config path or renamed into the release shape.

## Session

| Charter | Scenario | Persona | Tour | Status |
|---|---|---|---|---|
| — | APP-appimage-fuseless-launch | Lea | Feature Tour | Blocked (needs human verify) |
| — | APP-install-first-run-provision (Linux leg) | Lea | Feature Tour | Blocked (needs human verify) |

## Results

### The defect

The released AppImage failed to start on this host with "AppImages require FUSE to run". The
electron-builder 26.15.3 default toolset (`toolsets.appimage` unset, equivalent to `"0.0.0"`) embeds
a runtime that links libfuse2 dynamically, so any distribution that ships only FUSE 3 cannot mount
the image.

### The build

Pinning `toolsets.appimage: "1.0.3"` moves packaging to `buildStaticRuntimeAppImage`. The build log
records the new toolset and an unchanged updater path:

```
• building target=AppImage arch=x64
• downloaded label=appimage-tools-runtime-20251108.tar.gz progress=100%
• building embedded block map
```

`latest-linux.yml` and the block map were produced as before, so the electron-updater feed keeps its
shape.

### Artifact verification

| Check | Result |
|---|---|
| Embedded runtime | `ELF 64-bit LSB pie executable, static-pie linked` — AppImage/type2-runtime 20251108 |
| Dynamic dependencies | `not a dynamic executable` — no libfuse2 requirement |
| `--appimage-offset` | `944632` |
| AppImage type 2 magic (bytes 8–10) | `41 49 02` — desktop MIME detection still resolves `application/vnd.appimage` |
| `--appimage-mount` | mounted at `/tmp/.mount_CompozEEdmib` as `type fuse.CompozyOS-0.3.0-linux-x86_64.AppImage` |

### Observed launch and first-run provisioning

The operator launched the local AppImage twice on the FUSE 3 only host. Both runs mounted and opened
the app with no `--appimage-extract-and-run` fallback and no libfuse2 present. From an empty home the
run verified the bundled runtime digest, provisioned `~/.compozy/bin/compozy`, and started the daemon:

```
Status:  running          PID: 265462
Socket:  ~/.compozy/daemon.sock       HTTP: localhost:2123
compozy 0.3.0
```

The process table held exactly one daemon plus its three extension providers. `Health: degraded`
reflects this host carrying no provider CLI on `PATH`. The run did not reach the healthy full-product
end state required by APP-install-first-run-provision.

## Build caveat for this run

The local package was built by calling `desktop/scripts/build-runtime.ts` directly, which runs a
plain `go build` and therefore skips the `webBuild` and `webAssetsCheck` hooks that the release
pipeline runs first. The binary embeds `compozy-web-assets v0.0.203`, pinned on 2026-09-01, while
HEAD `b7667d8f` changed 440 files across `web/`, `internal/api`, and `internal/windowmanager` on
2026-09-02. Product-UI behavior on this build is therefore out of scope for this run; the launch,
mount, provisioning, and daemon-start claims above stand on their own evidence.

## Not covered

- The electron-updater walk across two AppImage versions stays with APP-app-auto-update; this run
  changed the runtime, not the update transport.
- The macOS leg of APP-install-first-run-provision is untouched by this change and keeps its prior
  evidence.

## Teardown

The operator reported stopping the app with SIGTERM, stopping the daemon with `compozy daemon stop`,
releasing the mount, and removing the temporary paths. The run was not bootstrapped through the
repository QA envelope and retained no `teardown.json` with `"clean": true`. This prose-only cleanup
does not satisfy the mandatory teardown evidence contract.

## Human verification needed

- Build through the release-config path and use the emitted
  `CompozyOS-<version>-linux-x64.AppImage` without renaming it.
- In a fresh isolated Linux QA envelope, use unique `COMPOZY_HOME`, HTTP port, and UDS path; install
  FUSE 3, confirm `libfuse.so.2` is absent, then launch without `APPIMAGE_EXTRACT_AND_RUN`.
- Reach the product window, confirm healthy runtime state through an independent read, run the
  extraction fallback separately, and retain the manifest teardown output with
  `teardown.json` reporting `"clean": true` and no survivors.

## Final status

- **Coverage:** static runtime, type 2 magic, updater files, mount, and provisioning were observed on
  a local package; release-shaped artifact identity, isolated full-product completion, and canonical
  teardown remain unverified.
- **Verdict:** blocked — the observations support the packaging change but cannot settle either QA
  scenario under the current evidence contract.
