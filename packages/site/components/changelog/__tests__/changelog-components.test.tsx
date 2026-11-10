import { fireEvent, render, screen, within } from "@testing-library/react";
import ChangelogPage from "@/app/changelog/page";
import ChangelogReleasePage, { generateMetadata } from "@/app/changelog/[version]/page";
import type { ChangelogRelease } from "@/lib/changelog/types";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

const changelogClient = vi.hoisted(() => ({
  loadChangelogReleaseByTag: vi.fn(),
  loadChangelogReleases: vi.fn(),
  loadReleasePullRequests: vi.fn(),
}));

vi.mock("@/lib/changelog/github-client", () => changelogClient);

vi.mock("next/server", () => ({ connection: vi.fn() }));

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

const NO_EVIDENCE = { status: "ready", pullRequests: [], contributors: [], missingNumbers: [] };

function release(version: string, overrides: Partial<ChangelogRelease> = {}): ChangelogRelease {
  return {
    version,
    name: null,
    body: "### Bug Fixes\n\n- Changed the runtime.",
    summary: `${version} summary`,
    publishedAt: "2026-05-01T00:00:00.000Z",
    channel: "beta",
    prerelease: true,
    url: `https://github.com/compozy/compozy/releases/tag/${version}`,
    sourceArchiveUrl: `https://api.github.com/repos/compozy/compozy/tarball/${version}`,
    sourceZipUrl: `https://api.github.com/repos/compozy/compozy/zipball/${version}`,
    assets: [],
    categories: [],
    headings: [],
    pullRequestNumbers: [],
    ...overrides,
  };
}

const PUBLISHED_BODY = [
  "## 0.5.0 - 2026-11-03",
  "",
  "### Bug Fixes",
  "",
  "- Keep permission receipts with their tool (#1158)",
  "",
  "### Features",
  "",
  "- Simplify spec-cycle to agents (#1152)",
  "",
  "### Release Notes",
  "",
  "#### Breaking Changes",
  "",
  "##### Spec delivery moves from Loops to agents",
  "Deliver a spec with `spec_conductor`, merged in #1152.",
  "",
  "### Migration",
  "",
  "Nothing under `~/.compozy` needs deleting.",
  "",
  "#### Features",
  "",
  "##### Agents can wait for other sessions",
  "A monitor waits on sessions.",
  "",
  "#### Fixes",
  "",
  "##### Allowed permissions stay with their tool",
  "Receipts no longer drift.",
  "",
  "#### Highlights",
  "",
  "##### Always-on agents keep watch over your projects",
  "Agents now come in two kinds.",
  "",
  "### Verification",
  "",
  "Every archive is signed.",
].join("\n");

