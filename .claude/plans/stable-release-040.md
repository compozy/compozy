# Leave beta: seamless v0.4.0 stable, stable-only releases afterwards

Decided 2026-10-23 by Pedro. The v0.3.0-beta.N line was scaffolding for replacing old Compozy.
The first stable is v0.4.0, and every release after it is stable. The beta machinery stays for
manual dispatch only. Legacy v0.2 maintenance ends.

## Success criteria

- Merging the `release/v0.4.0` PR publishes stable v0.4.0 automatically: GitHub latest, npm
  `latest`, Homebrew, and the desktop `channel-stable` feed mirrored to `channel-beta`.
- A desktop or CLI user on any `0.3.0-beta.N` sees v0.4.0 in the update pill and installs it
  in one click. After the update the desktop reads `channel-stable`.
- The v0.4.0 GitHub Release and site page show only the delta since the latest tag, prereleases
  included. They never repeat the beta trail.
- The version comes from a declared `Release-As: 0.4.0` footer. After that, feature releases bump
  the minor and fix-only releases bump the patch.

## Slice A — pr-release (`../pr-release`, commit to main, release v0.0.30)

1. `plan`: every channel picks the nearest reachable release tag, prereleases included
   (`internal/usecase/plan_release.go`). Update docs, skill references, and tests.
2. Release PR version: a `Release-As: X.Y.Z` footer in a commit since the last tag overrides
   the git-cliff bump. It must be strict SemVer and not lower than the computed version. If
   several footers disagree, the release fails.
3. Merge the pr-release release PR to publish v0.0.30.

## Slice B — Compozy PR (managed worktree, branch `feat/stable-release-040`)

1. Release workflow: `RELEASE_AUTO_CHANNEL=stable` publishes stable at the `package.json`
   version. Auto beta keeps working when the variable is `beta`. Drop the `legacy` dispatch
   option and contract asserts. Bump `PR_RELEASE_MODULE` to v0.0.30.
2. `cliff.toml`: set `features_always_bump_minor = true`.
3. Desktop: add the stable build channel and release config, and let `desktoprelease` publish
   stable. Mirror each stable to `channel-beta`. Validate compatibility (`min_app_version`)
   per feed.
4. Updater (`internal/update`): the beta track accepts a stable release >= current from any
   line.
5. npm: `latest` carries stable. CI cannot move the `beta` dist-tag, because trusted publishing
   authorizes `publish` only. Moving it is an optional manual operator step.
6. Docs and copy: install surfaces stop saying beta; fix the `MIGRATION_GUIDE` version labels;
   announce the end of v0.2; add a `highlight` release note.
7. The PR carries the `Release-As: 0.4.0` footer commit.

## Ops (need Pedro)

- Set the `RELEASE_AUTO_CHANNEL` repo variable to `stable` after the PR merges. Close #894
  `release/v0.3.0`. Freeze `legacy/v0.2`.
- QA walks before the merge cut: desktop beta.38 to 0.4.0, an older beta to 0.4.0, npm `@beta`,
  brew and npm from v0.2.15, and a fresh install.
