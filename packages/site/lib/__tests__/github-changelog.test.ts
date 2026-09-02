import { afterEach, describe, expect, it, vi } from "vitest";
import {
  loadChangelogReleaseByTag,
  loadChangelogReleases,
  loadReleasePullRequests,
} from "../changelog/github-client";

const releaseFixture = {
  tag_name: "v0.3.0-beta.3",
  name: "CompozyOS 0.3.0-beta.3 — beta",
  body: [
    "## v0.3.0-beta.3",
    "",
    "### Runtime",
    "",
    "- Preserve complete release notes in #123.",
    "",
    "### Release Notes",
    "",
    "This release keeps the full GitHub-authored narrative and contribution context.",
  ].join("\n"),
  html_url: "https://github.com/compozy/compozy/releases/tag/v0.3.0-beta.3",
  published_at: "2026-08-01T12:00:00Z",
  prerelease: true,
  draft: false,
  tarball_url: "https://api.github.com/repos/compozy/compozy/tarball/v0.3.0-beta.3",
  zipball_url: "https://api.github.com/repos/compozy/compozy/zipball/v0.3.0-beta.3",
  assets: [
    {
      name: "compozy_darwin_arm64.tar.gz",
      browser_download_url:
        "https://github.com/compozy/compozy/releases/download/v0.3.0-beta.3/compozy_darwin_arm64.tar.gz",
      size: 42,
      content_type: "application/gzip",
      download_count: 7,
    },
  ],
};

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});

describe("GitHub changelog boundary", () => {
  it("keeps only published releases at the cutoff while preserving GitHub release evidence", async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(
      jsonResponse([
        releaseFixture,
        {
          ...releaseFixture,
          tag_name: "v0.3.0-beta.2",
          published_at: null,
          draft: true,
        },
        {
          ...releaseFixture,
          tag_name: "v0.2.9",
          published_at: "2026-07-01T12:00:00Z",
          prerelease: false,
        },
      ])
    );
    vi.stubGlobal("fetch", fetchMock);

    const result = await loadChangelogReleases();

    expect(result).toEqual({
      status: "ready",
      releases: [
        expect.objectContaining({
          version: "v0.3.0-beta.3",
          body: releaseFixture.body,
          channel: "beta",
          categories: [{ id: "runtime", label: "Runtime", itemCount: 1 }],
          pullRequestNumbers: [123],
          assets: [expect.objectContaining({ name: "compozy_darwin_arm64.tar.gz", size: 42 })],
        }),
      ],
    });
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("returns an explicit unavailable result when GitHub violates the release schema", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse([{ tag_name: 3 }])));

    const result = await loadChangelogReleases();

    expect(result).toEqual({
      status: "unavailable",
      error: expect.objectContaining({ kind: "invalid-response" }),
    });
  });

  it("resolves a release by tag when the cached release list has not caught up yet", async () => {
    const fetchMock = vi.fn().mockResolvedValueOnce(jsonResponse(releaseFixture));
    vi.stubGlobal("fetch", fetchMock);

    const lookup = await loadChangelogReleaseByTag("v0.3.0-beta.3");

    expect(lookup).toEqual({
      status: "found",
      release: expect.objectContaining({
        version: "v0.3.0-beta.3",
        body: releaseFixture.body,
        pullRequestNumbers: [123],
      }),
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "https://api.github.com/repos/compozy/compozy/releases/tags/v0.3.0-beta.3",
      expect.objectContaining({ method: "GET" })
    );
  });

  it("separates a missing release by tag from a GitHub outage it must not hide", async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({ message: "Not Found" }, 404));
    vi.stubGlobal("fetch", fetchMock);

    expect(await loadChangelogReleaseByTag("v0.3.0-beta.99")).toEqual({ status: "missing" });
    expect(fetchMock).toHaveBeenCalledTimes(1);

    expect(await loadChangelogReleaseByTag("v0.2.9")).toEqual({ status: "missing" });
    expect(await loadChangelogReleaseByTag("  ")).toEqual({ status: "missing" });
    expect(fetchMock).toHaveBeenCalledTimes(1);

    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(jsonResponse({ message: "boom" }, 500)));

    expect(await loadChangelogReleaseByTag("v0.3.0-beta.3")).toEqual({
      status: "unavailable",
      error: expect.objectContaining({ kind: "github-response", status: 500 }),
    });
  });

  it("deduplicates human contributors from linked merged pull requests and omits bots", async () => {
    vi.stubEnv("COMPOZY_SITE_GITHUB_TOKEN", "test-token");
    const fetchMock = vi.fn().mockResolvedValue(
      jsonResponse({
        data: {
          repository: {
            pr_123: {
              number: 123,
              title: "Keep complete release notes",
              url: "https://github.com/compozy/compozy/pull/123",
              mergedAt: "2026-08-01T10:00:00Z",
              author: {
                __typename: "User",
                login: "octocat",
                avatarUrl: "https://avatars.githubusercontent.com/u/1?v=4",
                url: "https://github.com/octocat",
              },
            },
            pr_124: {
              number: 124,
              title: "Reuse contributor identity",
              url: "https://github.com/compozy/compozy/pull/124",
              mergedAt: "2026-08-01T11:00:00Z",
              author: {
                __typename: "User",
                login: "Octocat",
                avatarUrl: "https://avatars.githubusercontent.com/u/1?v=4",
                url: "https://github.com/octocat",
              },
            },
            pr_125: {
              number: 125,
              title: "Automated dependency update",
              url: "https://github.com/compozy/compozy/pull/125",
              mergedAt: "2026-08-01T11:30:00Z",
              author: {
                __typename: "Bot",
                login: "dependabot[bot]",
                avatarUrl: "https://avatars.githubusercontent.com/in/29110?v=4",
                url: "https://github.com/apps/dependabot",
              },
            },
            pr_126: null,
          },
        },
      })
    );
    vi.stubGlobal("fetch", fetchMock);

    const result = await loadReleasePullRequests([123, 124, 125, 126, 123]);

    expect(result).toEqual({
      status: "ready",
      pullRequests: [
        expect.objectContaining({
          number: 123,
          author: expect.objectContaining({ login: "octocat" }),
        }),
        expect.objectContaining({
          number: 124,
          author: expect.objectContaining({ login: "Octocat" }),
        }),
        expect.objectContaining({ number: 125, author: null }),
      ],
      contributors: [expect.objectContaining({ login: "octocat", pullRequestNumbers: [123, 124] })],
      missingNumbers: [126],
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "https://api.github.com/graphql",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ Authorization: "Bearer test-token" }),
      })
    );
  });
});
