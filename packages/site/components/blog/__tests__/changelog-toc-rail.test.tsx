import { render, screen, within } from "@testing-library/react";
import type { Release } from "#site/content";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { ChangelogTocRail } from "../changelog-toc-rail";

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: ReactNode;
  } & Omit<AnchorHTMLAttributes<HTMLAnchorElement>, "href">) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

function release(version: string, status: Release["status"] = "stable"): Release {
  return {
    version,
    date: "2026-05-01",
    status,
    summary: `${version} summary`,
    added: [],
    changed: [],
    fixed: [],
    breaking: [],
    body: "",
  };
}

describe("ChangelogTocRail", () => {
  it("renders release anchors and active version state", () => {
    render(
      <ChangelogTocRail
        activeVersion="v0.2.0"
        releases={[release("v0.2.0", "beta"), release("v0.1.0")]}
      />
    );

    expect(screen.getByRole("complementary", { name: "Changelog versions" })).toBeDefined();
    const navigation = screen.getByText("All versions").closest("div");
    expect(navigation).not.toBeNull();

    const latest = screen.getByRole("link", { name: "v0.2.0" });
    const previous = screen.getByRole("link", { name: "v0.1.0" });
    expect(latest.getAttribute("href")).toBe("#v0.2.0");
    expect(latest.getAttribute("class")).toContain("text-accent");
    expect(latest.getAttribute("aria-current")).toBe("location");
    expect(previous.getAttribute("href")).toBe("#v0.1.0");
    expect(previous.getAttribute("class")).not.toContain("text-accent");
    expect(previous.getAttribute("aria-current")).toBeNull();
  });

  it("routes update readers to the installation instructions", () => {
    render(<ChangelogTocRail releases={[release("v0.1.0")]} />);

    const upgrade = screen.getByText("Upgrade").closest("div");
    expect(upgrade).not.toBeNull();
    const link = within(upgrade ?? document.body).getByRole("link", {
      name: "Install instructions →",
    });

    expect(link.getAttribute("href")).toBe("/runtime/core/getting-started/installation");
  });
});
