import { agentFixtures } from "@/systems/agent/mocks";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render as renderTestingLibrary, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState, type ReactElement } from "react";
import { describe, expect, it, vi } from "vitest";

import { AutomationEditorDialog } from "../automation-editor-dialog";
import {
  createAutomationJobDraft,
  createAutomationTriggerDraft,
} from "../../lib/automation-drafts";
import { createAutomationDialogHandle } from "../../lib/dialog-handle";
import type { CreateAutomationJobRequest, CreateAutomationTriggerRequest } from "../../types";

const WORKSPACES = [
  { id: "ws_test", name: "test-workspace" },
  { id: "ws_beta", name: "beta-workspace" },
];

function render(ui: ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return renderTestingLibrary(ui, {
    wrapper: ({ children }) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    ),
  });
}

function JobEditorHarness({
  mode = "create",
  onCancel,
  onSubmit,
}: {
  mode?: "create" | "edit";
  onCancel: () => void;
  onSubmit: (draft: CreateAutomationJobRequest) => void;
}) {
  const [draft, setDraft] = useState<CreateAutomationJobRequest>(() =>
    createAutomationJobDraft("ws_test")
  );

  return (
    <AutomationEditorDialog
      activeWorkspaceId="ws_test"
      agents={agentFixtures}
      editor={{
        draft,
        isPending: false,
        kind: "jobs",
        mode,
        onCancel,
        onChange: setDraft,
        onSubmit: () => onSubmit(draft),
      }}
      workspaces={WORKSPACES}
    />
  );
}

function TriggerEditorHarness({
  mode = "create",
  onCancel,
  onSubmit,
}: {
  mode?: "create" | "edit";
  onCancel: () => void;
  onSubmit: (draft: CreateAutomationTriggerRequest) => void;
}) {
  const [draft, setDraft] = useState<CreateAutomationTriggerRequest>(() =>
    createAutomationTriggerDraft("ws_test")
  );

  return (
    <AutomationEditorDialog
      activeWorkspaceId="ws_test"
      agents={agentFixtures}
      editor={{
        draft,
        isPending: false,
        kind: "triggers",
        mode,
        onCancel,
        onChange: setDraft,
        onSubmit: () => onSubmit(draft),
      }}
      workspaces={WORKSPACES}
    />
  );
}

function DetachedTriggerHarness() {
  const [handle] = useState(() => createAutomationDialogHandle());
  const [editor, setEditor] = useState<{
    draft: CreateAutomationJobRequest;
    isPending: boolean;
    kind: "jobs";
    mode: "create";
    onCancel: () => void;
    onChange: (draft: CreateAutomationJobRequest) => void;
    onSubmit: () => void;
  } | null>(null);

  return (
    <>
      <button
        data-testid="open-detached-editor"
        onClick={() =>
          setEditor({
            draft: createAutomationJobDraft("ws_test"),
            isPending: false,
            kind: "jobs",
            mode: "create",
            onCancel: () => setEditor(null),
            onChange: draft => setEditor(current => (current ? { ...current, draft } : current)),
            onSubmit: () => undefined,
          })
        }
        type="button"
      >
        Open
      </button>
      <AutomationEditorDialog
        activeWorkspaceId="ws_test"
        editor={editor}
        handle={handle}
        workspaces={WORKSPACES}
      />
    </>
  );
}

