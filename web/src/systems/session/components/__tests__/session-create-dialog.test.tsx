import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { AgentPayload } from "@/systems/agent";
import { FIXTURE_AGENT_DEFINITION_DIGEST } from "@/systems/agent/mocks";
import type { WorkspacePayload } from "@/systems/workspace";

import { SessionCreateDialog, type SessionCreateDialogProps } from "../session-create-dialog";

// Invariant: session creation selects durable session identity and, since US-004, the first
// message the session will send — this dialog is the draft composer, because a session does not
// exist until it is created. Runtime selection still belongs to the created session's composer.
// Cancel means "stop creating the environment" while one is being built, and "dismiss" otherwise.
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

// The destination statement reads the shell's active profile, which is server
// state, so every tree that renders the dialog needs a query client.
const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false, gcTime: 0 } },
});

function makeProps(overrides: Partial<SessionCreateDialogProps> = {}): SessionCreateDialogProps {
  return {
    open: true,
    restoreFocusOnClose: true,
    onOpenChange: vi.fn(),
    mode: "simple",
    onModeChange: vi.fn(),
    agents,
    scope: "workspace",
    destinationLabel: workspace.name,
    sessionRoot: workspace.root_dir,
    destinationReady: true,
    sessionName: "",
    onSessionNameChange: vi.fn(),
    firstMessage: "",
    onFirstMessageChange: vi.fn(),
    selectedAgentName: "claude-agent",
    networkParticipation: { mode: "local", channelId: "", channelStrategy: "" },
    onAgentChange: vi.fn(),
    onNetworkParticipationChange: vi.fn(),
    onSubmit: vi.fn(),
    isSubmitting: false,
    isAwaitingEnvironment: false,
    onCancelEnvironment: vi.fn(),
    submitError: null,
    ...overrides,
  };
}

function renderDialog(overrides: Partial<SessionCreateDialogProps> = {}) {
  return render(
    <QueryClientProvider client={queryClient}>
      <UIProvider reducedMotion="never" skipAnimations>
        <SessionCreateDialog {...makeProps(overrides)} />
      </UIProvider>
    </QueryClientProvider>
  );
}

async function closeDialogFromFocusedLauncher(restoreFocusOnClose: boolean) {
  const renderSurface = (open: boolean) => (
    <QueryClientProvider client={queryClient}>
      <UIProvider reducedMotion="never" skipAnimations>
        <button data-testid="session-create-launcher" type="button">
          New session
        </button>
        <SessionCreateDialog {...makeProps({ open, restoreFocusOnClose })} />
      </UIProvider>
    </QueryClientProvider>
  );
  const view = render(renderSurface(false));
  const launcher = screen.getByTestId("session-create-launcher");
  launcher.focus();
  view.rerender(renderSurface(true));
  await waitFor(() => {
    expect(screen.getByTestId("session-create-dialog")).toContainElement(
      document.activeElement as HTMLElement
    );
  });
  view.rerender(renderSurface(false));
  await waitFor(() =>
    expect(screen.queryByTestId("session-create-dialog")).not.toBeInTheDocument()
  );
  return { launcher, unmount: view.unmount };
}

