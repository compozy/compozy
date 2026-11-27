// Suite: Separate computer sections, states, headers, drag refusal, search hiding and Not counted footer (T-F7)
// Invariant: Each computer section renders its truthful read state (S1-S6, Q9), never fabricates
//            a zero count for failed/connecting/offline_empty computers (S11), folds offline/empty
//            computers by default (C6, S3), suppresses `+` without sessions:operate (F11) or local
//            execution (S12), hides zero-match online/offline sections during search (Opus 6, Astra 15),
//            and refuses real cross-computer row drags with the canonical message (F7, Astra 21).
// Owning layer: component (systems/session/components/session-list)
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { LOCAL_COMPUTER_ID } from "@/lib/computer-scope";
import { LOCAL_COMPUTER } from "@/systems/computers/mocks/remote-computers-fixtures";

import type { ComputerSessionsSource } from "../../hooks/use-computer-sessions-sources";
import { useSessionListDnd } from "../../hooks/use-session-list-dnd";
import { buildComputerPopulationFilters } from "../../lib/session-list-computer-filter";
import { sessionKeys } from "../../lib/query-keys";
import { primarySessionFixture } from "../../mocks/fixtures";
import { sessionLifecycleActionsFixture } from "../../mocks/session-row-fixtures";
import type { SessionCatalogPageResponse, SessionPayload } from "../../types";
import { SessionListComputerSection } from "../session-list/session-list-computer-section";
import { SessionListTotals } from "../session-list/session-list-totals";

const EMPTY_SET = new Set<string>();

function makeSource(
  overrides: Partial<ComputerSessionsSource> & { key: string; name: string }
): ComputerSessionsSource {
  const isLocal = overrides.key === LOCAL_COMPUTER_ID;
  return {
    isLocal,
    computer: LOCAL_COMPUTER,
    read: "ok",
    detail: isLocal ? "This computer" : "via LAN",
    canOperate: true,
    canCreate: true,
    facets: undefined,
    count: 2,
    needsYou: 0,
    working: 0,
    laterCount: 0,
    workspaces: [],
    workspacesLoading: false,
    workspacesError: false,
    matchedWorkspaceIds: null,
    retry: vi.fn(),
    ...overrides,
  };
}

function renderSection(
  source: ComputerSessionsSource,
  options: {
    search?: string;
    onOpenComputerDetails?: (id: string) => void;
    client?: QueryClient;
  } = {}
) {
  const client =
    options.client ?? new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const onNewSession = vi.fn();
  const result = render(
    <QueryClientProvider client={client}>
      <SessionListComputerSection
        collapsedThreads={EMPTY_SET}
        groupBy="project"
        onNewSession={onNewSession}
        onOpenComputerDetails={options.onOpenComputerDetails}
        onSelectSession={vi.fn()}
        onThreadBranchChange={vi.fn()}
        search={options.search ?? ""}
        sessionActions={sessionLifecycleActionsFixture()}
        source={source}
        subagentThreads={{ has: () => false }}
        testIdPrefix="test"
      />
    </QueryClientProvider>
  );
  return { ...result, client, onNewSession };
}

function LocalDragSourceRow() {
  const dnd = useSessionListDnd(
    {
      folders: [],
      moveSession: vi.fn(),
      reorderFolders: vi.fn(),
      reorderSessions: vi.fn(),
    },
    { manual: true, reorderable: true, computerId: LOCAL_COMPUTER_ID }
  );
  const handlers = dnd.sessionRow(
    "local_sess_1",
    { folderId: null, ids: ["local_sess_1"] },
    undefined,
    undefined,
    false,
    LOCAL_COMPUTER_ID
  );
  return (
    <div data-testid="local-drag-row" {...handlers}>
      Local session
    </div>
  );
}

function createDataTransfer() {
  return {
    dropEffect: "move",
    effectAllowed: "all",
    setData: vi.fn(),
    getData: vi.fn(() => ""),
  };
}

