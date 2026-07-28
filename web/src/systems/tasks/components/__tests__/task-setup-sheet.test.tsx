import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  buildTaskExecutionProfileFixture,
  taskExecutionProfileFixture,
} from "../../mocks/fixtures";
import type { TaskProfileEditor } from "../../hooks/use-profile-editor";
import { TaskSetupSheet } from "../task-setup-sheet";

const runtime = {
  catalogLoaded: true,
  catalogLoading: false,
  catalogRefreshing: false,
  models: [],
  onRefreshCatalog: vi.fn(),
  providers: [],
  providersLoading: false,
};

function buildEditor(overrides: Partial<TaskProfileEditor> = {}): TaskProfileEditor {
  return {
    open: false,
    setOpen: vi.fn(),
    setValue: vi.fn(),
    submit: vi.fn().mockResolvedValue(undefined),
    value: null,
    ...overrides,
  };
}

describe("TaskSetupSheet", () => {
  it("Should lock setup mutations while a run is active", () => {
    const editor = buildEditor();
    render(
      <TaskSetupSheet
        clearOpen={false}
        editor={editor}
        hasActiveRun
        onClearOpenChange={vi.fn()}
        onClearProfile={vi.fn()}
        onOpenChange={vi.fn()}
        open
        profile={taskExecutionProfileFixture}
        runtime={runtime}
      />
    );

    expect(screen.getByTestId("tasks-setup-locked")).toHaveTextContent(
      "Editing is locked while a run is active"
    );
    expect(screen.queryByTestId("tasks-setup-edit")).toBeNull();
    expect(screen.queryByTestId("tasks-setup-clear")).toBeNull();
  });

  it("Should expose typed editing and render mutation progress as loading copy", () => {
    const editor = buildEditor({
      open: true,
      value: taskExecutionProfileFixture,
    });
    render(
      <TaskSetupSheet
        clearOpen={false}
        editor={editor}
        hasActiveRun={false}
        isSetPending
        onClearOpenChange={vi.fn()}
        onClearProfile={vi.fn()}
        onOpenChange={vi.fn()}
        open
        profile={taskExecutionProfileFixture}
        runtime={runtime}
      />
    );

    expect(screen.getByTestId("tasks-setup-form")).toBeInTheDocument();
    expect(screen.getByTestId("tasks-setup-editor-save")).toBeDisabled();
    expect(screen.getByTestId("tasks-setup-editor-save")).toHaveTextContent("Saving…");
  });

  it("Should render every configured review and participant routing selector", () => {
    const profile = buildTaskExecutionProfileFixture({
      participants: {
        allowed_channel_ids: ["channel-ops"],
        preferred_agent_names: ["release-writer"],
        preferred_capabilities: ["release.write"],
        preferred_peer_ids: ["peer-release"],
      },
      review: {
        allowed_channel_ids: ["channel-review"],
        allowed_peer_ids: ["peer-compliance"],
        model: "gpt-5.4",
        preferred_capabilities: ["review.trace"],
        provider: "openai",
      },
    });

    render(
      <TaskSetupSheet
        clearOpen={false}
        editor={buildEditor()}
        hasActiveRun={false}
        onClearOpenChange={vi.fn()}
        onClearProfile={vi.fn()}
        onOpenChange={vi.fn()}
        open
        profile={profile}
        runtime={runtime}
      />
    );

    const review = screen.getByRole("heading", { name: "Review" }).closest("section");
    expect(review).not.toBeNull();
    expect(within(review!).queryByText("Off")).toBeNull();
    expect(within(review!).getByText("openai · gpt-5.4")).toBeInTheDocument();
    expect(within(review!).getByText("channel-review")).toBeInTheDocument();
    expect(within(review!).getByText("peer-compliance")).toBeInTheDocument();
    expect(within(review!).getByText("review.trace")).toBeInTheDocument();
    expect(screen.getByText("channel-ops")).toBeInTheDocument();
    expect(screen.getByText("release-writer")).toBeInTheDocument();
    expect(screen.getByText("peer-release")).toBeInTheDocument();
    expect(screen.getByText("release.write")).toBeInTheDocument();
  });
});