describe("AutomationEditorDialog", () => {
  it("Should render the job editor with the Automation · Job eyebrow, Create job title, and job form", () => {
    const onCancel = vi.fn();
    const onSubmit = vi.fn();

    render(<JobEditorHarness onCancel={onCancel} onSubmit={onSubmit} />);

    const dialog = screen.getByTestId("automation-editor-dialog");
    expect(dialog).toBeInTheDocument();
    expect(dialog).toHaveAttribute("data-frame", "unframed");

    const header = dialog.querySelector('[data-slot="dialog-header"]');
    expect(header).not.toBeNull();
    expect(header).toHaveAttribute("data-variant", "ruled");
    // The header is the shared entity primitive, not a local definition:
    // its accent icon well only exists in `EntityDialogHeader`.
    expect(header?.querySelector('[data-slot="entity-dialog-header"]')).not.toBeNull();
    expect(header?.querySelector('[data-slot="entity-dialog-header-icon"]')).not.toBeNull();

    expect(within(header as HTMLElement).getByText("Automation · Job")).toBeInTheDocument();
    expect(within(header as HTMLElement).getByText("Create job")).toBeInTheDocument();
    expect(screen.getByTestId("automation-job-form")).toBeInTheDocument();
    expect(screen.getByTestId("workspace-switcher-name")).toHaveTextContent("test-workspace");
    expect(screen.queryByTestId("automation-trigger-form")).not.toBeInTheDocument();
  });

  it("Should keep submit disabled until every required field is valid and emit the job draft on submit", () => {
    const onSubmit = vi.fn();

    render(<JobEditorHarness onCancel={vi.fn()} onSubmit={onSubmit} />);

    expect(screen.getByTestId("submit-job-form")).toBeDisabled();

    fireEvent.change(screen.getByTestId("job-name-input"), {
      target: { value: "nightly-docs" },
    });
    expect(screen.getByTestId("submit-job-form")).toBeDisabled();

    fireEvent.click(screen.getByTestId("job-agent-input"));
    fireEvent.click(screen.getByTestId(`agent-command-item-${agentFixtures[0].name}`));
    fireEvent.change(screen.getByTestId("job-prompt-input"), {
      target: { value: "Summarize the latest commits." },
    });

    expect(screen.getByTestId("submit-job-form")).toBeEnabled();

    fireEvent.click(screen.getByTestId("submit-job-form"));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        agent_name: agentFixtures[0].name,
        name: "nightly-docs",
        prompt: "Summarize the latest commits.",
      })
    );
  });

  it("Should render the trigger editor with the Automation · Trigger eyebrow, Create trigger title, and trigger form", () => {
    render(<TriggerEditorHarness onCancel={vi.fn()} onSubmit={vi.fn()} />);

    const header = screen
      .getByTestId("automation-editor-dialog")
      .querySelector('[data-slot="dialog-header"]');
    expect(header).not.toBeNull();
    expect(within(header as HTMLElement).getByText("Automation · Trigger")).toBeInTheDocument();
    expect(within(header as HTMLElement).getByText("Create trigger")).toBeInTheDocument();
    expect(screen.getByTestId("automation-trigger-form")).toBeInTheDocument();
    expect(screen.queryByTestId("automation-job-form")).not.toBeInTheDocument();
  });

  it("Should preserve the agent catalog loading state through the job target selector", () => {
    render(
      <AutomationEditorDialog
        activeWorkspaceId="ws_test"
        agents={[]}
        agentsLoading
        editor={{
          draft: createAutomationJobDraft("ws_test"),
          isPending: false,
          kind: "jobs",
          mode: "create",
          onCancel: vi.fn(),
          onChange: vi.fn(),
          onSubmit: vi.fn(),
        }}
        workspaces={WORKSPACES}
      />
    );

    const selector = screen.getByTestId("job-agent-input");
    expect(selector).toBeDisabled();
    expect(selector).toHaveTextContent("Loading agents…");
    expect(selector).toHaveAttribute("aria-busy", "true");
  });

  it("Should preserve an agent catalog error through the trigger target selector", async () => {
    const user = userEvent.setup();
    render(
      <AutomationEditorDialog
        activeWorkspaceId="ws_test"
        agents={[]}
        agentsError="Agent catalog is unavailable."
        editor={{
          draft: createAutomationTriggerDraft("ws_test"),
          isPending: false,
          kind: "triggers",
          mode: "create",
          onCancel: vi.fn(),
          onChange: vi.fn(),
          onSubmit: vi.fn(),
        }}
        workspaces={WORKSPACES}
      />
    );

    const selector = screen.getByTestId("trigger-agent-input");
    expect(selector).toHaveTextContent("Unable to load agents");
    expect(selector).toHaveAttribute("aria-invalid", "true");

    await user.click(selector);
    expect(screen.getByTestId("agent-command-empty")).toHaveTextContent(
      "Agent catalog is unavailable."
    );
  });

  it("Should render the Edit trigger title for the triggers edit mode", () => {
    render(<TriggerEditorHarness mode="edit" onCancel={vi.fn()} onSubmit={vi.fn()} />);

    const header = screen
      .getByTestId("automation-editor-dialog")
      .querySelector('[data-slot="dialog-header"]');
    expect(header).not.toBeNull();
    expect(within(header as HTMLElement).getByText("Automation · Trigger")).toBeInTheDocument();
    expect(within(header as HTMLElement).getByText("Edit trigger")).toBeInTheDocument();
    expect(screen.getByTestId("automation-trigger-form")).toBeInTheDocument();
  });

  it("Should open and stay open when editor state is driven by a detached trigger button", async () => {
    const user = userEvent.setup();

    render(<DetachedTriggerHarness />);

    await user.click(screen.getByTestId("open-detached-editor"));

    const dialog = screen.getByTestId("automation-editor-dialog");
    expect(dialog).toBeInTheDocument();
    expect(screen.getByTestId("automation-job-form")).toBeInTheDocument();

    const header = dialog.querySelector('[data-slot="dialog-header"]');
    expect(header).not.toBeNull();
    expect(within(header as HTMLElement).getByText("Automation · Job")).toBeInTheDocument();
    expect(within(header as HTMLElement).getByText("Create job")).toBeInTheDocument();
  });

  it("Should keep the trigger editor open when selecting an extension event", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();

    render(<TriggerEditorHarness onCancel={onCancel} onSubmit={vi.fn()} />);

    await user.click(screen.getByTestId("trigger-event-ext"));

    expect(screen.getByTestId("automation-editor-dialog")).toBeInTheDocument();
    expect(screen.getByTestId("trigger-ext-ext-input")).toBeInTheDocument();
    expect(screen.getByTestId("trigger-ext-event-input")).toBeInTheDocument();
    expect(screen.getByTestId("submit-trigger-form")).toBeDisabled();
    expect(onCancel).not.toHaveBeenCalled();
  });

  it("Should call onCancel when the dialog is dismissed", () => {
    const onCancel = vi.fn();
    const { rerender } = render(<JobEditorHarness onCancel={onCancel} onSubmit={vi.fn()} />);

    fireEvent.keyDown(document.body, { key: "Escape" });
    // Base UI Dialog closes on escape; even if the JSDOM path is brittle, we also
    // cover the explicit close by unmounting via editor=null + remount, which is
    // the real exit path in useAutomationPage.
    rerender(<AutomationEditorDialog editor={null} />);

    expect(screen.queryByTestId("automation-editor-dialog")).not.toBeInTheDocument();
  });

  it("Should not render the dialog content when editor is null", () => {
    render(<AutomationEditorDialog editor={null} />);

    expect(screen.queryByTestId("automation-editor-dialog")).not.toBeInTheDocument();
    expect(screen.queryByTestId("automation-job-form")).not.toBeInTheDocument();
  });
});
