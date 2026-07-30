import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";
import type { AgentPayload } from "@/systems/agent";
import { FIXTURE_AGENT_DEFINITION_DIGEST } from "@/systems/agent/mocks";
import type { WorkspacePayload } from "@/systems/workspace";

import { SessionCreateDialog, type SessionCreateDialogProps } from "../session-create-dialog";

// Invariant: session creation selects durable session identity; its first prompt and runtime
// belong to the created session composer, never this dialog.
// Owning layer: session create dialog presentation. Canonical suite: this component test.
vi.mock("@/systems/status", () => ({
  useDaemonStatus: () => ({ data: undefined }),
}));

const agents: AgentPayload[] = [
  {
    name: "claude-agent",
    provider: "claude",
    prompt: "help",
    origin: "workspace",
    workspace_id: "ws_alpha",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
  {
    name: "codex-agent",
    provider: "codex",
    prompt: "code",
    origin: "workspace",
    workspace_id: "ws_alpha",
    definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
  },
];

const workspace: WorkspacePayload = {
  id: "ws_alpha",
  root_dir: "/workspace/alpha",
  add_dirs: [],
  name: "alpha",
  created_at: "2026-04-20T10:00:00Z",
  updated_at: "2026-04-20T10:00:00Z",
};

function getDialogBackdrop(): HTMLElement {
  const backdrop = document.querySelector('[data-slot="dialog-overlay"]');
  if (!(backdrop instanceof HTMLElement)) {
    throw new Error("Expected dialog backdrop to be rendered.");
  }
  return backdrop;
}

function makeProps(overrides: Partial<SessionCreateDialogProps> = {}): SessionCreateDialogProps {
  return {
    open: true,
    onOpenChange: vi.fn(),
    mode: "simple",
    onModeChange: vi.fn(),
    agents,
    workspace,
    workspaces: [{ id: workspace.id, name: workspace.name, root_dir: workspace.root_dir }],
    workspaceId: workspace.id,
    userHomeDir: undefined,
    onWorkspaceChange: vi.fn(),
    sessionName: "",
    onSessionNameChange: vi.fn(),
    workspacePath: "",
    onWorkspacePathChange: vi.fn(),
    selectedAgentName: "claude-agent",
    networkParticipation: { mode: "local", channelId: "", channelStrategy: "" },
    onAgentChange: vi.fn(),
    onNetworkParticipationChange: vi.fn(),
    onSubmit: vi.fn(),
    isSubmitting: false,
    submitError: null,
    ...overrides,
  };
}

function renderDialog(overrides: Partial<SessionCreateDialogProps> = {}) {
  return render(
    <UIProvider reducedMotion="never" skipAnimations>
      <SessionCreateDialog {...makeProps(overrides)} />
    </UIProvider>
  );
}

describe("SessionCreateDialog", () => {
  it("Should show only the Agent field in Simple mode", () => {
    renderDialog();

    expect(screen.getByTestId("session-create-agent-select")).toBeInTheDocument();
    expect(screen.queryByTestId("session-create-workspace-select")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-create-name-input")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-create-workspace-path-input")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-create-participation-mode")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-create-composer")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-create-runtime-select")).not.toBeInTheDocument();
  });

  it("Should reveal workspace and launch details in Advanced mode", () => {
    renderDialog({ mode: "advanced" });

    expect(screen.getByTestId("session-create-agent-select")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-workspace-select")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-name-input")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-workspace-path-input")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-participation-mode")).toBeInTheDocument();
  });

  it("Should report mode changes through the shared toolbar", async () => {
    const user = userEvent.setup();
    const onModeChange = vi.fn();
    renderDialog({ onModeChange });

    expect(screen.getByTestId("session-create-mode-simple")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    await user.click(screen.getByTestId("session-create-mode-advanced"));
    expect(onModeChange).toHaveBeenCalledWith("advanced");
  });

  it("Should report agent and name changes to the owner", async () => {
    const user = userEvent.setup();
    const onAgentChange = vi.fn();
    const onSessionNameChange = vi.fn();
    renderDialog({
      mode: "advanced",
      onAgentChange,
      onSessionNameChange,
    });

    await user.click(screen.getByTestId("session-create-agent-select"));
    await user.click(screen.getByTestId("agent-command-item-codex-agent"));
    await user.type(screen.getByTestId("session-create-name-input"), "A");

    expect(onAgentChange).toHaveBeenCalledWith("codex-agent");
    expect(onSessionNameChange).toHaveBeenCalledWith("A");
  });

  it("Should submit only through the Start session action", () => {
    const onSubmit = vi.fn();
    renderDialog({ onSubmit });

    expect(screen.getByTestId("session-create-submit")).toHaveTextContent("Start session");
    expect(screen.queryByRole("button", { name: /send/i })).not.toBeInTheDocument();
    fireEvent.click(screen.getByTestId("session-create-submit"));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("Should block submission until an available agent is selected", () => {
    const onSubmit = vi.fn();
    renderDialog({ onSubmit, selectedAgentName: "missing-agent" });

    expect(screen.getByTestId("session-create-submit")).toBeDisabled();
    fireEvent.click(screen.getByTestId("session-create-submit"));
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("Should block submission until named Live participation has a channel", () => {
    renderDialog({
      mode: "advanced",
      networkParticipation: { mode: "live", channelId: "", channelStrategy: "named" },
    });

    expect(screen.getByTestId("session-create-submit")).toBeDisabled();
    expect(screen.getByTestId("session-create-participation-channel")).toBeRequired();
  });

  it("Should disable the agent picker until a workspace is selected", () => {
    renderDialog({ workspace: undefined, workspaceId: null });

    expect(screen.getByTestId("session-create-agent-select")).toBeDisabled();
    expect(screen.getByTestId("session-create-agent-select")).toHaveTextContent(
      "Select a workspace first"
    );
    expect(screen.getByTestId("session-create-submit")).toBeDisabled();
  });

  it("Should keep creation feedback inline and prevent dismissal while submitting", () => {
    const onOpenChange = vi.fn();
    renderDialog({ isSubmitting: true, onOpenChange });

    expect(screen.getByTestId("session-create-pending-status")).toHaveTextContent(
      "CompozyOS durably accepts it"
    );
    expect(screen.getByTestId("session-create-submit")).toBeDisabled();
    fireEvent.click(getDialogBackdrop());
    expect(onOpenChange).not.toHaveBeenCalled();
  });

  it("Should associate a create failure with the form without closing the dialog", () => {
    renderDialog({ submitError: "Server rejected the session" });

    expect(screen.getByTestId("session-create-submit-error")).toHaveTextContent(
      "Server rejected the session"
    );
    expect(screen.getByTestId("session-create-dialog")).toBeInTheDocument();
  });
});
