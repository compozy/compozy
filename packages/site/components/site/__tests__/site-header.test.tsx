import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { loadGithubStars } from "@/components/site/github-stars-api";
import { resetGithubStarCount } from "@/components/site/hooks/use-github-star-count";
import { SiteHeader } from "@/components/site/site-header";
import { siteConfig } from "@/lib/site-config";

const mocks = vi.hoisted(() => ({ pathname: "/" }));

vi.mock("next/navigation", () => ({
  usePathname: () => mocks.pathname,
}));

vi.mock("@/components/site/github-stars-api", () => ({
  loadGithubStars: vi.fn(),
}));

const SECTIONS = [
  ["Docs", "/docs"],
  ["Marketplace", "/marketplace"],
  ["Blog", "/blog"],
  ["Changelog", "/changelog"],
  ["Download", "/download"],
] as const;

function menuPanel(button: HTMLElement): HTMLElement {
  const panel = document.getElementById(button.getAttribute("aria-controls") ?? "");
  expect(panel?.getAttribute("aria-label")).toBe("Site menu");
  return panel as HTMLElement;
}

describe("SiteHeader", () => {
  beforeEach(() => {
    mocks.pathname = "/";
    resetGithubStarCount();
    vi.mocked(loadGithubStars).mockReset();
    vi.mocked(loadGithubStars).mockResolvedValue(null);
  });

  it("links the logo home and every section to its real route", () => {
    render(<SiteHeader />);

    expect(screen.getByRole("link", { name: "CompozyOS home" }).getAttribute("href")).toBe("/");
    const nav = screen.getByRole("navigation", { name: "Site" });
    for (const [label, href] of SECTIONS) {
      expect(within(nav).getByRole("link", { name: label }).getAttribute("href")).toBe(href);
    }
  });

  it("marks only the current section, including on nested routes", () => {
    mocks.pathname = "/blog/introducing-compozyos";
    render(<SiteHeader />);

    const nav = screen.getByRole("navigation", { name: "Site" });
    expect(within(nav).getByRole("link", { name: "Blog" }).getAttribute("aria-current")).toBe(
      "page"
    );
    for (const [label] of SECTIONS.filter(([label]) => label !== "Blog")) {
      expect(
        within(nav).getByRole("link", { name: label }).getAttribute("aria-current")
      ).toBeNull();
    }
  });

  it("shows the live star count once GitHub answers, and only the label when it does not", async () => {
    vi.mocked(loadGithubStars).mockResolvedValue(2860);
    render(<SiteHeader />);

    const star = screen.getByRole("link", { name: /Star on GitHub/ });
    expect(star.getAttribute("href")).toBe(siteConfig.repoUrl);
    expect(star.getAttribute("rel")).toContain("noopener");
    await waitFor(() => expect(star.textContent).toContain("2,860"));
  });

  it("keeps the star button usable without a count when GitHub is unavailable", async () => {
    render(<SiteHeader />);

    await act(async () => {});
    expect(screen.getByRole("link", { name: "Star on GitHub" }).textContent).toBe(
      " Star on GitHub"
    );
  });

  it("opens the small-screen menu and closes it with Escape, returning focus", () => {
    render(<SiteHeader />);

    const button = screen.getByRole("button", { name: "Open menu" });
    const panel = menuPanel(button);
    expect(panel.hidden).toBe(true);

    fireEvent.click(button);
    expect(button.getAttribute("aria-expanded")).toBe("true");
    expect(panel.hidden).toBe(false);
    expect(within(panel).getAllByRole("link")).toHaveLength(SECTIONS.length);

    fireEvent.keyDown(document, { key: "Escape" });
    expect(panel.hidden).toBe(true);
    expect(document.activeElement).toBe(button);
  });

  it("closes the menu when a link in it is followed or the route changes", () => {
    const { rerender } = render(<SiteHeader />);
    const panel = menuPanel(screen.getByRole("button", { name: "Open menu" }));

    fireEvent.click(screen.getByRole("button", { name: "Open menu" }));
    fireEvent.click(within(panel).getByRole("link", { name: "Docs" }));
    expect(panel.hidden).toBe(true);

    fireEvent.click(screen.getByRole("button", { name: "Open menu" }));
    mocks.pathname = "/changelog";
    rerender(<SiteHeader />);
    expect(panel.hidden).toBe(true);
  });
});
