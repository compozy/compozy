import { loadChangelogReleases } from "../changelog/github-client";
import { compareReleaseTags } from "../changelog/release-markdown";
import { FALLBACK_RELEASE_TAG } from "./release-fallback";

export interface CurrentRelease {
  tag: string;
  source: "github" | "fallback";
}

/**
 * Picks the highest published semver tag from a release list. Prerelease
 * ordering follows semver, so a stable v0.3.0 outranks every v0.3.0-beta.N
 * the moment it ships.
 */
export function selectCurrentReleaseTag(releases: Array<{ version: string }>): string | null {
  let current: string | null = null;
  for (const { version } of releases) {
    // Self-comparison doubles as a semver validity check for the first candidate.
    const comparison =
      current === null
        ? compareReleaseTags(version, version)
        : compareReleaseTags(version, current);
    if (comparison === null) continue;
    if (current === null || comparison > 0) current = version;
  }
  return current;
}

/**
 * Resolves the latest published Compozy release from the GitHub Releases API
 * (cached with the changelog's five-minute revalidation window). Falls back to
 * the committed FALLBACK_RELEASE_TAG when the API is unavailable, so builds
 * stay hermetic and the public install surfaces never break.
 */
export async function getCurrentRelease(): Promise<CurrentRelease> {
  const result = await loadChangelogReleases();
  if (result.status === "ready") {
    const tag = selectCurrentReleaseTag(result.releases);
    if (tag !== null) return { tag, source: "github" };
  }
  return { tag: FALLBACK_RELEASE_TAG, source: "fallback" };
}
