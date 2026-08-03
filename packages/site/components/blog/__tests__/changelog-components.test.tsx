import { render, screen } from "@testing-library/react";
import ChangelogReleasePage from "@/app/changelog/[version]/page";
import type { ChangelogRelease } from "@/lib/changelog/types";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { ChangelogRail } from "../changelog-rail";
import { ReleaseEntry } from "../release-entry";
import { ReleaseMarkdown } from "../release-markdown";

const changelogClient = vi.hoisted(() => ({
  loadChangelogReleases: vi.fn(),
  loadReleasePullRequests: vi.fn(),
}));

vi.mock("@/lib/changelog/github-client", () => changelogClient);

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: { href: string; children: ReactNode } & Omit<
    AnchorHTMLAttributes<HTMLAnchorElement>,
    "href"
  >) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

function release(version: string, overrides: Partial<ChangelogRelease> = {}): ChangelogRelease {
  return {
    version,
    name: null,
    body: "### Runtime\n\n- Changed the runtime.",
    summary: `${version} summary`,
    publishedAt: "2026-05-01T00:00:00.000Z",
    channel: "beta",
    prerelease: true,
    url: `https://github.com/compozy/compozy/releases/tag/${version}`,
    sourceArchiveUrl: `https://api.github.com/repos/compozy/compozy/tarball/${version}`,
    sourceZipUrl: `https://api.github.com/repos/compozy/compozy/zipball/${version}`,
    assets: [],
    categories: [{ id: "runtime", label: "Runtime", itemCount: 1 }],
    headings: [{ id: "runtime", label: "Runtime", level: 3 }],
    pullRequestNumbers: [],
    ...overrides,
  };
}

describe("changelog public components", () => {
  it("compares a release with its semantic predecessor when a backport was published later", async () => {
    changelogClient.loadChangelogReleases.mockResolvedValueOnce({
      status: "ready",
      releases: [
        release("v1.4.0", { publishedAt: "2026-08-03T00:00:00Z" }),
        release("v1.5.0", { publishedAt: "2026-08-02T00:00:00Z" }),
        release("v1.3.0", { publishedAt: "2026-08-01T00:00:00Z" }),
      ],
    });
    changelogClient.loadReleasePullRequests.mockResolvedValueOnce({
      status: "ready",
      pullRequests: [],
      contributors: [],
      missingNumbers: [],
    });

    render(await ChangelogReleasePage({ params: Promise.resolve({ version: "v1.5.0" }) }));

    expect(
      screen
        .getByRole("button", { name: "Compare v1.5.0 with v1.4.0 on GitHub" })
        .getAttribute("href")
    ).toBe("https://github.com/compozy/compozy/compare/v1.4.0...v1.5.0");
  });

  it("does not compare the oldest release with a newer version published earlier", async () => {
    changelogClient.loadChangelogReleases.mockResolvedValueOnce({
      status: "ready",
      releases: [
        release("v1.0.0", { publishedAt: "2026-08-03T00:00:00Z" }),
        release("v1.1.0", { publishedAt: "2026-08-02T00:00:00Z" }),
      ],
    });
    changelogClient.loadReleasePullRequests.mockResolvedValueOnce({
      status: "ready",
      pullRequests: [],
      contributors: [],
      missingNumbers: [],
    });

    render(await ChangelogReleasePage({ params: Promise.resolve({ version: "v1.0.0" }) }));

    expect(screen.queryByRole("button", { name: /Compare .* on GitHub/ })).toBeNull();
  });

  it("links recent versions to dedicated release pages and preserves the four-item limit", () => {
    render(
      <ChangelogRail
        releases={["v0.5.0", "v0.4.0", "v0.3.0", "v0.2.0", "v0.1.0"].map(version =>
          release(version)
        )}
      />
    );

    expect(screen.getByRole("link", { name: /v0\.5\.0/ }).getAttribute("href")).toBe(
      "/changelog/v0.5.0"
    );
    expect(screen.getByRole("link", { name: /v0\.2\.0/ }).getAttribute("href")).toBe(
      "/changelog/v0.2.0"
    );
    expect(screen.queryByText("v0.1.0")).toBeNull();
  });

  it("uses the exact version as the title and presents GitHub release categories", () => {
    render(
      <ReleaseEntry
        release={release("v0.3.0-beta.3", {
          summary: "Complete release notes from GitHub.",
          assets: [
            {
              name: "compozy.tar.gz",
              url: "https://example.com/compozy.tar.gz",
              size: 42,
              contentType: "application/gzip",
              downloadCount: 7,
            },
          ],
          categories: [{ id: "runtime", label: "Runtime", itemCount: 4 }],
        })}
      />
    );

    const title = screen.getByRole("heading", { name: "v0.3.0-beta.3" });
    expect(title).toBeDefined();
    expect(screen.getByText("Complete release notes from GitHub.")).toBeDefined();
    expect(screen.getByText("Runtime · 4")).toBeDefined();
    expect(
      screen.getByRole("link", { name: "Read full release notes →" }).getAttribute("href")
    ).toBe("/changelog/v0.3.0-beta.3");
    const github = screen.getByRole("link", { name: /GitHub release/ });
    expect(github.getAttribute("target")).toBe("_blank");
    expect(github.getAttribute("rel")).toContain("noopener");
  });

  it("preserves deep GitHub release sections without skipping HTML heading levels", () => {
    render(
      <ReleaseMarkdown
        body={
          "### Release Notes\n\n#### Features\n\n##### Grouped skills\n\n###### Migration detail"
        }
        pullRequestNumbers={[]}
      />
    );

    expect(screen.getByRole("heading", { level: 2, name: "Release Notes" })).toBeDefined();
    expect(screen.getByRole("heading", { level: 3, name: "Features" })).toBeDefined();
    expect(screen.getByRole("heading", { level: 4, name: "Grouped skills" })).toBeDefined();
    expect(screen.getByRole("heading", { level: 5, name: "Migration detail" })).toBeDefined();
  });
});
