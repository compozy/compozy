import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ReactNode } from "react";
import { CompactFolder, CompactItem } from "../sidebar-compact-tree";

const mocks = vi.hoisted(() => ({
  depth: 0,
  folderOpen: true,
}));

vi.mock("next/navigation", () => ({
  usePathname: () => "/runtime/core/loops/catalog",
}));

vi.mock("fumadocs-ui/contexts/tree", () => ({
  useTreePath: () => [],
}));

vi.mock("fumadocs-ui/components/sidebar/base", () => ({
  useFolderDepth: () => mocks.depth,
  useFolder: () => ({
    open: mocks.folderOpen,
    setOpen: vi.fn(),
    depth: mocks.depth + 1,
    collapsible: true,
  }),
  SidebarItem: ({
    children,
    icon,
    className,
    active,
  }: {
    children: ReactNode;
    icon?: ReactNode;
    className?: string;
    active?: boolean;
  }) => (
    <a className={className} data-active={active} data-testid="sidebar-item">
      {icon}
      {children}
    </a>
  ),
  SidebarFolder: ({
    children,
    className,
    active,
  }: {
    children: ReactNode;
    className?: string;
    active?: boolean;
  }) => (
    <div
      className={className}
      data-active={active ? "true" : undefined}
      data-state="open"
      data-testid="sidebar-folder"
    >
      {children}
    </div>
  ),
  SidebarFolderLink: ({
    children,
    className,
    active,
  }: {
    children: ReactNode;
    className?: string;
    active?: boolean;
  }) => (
    <a className={className} data-active={active} data-testid="folder-link">
      {children}
    </a>
  ),
  SidebarFolderTrigger: ({ children, className }: { children: ReactNode; className?: string }) => (
    <button className={className} data-testid="folder-trigger" type="button">
      {children}
    </button>
  ),
  SidebarFolderContent: ({ children, className }: { children: ReactNode; className?: string }) => (
    <div className={className} data-testid="folder-content">
      {children}
    </div>
  ),
}));

vi.mock("../sidebar-folder-persist-effect", () => ({
  FolderOpenPersistWriter: () => null,
}));

describe("sidebar compact tree", () => {
  beforeEach(() => {
    mocks.depth = 0;
    mocks.folderOpen = true;
  });

  it("Should render top-level item icons and omit nested icons", () => {
    mocks.depth = 0;
    const { rerender } = render(
      <CompactItem
        item={{
          type: "page",
          name: "Loops",
          url: "/runtime/core/loops",
          icon: <span data-testid="page-icon" />,
        }}
      />
    );
    expect(screen.getByTestId("page-icon")).toBeTruthy();

    mocks.depth = 1;
    rerender(
      <CompactItem
        item={{
          type: "page",
          name: "Catalog",
          url: "/runtime/core/loops/catalog",
          icon: <span data-testid="nested-icon" />,
        }}
      />
    );
    expect(screen.queryByTestId("nested-icon")).toBeNull();
    expect(screen.getByTestId("sidebar-item").className).toContain("docs-sb-item--nested");
  });

  it("Should wrap folder children in a guide-line kids container", () => {
    render(
      <CompactFolder
        item={{
          type: "folder",
          $id: "core/loops",
          name: "Loops",
          icon: <span data-testid="folder-icon" />,
          index: {
            type: "page",
            name: "Loops",
            url: "/runtime/core/loops",
          },
          children: [],
        }}
      >
        <span>child</span>
      </CompactFolder>
    );
    expect(screen.getByTestId("folder-icon")).toBeTruthy();
    expect(screen.getByTestId("folder-content").className).toContain("docs-sb-kids-clip");
    expect(screen.getByText("child").parentElement?.className).toContain("docs-sb-kids");
  });

  it("Should truncate long item labels and expose the full name via title", () => {
    const longName = "Participation, Channels, and Peers";
    render(
      <CompactItem
        item={{
          type: "page",
          name: longName,
          url: "/runtime/core/network/channels-and-peers",
        }}
      />
    );
    const label = screen.getByText(longName);
    expect(label.tagName).toBe("SPAN");
    expect(label.className).toContain("truncate");
    expect(label.getAttribute("title")).toBe(longName);
    expect(screen.getByTestId("sidebar-item").className).toContain("overflow-hidden");
  });

  it("Should truncate long folder header labels and expose the full name via title", () => {
    const longName = "Delivery, Admission, and Usage";
    render(
      <CompactFolder
        item={{
          type: "folder",
          $id: "core/network/delivery",
          name: longName,
          index: {
            type: "page",
            name: longName,
            url: "/runtime/core/network/delivery-and-safety",
          },
          children: [],
        }}
      >
        <span>child</span>
      </CompactFolder>
    );
    const label = screen.getByText(longName);
    expect(label.tagName).toBe("SPAN");
    expect(label.className).toContain("truncate");
    expect(label.getAttribute("title")).toBe(longName);
  });
});
