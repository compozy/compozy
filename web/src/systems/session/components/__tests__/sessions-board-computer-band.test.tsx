// Suite: Kanban computer bands, folded facet counts and failure states (T-F13)
// Invariant: Folded bands render all 5 column counts from facets.board without issuing column
//            reads (K3, K5); failed computers render inside their own band under Separate (K6)
//            and omit counts rather than fabricating 0; offline bands dim.
// Owning layer: component (systems/session/components/session-board)
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { LOCAL_COMPUTER_ID } from "@/lib/computer-scope";
import { LOCAL_COMPUTER_FIXTURE } from "@/systems/computers/mocks/remote-computers-fixtures";

import type { ComputerSessionsSource } from "../../hooks/use-computer-sessions-sources";
import type { SessionsBoardComputerBand } from "../../hooks/use-sessions-board-computers";
import { SessionsBoardComputerBandView } from "../session-board/sessions-board-computer-band";
import type { SessionsBoardRow } from "../session-board/sessions-board-view";

function makeSource(
  overrides: Partial<ComputerSessionsSource> & { key: string; name: string }
): ComputerSessionsSource {
  const isLocal = overrides.key === LOCAL_COMPUTER_ID;
  return {
    isLocal,
    computer: LOCAL_COMPUTER_FIXTURE,
    read: "ok",
    detail: isLocal ? "This computer" : "via LAN",
    canOperate: true,
    canCreate: true,
    facets: undefined,
    count: 6,
    needsYou: 2,
    working: 1,
    laterCount: 0,
    workspaces: [],
    workspacesLoading: false,
    workspacesError: false,
    matchedWorkspaceIds: null,
    retry: vi.fn(),
    ...overrides,
  };
}

const BASE_ROW: SessionsBoardRow = {
  currentSessionId: null,
  collapsedColumns: {},
  onColumnCollapsedChange: vi.fn(),
  onOpen: vi.fn(),
  workspaceNameOf: () => null,
};

function renderBand(band: SessionsBoardComputerBand) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <SessionsBoardComputerBandView band={band} rank={0} row={BASE_ROW} />
    </QueryClientProvider>
  );
}

describe("SessionsBoardComputerBandView (T-F13)", () => {
  it("Should show all five column facet counts and needs-you signal on a folded band (K3, K5)", () => {
    const onToggle = vi.fn();
    const source = makeSource({ key: "cmp_devbox", name: "devbox", count: 8, needsYou: 2 });
    renderBand({
      source,
      collapsed: true,
      onToggle,
      baseFilters: {},
      lanes: null,
      boardCounts: {
        needs_you: 2,
        working: 1,
        waiting: 1,
        in_review: 1,
        done: 3,
      },
      workspaceNameOf: () => null,
    });

    expect(screen.getByTestId("sessions-board-band-cmp_devbox")).toHaveAttribute(
      "data-collapsed",
      "true"
    );
    expect(screen.getByTestId("sessions-board-band-cmp_devbox-count")).toHaveTextContent("8");
    const folded = screen.getByTestId("sessions-board-band-cmp_devbox-folded-counts");
    expect(folded).toHaveTextContent("Needs you");
    expect(folded).toHaveTextContent("Working");
    expect(folded).toHaveTextContent("Waiting");
    expect(folded).toHaveTextContent("In review");
    expect(folded).toHaveTextContent("Done");
    expect(folded).toHaveTextContent("3");

    fireEvent.click(screen.getByTestId("sessions-board-band-cmp_devbox-header"));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it("Should render failed computer inline inside its band with Retry and no fabricated count (K6)", () => {
    const retry = vi.fn();
    const source = makeSource({
      key: "cmp_fail",
      name: "hetzner",
      read: "failed",
      count: null,
      needsYou: 0,
      retry,
    });
    renderBand({
      source,
      collapsed: false,
      onToggle: vi.fn(),
      baseFilters: {},
      lanes: null,
      boardCounts: undefined,
      workspaceNameOf: () => null,
    });

    expect(screen.queryByTestId("sessions-board-band-cmp_fail-count")).not.toBeInTheDocument();
    expect(screen.getByText(/Couldn’t load sessions from hetzner/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(retry).toHaveBeenCalledTimes(1);
  });
});
