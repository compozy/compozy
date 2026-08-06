/**
 * Safety-net release tag used only when the GitHub Releases API cannot be
 * reached (offline builds, rate limits, missing token). It must always name a
 * published release, but it is allowed to lag behind: the live resolver in
 * `current-release.ts` is the primary source and refreshes every five minutes.
 * Update opportunistically; nothing in the release process depends on bumping
 * this value.
 */
export const FALLBACK_RELEASE_TAG = "v0.3.0-beta.5";