describe("changelog release page", () => {
  it("keeps every published section, the migration and each pull request link", async () => {
    const current = release("v0.5.0", {
      body: PUBLISHED_BODY,
      prerelease: false,
      channel: "stable",
      pullRequestNumbers: [1158, 1152, 1200],
      assets: [
        {
          name: "compozy_darwin_arm64.tar.gz",
          url: "https://example.com/compozy_darwin_arm64.tar.gz",
          size: 2048,
          contentType: "application/gzip",
          downloadCount: 7,
        },
      ],
    });
    changelogClient.loadChangelogReleases.mockResolvedValueOnce({
      status: "ready",
      releases: [current],
    });
    changelogClient.loadReleasePullRequests.mockResolvedValueOnce({
      status: "ready",
      pullRequests: [
        {
          number: 1200,
          title: "Mentioned only in the notes",
          url: "https://github.com/compozy/compozy/pull/1200",
          author: null,
        },
      ],
      contributors: [
        {
          login: "octocat",
          url: "https://github.com/octocat",
          avatarUrl: "https://avatars.githubusercontent.com/u/1",
          pullRequestNumbers: [1200],
        },
      ],
      missingNumbers: [],
    });

    render(await ChangelogReleasePage({ params: Promise.resolve({ version: "v0.5.0" }) }));

    expect(
      screen.getByRole("heading", {
        level: 1,
        name: "Always-on agents keep watch over your projects",
      })
    ).toBeDefined();
    for (const group of ["Highlights", "Breaking Changes", "Features", "Fixes", "Verification"]) {
      expect(screen.getByRole("heading", { name: group })).toBeDefined();
    }
    expect(screen.getByText("Migration")).toBeDefined();
    expect(screen.getByText(/needs deleting/)).toBeDefined();
    expect(screen.getByText("Every archive is signed.")).toBeDefined();
    expect(screen.getByRole("link", { name: "octocat" }).getAttribute("href")).toBe(
      "https://github.com/octocat"
    );

    fireEvent.click(screen.getByRole("button", { name: /Every merged change · 2/ }));
    const changes = screen.getByRole("button", { name: /Every merged change/ }).parentElement!;
    for (const pr of ["1158", "1152", "1200"]) {
      expect(
        within(changes)
          .getByRole("link", { name: `#${pr}` })
          .getAttribute("href")
      ).toBe(`https://github.com/compozy/compozy/pull/${pr}`);
    }
    expect(within(changes).getByText("Mentioned only in the notes")).toBeDefined();
    expect(
      within(changes).getByRole("link", { name: "compozy_darwin_arm64.tar.gz" })
    ).toBeDefined();
    expect(within(changes).getByRole("link", { name: "Source code (zip)" })).toBeDefined();
  });

  it("renders an exact release lookup when the cached collection is behind", async () => {
    const current = release("v0.3.0-beta.22");
    const predecessor = release("v0.3.0-beta.21");
    changelogClient.loadChangelogReleases.mockResolvedValueOnce({
      status: "ready",
      releases: [release("v0.3.0-beta.20")],
    });
    changelogClient.loadChangelogReleaseByTag.mockResolvedValueOnce({
      status: "found",
      release: current,
      releases: [current, predecessor, release("v0.3.0-beta.20")],
    });
    changelogClient.loadReleasePullRequests.mockResolvedValueOnce(NO_EVIDENCE);

    render(await ChangelogReleasePage({ params: Promise.resolve({ version: "v0.3.0-beta.22" }) }));

    expect(screen.getByRole("link", { name: "v0.3.0-beta.22" }).getAttribute("aria-current")).toBe(
      "true"
    );
    expect(
      screen.getByRole("link", { name: "Compare with v0.3.0-beta.21" }).getAttribute("href")
    ).toBe("https://github.com/compozy/compozy/compare/v0.3.0-beta.21...v0.3.0-beta.22");
  });

  it("keeps a genuinely missing exact release on the not-found path", async () => {
    changelogClient.loadChangelogReleases.mockResolvedValueOnce({ status: "ready", releases: [] });
    changelogClient.loadChangelogReleaseByTag.mockResolvedValueOnce({ status: "missing" });

    await expect(
      ChangelogReleasePage({ params: Promise.resolve({ version: "v0.3.0-beta.99" }) })
    ).rejects.toThrow("NEXT_HTTP_ERROR_FALLBACK;404");
  });

  it("does not describe an unavailable changelog lookup as a missing release", async () => {
    changelogClient.loadChangelogReleases.mockResolvedValueOnce({ status: "ready", releases: [] });
    changelogClient.loadChangelogReleaseByTag.mockResolvedValueOnce({
      status: "unavailable",
      error: { kind: "github-response", message: "GitHub returned 500", status: 500 },
    });

    await expect(
      generateMetadata({ params: Promise.resolve({ version: "v0.3.0-beta.22" }) })
    ).rejects.toThrow("GitHub changelog is unavailable: GitHub returned 500");
  });

  it("compares a release with its semantic predecessor when a backport was published later", async () => {
    changelogClient.loadChangelogReleases.mockResolvedValueOnce({
      status: "ready",
      releases: [
        release("v1.4.0", { publishedAt: "2026-08-03T00:00:00Z" }),
        release("v1.5.0", { publishedAt: "2026-08-02T00:00:00Z" }),
        release("v1.3.0", { publishedAt: "2026-08-01T00:00:00Z" }),
      ],
    });
    changelogClient.loadReleasePullRequests.mockResolvedValueOnce(NO_EVIDENCE);

    render(await ChangelogReleasePage({ params: Promise.resolve({ version: "v1.5.0" }) }));

    expect(screen.getByRole("link", { name: "Compare with v1.4.0" }).getAttribute("href")).toBe(
      "https://github.com/compozy/compozy/compare/v1.4.0...v1.5.0"
    );
  });

  it("does not compare the oldest release with a newer version published earlier", async () => {
    changelogClient.loadChangelogReleases.mockResolvedValueOnce({
      status: "ready",
      releases: [
        release("v1.0.0", { publishedAt: "2026-08-03T00:00:00Z" }),
        release("v1.1.0", { publishedAt: "2026-08-02T00:00:00Z" }),
      ],
    });
    changelogClient.loadReleasePullRequests.mockResolvedValueOnce(NO_EVIDENCE);

    render(await ChangelogReleasePage({ params: Promise.resolve({ version: "v1.0.0" }) }));

    expect(screen.queryByRole("link", { name: /^Compare with/ })).toBeNull();
  });
});

describe("changelog index", () => {
  it("renders the unavailable state when GitHub release data cannot be loaded", async () => {
    changelogClient.loadChangelogReleases.mockResolvedValueOnce({
      status: "unavailable",
      error: { kind: "network", message: "GitHub could not be reached" },
    });

    render(await ChangelogPage());

    expect(screen.getByText("The release notes couldn't be loaded")).toBeDefined();
    expect(screen.getByRole("link", { name: "GitHub Releases" }).getAttribute("href")).toBe(
      "https://github.com/compozy/compozy/releases"
    );
  });
});
