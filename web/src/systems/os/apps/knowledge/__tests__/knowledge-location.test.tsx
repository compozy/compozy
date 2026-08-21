// Suite: Knowledge window location adapter
// Invariant: the external-store selector returns the stored route search reference;
// parsing that search must not create a fresh getSnapshot value and loop React renders.
// Owning layer: Knowledge's OS-window route adapter.
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TopbarSlotProvider } from "@compozy/ui";

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

vi.mock("../use-knowledge-page", () => ({
  useKnowledgePage: vi.fn(() => ({
    activeScope: "global",
    canCreateMemory: false,
    guardMessage: "Choose a workspace.",
    selectedMemory: null,
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
});
