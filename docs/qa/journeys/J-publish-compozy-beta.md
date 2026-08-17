# J-publish-compozy-beta: Publish one beta through the guarded release workflow

```mermaid
flowchart TD
    A[Entry: approved candidate on main] --> B[Dispatch release workflow with exact version and beta channel]
    B --> C[Planner and preflight confirm the candidate]
    C --> C0[Build archives and prove every direct-update artifact satisfies the runtime artifact policy]
    C0 --> D0[Stage production release: annotated tag and GoReleaser draft GitHub release; package-manager publication remains disabled]
    C0 -.->|archive is incompatible| X0[Abandon: stop before publication with measured archive or binary details]
    D0 --> C1{Desktop signing material complete for every platform?}
    C1 -->|incomplete| X3[Abandon: desktop lane asserts signing material before building and hard-fails - nothing reaches the update feed]
    C1 -->|complete| C2[Build, sign, and notarize the three desktop platforms]
    C2 -->|one platform fails| X4[Abandon: no feed published and the GitHub release stays draft - operator gets explicit evidence, never a silent platform drop]
    C2 --> C3[Build exact platform packages, publish immutable release assets, then CAS one audited channel commit]
    C3 --> D[Sole finalizer publishes the GitHub release]
    D --> D1[Install the prepared CLI package on macOS and Linux; postinstall downloads the public GitHub archives]
    D1 -->|archive download or binary check fails| X5[Abandon: keep npm unpublished and preserve the public GitHub release for repair]
    D1 --> D2[Publish the verified CLI package, extension SDK, and eligible Homebrew formula]
    D2 --> E{Read published npm channel policy}
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
      expected_observable: "GoReleaser validates each direct-update archive and stages a draft GitHub release while its npm publisher remains disabled"
    - step: 3
      verb: "Wait for the public npm channel policy"
      expected_observable: "Stale or missing dist-tags are re-read within a fixed deadline, while terminal query and policy errors stop immediately"
    - step: 4
      verb: "Observe the desktop release lane (added 2026-08-10, desktop-app workstream)"
      expected_observable: "Incomplete signing material hard-fails before desktop builds; every immutable payload and signed compatibility catalog verifies before one audited channel-beta commit and ref CAS; a partial platform failure leaves the channel ref unchanged and the GitHub release draft with explicit evidence; package managers remain unpublished"
    - step: 5
      verb: "Confirm the release from a fresh public read"
      expected_observable: "A prepared CLI package installs against public GitHub archives on macOS and Linux before npm publication; both packages then expose the requested beta version, latest remains unchanged, and the production job is green"
  goal:
    observable: "One production run publishes the exact candidate and closes only after every public channel policy is observable"
    side_effects: [git-tags-created, github-release-published, npm-packages-published]
  true_end_state: "The production workflow is green, the GitHub release is a prerelease, both npm beta dist-tags name the exact version, and neither immutable package needs to be republished."
  exit:
    natural: "Hand the published version to the post-release install and provenance checks."
  abandonment:
    - at_step: 2
      how: "A built archive or extracted binary exceeds the runtime-owned artifact policy."
      resume: "Reduce the artifact or deliberately revise the single shared policy before rerunning the release; publication remains stopped."
    - at_step: 3
      how: "The registry query fails, a beta moves latest, malformed policy data is returned, or the readiness deadline expires."
      resume: "Stop for incident review with the last observation; never overwrite tags or republish an immutable npm version."
  crosses: [release-workflow, github-releases, sigstore, npm-cli-package, npm-extension-sdk, desktop-release-lane, desktop-channel-branch]
```
