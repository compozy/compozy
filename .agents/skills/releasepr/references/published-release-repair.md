# Correct a published release

Use this branch when a tagged release already exists and its notes or changelog page are wrong.

1. Pin the published tag and commit, its actual predecessor, the existing release body, and the website's data source. Read the consuming workflow and relevant channel variables before predicting what a future merge will publish; template prefixes and a release PR title do not establish the channel.
2. Trace each proposed persistence path through the next release. Active `.release-notes/*.md` entries feed a future release; use them only for changes that belong there. Editing generated `RELEASE_BODY.md` alone is not durable when the workflow regenerates it. Identify the canonical historical source, if the consumer has one, and verify its regeneration behavior.
3. Reconstruct missing notes from the pinned released range and verify each claim against the merged changes. Prepare one replacement body for the existing GitHub Release and update the consumer's historical source when applicable. Publish the correction within the user's authorization; preserve the tag, release identity, and assets.
4. Diagnose a missing website page separately from missing release notes. Verify the actual body/data source and rendered page: a successful HTTP status can be a login page or stale cache. A website code fix follows its own implementation and verification gates.
5. Read back the corrected release and the public page, then verify that the next release's note selection does not repeat these historical changes. Record the target tag, changed sources, and this evidence.

If the historical source or regeneration path cannot be established, report that specific uncertainty before claiming the correction will survive the next release. Do not republish or retag an existing version to repair its text.
