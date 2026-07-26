import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { DocsHeader } from "../docs-header";
import type { ComponentType, ReactNode } from "react";

type NavItem = {
  children?: ReactNode;
  icon?: ReactNode;
  label?: string;
  text?: string;
  type?: string;
  url?: string;
};

type TabItem = {
  props?: Record<string, unknown>;
  title: string;
  unlisted?: boolean;
  url: string;
};

type SlotComponentProps = {
  children?: ReactNode;
  className?: string;
  hideIfDisabled?: boolean;
};

type SidebarSlots = {
  collapseTrigger: ComponentType<SlotComponentProps>;
  trigger: ComponentType<SlotComponentProps>;
  useSidebar: () => { open: boolean };
};

type LayoutState = {
  isNavTransparent: boolean;
  navItems: NavItem[];
  props: {
    nav?: {
      children?: ReactNode;
      mode?: string;
    };
    sidebar: {
      collapsible?: boolean;
    };
    tabMode?: string;
    tabs: TabItem[];
  };
  slots: {
    navTitle?: ComponentType<SlotComponentProps>;
    searchTrigger?: {
      full: ComponentType<SlotComponentProps>;
      sm: ComponentType<SlotComponentProps>;
    };
    sidebar?: SidebarSlots;
    themeSwitch?: ComponentType<SlotComponentProps>;
  };
};

const mocks = vi.hoisted(() => {
  return {
    state: null as LayoutState | null,
  };
});

vi.mock("next/navigation", () => ({
  usePathname: () => "/runtime/cli-reference/agh/",
}));

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("fumadocs-ui/layouts/notebook", () => ({
  useNotebookLayout: () => mocks.state,
}));

vi.mock("fumadocs-ui/layouts/shared", () => ({
  isLayoutTabActive: (tab: TabItem, pathname: string) => pathname.startsWith(tab.url),
  LinkItem: ({ children, item, ...props }: { children?: ReactNode; item: NavItem }) => (
    <a href={item.url} {...props}>
      {children ?? item.label}
    </a>
  ),
}));

vi.mock("../header-search-input", () => ({
  HeaderSearchInput: ({ className }: SlotComponentProps) => (
    <input aria-label="Search docs" className={className} type="search" />
  ),
}));

describe("DocsHeader", () => {
  beforeEach(() => {
    const CollapseTrigger = ({ children, className }: SlotComponentProps) => (
      <button className={className} type="button">
        collapse {children}
      </button>
    );
    const SidebarTrigger = ({ children, className }: SlotComponentProps) => (
      <button className={className} type="button">
        sidebar {children}
      </button>
    );
    const NavTitle = ({ className }: SlotComponentProps) => (
      <a className={className} href="/" aria-label="AGH docs home">
        AGH
      </a>
    );
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
    const ThemeSwitch = () => <button type="button">Theme</button>;

    mocks.state = {
      isNavTransparent: true,
      navItems: [
        { text: "Runtime", type: "main", url: "/runtime/" },
        { text: "AGH Network", type: "main", url: "/protocol/" },
        { label: "GitHub", type: "icon", url: "https://github.com/compozy", icon: "GH" },
        { type: "menu", text: "Ignored menu" },
      ],
      props: {
        nav: { children: <span>custom nav child</span>, mode: "auto" },
        sidebar: { collapsible: true },
        tabMode: "navbar",
        tabs: [
          { title: "Runtime", url: "/runtime/" },
          { title: "CLI Reference", url: "/runtime/cli-reference/" },
          { title: "Draft", url: "/runtime/draft/", unlisted: true },
        ],
      },
      slots: {
        navTitle: NavTitle,
        searchTrigger: {
          full: SearchFull,
          sm: SearchSmall,
        },
        sidebar: {
          collapseTrigger: CollapseTrigger,
          trigger: SidebarTrigger,
          useSidebar: () => ({ open: false }),
        },
        themeSwitch: ThemeSwitch,
      },
    };
  });

  it("renders searchable docs navigation with accessible icon links", () => {
    render(<DocsHeader />);

    expect(
      screen
        .getAllByRole("link", { name: "Runtime" })
        .some(link => link.getAttribute("href") === "/runtime/")
    ).toBe(true);
    expect(screen.getByRole("link", { name: "AGH Network" }).getAttribute("href")).toBe(
      "/protocol/"
    );
    expect(screen.queryByText("Ignored menu")).toBeNull();
    expect(screen.getByRole("link", { name: "GitHub" }).getAttribute("href")).toBe(
      "https://github.com/compozy"
    );
    expect(screen.getByRole("searchbox", { name: "Search docs" })).toBeDefined();
    expect(screen.getAllByRole("button", { name: "Search docs" })).toHaveLength(1);
    expect(screen.getByRole("link", { name: "AGH docs home" }).getAttribute("href")).toBe("/");
  });

  it("marks the active layout tab and hides inactive unlisted tabs", () => {
    render(<DocsHeader />);

    const runtimeTab = screen
      .getAllByRole("link", { name: "Runtime" })
      .find(link => link.getAttribute("class")?.includes("border-b-2"));

    expect(runtimeTab?.getAttribute("class")).not.toContain("text-fd-primary");
    expect(screen.getByRole("link", { name: "CLI Reference" }).getAttribute("class")).toContain(
      "text-fd-primary"
    );
    expect(
      screen.getByRole("link", { name: "Draft", hidden: true }).getAttribute("class")
    ).toContain("hidden");
  });

  it("omits layout tabs when the notebook layout is not in navbar mode", () => {
    const state = mocks.state;
    if (!state) throw new Error("missing docs header mock state");

    mocks.state = {
      ...state,
      props: {
        ...state.props,
        tabMode: "sidebar",
      },
    };

    render(<DocsHeader />);

    expect(screen.queryByRole("link", { name: "CLI Reference" })).toBeNull();
  });

  it("keeps fallback nav keys unique when custom items lack identifiers", () => {
    const state = mocks.state;
    if (!state) throw new Error("missing docs header mock state");

    const errorSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    mocks.state = {
      ...state,
      navItems: [
        ...state.navItems,
        { children: <span>First custom item</span>, type: "custom" },
        { children: <span>Second custom item</span>, type: "custom" },
      ],
    };

    try {
      render(<DocsHeader />);

      expect(screen.getByText("First custom item")).toBeDefined();
      expect(screen.getByText("Second custom item")).toBeDefined();
      expect(errorSpy.mock.calls.flat().join(" ")).not.toContain("same key");
    } finally {
      errorSpy.mockRestore();
    }
  });
});
