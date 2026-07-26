import { render, screen } from "@testing-library/react";
import type { AnchorHTMLAttributes, ReactNode } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { TocRail } from "../toc-rail";
import { flattenToc, type TocEntryNode } from "../toc-utils";

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

class MockIntersectionObserver {
  readonly observe = vi.fn();
  readonly disconnect = vi.fn();
}

describe("blog TocRail", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("flattens nested table-of-contents entries with stable depths", () => {
    const toc: TocEntryNode[] = [
      {
        title: "Setup",
        url: "#setup",
        items: [{ title: "Credentials", url: "#credentials" }],
      },
      { title: "Flow", url: "#flow" },
    ];

    expect(flattenToc(toc)).toEqual([
      { title: "Setup", url: "#setup", depth: 2 },
      { title: "Credentials", url: "#credentials", depth: 3 },
      { title: "Flow", url: "#flow", depth: 2 },
    ]);
  });

  it("renders accessible links and marks the initial active section", () => {
    vi.stubGlobal("IntersectionObserver", MockIntersectionObserver);
    document.body.innerHTML = '<h2 id="setup">Setup</h2><h3 id="credentials">Credentials</h3>';

    render(
      <TocRail
        items={[
          { title: "Setup", url: "#setup", depth: 2 },
          { title: "Credentials", url: "#credentials", depth: 3 },
        ]}
      />
    );

    const setup = screen.getByRole("link", { name: "Setup" });
    const credentials = screen.getByRole("link", { name: "Credentials" });

    expect(screen.getByRole("complementary", { name: "Blog table of contents" })).toBeDefined();
    expect(screen.getByText("On this page")).toBeDefined();
    expect(setup.getAttribute("href")).toBe("#setup");
    expect(setup.getAttribute("aria-current")).toBe("location");
    expect(credentials.getAttribute("href")).toBe("#credentials");
    expect(credentials.getAttribute("aria-current")).toBeNull();
    expect(credentials.getAttribute("class")).toContain("pl-3");
  });

  it("keeps the table of contents usable when IntersectionObserver is unavailable", () => {
    document.body.innerHTML = '<h2 id="setup">Setup</h2>';

    render(<TocRail items={[{ title: "Setup", url: "#setup", depth: 2 }]} />);

    const setup = screen.getByRole("link", { name: "Setup" });
    expect(setup.getAttribute("href")).toBe("#setup");
    expect(setup.getAttribute("aria-current")).toBe("location");
  });

  it("does not render an empty table-of-contents rail", () => {
    const { container } = render(<TocRail items={[]} />);

    expect(container.firstChild).toBeNull();
  });
});
