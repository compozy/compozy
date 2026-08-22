import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
// Suite: Jobs window catalog
// Invariant: Automation suggestions reach the UI only through the unfiltered
// zero-inventory empty state — gated by a runtime workspace and non-global
// scope; loading, filtered-empty, and populated catalogs never render them.
// Boundary IN: Jobs catalog composition for active workspace and explicit global scope.
// Boundary OUT: Suggestion fetching and actions, owned by automation hook/component suites.
import { screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
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
      runtimeWorkspaceId: "ws_test" as string | null,
    },
  },
}));

vi.mock("@/systems/automation/components/automation-editor-dialog", () => ({
  AutomationEditorDialog: () => null,
}));

vi.mock("@/systems/automation/components/automation-jobs-catalog", () => ({
  AutomationJobsCatalog: (props: {
    isLoading: boolean;
    onCreate: () => void;
    unfilteredEmptyPanel?: ReactNode;
    view: string;
  }) => {
    jobsCatalog(props);
    return (
      <div data-testid="automation-jobs-catalog">
        <button data-testid="automation-jobs-catalog-create" onClick={props.onCreate} type="button">
          Create
        </button>
        {props.unfilteredEmptyPanel}
      </div>
    );
  },
}));

vi.mock("@/systems/automation/components/automation-list-filters", () => ({
  AutomationListFilters: () => null,
}));

vi.mock("@/systems/automation/components/automation-suggestions-panel", () => ({
  AutomationSuggestionsPanel: ({ workspaceID }: { workspaceID: string }) => {
    suggestionPanel(workspaceID);
    return <div data-testid="automation-suggestions-panel" />;
  },
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
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
    runtimeWorkspaceId: "ws_test",
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

/** The catalog resolves the active profile, which is a server read. */
const jobsQueryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

function withQueryClient(node: ReactNode) {
  return <QueryClientProvider client={jobsQueryClient}>{node}</QueryClientProvider>;
}

describe("JobsCatalogLocation", () => {
  it("Should hand zero-inventory suggestions to the empty state unless scope is Global or no runtime workspace exists", () => {
    const { rerender } = render(withQueryClient(<JobsCatalogLocation search={{}} />));

    expect(screen.getByTestId("automation-suggestions-panel")).toBeInTheDocument();
    expect(suggestionPanel).toHaveBeenLastCalledWith("ws_test");

    rerender(withQueryClient(<JobsCatalogLocation search={{ scope: "global" }} />));
    expect(screen.queryByTestId("automation-suggestions-panel")).not.toBeInTheDocument();

    workspaceContext.current = {
      activeWorkspace: undefined,
      activeWorkspaceId: null,
      runtimeWorkspaceId: "ws_home",
    };
    rerender(withQueryClient(<JobsCatalogLocation search={{}} />));
    expect(suggestionPanel).toHaveBeenLastCalledWith("ws_home");

    workspaceContext.current = {
      activeWorkspace: undefined,
      activeWorkspaceId: null,
      runtimeWorkspaceId: null,
    };
    rerender(withQueryClient(<JobsCatalogLocation search={{}} />));
    expect(screen.queryByTestId("automation-suggestions-panel")).not.toBeInTheDocument();
  });

  it("Should never render suggestions once the catalog holds inventory", () => {
    jobsPage.current = { ...jobsPage.current, jobs: [], total: 12 };

    render(withQueryClient(<JobsCatalogLocation search={{}} />));

    expect(screen.queryByTestId("automation-suggestions-panel")).not.toBeInTheDocument();
    expect(jobsCatalog).toHaveBeenLastCalledWith(
      expect.objectContaining({ unfilteredEmptyPanel: null })
    );
  });

  it("Should withhold suggestions from the filtered-empty state", () => {
    jobsPage.current = { ...jobsPage.current, hasActiveFilters: true, total: 0 };

    render(withQueryClient(<JobsCatalogLocation search={{}} />));

    expect(screen.queryByTestId("automation-suggestions-panel")).not.toBeInTheDocument();
    expect(jobsCatalog).toHaveBeenLastCalledWith(
      expect.objectContaining({ unfilteredEmptyPanel: null })
    );
  });

  it("Should delegate card-mode loading geometry to the automation catalog", () => {
    jobsPage.current = { ...jobsPage.current, isLoading: true, view: "cards" };

    render(withQueryClient(<JobsCatalogLocation search={{ view: "cards" }} />));

    expect(screen.getByTestId("automation-jobs-catalog")).toBeInTheDocument();
    expect(jobsCatalog).toHaveBeenLastCalledWith(
      expect.objectContaining({ isLoading: true, view: "cards" })
    );
    expect(screen.queryByTestId("automation-suggestions-panel")).not.toBeInTheDocument();
  });

  it("Should pass the route create action through the catalog shell", async () => {
    const user = userEvent.setup();
    const handleCreate = vi.fn();
    jobsPage.current = { ...jobsPage.current, handleCreate };

    render(withQueryClient(<JobsCatalogLocation search={{}} />));
    await user.click(screen.getByTestId("automation-jobs-catalog-create"));

    expect(handleCreate).toHaveBeenCalledTimes(1);
  });
});
