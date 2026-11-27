// Suite: Separate computer sections, states, headers, drag refusal and Not counted footer (T-F7)
// Invariant: Each computer section renders its truthful read state (S1-S6, Q9), never fabricates
//            a zero count for failed/connecting/offline_empty computers (S11), folds offline/empty
//            computers by default (C6, S3), suppresses `+` without sessions:operate (F11) or local
//            execution (S12), and surfaces cross-computer drag refusal (F7).
// Owning layer: component (systems/session/components/session-list)
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { LOCAL_COMPUTER_ID } from "@/lib/computer-scope";
import { LOCAL_COMPUTER_FIXTURE } from "@/systems/computers/mocks/remote-computers-fixtures";

import type { ComputerSessionsSource } from "../../hooks/use-computer-sessions-sources";
import { sessionDragComputerStore } from "../../lib/session-row-ref";
import { sessionLifecycleActionsFixture } from "../../mocks/session-row-fixtures";
import { SessionListComputerSection } from "../session-list/session-list-computer-section";
import { SessionListTotals } from "../session-list/session-list-totals";

const EMPTY_SET = new Set<string>();

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
  options: { search?: string; onOpenComputerDetails?: (id: string) => void } = {}
) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
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
  return { ...result, onNewSession };
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
    expect(screen.getByText("Update Compozy on gpu-vm to list its sessions.")).toBeInTheDocument();
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

  it("Should suppress + when canCreate is false (S12 / F11) and show cross-computer drag refusal (F7)", () => {
    const noOperate = makeSource({
      key: "cmp_ro",
      name: "readonly-box",
      read: "ok",
      count: 1,
      canOperate: false,
      canCreate: false,
    });
    renderSection(noOperate);
    expect(screen.queryByRole("button", { name: /New session/i })).not.toBeInTheDocument();

    // Dragging from another computer refuses drops with the canonical message.
    act(() => {
      sessionDragComputerStore.trigger.dragStarted({ computerId: LOCAL_COMPUTER_ID });
    });
    const section = screen.getByTestId("test-computer-cmp_ro");
    fireEvent.dragOver(section);
    expect(screen.getByTestId("test-computer-cmp_ro-drop-refused")).toHaveTextContent(
      "Sessions stay on the computer that runs them."
    );
    act(() => {
      sessionDragComputerStore.trigger.dragEnded();
    });
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
