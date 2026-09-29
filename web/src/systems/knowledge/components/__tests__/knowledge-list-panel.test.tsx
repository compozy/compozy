import { UIProvider } from "@compozy/ui";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { KnowledgeMemoryItem } from "../../types";

import { KnowledgeListPanel } from "../knowledge-list-panel";

const PROFILE: KnowledgeMemoryItem = {
  filename: "user-role.md",
  key: "profile:user-role.md",
  mod_time: "2026-04-09T10:00:00Z",
  name: "User Role",
  scope: "profile",
  type: "user",
  recall_count: 2,
  injection: true,
  system_managed: false,
  description: "Guidance that shapes the assistant's tone and ownership.",
};

const WORKSPACE: KnowledgeMemoryItem = {
  filename: "project-context.md",
  key: "workspace:project-context.md",
  mod_time: "2026-04-09T08:00:00Z",
  name: "Project Context",
  scope: "workspace",
  type: "project",
  recall_count: 0,
  injection: true,
  system_managed: false,
  description: "Workspace-local notes about rollout.",
  workspace_id: "ws_launch",
};

const AGENT: KnowledgeMemoryItem = {
  filename: "cto-tone.md",
  key: "agent:cto-tone.md",
  mod_time: "2026-04-09T09:00:00Z",
  name: "CTO Tone",
  scope: "agent",
  agent_name: "cto",
  agent_tier: "workspace",
  workspace_id: "ws_launch",
  type: "user",
  recall_count: 5,
  injection: true,
  system_managed: false,
  staleness_banner: "Updated >7 days after last recall",
};

const ALL: KnowledgeMemoryItem[] = [PROFILE, WORKSPACE, AGENT];

function renderPanel(props: Partial<React.ComponentProps<typeof KnowledgeListPanel>> = {}) {
  const merged: React.ComponentProps<typeof KnowledgeListPanel> = {
    memories: ALL,
    onSearchChange: vi.fn(),
    onSelectMemory: vi.fn(),
    searchQuery: "",
    selectedMemoryKey: null,
    ...props,
  };
  return render(
    <UIProvider reducedMotion="never" skipAnimations>
      <KnowledgeListPanel {...merged} />
    </UIProvider>
  );
}

