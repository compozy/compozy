import { render, screen } from "@testing-library/react";
import type { AnchorHTMLAttributes, ComponentType, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { HomeHeader } from "../home-header";

type SlotComponentProps = {
  className?: string;
  hideIfDisabled?: boolean;
};

type HomeLayoutSlots = {
  searchTrigger?: {
    full: ComponentType<SlotComponentProps>;
    sm: ComponentType<SlotComponentProps>;
  };
};

const mocks = vi.hoisted(() => ({
  pathname: "/",
  slots: {} as HomeLayoutSlots,
}));

vi.mock("next/navigation", () => ({
  usePathname: () => mocks.pathname,
}));

vi.mock("fumadocs-ui/layouts/home", () => ({
  useHomeLayout: () => ({
    slots: mocks.slots,
  }),
}));

vi.mock("../header-search-input", () => ({
  HeaderSearchInput: ({ className }: SlotComponentProps) => (
    <input aria-label="Search docs" className={className} type="search" />
  ),
}));

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

describe("HomeHeader", () => {
  beforeEach(() => {
    const SearchFull = ({ className }: SlotComponentProps) => (
      <button className={className} type="button">
        Search docs
      </button>
    );
    const SearchSmall = ({ className }: SlotComponentProps) => (
      <button className={className} type="button" aria-label="Search docs">
        Search
      </button>
    );

    mocks.pathname = "/";
    mocks.slots = {
      searchTrigger: {
        full: SearchFull,
        sm: SearchSmall,
      },
    };
  });

  it("renders the shared full AGH logo in the home link", () => {
    render(<HomeHeader />);

    const link = screen.getByRole("link", { name: "AGH home" });
    const logo = link.querySelector('[data-slot="logo"]');

    expect(link.getAttribute("href")).toBe("/");
    expect(logo).not.toBeNull();
    expect(logo?.getAttribute("data-variant")).toBe("logo");
    expect(logo?.getAttribute("viewBox")).toBe("0 0 972 386");
    expect(logo?.getAttribute("aria-hidden")).toBe("true");
  });

  it("renders the public navigation links in desktop and mobile headers", () => {
    mocks.pathname = "/protocol/delivery";

    render(<HomeHeader />);

    const expectedLinks = [
      { href: "/", label: "Home" },
      { href: "/runtime", label: "Runtime" },
      { href: "/protocol", label: "AGH Network" },
      { href: "/blog", label: "Blog" },
      { href: "/changelog", label: "Changelog" },
    ];

    for (const expected of expectedLinks) {
      const links = screen.getAllByRole("link", { name: expected.label });

      expect(links).toHaveLength(2);
      for (const link of links) {
        expect(link.getAttribute("href")).toBe(expected.href);
      }
    }

    for (const link of screen.getAllByRole("link", { name: "AGH Network" })) {
      expect(link.getAttribute("class")).toContain("text-fg");
      expect(link.getAttribute("class")).toContain("bg-elevated");
      expect(link.getAttribute("aria-current")).toBe("location");
    }
    for (const link of screen.getAllByRole("link", { name: "Home" })) {
      expect(link.getAttribute("class")).not.toContain("bg-elevated");
      expect(link.getAttribute("aria-current")).toBeNull();
    }
  });

  it("marks exact active navigation links as the current page", () => {
    mocks.pathname = "/blog";

    render(<HomeHeader />);

    for (const link of screen.getAllByRole("link", { name: "Blog" })) {
      expect(link.getAttribute("aria-current")).toBe("page");
      expect(link.getAttribute("class")).toContain("text-fg");
      expect(link.getAttribute("class")).toContain("bg-elevated");
    }
  });

  it("keeps search controls and the GitHub icon link accessible", () => {
    render(<HomeHeader />);

    expect(screen.getByRole("searchbox", { name: "Search docs" })).toBeDefined();
    expect(screen.getAllByRole("button", { name: "Search docs" })).toHaveLength(1);

    const githubLink = screen.getByRole("link", { name: "AGH on GitHub" });
    expect(githubLink.getAttribute("href")).toBe("https://github.com/compozy");
    expect(githubLink.getAttribute("target")).toBe("_blank");
    expect(githubLink.getAttribute("rel")).toContain("noopener");
    expect(githubLink.getAttribute("rel")).toContain("noreferrer");
    expect(githubLink.getAttribute("class")).toContain("md:hidden");
  });

  it("omits search controls when the home layout does not provide search slots", () => {
    mocks.slots = {};

    render(<HomeHeader />);

    expect(screen.queryByRole("button", { name: "Search docs" })).toBeNull();
    expect(screen.queryByRole("searchbox", { name: "Search docs" })).toBeNull();
  });
});
