# BUG-20260807-npm-dist-tag-readiness: A published beta can fail while npm channel metadata propagates

- **Status:** open
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P1
- **Persona Affected:** Dora
- **Journey Step:** J-publish-compozy-beta, step 3
- **Scenarios:** REL-published-npm-channel-readiness
- **Found:** 2026-08-07 · **Report:** docs/qa/reports/2026-08-07-release-cosign-v3.md
- **Origin:** n/a

## Summary

Dora saw a failed production release after GitHub, the CLI package, and the extension SDK had all
published successfully. The failure incorrectly treated an npm dist-tag response that was still on
the previous beta as a terminal channel-policy violation.

## Reproduction

- **Charter:** CH-published-npm-channel-readiness · **Tour:** Network Tour
- **Environment:** GitHub-hosted Ubuntu runner / public npm registry / en-US

1. Dispatch production beta.6 from the approved release workflow.
2. Wait for GoReleaser and the extension SDK publisher to report successful publication.
3. Observe the immediate `npm view` check of both package dist-tags.

**Expected:** The workflow tolerates bounded registry propagation, then closes when both beta tags
name the released version; true query and channel-policy failures still stop immediately.

**Actual:** The first SDK dist-tag read still named beta.5, so the workflow failed about two seconds
after npm accepted beta.6. A later public read showed beta.6 without any republish.

## Evidence

- https://github.com/compozy/compozy/actions/runs/31207649258/job/92962915716
- The SDK publish completed with `@compozy/extension-sdk@0.3.0-beta.6`; the following verifier read
  `beta: 0.3.0-beta.5`; a later registry read returned `beta: 0.3.0-beta.6`.

## Fix

- **Root cause:** the workflow made one immediate dist-tag query even though npm registry metadata is
  eventually consistent and publicly cacheable.
- **Fix commit:** pending
- **Regression test:** `internal/config/release_config_test.go::TestNPMReleasePolicyWaitsForRegistryReadiness`

## Verification

- **Retested:** pending the next hosted production release under the same Dora journey.
- **Result:** local regressions cover convergence, bounded timeout evidence, and immediate terminal
  policy rejection; hosted verification remains required.
