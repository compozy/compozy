import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@agh/ui";

import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";

import { AgentCreateDialog, type AgentCreateDialogProps } from "../agent-create-dialog";
import {
  createDefaultAgentCreateDraft,
  type AgentCreateDialogDraft,
} from "../../lib/agent-create-draft";

const providers: RuntimeProviderOption[] = [
  { id: "codex", name: "Codex", harness: "acp", runtime_provider: "codex" },
  { id: "claude", name: "Claude Code", harness: "acp", runtime_provider: "claude" },
];

const runtimeModels: RuntimeModelOption[] = [
  {
    id: "gpt-5.6-sol",
    provider: "codex",
    name: "GPT-5.6 Sol",
    efforts: ["low", "high"],
    availability: "live",
    curated: true,
  },
  {
    id: "claude-sonnet-5",
    provider: "claude",
    name: "Claude Sonnet 5",
    efforts: ["low", "high"],
    availability: "live",
    curated: true,
  },
];

function validDraft(overrides: Partial<AgentCreateDialogDraft> = {}): AgentCreateDialogDraft {
  return {
    ...createDefaultAgentCreateDraft(true),
    name: "release-captain",
    provider: "codex",
    prompt: "Own release readiness.",
    ...overrides,
  };
}

function makeProps(overrides: Partial<AgentCreateDialogProps> = {}): AgentCreateDialogProps {
  return {
    open: true,
    onOpenChange: vi.fn(),
    draft: createDefaultAgentCreateDraft(true),
    onDraftChange: vi.fn(),
    onSubmit: vi.fn(),
    onRefreshCatalog: vi.fn(),
    onOpenProviderSettings: vi.fn(),
    providerOptions: providers,
    providersLoading: false,
    providersError: null,
    runtimeModels,
    modelCatalogLoading: false,
    modelCatalogLoaded: true,
    modelCatalogRefreshing: false,
    modelCatalogError: null,
    submitError: null,
    isSubmitting: false,
    hasActiveWorkspace: true,
    workspaceId: "ws_alpha",
    workspaceName: "alpha",
    ...overrides,
  };
}

function renderStatefulDialog(props: Partial<AgentCreateDialogProps> = {}) {
  const baseProps = makeProps(props);

  function Harness() {
    const [draft, setDraft] = useState(baseProps.draft);
    return <AgentCreateDialog {...baseProps} draft={draft} onDraftChange={setDraft} />;
  }

  return render(
    <UIProvider reducedMotion="always">
      <Harness />
    </UIProvider>
  );
}

