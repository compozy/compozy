# J-publish-compozy-beta: Publish one beta through the guarded release workflow

```mermaid
flowchart TD
    A[Entry: approved candidate on main] --> B[Dispatch release workflow with exact version and beta channel]
    B --> C[Planner and preflight confirm the candidate]
    C --> D0[Stage production release: annotated tag, GoReleaser draft GitHub release, both npm packages published, Homebrew formula published]
    D0 --> C1{Desktop signing material complete for every platform?}
    C1 -->|incomplete| X3[Abandon: desktop lane asserts signing material before building and hard-fails - nothing reaches the update feed]
    C1 -->|complete| C2[Build, sign, and notarize the three desktop platforms]
    C2 -->|one platform fails| X4[Abandon: no feed published and the GitHub release stays draft - operator gets explicit evidence, never a silent platform drop]
    C2 --> C3[Build and sign canonical feeds, publish exact immutable payloads, then no-cache latest.json and runtime.json, and re-verify from the public origin]
    C3 --> D[Sole finalizer publishes the GitHub draft last - it never touches npm or Homebrew]
    D --> E{Read published npm channel policy}
    E -->|requested dist-tag is stale or absent| F[Wait and read the registry again within the deadline]
    F --> E
    E -->|terminal query or policy error| X1[Abandon: stop for incident review without republishing]
    E -->|both packages converge and latest is unchanged| G[True end: production job is green and public channels name the exact beta]
    F -.->|deadline expires| X2[Abandon: preserve the last observation for incident review]
```

```yaml
journey:
  id: J-publish-compozy-beta
  name: "Publish one guarded Compozy beta"
  value_statement: "A release administrator can publish one immutable beta and distinguish temporary registry propagation from a real channel-policy failure without republishing it."
  personas: [Dora]
  entry_points:
    - url: ".github/workflows/release.yml"
      origin: direct
    - url: "GitHub Actions / Release / Run workflow"
      origin: in-app-nav
  actions:
    - step: 1
      verb: "Dispatch the approved commit, version, and beta channel"
      expected_observable: "The release plan and preflight name the exact checked-out candidate before publication starts"
    - step: 2
      verb: "Observe the staged production release"
      expected_observable: "GoReleaser stages a draft GitHub release while both npm packages and the Homebrew formula publish for the same version — the draft is not public yet"
    - step: 3
      verb: "Wait for the public npm channel policy"
      expected_observable: "Stale or missing dist-tags are re-read within a fixed deadline, while terminal query and policy errors stop immediately"
    - step: 4
      verb: "Observe the desktop release lane (added 2026-08-10, desktop-app workstream)"
      expected_observable: "Incomplete signing material hard-fails before desktop builds; payloads publish before the no-cache manifests and are re-verified from the public origin; a partial platform failure publishes no feed and leaves the GitHub release a draft with explicit evidence; the sole finalizer publishes only the GitHub draft (npm and Homebrew were already published at staging); the stable prefix stays blocked"
    - step: 5
      verb: "Confirm the release from a fresh public read"
      expected_observable: "Both packages expose the requested beta version, latest remains unchanged, and the production job is green"
  goal:
    observable: "One production run publishes the exact candidate and closes only after every public channel policy is observable"
    side_effects: [git-tags-created, github-release-published, npm-packages-published]
  true_end_state: "The production workflow is green, the GitHub release is a prerelease, both npm beta dist-tags name the exact version, and neither immutable package needs to be republished."
  exit:
    natural: "Hand the published version to the post-release install and provenance checks."
  abandonment:
    - at_step: 3
      how: "The registry query fails, a beta moves latest, malformed policy data is returned, or the readiness deadline expires."
      resume: "Stop for incident review with the last observation; never overwrite tags or republish an immutable npm version."
  crosses: [release-workflow, github-releases, sigstore, npm-cli-package, npm-extension-sdk, desktop-release-lane, desktop-update-feeds, minisign-feed-key-custody]
```