function renderDialogWithFocusSource(overrides: Partial<SessionCreateDialogProps> = {}) {
  const renderTree = (nextOverrides: Partial<SessionCreateDialogProps>) => (
    <QueryClientProvider client={queryClient}>
      <UIProvider reducedMotion="never" skipAnimations>
        <button data-testid="session-create-focus-source" type="button">
          New session
        </button>
        <SessionCreateDialog {...makeProps(nextOverrides)} />
      </UIProvider>
    </QueryClientProvider>
  );
  const view = render(renderTree(overrides));
  return {
    ...view,
    rerenderDialog: (nextOverrides: Partial<SessionCreateDialogProps>) =>
      view.rerender(renderTree(nextOverrides)),
  };
}
describe("SessionCreateDialog", () => {
  it("Should show only the Agent and first-message fields in Simple mode", () => {
    renderDialog();

    expect(screen.getByTestId("session-create-agent-select")).toBeInTheDocument();
    // The first message is the point of starting a session, not a launch detail,
    // so it is never behind the Advanced disclosure.
    expect(screen.getByTestId("session-create-composer")).toBeInTheDocument();
    expect(screen.queryByTestId("session-create-workspace-select")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-create-name-input")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-create-participation-mode")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-create-runtime-select")).not.toBeInTheDocument();
  });

  it("Should reveal name and launch details in Advanced mode", () => {
    renderDialog({ mode: "advanced" });

    expect(screen.getByTestId("session-create-agent-select")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-composer")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-name-input")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-participation-mode")).toBeInTheDocument();
    expect(screen.getByTestId("workspace-scope-statement")).toHaveTextContent(
      "Runs in alpha — /workspace/alpha"
    );
    // The destination chip rides the same statement and appears only under the
    // aggregate; a scoped draft already shows whose session it will be.
    expect(screen.queryByTestId("profile-destination-chip")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-create-workspace-select")).not.toBeInTheDocument();
  });

  it("Should report the typed first message to the owner", async () => {
    const user = userEvent.setup();
    const onFirstMessageChange = vi.fn();
    renderDialog({ onFirstMessageChange });

    await user.type(screen.getByTestId("session-create-composer"), "Go");

    expect(onFirstMessageChange).toHaveBeenCalled();
  });

  it("Should route Cancel to the environment while one is being created", async () => {
    const user = userEvent.setup();
    const onCancelEnvironment = vi.fn();
    const onOpenChange = vi.fn();
    renderDialog({ isAwaitingEnvironment: true, onCancelEnvironment, onOpenChange });

    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onCancelEnvironment).toHaveBeenCalledTimes(1);
    // Stopping the worktree is not dismissing the dialog; the draft stays put.
    expect(onOpenChange).not.toHaveBeenCalled();
    expect(screen.getByTestId("session-create-environment-status")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-submit")).toBeDisabled();
  });

  it("Should freeze advanced launch inputs while an environment is being created", () => {
    renderDialog({
      environment: {
        value: { kind: "new", name: "hotfix-cors", previous: { kind: "root" } },
        worktrees: [],
        onChange: vi.fn(),
        materialization: {
          status: "creating",
          phase: "creating checkout",
          start: vi.fn(),
          cancel: vi.fn(),
          recover: vi.fn(),
          retry: vi.fn(),
          reset: vi.fn(),
        },
      },
      isAwaitingEnvironment: true,
      mode: "advanced",
    });

    expect(screen.getByTestId("session-create-environment")).toBeDisabled();
    expect(screen.getByTestId("session-create-name-input")).toBeDisabled();
    expect(screen.getByTestId("session-create-participation-mode")).toBeDisabled();
    expect(screen.queryByTestId("session-create-workspace-select")).not.toBeInTheDocument();
  });

  it("Should route Cancel to dismissal when no environment is being created", async () => {
    const user = userEvent.setup();
    const onCancelEnvironment = vi.fn();
    const onOpenChange = vi.fn();
    renderDialog({ onCancelEnvironment, onOpenChange });

    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(onCancelEnvironment).not.toHaveBeenCalled();
  });

  // US-004 AC-3: a failed environment must never be a dead end, and never cost
  // the operator the message they already wrote.
  it("Should offer retry, another environment, and the workspace root when creation fails", () => {
    renderDialog({
      firstMessage: "Investigate the regression",
      mode: "advanced",
      environment: {
        value: { kind: "new", name: "hotfix-cors" },
        worktrees: [],
        onChange: vi.fn(),
        materialization: {
          status: "failed",
          error: "fatal: could not create work tree dir",
          start: vi.fn(),
          cancel: vi.fn(),
          recover: vi.fn(),
          retry: vi.fn(),
          reset: vi.fn(),
        },
      },
    });

    expect(screen.getByText("fatal: could not create work tree dir")).toBeInTheDocument();
    expect(screen.getByTestId("session-environment-retry")).toBeInTheDocument();
    expect(screen.getByTestId("session-environment-root")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-environment")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-composer")).toHaveValue("Investigate the regression");
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

  it("Should disable the agent picker until a destination is ready", () => {
    renderDialog({ destinationReady: false });

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

  it("Should not restore focus into the source window after successful creation", async () => {
    const view = renderDialogWithFocusSource({ open: false });
    const sourceWindowFocusTarget = screen.getByTestId("session-create-focus-source");
    sourceWindowFocusTarget.focus();
    expect(sourceWindowFocusTarget).toHaveFocus();

    view.rerenderDialog({ isSubmitting: true, open: true });
    await waitFor(() => expect(sourceWindowFocusTarget).not.toHaveFocus());

    view.rerenderDialog({ open: false });
    await waitFor(() =>
      expect(screen.queryByTestId("session-create-dialog")).not.toBeInTheDocument()
    );
    expect(sourceWindowFocusTarget).not.toHaveFocus();
  });

  it("Should restore focus into the source window after ordinary dismissal", async () => {
    const view = renderDialogWithFocusSource({ open: false });
    const focusSource = screen.getByTestId("session-create-focus-source");
    focusSource.focus();

    view.rerenderDialog({ open: true });
    await waitFor(() => expect(focusSource).not.toHaveFocus());

    view.rerenderDialog({ open: false });
    await waitFor(() => expect(focusSource).toHaveFocus());
  });

  it("Should associate a create failure with the form without closing the dialog", () => {
    renderDialog({ submitError: "Server rejected the session" });

    expect(screen.getByTestId("session-create-submit-error")).toHaveTextContent(
      "Server rejected the session"
    );
    expect(screen.getByTestId("session-create-dialog")).toBeInTheDocument();
  });

  it("Should restore launcher focus only for a normal dismissal", async () => {
    const dismissed = await closeDialogFromFocusedLauncher(true);
    expect(dismissed.launcher).toHaveFocus();
    dismissed.unmount();

    const handedOff = await closeDialogFromFocusedLauncher(false);
    expect(handedOff.launcher).not.toHaveFocus();
  });
});
