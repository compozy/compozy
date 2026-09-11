// Suite: task editor dialog tier presence
// Invariant: the deep-linkable task editor dialogs offer the interactive modal
// only on a tier whose route matrix registers the task create/PATCH routes —
// POST /api/tasks (and the first-run enqueue) and PATCH /api/tasks/:id register
// only under `includeTaskMutations` (`routes.go:216-221`), with no 403 code for
// the truthful loopback-only state to render (BR-3). On remote tiers the
// modals go absent (never disabled), so the deep links land on the read view
// the window renders beneath; the local latch keeps the editor (BR-5).
// Boundary IN: the dialog components' tier gating and modal presence.
// Boundary OUT: the editor state hooks (use-task-editor-state suite), the
// presentational modal (task-editor-modal suite), and the window underlays
// (tasks-catalog-location / task-detail-location suites).
import { render, screen } from "@testing-library/react";
import { TooltipProvider, UIProvider } from "@compozy/ui";
import type { ReactElement } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { latchGatewayTierForTest } from "@/test/gateway-tier";

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  createSubmit: vi.fn(),
  createTemplateChange: vi.fn(),
  createSetDraft: vi.fn(),
  editSubmit: vi.fn(),
  editSetDraft: vi.fn(),
}));

vi.mock("@/systems/status", () => ({
  useDaemonStatus: () => ({ data: undefined }),
}));

vi.mock("../hooks/use-tasks-navigation", () => ({
  useTasksNavigation: () => mocks.navigate,
}));

vi.mock("../../../hooks/use-window-live-data-enabled", () => ({
  useCurrentWindowLiveDataEnabled: () => true,
}));

vi.mock("@/systems/workspace/hooks/use-active-workspace", () => ({
  useActiveWorkspace: () => ({
    workspaces: [{ id: "ws_alpha", name: "Alpha", root_dir: "/workspace/alpha" }],
  }),
}));

vi.mock("@/systems/tasks", async importOriginal => {
  const actual = await importOriginal<typeof import("@/systems/tasks")>();
  const editorLib = await import("@/systems/tasks/lib/task-editor");
  const draft = { ...editorLib.createTaskEditorDraft("one_shot", "ws_alpha"), title: "Review" };
  return {
    ...actual,
    useTaskCreateState: () => ({
      draft,
      handleTemplateChange: mocks.createTemplateChange,
      handleSubmit: mocks.createSubmit,
      isScopeResolving: false,
      isSubmitting: false,
      profileDestination: null,
      setDraft: mocks.createSetDraft,
      templateId: "one_shot",
      workspaces: [],
    }),
    useTaskEditState: () => ({
      draft,
      handleSubmit: mocks.editSubmit,
      isInitialized: true,
      isLoading: false,
      isSubmitting: false,
      setDraft: mocks.editSetDraft,
      task: null,
    }),
  };
});

import { TaskCreateDialog, TaskEditDialog } from "../task-editor-dialogs";

function renderDialog(ui: ReactElement) {
  return render(
    <UIProvider reducedMotion="always">
      <TooltipProvider delay={0}>{ui}</TooltipProvider>
    </UIProvider>
  );
}

describe("TaskEditorDialogs", () => {
  let unlatch: () => void;
  beforeEach(() => {
    // The editors render their interactive shape on the local tier (BR-5).
    unlatch = latchGatewayTierForTest("local");
  });
  afterEach(() => unlatch());

  it("Should render the create editor on the local tier", () => {
    renderDialog(<TaskCreateDialog catalogMode="list" search={{}} />);

    expect(screen.getByTestId("task-editor-modal")).toHaveAttribute("data-mode", "new");
  });

  it("Should render the edit editor on the local tier", () => {
    renderDialog(<TaskEditDialog search={{ tab: "overview" }} taskId="task_001" />);

    expect(screen.getByTestId("task-editor-modal")).toHaveAttribute("data-mode", "edit");
  });

  // Invariant: the deep-linked /tasks/$id/edit offers no editor modal on a
  // remote tier (BR-1 — absent, never disabled); the window underlay's task
  // detail read view is what the location lands on, and no save path exists
  // to fire the doomed PATCH.
  // Owning layer: task editor dialog composition.
  // Canonical suite: TaskEditorDialogs component tests.
  it("Should not offer the edit editor on a remote tier, leaving the deep link on the read view", () => {
    unlatch();
    unlatch = latchGatewayTierForTest("private");
    const { container } = renderDialog(
      <TaskEditDialog search={{ tab: "overview" }} taskId="task_001" />
    );

    expect(screen.queryByTestId("task-editor-modal")).not.toBeInTheDocument();
    expect(container).toBeEmptyDOMElement();
    expect(mocks.editSubmit).not.toHaveBeenCalled();
  });

  // Invariant: the deep-linked /tasks/new offers no editor modal on a remote
  // tier (BR-1 — absent, never disabled); the window underlay's catalog read
  // view is what the location lands on, and no save path exists to fire the
  // doomed POST.
  // Owning layer: task editor dialog composition.
  // Canonical suite: TaskEditorDialogs component tests.
  it("Should not offer the create editor on a remote tier, leaving the deep link on the catalog read view", () => {
    unlatch();
    unlatch = latchGatewayTierForTest("private");
    const { container } = renderDialog(<TaskCreateDialog catalogMode="list" search={{}} />);

    expect(screen.queryByTestId("task-editor-modal")).not.toBeInTheDocument();
    expect(container).toBeEmptyDOMElement();
    expect(mocks.createSubmit).not.toHaveBeenCalled();
  });
});
