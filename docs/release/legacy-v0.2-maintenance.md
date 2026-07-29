# Emergency legacy/v0.2 maintenance release

Use this runbook only for a critical v0.2 fix after the v0.3 cut. The release owner executes every
external step. The read-only planner validates identity; the default-branch workflow owns the tag
and publication.

## Preconditions

- The issue is critical and cannot wait for a v0.3 fix.
- The fix and its regression test are committed on `legacy/v0.2` only.
- The legacy branch's full verification and release dry-run are green.
- The candidate is based on the current remote `legacy/v0.2`, with no v0.3 runtime or installer
  backport.
- The proposed unprefixed version advances the v0.2 stable line, for example `0.2.16`.

Record the candidate commit and current npm/Homebrew state before dispatch. Stop if the proposed tag
exists locally or on `origin`.

## Publish

Dispatch the release workflow definition from `main`, but pass the legacy branch as the authoritative
release ref:

```bash
export LEGACY_VERSION="0.2.16"
gh workflow run release.yml --repo compozy/compozy --ref main \
  -f release_ref=legacy/v0.2 \
  -f release_version="$LEGACY_VERSION" \
  -f release_channel=legacy
```

The pinned `releasepr@v0.0.25` planner must resolve `legacy/v0.2` to the checked-out legacy commit,
emit the nearest stable predecessor and exact predecessor-to-commit range, and emit
`github_prerelease=false`, `github_make_latest=true`, `npm_tag=latest`, and
`homebrew_skip_upload=false`. The checked-out branch's GoReleaser configuration owns its v0.2
artifacts; the planner must not rename them or infer identity from the branch name.

Watch the run to completion. If GitHub or npm publication succeeds before a later step fails, stop
for incident review. Never move the annotated tag or reuse the immutable npm version.

## Verify

1. The GitHub release is non-prerelease and latest, and its annotated tag points at the recorded
   legacy commit.
2. `npm view @compozy/cli dist-tags.latest` equals the new v0.2 version. The `beta` tag remains on the
   current v0.3 beta.
3. The `compozy` Homebrew formula points at the new v0.2 archive and checksum. It does not claim a
   v0.3 artifact.
4. The legacy binary installs and reports the expected tag in a clean environment.
5. The v0.3 hosted installer at `compozy.com/install.sh` is unchanged and still serves only the v0.3
   Sigstore contract.
6. The maintenance notice remains at the top of the legacy README.

After the emergency release, restore the Homebrew beta-window disable notice if the publication
tool replaced it. Record all evidence and the incident that justified the legacy release.