describe("SessionListComputerSection and Not counted footer (T-F7)", () => {
  it("Should render failed, needs_pro, incompatible and offline_empty notes without fabricating counts", () => {
    const failed = makeSource({
      key: "cmp_fail",
      name: "devbox",
      read: "failed",
      count: null,
    });
    renderSection(failed);
    expect(screen.getByText(/Couldn’t load sessions from devbox/i)).toBeInTheDocument();
    const retry = screen.getByRole("button", { name: "Retry" });
    fireEvent.click(retry);
    expect(failed.retry).toHaveBeenCalledTimes(1);

    const needsPro = makeSource({
      key: "cmp_pro",
      name: "mini-build",
      read: "needs_pro",
      detail: "Needs Pro",
      count: null,
      canOperate: false,
      canCreate: false,
    });
    renderSection(needsPro);
    expect(
      screen.getByText("Pro lists sessions from more than one remote computer.")
    ).toBeInTheDocument();

    const onDetails = vi.fn();
    const incompatible = makeSource({
      key: "cmp_old",
      name: "gpu-vm",
      read: "incompatible",
      detail: "Update needed",
      count: null,
      canOperate: false,
      canCreate: false,
    });
    renderSection(incompatible, { onOpenComputerDetails: onDetails });
    expect(
      screen.getByText("Update CompozyOS on gpu-vm to list its sessions.")
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Details" }));
    expect(onDetails).toHaveBeenCalledWith("cmp_old");

    // Offline empty defaults to folded (C6/S3) and offers Reconnect, never 0 count.
    const offlineEmpty = makeSource({
      key: "cmp_off",
      name: "homelab",
      read: "offline_empty",
      detail: "Offline · 2h ago",
      count: null,
      canOperate: false,
      canCreate: false,
    });
    renderSection(offlineEmpty);
    const offlineSection = screen.getByTestId("test-computer-cmp_off");
    expect(offlineSection).toHaveAttribute("data-collapsed", "true");
    expect(screen.getByRole("button", { name: "Reconnect" })).toBeInTheDocument();
  });

  it("Should suppress + when canCreate is false (S12 / F11) and refuse a real cross-computer row drag (F7 / Astra 21)", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const noOperate = makeSource({
      key: "cmp_ro",
      name: "readonly-box",
      read: "ok",
      count: 1,
      canOperate: false,
      canCreate: false,
    });
    render(
      <QueryClientProvider client={client}>
        <LocalDragSourceRow />
        <SessionListComputerSection
          collapsedThreads={EMPTY_SET}
          groupBy="project"
          onNewSession={vi.fn()}
          onSelectSession={vi.fn()}
          onThreadBranchChange={vi.fn()}
          search=""
          sessionActions={sessionLifecycleActionsFixture()}
          source={noOperate}
          subagentThreads={{ has: () => false }}
          testIdPrefix="test"
        />
      </QueryClientProvider>
    );
    expect(screen.queryByRole("button", { name: /New session/i })).not.toBeInTheDocument();

    const dragRow = screen.getByTestId("local-drag-row");
    const section = screen.getByTestId("test-computer-cmp_ro");
    const dataTransfer = createDataTransfer();

    fireEvent.dragStart(dragRow, { dataTransfer });
    fireEvent.dragOver(section, { dataTransfer });
    expect(screen.getByTestId("test-computer-cmp_ro-drop-refused")).toHaveTextContent(
      "Sessions stay on the computer that runs them."
    );
    expect(dataTransfer.dropEffect).toBe("none");

    fireEvent.dragEnd(dragRow, { dataTransfer });
    expect(screen.queryByTestId("test-computer-cmp_ro-drop-refused")).not.toBeInTheDocument();
  });

  it("Should hide online zero-match sections and filter offline sections from cached rows during search (Opus 6 / Astra 15)", () => {
    const onlineZero = makeSource({
      key: "cmp_zero",
      name: "empty-box",
      read: "ok",
      count: 0,
    });
    const { container } = renderSection(onlineZero, { search: "webhook" });
    expect(container).toBeEmptyDOMElement();

    const offlineClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const cachedPage: Omit<SessionCatalogPageResponse, "sessions"> & {
      sessions: SessionPayload[];
    } = {
      sessions: [
        {
          ...primarySessionFixture,
          id: "off_1",
          name: "Retry SPEI webhook",
          computerId: "cmp_off",
        },
        { ...primarySessionFixture, id: "off_2", name: "Vacuum archive", computerId: "cmp_off" },
      ],
      page: { has_more: false, next_cursor: "", limit: 50, total: 2 },
    };
    offlineClient.setQueryData(
      sessionKeys.forComputer("cmp_off").catalogPage({
        all_workspaces: true,
        all_profiles: true,
        ...buildComputerPopulationFilters({
          shelf: "active",
          showAutomation: false,
          excludePinned: true,
          search: "webhook",
        }),
      }),
      cachedPage
    );

    const unrelated = [
      { id: "archived-match", filters: { archive: "only" as const } },
      { id: "later-match", filters: { later: "only" as const } },
      { id: "other-worktree", filters: { worktree: "different-worktree" } },
    ];
    for (const { id, filters } of unrelated) {
      offlineClient.setQueryData(
        sessionKeys.forComputer("cmp_off").catalogPage({
          all_workspaces: true,
          all_profiles: true,
          ...buildComputerPopulationFilters({
            shelf: "active",
            showAutomation: false,
            excludePinned: true,
            search: "webhook",
          }),
          ...filters,
        }),
        { ...cachedPage, sessions: [{ ...primarySessionFixture, id, name: "Unrelated webhook" }] }
      );
    }

    const offlineSource = makeSource({
      key: "cmp_off",
      name: "workstation",
      read: "offline",
      detail: "Offline · 2h ago",
      count: 2,
      canOperate: false,
      canCreate: false,
    });

    const matchRender = renderSection(offlineSource, {
      search: "webhook",
      client: offlineClient,
    });
    expect(matchRender.getByTestId("test-computer-cmp_off")).toBeInTheDocument();
    expect(
      matchRender.getByTestId("test-computer-cmp_off-offline-search-note")
    ).toBeInTheDocument();
    expect(matchRender.queryByText("Unrelated webhook")).not.toBeInTheDocument();
    matchRender.unmount();

    const noMatchRender = renderSection(offlineSource, {
      search: "definitely-absent",
      client: offlineClient,
    });
    expect(noMatchRender.container).toBeEmptyDOMElement();
  });

  it("Should render Not counted footer naming unanswered computers (R9)", () => {
    render(
      <SessionListTotals
        testIdPrefix="test"
        totals={{ needsYou: 2, working: 1, idle: 4 }}
        uncountedNames={["devbox", "homelab"]}
      />
    );
    expect(screen.getByTestId("test-not-counted")).toHaveTextContent(
      "Not counted: devbox, homelab"
    );
  });
});
