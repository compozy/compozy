// Suite: Jobs window catalog
// Invariant: Workspace automation suggestions remain visible after the jobs route moves into the OS window.
// Boundary IN: Jobs catalog composition for active workspace and explicit global scope.
// Boundary OUT: Suggestion fetching and actions, owned by automation hook/component suites.
import { screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { renderWithTopbar as render } from "@/test/render-with-topbar";

import { JobsCatalogLocation } from "../jobs-catalog-location";

const { jobsCatalog, jobsPage, suggestionPanel, workspaceContext } = vi.hoisted(() => ({
  jobsCatalog: vi.fn(),
  jobsPage: { current: {} as Record<string, unknown> },
  suggestionPanel: vi.fn(),
  workspaceContext: {
    current: {
      activeWorkspace: { id: "ws_test", name: "Test workspace" } as
        | { id: string; name: string }
        | undefined,
      activeWorkspaceId: "ws_test" as string | null,
    },
  },
}));

vi.mock("@/systems/automation", () => ({
  AutomationEditorDialog: () => null,
  AutomationJobsCatalog: (props: { isLoading: boolean; view: string }) => {
    jobsCatalog(props);
    return <div data-testid="automation-jobs-catalog" />;
  },
  AutomationListFilters: () => null,
  AutomationSuggestionsPanel: ({ workspaceID }: { workspaceID: string }) => {
    suggestionPanel(workspaceID);
    return <div data-testid="automation-suggestions-panel" />;
  },
}));

vi.mock("@/systems/workspace", () => ({
  useActiveWorkspace: () => workspaceContext.current,
}));

vi.mock("../../automation/use-automation-page", () => ({
  useAutomationJobsPage: () => jobsPage.current,
}));

beforeEach(() => {
  jobsCatalog.mockReset();
  suggestionPanel.mockReset();
  workspaceContext.current = {
    activeWorkspace: { id: "ws_test", name: "Test workspace" },
    activeWorkspaceId: "ws_test",
  };
  jobsPage.current = {
    clearFilters: vi.fn(),
    editorDialogProps: {},
    enabledFilter: undefined,
    error: null,
    errorMessage: null,
    handleCreate: vi.fn(),
    hasActiveFilters: false,
    hasNextPage: false,
    isFetchingNextPage: false,
    isLoading: false,
    jobs: [],
    loadMore: vi.fn(),
    onRunJob: vi.fn(),
    runDisabled: false,
    runPendingIds: new Set<string>(),
    runtimeUnavailableMessage: null,
    scopeFilter: "workspace",
    searchQuery: "",
    setEnabledFilter: vi.fn(),
    setScopeFilter: vi.fn(),
    setSearchQuery: vi.fn(),
    setSourceFilter: vi.fn(),
    setView: vi.fn(),
    sourceFilter: undefined,
    total: 0,
    view: "list",
  };
});

describe("JobsCatalogLocation", () => {
  it("Should show suggestions only for an active workspace in a non-global catalog", () => {
    const { rerender } = render(<JobsCatalogLocation search={{}} />);

    expect(screen.getByTestId("automation-suggestions-panel")).toBeInTheDocument();
    expect(suggestionPanel).toHaveBeenLastCalledWith("ws_test");

    rerender(<JobsCatalogLocation search={{ scope: "global" }} />);
    expect(screen.queryByTestId("automation-suggestions-panel")).not.toBeInTheDocument();

    workspaceContext.current = { activeWorkspace: undefined, activeWorkspaceId: null };
    rerender(<JobsCatalogLocation search={{}} />);
    expect(screen.queryByTestId("automation-suggestions-panel")).not.toBeInTheDocument();
  });

  it("Should delegate card-mode loading geometry to the automation catalog", () => {
    jobsPage.current = { ...jobsPage.current, isLoading: true, view: "cards" };

    render(<JobsCatalogLocation search={{ view: "cards" }} />);

    expect(screen.getByTestId("automation-jobs-catalog")).toBeInTheDocument();
    expect(jobsCatalog).toHaveBeenLastCalledWith(
      expect.objectContaining({ isLoading: true, view: "cards" })
    );
    expect(screen.queryByTestId("automation-suggestions-panel")).not.toBeInTheDocument();
  });
});
