// Suite: palette domain view
// Invariant: a domain view with nothing to list and its read still out holds the rows' shape under
// one named busy region; once the read answers empty, the view says so in words.
// Owning layer: use-os-palette-domain-view.tsx. Canonical suite: this file.
// Boundary OUT: the domain reads themselves (use-os-palette-domain-search).
import { render, renderHook, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { OsPaletteDomainSection } from "../use-os-palette-domain-search";

const harness = vi.hoisted(() => ({
  sections: [] as OsPaletteDomainSection[],
  signalsLoading: false,
}));

vi.mock("@/systems/workspace", async importOriginal => ({
  ...(await importOriginal<typeof import("@/systems/workspace")>()),
  useActiveWorkspace: () => ({
    dataScope: { kind: "all" as const },
    runtimeWorkspaceId: "ws-home",
    workspaces: [],
  }),
}));

vi.mock("../use-cmd-palette-rank-signals", () => ({
  useCmdPaletteRankSignals: () => ({ data: null, loading: harness.signalsLoading }),
}));

vi.mock("../use-os-palette-domain-search", async importOriginal => ({
  ...(await importOriginal<typeof import("../use-os-palette-domain-search")>()),
  useOsPaletteDomainSearch: () => harness.sections,
}));

import { PALETTE_VIEWS } from "../../lib/palette-view-registry";
import { useOsPaletteDomainView } from "../use-os-palette-domain-view";

function tasksView() {
  const { result } = renderHook(() =>
    useOsPaletteDomainView(PALETTE_VIEWS.tasks!, {
      query: "",
      onDismiss: vi.fn(),
      openDomainRow: vi.fn(),
    })
  );
  return result.current;
}

describe("useOsPaletteDomainView", () => {
  beforeEach(() => {
    harness.sections = [];
    harness.signalsLoading = false;
  });

  it("Should hold the rows' shape in one busy region while the first read is out", () => {
    harness.sections = [{ title: "Tasks", rows: [], total: 0, loading: true, error: null }];

    render(<>{tasksView().empty}</>);

    expect(screen.getByRole("status")).toHaveTextContent("Loading tasks");
    expect(screen.queryByText("Loading tasks…")).not.toBeInTheDocument();
  });

  it("Should say so in words once the read answers with nothing", () => {
    harness.sections = [{ title: "Tasks", rows: [], total: 0, loading: false, error: null }];

    render(<>{tasksView().empty}</>);

    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(screen.getByText(/no tasks/i)).toBeInTheDocument();
  });
});
