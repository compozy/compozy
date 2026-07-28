import { render, screen } from "@testing-library/react";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { BlogEmptyState } from "../empty-state";

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

describe("BlogEmptyState", () => {
  it("renders an actionable public empty state", () => {
    render(
      <BlogEmptyState
        eyebrow="Archive pending"
        title="More field notes are being prepared."
        description="Use the RSS feed to catch the next runtime note."
        primaryAction={{ href: "/blog/feed.xml", label: "Subscribe via RSS" }}
        secondaryAction={{ href: "/changelog", label: "Open the changelog" }}
      />
    );

    expect(
      screen.getByRole("heading", { name: "More field notes are being prepared." })
    ).toBeDefined();
    expect(screen.getByText("Use the RSS feed to catch the next runtime note.")).toBeDefined();
    expect(screen.getByRole("link", { name: "Subscribe via RSS" }).getAttribute("href")).toBe(
      "/blog/feed.xml"
    );
    expect(screen.getByRole("link", { name: "Open the changelog" }).getAttribute("href")).toBe(
      "/changelog"
    );
  });
});