describe("AgentCreateDialog", () => {
  it("Should present a create-only editor with no wizard chrome left", () => {
    renderStatefulDialog();

    expect(screen.getByRole("dialog", { name: "Create agent" })).toBeInTheDocument();
    expect(screen.queryByTestId("agent-create-stepper")).toBeNull();
    expect(screen.queryByTestId("agent-create-progress")).toBeNull();
    expect(screen.queryByTestId("agent-create-next")).toBeNull();
    expect(screen.queryByTestId("agent-create-back")).toBeNull();
  });

  it("Should keep naming and catalog placement in Simple and reveal the rest only in Advanced", async () => {
    const user = userEvent.setup();
    renderStatefulDialog();

    expect(screen.getByTestId("agent-create-name")).toBeInTheDocument();
    expect(screen.getByTestId("agent-create-prompt")).toBeInTheDocument();
    expect(screen.getByTestId("agent-create-runtime")).toBeInTheDocument();
    // Category path names where the agent files in the catalog — a decision made
    // while naming it, so it sits beside the runtime rather than under Advanced.
    expect(screen.getByTestId("agent-create-category-path")).toBeInTheDocument();
    expect(screen.queryByTestId("agent-create-permissions")).toBeNull();
    expect(screen.queryByTestId("agent-create-tools-input")).toBeNull();
    expect(screen.queryByTestId("agent-create-toolsets-input")).toBeNull();
    expect(screen.queryByTestId("agent-create-deny-tools-input")).toBeNull();
    expect(screen.queryByTestId("agent-create-disabled-skills-input")).toBeNull();
    expect(screen.queryByTestId("agent-create-command")).toBeNull();

    await user.click(screen.getByTestId("agent-create-mode-advanced"));

    expect(screen.getByTestId("agent-create-permissions")).toBeInTheDocument();
    expect(screen.getByTestId("agent-create-tools-input")).toBeInTheDocument();
    expect(screen.getByTestId("agent-create-command")).toBeInTheDocument();
    // Advanced never hides a required field.
    expect(screen.getByTestId("agent-create-name")).toBeInTheDocument();
    expect(screen.getByTestId("agent-create-prompt")).toBeInTheDocument();
    expect(screen.getByTestId("agent-create-category-path")).toBeInTheDocument();
  });

  it("Should never offer an MCP servers control, which the create contract cannot accept", async () => {
    const user = userEvent.setup();
    renderStatefulDialog({ draft: validDraft() });

    await user.click(screen.getByTestId("agent-create-mode-advanced"));

    expect(screen.queryByText(/mcp/i)).toBeNull();
  });

  it("Should submit from Simple once name, prompt and runtime are valid", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    renderStatefulDialog({ draft: validDraft(), onSubmit });

    const submit = screen.getByTestId("submit-agent-create");
    expect(submit).toBeEnabled();
    await user.click(submit);

    expect(onSubmit).toHaveBeenCalledOnce();
  });

  it("Should block submit while a required Simple field is empty", async () => {
    const user = userEvent.setup();
    renderStatefulDialog();

    expect(screen.getByTestId("submit-agent-create")).toBeDisabled();

    await user.type(screen.getByTestId("agent-create-name"), "../bad");

    expect(screen.getByTestId("agent-create-name-error")).toHaveTextContent(
      "Agent names cannot be . or .. and cannot contain path separators."
    );
    expect(screen.getByTestId("submit-agent-create")).toBeDisabled();
  });

  it("Should reveal Advanced instead of submitting when only a hidden field is invalid", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    renderStatefulDialog({
      draft: validDraft({ tools: ["  "] }),
      onSubmit,
    });

    expect(screen.queryByTestId("agent-create-tools-input")).toBeNull();
    await user.click(screen.getByTestId("submit-agent-create"));

    // A disabled primary with no visible cause is a dead end: the tier that owns
    // the invalid field has to open before the submit is refused.
    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getByTestId("agent-create-tools-error")).toHaveTextContent(
      "Tool entries cannot be blank."
    );
    expect(screen.getByTestId("agent-create-tools-input")).toBeInTheDocument();
  });

  it("Should surface an invalid category path without leaving Simple", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    renderStatefulDialog({
      draft: validDraft({ categoryPath: "operations//incident" }),
      onSubmit,
    });

    await user.click(screen.getByTestId("submit-agent-create"));

    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getByTestId("agent-create-category-path-error")).toHaveTextContent(
      "Category path cannot contain blank segments."
    );
    expect(screen.getByTestId("agent-create-mode-simple")).toHaveAttribute("aria-pressed", "true");
  });

  it("Should keep authored advanced values when leaving Advanced", async () => {
    const user = userEvent.setup();
    renderStatefulDialog({ draft: validDraft() });

    await user.click(screen.getByTestId("agent-create-mode-advanced"));
    await user.type(screen.getByTestId("agent-create-category-path"), "operations/incident");
    await user.click(screen.getByTestId("agent-create-mode-simple"));
    await user.click(screen.getByTestId("agent-create-mode-advanced"));

    expect(screen.getByTestId("agent-create-category-path")).toHaveValue("operations/incident");
  });

  it("Should surface provider errors inline after switching scope", async () => {
    const user = userEvent.setup();
    renderStatefulDialog({
      providerOptions: [],
      providersLoading: false,
      providersError: "Unable to load global provider settings.",
    });

    await user.click(screen.getByTestId("agent-create-scope-global"));
    await user.type(screen.getByTestId("agent-create-name"), "global-reviewer");

    expect(screen.getByTestId("agent-create-provider-error")).toHaveTextContent(
      "Unable to load global provider settings."
    );
    expect(screen.getByTestId("submit-agent-create")).toBeDisabled();
  });

  it("Should keep the active workspace read-only while allowing scope changes", async () => {
    const user = userEvent.setup();
    renderStatefulDialog({ draft: validDraft({ scope: "workspace" }) });

    expect(screen.getByTestId("agent-create-scope-global")).toBeEnabled();
    expect(screen.getByTestId("agent-create-scope-workspace")).toBeEnabled();
    expect(screen.getByTestId("agent-create-workspace-select")).toBeDisabled();

    await user.click(screen.getByTestId("agent-create-scope-global"));
    expect(screen.queryByTestId("agent-create-workspace-select")).toBeNull();

    await user.click(screen.getByTestId("agent-create-scope-workspace"));
    expect(screen.getByTestId("agent-create-workspace-select")).toBeDisabled();
  });

  it("Should fail closed on an off-contract reasoning effort", () => {
    renderStatefulDialog({
      draft: validDraft({
        model: "gpt-5.6-sol",
        reasoningEffort: "ultra" as AgentCreateDialogDraft["reasoningEffort"],
      }),
    });

    expect(screen.getByTestId("agent-create-provider-error")).toHaveTextContent(
      "Choose a valid reasoning effort."
    );
    expect(screen.getByTestId("submit-agent-create")).toBeDisabled();
  });

  it("Should clear authored runtime overrides back to project defaults", async () => {
    const user = userEvent.setup();
    renderStatefulDialog({
      draft: validDraft({
        model: "gpt-5.6-sol",
        reasoningEffort: "high",
      }),
    });

    await user.click(screen.getByTestId("agent-create-runtime-use-project-defaults"));

    expect(screen.getByTestId("agent-create-runtime-inherited")).toHaveTextContent(
      "Project runtime defaults will be used."
    );
    expect(screen.queryByTestId("agent-create-runtime-use-project-defaults")).toBeNull();
  });

  it("Should translate the selected permission policy into its contract consequence", async () => {
    const user = userEvent.setup();
    renderStatefulDialog({ draft: validDraft() });

    await user.click(screen.getByTestId("agent-create-mode-advanced"));

    expect(screen.getByTestId("agent-create-permissions-consequence")).toHaveTextContent(
      "The definition omits permissions"
    );

    await user.click(screen.getByTestId("agent-create-permissions-approve-reads"));

    expect(screen.getByTestId("agent-create-permissions-consequence")).toHaveTextContent(
      "Sessions inherit approve-reads."
    );
  });

  it("Should accept comma and newline token input while preserving de-duped order", async () => {
    const user = userEvent.setup();
    renderStatefulDialog({ draft: validDraft() });

    await user.click(screen.getByTestId("agent-create-mode-advanced"));
    const input = screen.getByTestId("agent-create-tools-input");

    await user.type(input, "agh__skill_view,");
    await user.type(input, "mcp__github__*{enter}");
    await user.type(input, "agh__skill_view{enter}");

    expect(screen.getByTestId("agent-create-tools-tokens")).toHaveTextContent("agh__skill_view");
    expect(screen.getByTestId("agent-create-tools-tokens")).toHaveTextContent("mcp__github__*");
    expect(screen.getAllByLabelText("Remove agh__skill_view")).toHaveLength(1);
  });

  it("Should retain the draft and stay open when the save fails", () => {
    renderStatefulDialog({
      draft: validDraft(),
      submitError: "agent definition already exists",
    });

    expect(screen.getByTestId("agent-create-submit-error")).toHaveTextContent(
      "agent definition already exists"
    );
    expect(screen.getByTestId("agent-create-name")).toHaveValue("release-captain");
    expect(screen.getByTestId("submit-agent-create")).toBeEnabled();
  });

  it("Should disable controls while submitting", () => {
    renderStatefulDialog({
      draft: validDraft(),
      isSubmitting: true,
    });

    expect(screen.getByTestId("submit-agent-create")).toBeDisabled();
    expect(screen.getByTestId("agent-create-cancel")).toBeDisabled();
  });
});
