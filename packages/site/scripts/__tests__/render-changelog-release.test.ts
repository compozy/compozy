import { describe, expect, it } from "vitest";

import { compareUrl, releaseStatus } from "../render-changelog-release";

describe("releaseStatus", () => {
  it("Should report the prerelease channel even when the release carries breaking changes", () => {
    expect(releaseStatus("v0.3.0-beta.1", ["drop legacy runtime"])).toBe("beta");
    expect(releaseStatus("v0.3.0-rc.1", ["drop legacy runtime"])).toBe("alpha");
  });

  it("Should report breaking only for final releases with breaking changes", () => {
    expect(releaseStatus("v0.3.0", ["drop legacy runtime"])).toBe("breaking");
    expect(releaseStatus("v1.2.0", ["drop legacy runtime"])).toBe("breaking");
  });

  it("Should report stable for final 1.x+ releases and alpha for final 0.x releases", () => {
    expect(releaseStatus("v1.2.3", [])).toBe("stable");
    expect(releaseStatus("v0.3.0", [])).toBe("alpha");
  });
});

describe("compareUrl", () => {
  it("Should compare against the previous real tag when one exists", () => {
    expect(
      compareUrl(
        { version: "v0.3.0-beta.1", context: [], githubOwner: "compozy", githubRepo: "compozy" },
        "v0.2.15",
        "v0.3.0-beta.1"
      )
    ).toBe("https://github.com/compozy/compozy/compare/v0.2.15...v0.3.0-beta.1");
  });

  it("Should fall back to the release tag page when no previous tag exists", () => {
    expect(
      compareUrl(
        { version: "v0.3.0-beta.1", context: [], githubOwner: "compozy", githubRepo: "compozy" },
        undefined,
        "v0.3.0-beta.1"
      )
    ).toBe("https://github.com/compozy/compozy/releases/tag/v0.3.0-beta.1");
  });
});
