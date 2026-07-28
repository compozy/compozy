// Suite: automation suggestion consent card
// Invariant: a proposal remains actionable until its server-backed action succeeds.
// Boundary IN: suggestion card rendering and operator actions.
// Boundary OUT: TanStack mutations and HTTP adapters, covered by their canonical suites.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { AutomationSuggestionsCard } from "../automation-suggestions-card";
import { AutomationSuggestionsPanel } from "../automation-suggestions-panel";
import type { AutomationSuggestion } from "../../types";

const suggestionHooks = vi.hoisted(() => ({
  accept: { mutateAsync: vi.fn() },
  dismiss: { mutateAsync: vi.fn() },
  query: {
    current: {
      data: undefined as { suggestions: AutomationSuggestion[] } | undefined,
      isError: false,
      isPending: false,
      refetch: vi.fn(),
    },
  },
}));

vi.mock("../../hooks/use-automation-suggestion-actions", () => ({
  useAcceptAutomationSuggestion: () => suggestionHooks.accept,
  useDismissAutomationSuggestion: () => suggestionHooks.dismiss,
}));

vi.mock("../../hooks/use-automation-suggestions", () => ({
  useAutomationSuggestions: () => suggestionHooks.query.current,
}));

const suggestion: AutomationSuggestion = {
  created_at: "2026-07-18T12:00:00Z",
  dedup_key: "daily-review",
  id: "suggestion_daily_review",
  payload: {
    agent_name: "reviewer",
    created_at: "2026-07-18T12:00:00Z",
    enabled: true,
    fire_limit: { max: 12, window: "1h" },
    id: "job_daily_review",
    name: "Daily review",
    prompt: "Review changes merged since the previous run.",
    retry: { base_delay: "2s", max_retries: 3, strategy: "none" },
    schedule: { expr: "0 9 * * *", mode: "cron" },
    scope: "workspace",
    source: "dynamic",
    target_kind: "prompt",
    updated_at: "2026-07-18T12:00:00Z",
    workspace_id: "ws_alpha",
  },
  source: "catalog",
  status: "pending",
  workspace_id: "ws_alpha",
};

describe("AutomationSuggestionsCard", () => {
  it("explains the proposed consequence and exposes explicit consent actions", () => {
    const onAccept = vi.fn();
    const onDismiss = vi.fn();
    render(
      <AutomationSuggestionsCard
        onAccept={onAccept}
        onDismiss={onDismiss}
        suggestions={[suggestion]}
      />
    );

    expect(screen.getByText("Daily review")).toBeInTheDocument();
    expect(screen.getByTestId(`automation-suggestion-${suggestion.id}`)).toHaveTextContent(
      /Creates an enabled workspace Job for reviewer\. Schedule: Cron 0 9/
    );
    expect(screen.getByText(/Review changes merged since the previous run/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Create job" }));
    fireEvent.click(screen.getByRole("button", { name: "Dismiss" }));
    expect(onAccept).toHaveBeenCalledWith(suggestion.id);
    expect(onDismiss).toHaveBeenCalledWith(suggestion.id);
  });

  it("keeps a failed proposal visible and lets the operator retry the same action", () => {
    const onAccept = vi.fn();
    render(
      <AutomationSuggestionsCard
        actionErrors={{
          [suggestion.id]: "Couldn't create this job. Review the proposal and try again.",
        }}
        onAccept={onAccept}
        onDismiss={vi.fn()}
        suggestions={[suggestion]}
      />
    );

    expect(screen.getByTestId(`automation-suggestion-${suggestion.id}`)).toBeInTheDocument();
    expect(screen.getByRole("alert")).toHaveTextContent("Couldn't create this job");
    fireEvent.click(screen.getByRole("button", { name: "Create job" }));
    expect(onAccept).toHaveBeenCalledWith(suggestion.id);
  });

  it("shows the known server reason when creating a suggested job fails", async () => {
    suggestionHooks.query.current = {
      data: { suggestions: [suggestion] },
      isError: false,
      isPending: false,
      refetch: vi.fn(),
    };
    suggestionHooks.accept.mutateAsync.mockRejectedValueOnce(
      new Error("An automation lifecycle command is already running.")
    );

    render(<AutomationSuggestionsPanel workspaceID="ws_alpha" />);
    fireEvent.click(screen.getByRole("button", { name: "Create job" }));

    expect(
      await screen.findByText("An automation lifecycle command is already running.")
    ).toHaveAttribute("role", "alert");
    expect(screen.getByTestId(`automation-suggestion-${suggestion.id}`)).toBeInTheDocument();
  });

  it("disables both row actions while one server action is pending", () => {
    render(
      <AutomationSuggestionsCard
        onAccept={vi.fn()}
        onDismiss={vi.fn()}
        pendingActions={{ [suggestion.id]: "accept" }}
        suggestions={[suggestion]}
      />
    );

    expect(screen.getByRole("button", { name: "Create job" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Dismiss" })).toBeDisabled();
    expect(screen.getByTestId(`automation-suggestion-${suggestion.id}`)).toHaveAttribute(
      "aria-busy",
      "true"
    );
  });

  it("matches the card geometry while loading, recovers list errors, and hides empty results", () => {
    const onRetry = vi.fn();
    const { rerender } = render(
      <AutomationSuggestionsCard
        isLoading
        onAccept={vi.fn()}
        onDismiss={vi.fn()}
        suggestions={[]}
      />
    );
    expect(screen.getByTestId("automation-suggestions-loading")).toHaveAttribute(
      "aria-busy",
      "true"
    );

    rerender(
      <AutomationSuggestionsCard
        errorMessage="Couldn't load automation suggestions. Retry the request."
        onAccept={vi.fn()}
        onDismiss={vi.fn()}
        onRetry={onRetry}
        suggestions={[]}
      />
    );
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetry).toHaveBeenCalledOnce();

    rerender(<AutomationSuggestionsCard onAccept={vi.fn()} onDismiss={vi.fn()} suggestions={[]} />);
    expect(screen.queryByTestId("automation-suggestions-card")).not.toBeInTheDocument();
  });
});
