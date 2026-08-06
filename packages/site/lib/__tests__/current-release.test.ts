import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadChangelogReleases } from "../changelog/github-client";
import type { ChangelogRelease, ChangelogReleasesResult } from "../changelog/types";
import { getCurrentRelease, selectCurrentReleaseTag } from "../release/current-release";
import { FALLBACK_RELEASE_TAG } from "../release/release-fallback";

vi.mock("../changelog/github-client", () => ({
  loadChangelogReleases: vi.fn(),
}));

function release(version: string): ChangelogRelease {
  return {
    version,
    name: null,
    body: "",
    summary: `Published release notes for ${version}.`,
    publishedAt: "2026-08-01T00:00:00Z",
    channel: "beta",
    prerelease: true,
    url: `https://github.com/compozy/compozy/releases/tag/${version}`,
    sourceArchiveUrl: "https://example.invalid/tarball",
    sourceZipUrl: "https://example.invalid/zipball",
    assets: [],
    categories: [],
    headings: [],
    pullRequestNumbers: [],
  };
}

function ready(versions: string[]): ChangelogReleasesResult {
  return { status: "ready", releases: versions.map(release) };
}

describe("current release resolution", () => {
  beforeEach(() => {
    vi.mocked(loadChangelogReleases).mockReset();
  });

  it("selects the highest semver tag regardless of list order", () => {
    const releases = ["v0.3.0-beta.3", "v0.3.0-beta.5", "v0.3.0-beta.4"].map(release);
    expect(selectCurrentReleaseTag(releases)).toBe("v0.3.0-beta.5");
  });

  it("orders prerelease identifiers numerically", () => {
    const releases = ["v0.3.0-beta.9", "v0.3.0-beta.10"].map(release);
    expect(selectCurrentReleaseTag(releases)).toBe("v0.3.0-beta.10");
  });

  it("prefers a stable release over every prerelease of the same version", () => {
    const releases = ["v0.3.0-beta.9", "v0.3.0", "v0.3.0-beta.10"].map(release);
    expect(selectCurrentReleaseTag(releases)).toBe("v0.3.0");
  });

  it("skips tags that do not parse as semver", () => {
    const releases = ["nightly-2026-08-01", "v0.3.0-beta.2"].map(release);
    expect(selectCurrentReleaseTag(releases)).toBe("v0.3.0-beta.2");
  });

  it("returns null when no release carries a valid tag", () => {
    expect(selectCurrentReleaseTag([])).toBeNull();
    expect(selectCurrentReleaseTag([release("not-a-tag")])).toBeNull();
  });

  it("resolves the latest published release from GitHub", async () => {
    vi.mocked(loadChangelogReleases).mockResolvedValue(
      ready(["v0.3.0-beta.4", "v0.3.0-beta.5", "v0.3.0-beta.1"])
    );

    await expect(getCurrentRelease()).resolves.toEqual({
      tag: "v0.3.0-beta.5",
      source: "github",
    });
  });

  it("falls back to the committed tag when GitHub is unavailable", async () => {
    vi.mocked(loadChangelogReleases).mockResolvedValue({
      status: "unavailable",
      error: { kind: "network", message: "GitHub could not be reached" },
    });

    await expect(getCurrentRelease()).resolves.toEqual({
      tag: FALLBACK_RELEASE_TAG,
      source: "fallback",
    });
  });

  it("falls back when GitHub returns no usable release tags", async () => {
    vi.mocked(loadChangelogReleases).mockResolvedValue(ready(["nightly-2026-08-01"]));

    await expect(getCurrentRelease()).resolves.toEqual({
      tag: FALLBACK_RELEASE_TAG,
      source: "fallback",
    });
  });

  it("keeps the committed fallback tag renderable as an explicit pin", () => {
    expect(FALLBACK_RELEASE_TAG).toMatch(/^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/);
  });
});
