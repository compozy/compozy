import { render, screen, within } from "@testing-library/react";
import type { Release } from "#site/content";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import { ChangelogRail } from "../changelog-rail";
import { ReleaseEntry } from "../release-entry";

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

function release(overrides: Partial<Release> & Pick<Release, "version">): Release {
  return {
    version: overrides.version,
    date: overrides.date ?? "2026-05-01T00:00:00.000Z",
    status: overrides.status ?? "stable",
    summary: overrides.summary ?? `${overrides.version} summary`,
    added: overrides.added ?? [],
    changed: overrides.changed ?? [],
    fixed: overrides.fixed ?? [],
    breaking: overrides.breaking ?? [],
    compareUrl: overrides.compareUrl,
    body: overrides.body ?? "",
  };
}

describe("changelog public components", () => {
  it("links rail entries to exact release anchors and preserves the four-item limit", () => {
    render(
      <ChangelogRail
        releases={[
          release({ version: "v0.5.0", status: "breaking", summary: "Breaking release" }),
          release({ version: "v0.4.0", status: "beta", summary: "Beta release" }),
          release({ version: "v0.3.0", status: "alpha", summary: "Alpha release" }),
          release({ version: "v0.2.0", status: "stable", summary: "Stable release" }),
          release({ version: "v0.1.0", summary: "Hidden old release" }),
        ]}
      />
    );

    expect(screen.getByRole("complementary", { name: "Recent changelog releases" })).toBeDefined();
    expect(screen.getByRole("link", { name: /all versions/i }).getAttribute("href")).toBe(
      "/changelog"
    );
    expect(screen.getByRole("link", { name: "Open the changelog" }).getAttribute("href")).toBe(
      "/changelog"
    );

    const breaking = screen.getByRole("link", { name: /v0\.5\.0.*Breaking release/s });
    const beta = screen.getByRole("link", { name: /v0\.4\.0.*Beta release/s });
    const alpha = screen.getByRole("link", { name: /v0\.3\.0.*Alpha release/s });
    const stable = screen.getByRole("link", { name: /v0\.2\.0.*Stable release/s });

    expect(breaking.getAttribute("href")).toBe("/changelog#v0.5.0");
    expect(beta.getAttribute("href")).toBe("/changelog#v0.4.0");
    expect(alpha.getAttribute("href")).toBe("/changelog#v0.3.0");
    expect(stable.getAttribute("href")).toBe("/changelog#v0.2.0");
    expect(within(breaking).getByText("v0.5.0").getAttribute("class")).toContain("bg-danger-tint");
    expect(within(beta).getByText("v0.4.0").getAttribute("class")).toContain("bg-info-tint");
    expect(within(alpha).getByText("v0.3.0").getAttribute("class")).toContain("bg-accent-tint");
    expect(within(stable).getByText("v0.2.0").getAttribute("class")).toContain("bg-success-tint");
    expect(within(breaking).getByText("May 01 · 2026").closest("time")?.dateTime).toBe(
      "2026-05-01T00:00:00.000Z"
    );
    expect(screen.queryByText("Hidden old release")).toBeNull();
  });

  it("renders release entries with stable anchors, sections, and opener-safe compare links", () => {
    render(
      <ReleaseEntry
        release={release({
          version: "v0.6.0",
          status: "beta",
          summary: "Runtime surface polish",
          compareUrl: "https://github.com/compozy/agh/compare/v0.5.0...v0.6.0",
          added: ["Added docs navigation checks."],
          fixed: ["Fixed release copy drift."],
        })}
      />
    );

    const entry = screen.getByRole("article");
    const compare = screen.getByRole("link", { name: "Compare v0.6.0 on GitHub" });

    expect(entry.getAttribute("id")).toBe("v0.6.0");
    expect(screen.getByRole("heading", { name: "Runtime surface polish" })).toBeDefined();
    expect(screen.getByText("ADDED")).toBeDefined();
    expect(screen.getByText("Added docs navigation checks.")).toBeDefined();
    expect(screen.getByText("FIXED")).toBeDefined();
    expect(screen.getByText("Fixed release copy drift.")).toBeDefined();
    expect(screen.getByText("May 01, 2026").closest("time")?.dateTime).toBe(
      "2026-05-01T00:00:00.000Z"
    );
    expect(screen.queryByText("CHANGED")).toBeNull();
    expect(screen.queryByText("BREAKING")).toBeNull();
    expect(compare.getAttribute("href")).toBe(
      "https://github.com/compozy/agh/compare/v0.5.0...v0.6.0"
    );
    expect(compare.getAttribute("target")).toBe("_blank");
    expect(compare.getAttribute("rel")).toContain("noopener");
    expect(compare.getAttribute("rel")).toContain("noreferrer");
  });

  it("renders duplicate release bullets without duplicate-key warnings", () => {
    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});

    try {
      render(
        <ReleaseEntry
          release={release({
            version: "v0.6.1",
            added: [
              "Added duplicate-safe release bullets.",
              "Added duplicate-safe release bullets.",
            ],
          })}
        />
      );

      expect(screen.getAllByText("Added duplicate-safe release bullets.")).toHaveLength(2);
      expect(errorSpy.mock.calls.flat().join(" ")).not.toContain("same key");
    } finally {
      errorSpy.mockRestore();
    }
  });
});
