# Release recovery

For an isolated external failure, use `gh run rerun <run-id> --failed` to retain
successful jobs. A rerun uses the original workflow revision, so it cannot apply
a workflow repair merged later.

After a workflow repair, dispatch `release.yml` from `main` with the original
`release_ref`, `release_version`, and `release_channel`, plus
`desktop_artifacts_run_id=<original-run-id>`. The source run must be complete,
belong to this repository's release workflow, and target the exact release
commit. Each desktop matrix entry reuses its non-expired completed artifact when
available, or builds the missing target. A different source commit, ambiguous
artifact, failed download, or mismatched version stops recovery. Reused packages
still pass the normal packaged-desktop smoke and publication inventory checks.

The Intel macOS production job forces Apple's ticket requests to HTTPS over TCP
because GitHub-hosted Intel runners drop HTTP/3 connections with
`NSURLErrorDomain -1005` / `stapler` error 68. This is the upstream mitigation from
[actions/runner-images#14924](https://github.com/actions/runner-images/issues/14924).
Remove the runner-only UDP/443 rule when that upstream networking defect is
resolved. Signature, notarization, stapling, and Gatekeeper checks remain required.

Keep package publication in the normal release workflow so npm OIDC identity and
provenance remain valid. The SDK first-publication prerequisites are documented
in [SDK release publishing](sdk-release-publishing.md).
