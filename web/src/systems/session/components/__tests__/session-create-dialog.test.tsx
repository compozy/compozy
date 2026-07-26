import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@agh/ui";
import type { AgentPayload } from "@/systems/agent";
import { FIXTURE_AGENT_DEFINITION_DIGEST } from "@/systems/agent/mocks";
import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";
import type { WorkspacePayload } from "@/systems/workspace";

import { SessionCreateDialog, type SessionCreateDialogProps } from "../session-create-dialog";

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

const runtimeProviders: RuntimeProviderOption[] = [
  { id: "claude", name: "Claude Code", harness: "acp", runtime_provider: "claude" },
  { id: "codex", name: "Codex", runtime_provider: "codex" },
  { id: "cursor", name: "Cursor", harness: "acp", runtime_provider: "cursor" },
  { id: "openrouter", name: "OpenRouter", harness: "pi_acp", runtime_provider: "openrouter" },
];

const runtimeModels: RuntimeModelOption[] = [
  {
    id: "gpt-5.4",
    provider: "codex",
    name: "GPT-5.4",
    efforts: ["low", "medium", "high"],
    availability: "live",
    curated: true,
  },
  {
    id: "gpt-5.4-mini",
    provider: "codex",
    name: "GPT-5.4 Mini",
    efforts: [],
    availability: "stale",
    curated: true,
  },
];

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
    mode: "advanced",
    onModeChange: vi.fn(),
    agents,
    workspace,
    workspaces: [{ id: workspace.id, name: workspace.name, root_dir: workspace.root_dir }],
    workspaceId: workspace.id,
    onWorkspaceChange: vi.fn(),
    sessionName: "",
    onSessionNameChange: vi.fn(),
    workspacePath: "",
    onWorkspacePathChange: vi.fn(),
    selectedAgentName: "claude-agent",
    runtimeValue: { provider: "claude", model: "", reasoning_effort: "" },
    runtimeProviders,
    runtimeModels: [],
    catalogStale: false,
    catalogLoading: false,
    catalogLoaded: true,
    catalogError: null,
    catalogRefreshing: false,
    catalogRefreshError: null,
    providersLoading: false,
    providersError: null,
    hasProviderOptions: true,
    networkParticipation: {
      mode: "local",
      channelId: "",
      channelStrategy: "",
    },
    promptValue: "Draft the release notes.",
    onPromptChange: vi.fn(),
    onAgentChange: vi.fn(),
    onRuntimeChange: vi.fn(),
    onNetworkParticipationChange: vi.fn(),
    onCatalogRefresh: vi.fn(),
    onOpenProviderSettings: vi.fn(),
    onSubmit: vi.fn(),
    isSubmitting: false,
    submitError: null,
    ...overrides,
  };
}

function renderDialog(overrides: Partial<SessionCreateDialogProps> = {}) {
  return render(
    <UIProvider reducedMotion="always">
      <SessionCreateDialog {...makeProps(overrides)} />
    </UIProvider>
  );
}

async function openRuntimePopup(user: ReturnType<typeof userEvent.setup>) {
  // The selector trigger is a single button — clicking it toggles the popup.
  await user.click(screen.getByTestId("session-create-runtime-select"));
  return screen.findByTestId("runtime-selector-popup");
}

