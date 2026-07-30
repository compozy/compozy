import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { FrameworkProvider } from "fumadocs-core/framework";
import type { Folder, Root } from "fumadocs-core/page-tree";
import { SidebarProvider } from "fumadocs-ui/components/sidebar/base";
import { TreeContextProvider } from "fumadocs-ui/contexts/tree";
import type { ReactNode } from "react";
import { act } from "react";
import { hydrateRoot } from "react-dom/client";
import { renderToString } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { CompactFolder } from "../sidebar-compact-tree";

const mocks = vi.hoisted(() => ({
  pathname: "/docs/loops/catalog",
}));

vi.mock("next/navigation", () => ({
  usePathname: () => mocks.pathname,
}));

function createFolder(folderKey = "loops", page = "catalog"): Folder {
  const folderUrl = `/docs/${folderKey}`;
  const index: NonNullable<Folder["index"]> = {
    type: "page",
    name: `${folderKey} overview`,
    url: folderUrl,
  };

  return {
    type: "folder",
    $id: folderKey,
    name: folderKey,
    index,
    children: [
      index,
      {
        type: "page",
        name: page,
        url: `${folderUrl}/${page}`,
      },
    ],
  };
}

function SidebarHarness({ folder, children }: { folder: Folder; children: ReactNode }) {
  const tree: Root = {
    $id: `docs-${folder.$id ?? "root"}`,
    name: "Docs",
    children: [folder],
  };

  return (
    <FrameworkProvider
      useParams={() => ({})}
      usePathname={() => mocks.pathname}
      useRouter={() => ({ push: () => undefined, refresh: () => undefined })}
    >
      <TreeContextProvider tree={tree}>
        <SidebarProvider>
          <CompactFolder item={folder}>{children}</CompactFolder>
        </SidebarProvider>
      </TreeContextProvider>
    </FrameworkProvider>
  );
}

function renderFolder(folder: Folder, children: ReactNode) {
  return render(<SidebarHarness folder={folder}>{children}</SidebarHarness>);
}

function folderState(name: string): string | null | undefined {
  const control = screen.queryByRole("button", { name }) ?? screen.getByRole("link", { name });
  return control.closest(".docs-sb-folder")?.getAttribute("data-state");
}

describe("sidebar compact tree", () => {
  beforeEach(() => {
    mocks.pathname = "/docs/loops/catalog";
    window.localStorage.clear();
    vi.stubGlobal(
      "matchMedia",
      (query: string): MediaQueryList => ({
        addEventListener: () => undefined,
        addListener: () => undefined,
        dispatchEvent: () => false,
        matches: false,
        media: query,
        onchange: null,
        removeEventListener: () => undefined,
        removeListener: () => undefined,
      })
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("Should let a persisted closed preference survive active-route hydration", async () => {
    const folder = createFolder();
    const container = document.createElement("div");
    container.innerHTML = renderToString(
      <SidebarHarness folder={folder}>
        <span data-testid="folder-content">Catalog</span>
      </SidebarHarness>
    );
    document.body.append(container);
    window.localStorage.setItem("compozy.docs.sidebar.folder:loops", "0");

    const root = hydrateRoot(
      container,
      <SidebarHarness folder={folder}>
        <span data-testid="folder-content">Catalog</span>
      </SidebarHarness>
    );

    await waitFor(() => {
      expect(window.localStorage.getItem("compozy.docs.sidebar.folder:loops")).toBe("0");
      expect(container.querySelector(".docs-sb-folder")?.getAttribute("data-state")).toBe("closed");
    });

    await act(async () => root.unmount());
    container.remove();
  });

  it("Should keep a non-active folder closed when its persisted preference is closed", () => {
    const folder = createFolder();
    mocks.pathname = "/docs/agents/providers";
    window.localStorage.setItem("compozy.docs.sidebar.folder:loops", "0");

    renderFolder(folder, <span data-testid="folder-content">Catalog</span>);

    expect(screen.queryByTestId("folder-content")).toBeNull();
  });

  it("Should keep a hoisted overview on a collapsible category header", () => {
    const folder = createFolder();

    renderFolder(folder, <span>Catalog</span>);

    expect(screen.getByRole("button", { name: "loops" })).toBeTruthy();
    expect(screen.queryByRole("link", { name: "loops" })).toBeNull();
  });

  it("Should restore a user-closed active folder after remount", async () => {
    const folder = createFolder();
    const first = renderFolder(folder, <span data-testid="folder-content">Catalog</span>);

    const trigger = screen.getByRole("button", { name: "loops" });
    expect(folderState("loops")).toBe("open");
    fireEvent.click(trigger);

    await waitFor(() => {
      expect(window.localStorage.getItem("compozy.docs.sidebar.folder:loops")).toBe("0");
    });
    first.unmount();

    renderFolder(folder, <span data-testid="folder-content">Catalog</span>);
    expect(screen.queryByTestId("folder-content")).toBeNull();
    expect(folderState("loops")).toBe("closed");
  });

  it("Should persist the current open state after a folder identity change", async () => {
    const loops = createFolder();
    const { rerender } = renderFolder(loops, <span>Catalog</span>);

    await waitFor(() => {
      expect(window.localStorage.getItem("compozy.docs.sidebar.folder:loops")).toBe("1");
    });

    mocks.pathname = "/docs/agents/providers";
    const agents = createFolder("agents", "providers");
    rerender(
      <SidebarHarness folder={agents}>
        <span>Providers</span>
      </SidebarHarness>
    );

    await waitFor(() => {
      expect(window.localStorage.getItem("compozy.docs.sidebar.folder:agents")).toBe("1");
    });
  });
});
