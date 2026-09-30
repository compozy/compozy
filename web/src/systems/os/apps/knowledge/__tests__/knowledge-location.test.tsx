// Suite: Knowledge window location adapter
// Invariant: the external-store selector returns the stored route search reference;
// parsing that search must not create a fresh getSnapshot value and loop React renders, and the
// head keeps the root identity while a memory is selected beside the list (master–detail).
// Owning layer: Knowledge's OS-window route adapter.
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { Topbar, TopbarSlotProvider } from "@compozy/ui";

vi.mock("../../../hooks/use-desktop", async () => {
  const { useSyncExternalStore } = await import("react");
  const state = {
    windows: {
      "window:knowledge": {
        route: {
          pathname: "/knowledge",
          search: { memory: "operator-notes.md", scope: "global" },
        },
      },
    },
  };
  const subscribe = () => () => undefined;
  return {
    useDesktop: (selector: (value: typeof state) => unknown) =>
      useSyncExternalStore(
        subscribe,
        () => selector(state),
        () => selector(state)
      ),
  };
});

const pageMock = vi.hoisted(() => ({ selectedMemory: null as null | { name: string } }));

vi.mock("../use-knowledge-page", () => ({
  useKnowledgePage: vi.fn(() => ({
    activeScope: "global",
    canCreateMemory: false,
    guard: { title: "Open a project first", description: "Open a project." },
    selectedMemory: pageMock.selectedMemory,
    setActiveScope: vi.fn(),
    setCreateOpen: vi.fn(),
    setSelectedMemoryKey: vi.fn(),
  })),
}));

import { KnowledgeLocation } from "../knowledge-location";

describe("KnowledgeLocation", () => {
  it("Should render from a stable window search snapshot", () => {
    render(
      <TopbarSlotProvider>
        <KnowledgeLocation windowId="window:knowledge" />
      </TopbarSlotProvider>
    );

    expect(screen.getByTestId("knowledge-guard")).toBeInTheDocument();
  });

  it("Should keep the root head while a memory is selected beside the list", () => {
    pageMock.selectedMemory = { name: "operator-notes" };
    render(
      <TopbarSlotProvider>
        <Topbar title="Fallback" />
        <KnowledgeLocation windowId="window:knowledge" />
      </TopbarSlotProvider>
    );

    expect(screen.getByRole("heading", { level: 1, name: "Knowledge" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Back one level" })).toBeNull();
    expect(document.querySelector('[data-slot="topbar-crumbs"]')).toBeNull();
    pageMock.selectedMemory = null;
  });
});
