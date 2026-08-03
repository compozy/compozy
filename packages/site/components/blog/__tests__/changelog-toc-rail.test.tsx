import { render, screen, within } from "@testing-library/react";
import type { ChangelogRelease } from "@/lib/changelog/types";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { ChangelogTocRail } from "../changelog-toc-rail";

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

function release(version: string): ChangelogRelease {
  return {
    version,
    name: null,
    body: "",
    summary: version,
    publishedAt: "2026-05-01T00:00:00Z",
    channel: "beta",
    prerelease: true,
    url: "https://example.com/release",
    assets: [],
    sourceArchiveUrl: "https://example.com/source.tar.gz",
    sourceZipUrl: "https://example.com/source.zip",
    categories: [],
    headings: [],
    pullRequestNumbers: [],
  };
}

describe("ChangelogTocRail", () => {
  it("routes each exact version to its release page and marks the active version", () => {
    render(
      <ChangelogTocRail
        activeVersion="v0.3.0-beta.2"
        releases={[release("v0.3.0-beta.2"), release("v0.3.0-beta.1")]}
      />
    );
    const latest = screen.getByRole("link", { name: "v0.3.0-beta.2" });
    expect(latest.getAttribute("href")).toBe("/changelog/v0.3.0-beta.2");
    expect(latest.getAttribute("aria-current")).toBe("location");
    expect(screen.getByRole("link", { name: "v0.3.0-beta.1" }).getAttribute("href")).toBe(
      "/changelog/v0.3.0-beta.1"
    );
  });

  it("routes update readers to the installation instructions", () => {
    render(<ChangelogTocRail releases={[release("v0.3.0-beta.1")]} />);
    const upgrade = screen.getByText("Upgrade").closest("div");
    expect(
      within(upgrade ?? document.body)
        .getByRole("link", { name: "Install instructions →" })
        .getAttribute("href")
    ).toBe("/docs/getting-started/installation");
  });
});