describe("KnowledgeListPanel", () => {
  it("Should render groups in scope order with count chips", () => {
    renderPanel();
    const groups = screen.getAllByTestId(/^knowledge-group-/).filter(element => {
      const id = element.getAttribute("data-testid") ?? "";
      return [
        "knowledge-group-profile",
        "knowledge-group-workspace",
        "knowledge-group-agent",
      ].includes(id);
    });
    expect(groups[0]).toHaveAttribute("data-testid", "knowledge-group-profile");
    expect(groups[1]).toHaveAttribute("data-testid", "knowledge-group-workspace");
    expect(groups[2]).toHaveAttribute("data-testid", "knowledge-group-agent");
    expect(
      within(screen.getByTestId("knowledge-group-header-profile")).getByText("1")
    ).toBeInTheDocument();
  });

  it("Should render a flat list without a group header when only one scope is present", () => {
    renderPanel({ memories: [PROFILE] });
    expect(screen.queryByTestId("knowledge-group-profile")).not.toBeInTheDocument();
    expect(screen.getByTestId("memory-item-profile:user-role.md")).toBeInTheDocument();
  });

  it("Should render the memory type as a plain word instead of the wire enum", () => {
    renderPanel();
    const userLabels = screen.getAllByTestId("type-badge-user");
    expect(userLabels.length).toBeGreaterThanOrEqual(2);
    for (const label of userLabels) {
      expect(label).toHaveTextContent("About you");
    }
    expect(screen.getByTestId("type-badge-project")).toHaveTextContent("Project decision");
  });

  it("Should limit each row to one warning signal and drop redundant scope pills", () => {
    renderPanel();
    expect(screen.queryByTestId(/^scope-badge-/)).not.toBeInTheDocument();
    expect(screen.queryByTestId(/^agent-tier-badge-/)).not.toBeInTheDocument();
    expect(screen.queryByTestId("agent-name-badge")).not.toBeInTheDocument();
    expect(screen.queryByTestId("recall-count-badge")).not.toBeInTheDocument();
    const stale = screen.getByTestId("staleness-badge");
    expect(stale).toHaveTextContent("Outdated");
    expect(stale).toHaveAttribute("data-tone", "warning");
  });

  it("Should show the empty fallback when there are no memories", () => {
    renderPanel({ memories: [] });
    const empty = screen.getByTestId("knowledge-list-empty");
    expect(empty).toBeInTheDocument();
    expect(within(empty).getByText("No knowledge yet", { selector: "h3" })).toBeInTheDocument();
  });

  it("Should show the loading state while loading and the list is empty", () => {
    renderPanel({ isLoading: true, memories: [] });
    expect(screen.getByTestId("knowledge-list-loading")).toBeInTheDocument();
  });

  it("Should show the error fallback when errorMessage is set and the list is empty", () => {
    renderPanel({ errorMessage: "Network failure", memories: [] });
    expect(screen.getByTestId("knowledge-list-error")).toBeInTheDocument();
    expect(screen.getByText("Network failure")).toBeInTheDocument();
  });

  it("Should expose search messaging through searchInfo and the no-matches empty state", () => {
    renderPanel({ searchMode: true, searchInfo: "0 matches", memories: [] });
    expect(screen.getByTestId("knowledge-search-info")).toHaveTextContent("0 matches");
    expect(screen.getByTestId("knowledge-list-empty")).toHaveTextContent("No matches");
  });

  it("Should emit onSearchChange with the typed query", () => {
    const onSearchChange = vi.fn();
    renderPanel({ onSearchChange });
    const input = screen.getByLabelText("Search knowledge");
    expect(input).toHaveAttribute("data-testid", "knowledge-search-input");
    expect(input).toHaveAttribute("placeholder", "Search knowledge");
    fireEvent.change(input, { target: { value: "alpha" } });
    expect(onSearchChange).toHaveBeenCalledWith("alpha");
  });

  it("Should emit onSelectMemory with the canonical key when a row is clicked", async () => {
    const user = userEvent.setup();
    const onSelectMemory = vi.fn();
    renderPanel({ onSelectMemory });
    await user.click(screen.getByTestId("memory-item-workspace:project-context.md"));
    expect(onSelectMemory).toHaveBeenCalledWith("workspace:project-context.md");
  });

  it("Should fall back to scope:filename when memory.key is missing", () => {
    renderPanel({
      memories: [
        PROFILE,
        {
          ...WORKSPACE,
          key: undefined,
        },
      ],
    });

    expect(screen.getByTestId("memory-item-workspace:project-context.md")).toBeInTheDocument();
  });

  it("Should render the selection indicator only on the selected row", () => {
    renderPanel({ selectedMemoryKey: "workspace:project-context.md" });
    const selected = screen.getByTestId("memory-item-workspace:project-context.md");
    expect(
      within(selected).getByText("Project Context").closest('[data-slot="item"]')
    ).toHaveAttribute("aria-pressed", "true");
    expect(
      selected.querySelector('[data-slot="item-selection-indicator"][data-indicator="rail"]')
    ).not.toBeNull();
    const unselected = screen.getByTestId("memory-item-profile:user-role.md");
    expect(
      unselected.querySelector('[data-slot="item-selection-indicator"][data-indicator="rail"]')
    ).toBeNull();
  });

  it("Should render row timestamps through the shared <Time> primitive", () => {
    renderPanel();
    const row = screen.getByTestId("memory-item-profile:user-role.md");
    const timeEl = row.querySelector("time[datetime]");
    expect(timeEl).not.toBeNull();
    expect(timeEl?.getAttribute("datetime")).toBe(PROFILE.mod_time);
  });

  it("Should expose an accessible loading-aware control for the next catalog page", async () => {
    const user = userEvent.setup();
    const onLoadMore = vi.fn();
    const { rerender } = renderPanel({ hasMore: true, onLoadMore, totalCount: 9 });

    const button = screen.getByRole("button", { name: "Load more knowledge" });
    await user.click(button);
    expect(onLoadMore).toHaveBeenCalledOnce();

    rerender(
      <UIProvider reducedMotion="never" skipAnimations>
        <KnowledgeListPanel
          hasMore
          isLoadingMore
          memories={ALL}
          onLoadMore={onLoadMore}
          onSearchChange={vi.fn()}
          onSelectMemory={vi.fn()}
          searchQuery=""
          selectedMemoryKey={null}
          totalCount={9}
        />
      </UIProvider>
    );
    expect(screen.getByRole("button", { name: "Loading more knowledge" })).toBeDisabled();
  });

  it("Should preserve loaded memories and expose retry after a continuation error", async () => {
    const user = userEvent.setup();
    const onLoadMore = vi.fn();
    const onRetry = vi.fn();
    renderPanel({
      errorMessage: "Next page unavailable",
      hasMore: true,
      onLoadMore,
      onRetry,
      totalCount: 9,
    });

    expect(screen.getByTestId("memory-item-profile:user-role.md")).toBeInTheDocument();
    expect(screen.getByTestId("knowledge-list-pagination-error")).toHaveTextContent(
      "Next page unavailable"
    );
    await user.click(screen.getByRole("button", { name: "Retry loading knowledge" }));
    expect(onRetry).toHaveBeenCalledOnce();
    expect(onLoadMore).not.toHaveBeenCalled();
    expect(screen.queryByRole("button", { name: "Load more knowledge" })).not.toBeInTheDocument();
  });

  it("Should preserve recall results and retry a failed recall refresh", async () => {
    const user = userEvent.setup();
    const onRetry = vi.fn();
    renderPanel({
      errorMessage: "Recall refresh failed",
      onRetry,
      searchMode: true,
      searchQuery: "operator",
    });

    expect(screen.getByTestId("memory-item-profile:user-role.md")).toBeInTheDocument();
    expect(screen.getByTestId("knowledge-list-pagination-error")).toHaveTextContent(
      "Recall refresh failed"
    );
    await user.click(screen.getByRole("button", { name: "Retry loading knowledge" }));
    expect(onRetry).toHaveBeenCalledTimes(1);
  });
});