describe("SessionCreateDialog", () => {
  it("Should show identity fields and the composer without Advanced fields in Simple", () => {
    renderDialog({ mode: "simple" });

    expect(screen.getByTestId("session-create-agent-select")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-workspace-select")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-name-input")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-composer")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-runtime-select")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-send")).toBeInTheDocument();
    expect(screen.queryByTestId("session-create-workspace-path-input")).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-create-participation-mode")).not.toBeInTheDocument();
  });

  it("Should reveal the launch overrides in Advanced without hiding the Simple fields", () => {
    renderDialog({ mode: "advanced" });

    expect(screen.getByTestId("session-create-agent-select")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-workspace-select")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-name-input")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-runtime-select")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-workspace-path-input")).toBeInTheDocument();
  });

  it("Should report the mode through the shared toolbar", async () => {
    const user = userEvent.setup();
    const onModeChange = vi.fn();
    renderDialog({ mode: "simple", onModeChange });

    expect(screen.getByTestId("session-create-mode-simple")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    await user.click(screen.getByTestId("session-create-mode-advanced"));
    expect(onModeChange).toHaveBeenCalledWith("advanced");
  });

  it("Should expose exactly one close route", () => {
    renderDialog();
    const host = screen.getByTestId("session-create-dialog");
    expect(host.querySelectorAll('[data-slot="entity-dialog-header-close"]')).toHaveLength(1);
  });

  it("Should report the session name and workspace selection to the owner", async () => {
    const user = userEvent.setup();
    const onSessionNameChange = vi.fn();
    renderDialog({ mode: "simple", onSessionNameChange });

    await user.type(screen.getByTestId("session-create-name-input"), "A");
    expect(onSessionNameChange).toHaveBeenCalledWith("A");
  });

  it("Should render the runtime selector wired to the selected provider", () => {
    renderDialog();

    const trigger = screen.getByTestId("session-create-runtime-select");
    // The provider renders as its mark only; the selected identity reaches
    // assistive tech through the composed caption + value accessible name.
    expect(trigger).toHaveAccessibleName(/Claude Code/);
    expect(trigger).not.toHaveAttribute("aria-disabled", "true");
  });

  it("Should preselect the incoming agent name in the agent picker trigger", () => {
    renderDialog({ selectedAgentName: "codex-agent" });

    expect(screen.getByTestId("session-create-agent-select")).toHaveTextContent("codex-agent");
  });

  it("Should call onAgentChange when the operator picks a different agent", async () => {
    const user = userEvent.setup();
    const onAgentChange = vi.fn();
    renderDialog({ onAgentChange });

    await user.click(screen.getByTestId("session-create-agent-select"));
    await user.click(screen.getByTestId("agent-command-item-codex-agent"));
    expect(onAgentChange).toHaveBeenCalledWith("codex-agent");
  });

  it("Should surface a stale catalog notice inside the selector without blocking submit", async () => {
    const user = userEvent.setup();
    renderDialog({ runtimeModels, catalogStale: true });

    // Catalog state belongs to the selector, not the dialog chrome: no banner
    // appears until the popup that owns the catalog is opened.
    expect(screen.queryByTestId("session-create-catalog-stale")).not.toBeInTheDocument();

    await openRuntimePopup(user);
    expect(screen.getByTestId("session-create-catalog-stale")).toHaveTextContent(
      "Some models are stale"
    );
    expect(screen.getByTestId("session-create-send")).toBeEnabled();
  });

  it("Should surface catalog source errors inside the selector while keeping it usable", async () => {
    const user = userEvent.setup();
    renderDialog({ runtimeModels: [], catalogError: "catalog upstream failed" });

    await openRuntimePopup(user);
    expect(screen.getByTestId("session-create-catalog-error")).toHaveTextContent(
      "catalog upstream failed"
    );
    expect(screen.getByTestId("session-create-runtime-select")).not.toHaveAttribute(
      "aria-disabled",
      "true"
    );
  });

  it("Should expose a refresh control that triggers onCatalogRefresh", async () => {
    const onCatalogRefresh = vi.fn();
    const user = userEvent.setup();
    renderDialog({ runtimeModels, onCatalogRefresh });

    await openRuntimePopup(user);
    await user.click(screen.getByTestId("runtime-selector-refresh"));
    expect(onCatalogRefresh).toHaveBeenCalledTimes(1);
  });

  it("Should call onSubmit only once when the form is submitted with a valid draft", () => {
    const onSubmit = vi.fn();
    render(<SessionCreateDialog {...makeProps({ onSubmit })} />);

    fireEvent.click(screen.getByTestId("session-create-send"));
    expect(onSubmit).toHaveBeenCalledTimes(1);
  });

  it("Should block submit until named Live participation has a channel", () => {
    const onSubmit = vi.fn();
    renderDialog({
      networkParticipation: { mode: "live", channelId: "", channelStrategy: "named" },
      onSubmit,
    });

    expect(screen.getByTestId("session-create-send")).toBeDisabled();
    expect(screen.getByTestId("session-create-participation-channel")).toBeRequired();
    fireEvent.click(screen.getByTestId("session-create-send"));
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("Should disable submit when no providers are available and surface an empty-state note", () => {
    renderDialog({
      runtimeProviders: [],
      hasProviderOptions: false,
      runtimeValue: { provider: "", model: "", reasoning_effort: "" },
    });

    expect(screen.getByTestId("session-create-send")).toBeDisabled();
    expect(screen.getByTestId("session-create-providers-empty")).toBeInTheDocument();
  });

  it("Should surface providersError when the workspace provider list fails to load", () => {
    renderDialog({
      providersError: "Unable to load provider options for this workspace.",
    });

    expect(screen.getByTestId("session-create-providers-error")).toHaveTextContent(
      "Unable to load provider options for this workspace."
    );
  });

  it("Should surface submitError when creation fails", () => {
    renderDialog({ submitError: "Server rejected the session" });

    expect(screen.getByTestId("session-create-submit-error")).toHaveTextContent(
      "Server rejected the session"
    );
    expect(screen.getByTestId("session-create-prompt")).toHaveAttribute(
      "aria-describedby",
      "session-create-submit-error"
    );
  });

  it("Should disable submit when the current selections are no longer available", () => {
    renderDialog({
      selectedAgentName: "missing-agent",
      runtimeValue: { provider: "missing-provider", model: "", reasoning_effort: "" },
    });

    expect(screen.getByTestId("session-create-send")).toBeDisabled();
  });

  it("Should block a non-catalog model and explain how to recover inline", () => {
    renderDialog({
      runtimeModels,
      runtimeValue: {
        provider: "cursor",
        model: "cursor-grok-4.5-high",
        reasoning_effort: "",
      },
    });

    expect(screen.getByTestId("session-create-send")).toBeDisabled();
    expect(screen.getByTestId("session-create-model-error")).toHaveTextContent(
      "not in the selected provider catalog"
    );
    expect(screen.getByTestId("session-create-model-error")).toHaveAttribute("role", "alert");
  });

  it("Should disable the runtime selector while providers are loading", () => {
    renderDialog({
      runtimeProviders: [],
      hasProviderOptions: false,
      providersLoading: true,
      runtimeValue: { provider: "", model: "", reasoning_effort: "" },
    });

    // The single-button trigger disables natively (not merely aria-disabled).
    expect(screen.getByTestId("session-create-runtime-select")).toBeDisabled();
    expect(screen.getByTestId("session-create-send")).toBeDisabled();
  });

  it("Should disable both pickers until a workspace is selected", () => {
    renderDialog({
      workspace: undefined,
      workspaceId: null,
      selectedAgentName: "claude-agent",
      runtimeValue: { provider: "claude", model: "", reasoning_effort: "" },
    });

    expect(screen.getByTestId("session-create-agent-select")).toBeDisabled();
    expect(screen.getByTestId("session-create-agent-select")).toHaveTextContent(
      "Select a workspace first"
    );
    expect(screen.queryByTestId("session-create-agent-default")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-create-runtime-select")).toBeDisabled();
    expect(screen.queryByTestId("session-create-providers-empty")).not.toBeInTheDocument();
  });

  it("Should not render blank agent provider metadata for inherited providers", () => {
    renderDialog({
      agents: [
        {
          name: "general",
          provider: "",
          prompt: "help",
          origin: "workspace",
          workspace_id: "ws_alpha",
          definition_digest: FIXTURE_AGENT_DEFINITION_DIGEST,
        },
      ],
      selectedAgentName: "general",
      runtimeValue: { provider: "codex", model: "", reasoning_effort: "" },
    });

    expect(screen.queryByTestId("session-create-agent-default")).not.toBeInTheDocument();
    expect(screen.getByTestId("session-create-runtime-select")).toHaveTextContent("Codex");
  });

  it("Should announce truthful pending feedback and block dismissal while submit is in flight", () => {
    const onOpenChange = vi.fn();
    render(<SessionCreateDialog {...makeProps({ isSubmitting: true, onOpenChange })} />);

    expect(screen.getByRole("status")).toBeInTheDocument();
    expect(screen.getByTestId("session-create-send")).toBeDisabled();
    expect(screen.getByTestId("session-create-prompt")).toBeDisabled();
    const pending = screen.getByTestId("session-create-pending-status");
    expect(pending).toHaveAttribute("role", "status");
    expect(pending).toHaveTextContent("AGH durably accepts it");
    fireEvent.click(getDialogBackdrop());
    expect(onOpenChange).not.toHaveBeenCalled();
  });

  it("Should allow backdrop dismissal when submit is idle", () => {
    const onOpenChange = vi.fn();
    render(<SessionCreateDialog {...makeProps({ onOpenChange })} />);

    fireEvent.click(getDialogBackdrop());
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should keep the Send disc as the only creation action in the dialog", () => {
    renderDialog();

    expect(screen.queryByRole("button", { name: /start session/i })).not.toBeInTheDocument();
    expect(screen.queryByTestId("session-create-dialog-cancel")).not.toBeInTheDocument();
    const submitters = screen
      .getAllByRole("button")
      .filter(button => button.getAttribute("type") === "submit");
    expect(submitters).toEqual([screen.getByTestId("session-create-send")]);
  });

  it("Should close via the header close button", () => {
    const onOpenChange = vi.fn();
    renderDialog({ onOpenChange });

    fireEvent.click(screen.getByRole("button", { name: /close/i }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  describe("first-message composer", () => {
    it("Should land focus on the message even though the composer renders last", async () => {
      renderDialog({ promptValue: "" });

      await waitFor(() => expect(screen.getByTestId("session-create-prompt")).toHaveFocus());
    });

    it("Should forward typed characters through onPromptChange", async () => {
      const user = userEvent.setup();
      const onPromptChange = vi.fn();
      renderDialog({ onPromptChange, promptValue: "" });

      await user.type(screen.getByTestId("session-create-prompt"), "hi");
      expect(onPromptChange).toHaveBeenCalledWith("h");
      expect(onPromptChange).toHaveBeenCalledWith("i");
    });

    it("Should send exactly once on Enter with a populated draft", () => {
      const onSubmit = vi.fn();
      renderDialog({ onSubmit });

      fireEvent.keyDown(screen.getByTestId("session-create-prompt"), { key: "Enter" });
      expect(onSubmit).toHaveBeenCalledTimes(1);
    });

    it("Should insert a newline instead of sending on Shift+Enter", async () => {
      const user = userEvent.setup();
      const onSubmit = vi.fn();

      function ControlledPromptDialog() {
        const [promptValue, setPromptValue] = useState("Draft the release notes.");
        return (
          <SessionCreateDialog
            {...makeProps({ onSubmit, onPromptChange: setPromptValue, promptValue })}
          />
        );
      }

      render(
        <UIProvider reducedMotion="always">
          <ControlledPromptDialog />
        </UIProvider>
      );

      const prompt = screen.getByTestId("session-create-prompt");
      await user.click(prompt);
      await user.keyboard("{Shift>}{Enter}{/Shift}");
      expect(prompt).toHaveValue("Draft the release notes.\n");
      expect(onSubmit).not.toHaveBeenCalled();
    });

    it("Should never send while an IME composition is active", () => {
      const onSubmit = vi.fn();
      renderDialog({ onSubmit });

      fireEvent.keyDown(screen.getByTestId("session-create-prompt"), {
        key: "Enter",
        isComposing: true,
      });
      expect(onSubmit).not.toHaveBeenCalled();
    });

    it("Should refuse a whitespace-only draft from both Send and Enter", () => {
      const onSubmit = vi.fn();
      renderDialog({ onSubmit, promptValue: "   \n  " });

      expect(screen.getByTestId("session-create-send")).toBeDisabled();
      fireEvent.click(screen.getByTestId("session-create-send"));
      fireEvent.keyDown(screen.getByTestId("session-create-prompt"), { key: "Enter" });
      expect(onSubmit).not.toHaveBeenCalled();
    });
  });
});
