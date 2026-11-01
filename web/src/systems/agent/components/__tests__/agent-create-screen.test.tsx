import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import { draftAgentInstructions } from "../../adapters/agent-instance-api";

vi.mock("../../adapters/agent-instance-api", () => ({ draftAgentInstructions: vi.fn() }));

import { UIProvider } from "@compozy/ui";

import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";

import { AgentCreateScreen, type AgentCreateScreenProps } from "../agent-create-screen";
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

const workspaces = [
  { id: "ws_alpha", name: "alpha" },
  { id: "ws_beta", name: "beta" },
].map(workspace => ({
  ...workspace,
  add_dirs: [],
  created_at: "2026-10-31T00:00:00Z",
  root_dir: `/work/${workspace.name}`,
  updated_at: "2026-10-31T00:00:00Z",
}));

function validDraft(overrides: Partial<AgentCreateDialogDraft> = {}): AgentCreateDialogDraft {
  return {
    ...createDefaultAgentCreateDraft(true, "ws_alpha"),
    displayName: "Release Captain",
    provider: "codex",
    prompt: "Own release readiness.",
    ...overrides,
  };
}

function makeProps(overrides: Partial<AgentCreateScreenProps> = {}): AgentCreateScreenProps {
  return {
    open: true,
    onOpenChange: vi.fn(),
    draft: createDefaultAgentCreateDraft(true, "ws_alpha"),
    onDraftChange: vi.fn(),
    onSubmit: vi.fn(),
    onRefreshCatalog: vi.fn(),
    onOpenProviderSettings: vi.fn(),
    onOpenCreatedAgent: vi.fn(),
    providerOptions: providers,
    providersLoading: false,
    providersError: null,
    runtimeModels,
    modelCatalogLoading: false,
    modelCatalogRefreshing: false,
    modelCatalogError: null,
    submitError: null,
    isSubmitting: false,
    hasActiveWorkspace: true,
    workspaceName: "alpha",
    takenNames: new Set(["reviewer"]),
    workspaces,
    instanceCapacity: { limit: 10, active: 2, reached: false },
    ...overrides,
  };
}

function renderScreen(props: Partial<AgentCreateScreenProps> = {}) {
  const baseProps = makeProps(props);

  function Harness() {
    const [draft, setDraft] = useState(baseProps.draft);
    return <AgentCreateScreen {...baseProps} draft={draft} onDraftChange={setDraft} />;
  }

  return render(
    <QueryClientProvider client={new QueryClient()}>
      <UIProvider reducedMotion="never" skipAnimations>
        <Harness />
      </UIProvider>
    </QueryClientProvider>
  );
}

const advancedToggle = () => screen.getByRole("button", { name: "Advanced" });

describe("AgentCreateScreen", () => {
  it("Should stop a write in flight when the screen closes", async () => {
    const user = userEvent.setup();
    vi.mocked(draftAgentInstructions).mockReturnValue(new Promise(() => undefined));
    const { unmount } = renderScreen({ draft: validDraft({ provider: "codex" }) });

    await user.click(screen.getByTestId("agent-fill-with-ai"));
    await user.type(screen.getByLabelText("What should this agent do?"), "Review releases");
    await user.click(screen.getByRole("button", { name: "Write it" }));
    const signal = vi.mocked(draftAgentInstructions).mock.lastCall?.[2];
    unmount();

    expect(signal?.aborted).toBe(true);
  });

  it("Should draft instructions with the selected runtime without saving the agent", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    vi.mocked(draftAgentInstructions).mockResolvedValueOnce("Review release readiness.");
    renderScreen({
      draft: validDraft({ model: "gpt-5.6-sol", reasoningEffort: "high" }),
      onSubmit,
    });
    await user.click(screen.getByTestId("agent-fill-with-ai"));
    await user.type(
      screen.getByRole("textbox", { name: "What should this agent do?" }),
      "Review releases"
    );
    await user.click(screen.getByRole("button", { name: "Write it" }));
    expect(await screen.findByText("Instructions added")).toBeInTheDocument();
    expect(draftAgentInstructions).toHaveBeenCalledWith(
      {
        goal: "Review releases",
        runtime: { provider: "codex", model: "gpt-5.6-sol", reasoning_effort: "high" },
      },
      "default",
      expect.any(AbortSignal)
    );
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("Should show one screen with the fields, How it works and a live preview", () => {
    renderScreen();

    expect(screen.getByRole("dialog", { name: "New agent" })).toBeInTheDocument();
    expect(screen.getByTestId("agent-create-name")).toBeInTheDocument();
    expect(screen.getByTestId("agent-create-description")).toBeInTheDocument();
    expect(screen.getByTestId("agent-create-runtime")).toBeInTheDocument();
    expect(screen.getByText("How it works")).toBeInTheDocument();
    expect(screen.getByTestId("agent-create-preview")).toHaveTextContent("Your agent");
    expect(screen.queryByTestId("agent-create-mode-advanced")).toBeNull();
    expect(screen.queryByTestId("agent-create-category-path")).toBeNull();
  });

  it("Should derive the Short name from the name and keep it editable under Advanced", async () => {
    const user = userEvent.setup();
    renderScreen();

    await user.type(screen.getByTestId("agent-create-name"), "Triage Lead!");
    await user.click(advancedToggle());

    expect(screen.getByTestId("agent-create-short-name")).toHaveAttribute(
      "placeholder",
      "triage-lead"
    );
    expect(screen.getByTestId("agent-create-preview")).toHaveTextContent("Triage Lead!");
  });

  it("Should report a taken name before submit and offer a free one", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    renderScreen({ draft: validDraft({ displayName: "Reviewer" }), onSubmit });

    expect(screen.getByTestId("agent-create-name-error")).toHaveTextContent(
      "You already have an agent with this name."
    );
    expect(screen.getByTestId("submit-agent-create")).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Use Reviewer 2" }));

    expect(screen.getByTestId("agent-create-name")).toHaveValue("Reviewer 2");
    expect(screen.queryByTestId("agent-create-name-error")).toBeNull();
    expect(screen.getByTestId("submit-agent-create")).toBeEnabled();
  });

  it("Should open Advanced for a Short name when the name has no valid characters", () => {
    renderScreen({ draft: validDraft({ displayName: "🦊" }) });

    expect(screen.getByTestId("agent-create-name-error")).toHaveTextContent(
      "Add at least one letter or number to the name."
    );
    expect(screen.getByTestId("agent-create-short-name")).toBeInTheDocument();
    expect(screen.getByTestId("submit-agent-create")).toBeDisabled();
  });

  it("Should confirm before discarding entered data, and close straight away when empty", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    const { unmount } = renderScreen({ draft: validDraft(), onOpenChange });

    await user.click(screen.getByTestId("agent-create-cancel"));
    expect(onOpenChange).not.toHaveBeenCalled();
    await user.click(await screen.findByRole("button", { name: "Discard" }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
    unmount();

    const onEmptyClose = vi.fn();
    renderScreen({ onOpenChange: onEmptyClose });
    await user.click(screen.getByTestId("agent-create-cancel"));
    expect(onEmptyClose).toHaveBeenCalledWith(false);
  });

  it("Should show Where it works only for Stays on, and say what the primary will do", async () => {
    const user = userEvent.setup();
    renderScreen({ draft: validDraft() });

    expect(screen.queryByTestId("agent-create-where")).toBeNull();
    expect(screen.getByTestId("submit-agent-create")).toHaveTextContent("Create agent");

    await user.click(screen.getByTestId("agent-create-kind-stays-on"));

    expect(screen.getByTestId("agent-create-where")).toHaveTextContent("alpha");
    expect(screen.getByTestId("submit-agent-create")).toHaveTextContent("Create and start");
  });

  it("Should find a project in Where it works and offer Global and Decide later", async () => {
    const user = userEvent.setup();
    renderScreen({ draft: validDraft({ kind: "stays_on" }) });

    await user.click(screen.getByTestId("agent-create-where"));
    expect(await screen.findByText("Global")).toBeInTheDocument();
    await user.click(screen.getByText("Decide later"));

    expect(screen.getByTestId("agent-create-where")).toHaveTextContent("Decide later");
    expect(screen.getByTestId("submit-agent-create")).toHaveTextContent("Create agent");
    expect(screen.getByText("Nothing runs until you add it to a project.")).toBeInTheDocument();
  });

  it("Should show the limit before submit and create the agent unstarted", () => {
    renderScreen({
      instanceCapacity: { limit: 10, active: 10, reached: true },
      draft: validDraft({ kind: "stays_on" }),
    });

    expect(screen.getByTestId("agent-create-limit")).toHaveTextContent(
      "10 of 10 agents are staying on."
    );
    expect(screen.queryByTestId("agent-create-where")).toBeNull();
    expect(screen.getByTestId("submit-agent-create")).toHaveTextContent("Create agent");
    expect(screen.getByText("You can start it from its page later.")).toBeInTheDocument();
  });

  it("Should show a provider that isn't signed in before submit", async () => {
    const onOpenProviderSettings = vi.fn();
    const user = userEvent.setup();
    renderScreen({
      draft: validDraft({ kind: "stays_on" }),
      onOpenProviderSettings,
      providerOptions: [{ ...providers[0]!, needs_auth: true }],
    });

    expect(screen.getByText("Codex isn't signed in,")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Sign in" }));
    expect(onOpenProviderSettings).toHaveBeenCalledOnce();
    expect(screen.getByTestId("submit-agent-create")).toHaveTextContent("Create agent");
  });

  it("Should report a failed start and offer the created agent", async () => {
    const onOpenCreatedAgent = vi.fn();
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    renderScreen({
      draft: validDraft({ kind: "stays_on" }),
      onOpenCreatedAgent,
      onSubmit,
      startFailure: "This agent can't reach that workspace.",
    });

    expect(screen.getByTestId("agent-create-start-failure")).toHaveTextContent(
      "Release Captain was created, but it couldn't start"
    );
    expect(screen.getByTestId("agent-create-preview")).toHaveTextContent("Couldn't start");
    expect(screen.queryByTestId("agent-create-name")).toBeNull();

    await user.click(screen.getByTestId("submit-agent-create"));

    expect(onOpenCreatedAgent).toHaveBeenCalledOnce();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("Should name an On demand agent's destination in the footer", () => {
    const { unmount } = renderScreen({ draft: validDraft({ scope: "workspace" }) });
    expect(screen.getByTestId("workspace-scope-statement")).toHaveTextContent("Creates in alpha");
    unmount();

    renderScreen({ draft: validDraft({ scope: "global" }) });
    expect(screen.getByTestId("workspace-scope-statement")).toHaveTextContent("Creates in User");
  });

  it("Should head a duplicate with its source", () => {
    renderScreen({ draft: validDraft(), duplicateSourceName: "orchestrator" });
    expect(screen.getByRole("dialog", { name: "Duplicate orchestrator" })).toBeInTheDocument();
  });

  it("Should never offer an MCP servers control, which the create contract cannot accept", async () => {
    const user = userEvent.setup();
    renderScreen({ draft: validDraft() });

    await user.click(advancedToggle());

    expect(screen.queryByText(/mcp/i)).toBeNull();
  });

  it("Should submit once name, prompt and runtime are valid", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    renderScreen({ draft: validDraft(), onSubmit });

    const submit = screen.getByTestId("submit-agent-create");
    expect(submit).toBeEnabled();
    await user.click(submit);

    expect(onSubmit).toHaveBeenCalledOnce();
  });

  it("Should block a Short name the daemon would reject and say why", async () => {
    const onSubmit = vi.fn();
    const user = userEvent.setup();
    renderScreen({ draft: validDraft(), onSubmit });

    await user.click(advancedToggle());
    await user.type(screen.getByTestId("agent-create-short-name"), "Audio Designer");

    expect(screen.getByTestId("agent-create-short-name-error")).toHaveTextContent(
      "Start with a lowercase letter, use only lowercase letters, numbers, hyphens, or underscores, and use at most 106 characters."
    );
    expect(screen.getByTestId("submit-agent-create")).toBeDisabled();
  });

  it("Should reveal Advanced instead of submitting when only a hidden field is invalid", async () => {
    const onSubmit = vi.fn();
    renderScreen({ draft: validDraft({ tools: ["  "] }), onSubmit });

    // A disabled primary with no visible cause is a dead end: the fold that owns
    // the invalid field opens on its own.
    expect(screen.getByTestId("agent-create-tools-error")).toHaveTextContent(
      "Tool entries cannot be blank."
    );
    expect(screen.getByTestId("agent-create-tools-input")).toBeInTheDocument();
  });

  it("Should reveal Advanced to surface an invalid group", async () => {
    const user = userEvent.setup();
    const onSubmit = vi.fn();
    renderScreen({ draft: validDraft({ categoryPath: "operations//incident" }), onSubmit });

    await user.click(screen.getByTestId("submit-agent-create"));

    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getByTestId("agent-create-category-path-error")).toHaveTextContent(
      "Group can't contain empty parts between slashes."
    );
  });

  it("Should keep authored advanced values across closing the fold", async () => {
    const user = userEvent.setup();
    renderScreen({ draft: validDraft() });

    await user.click(advancedToggle());
    await user.type(screen.getByTestId("agent-create-category-path"), "operations/incident");
    await user.click(advancedToggle());
    await user.click(advancedToggle());

    expect(screen.getByTestId("agent-create-category-path")).toHaveValue("operations/incident");
  });

  it("Should surface provider errors inline", () => {
    renderScreen({
      draft: validDraft({ scope: "global" }),
      providerOptions: [],
      providersLoading: false,
      providersError: "Unable to load global provider settings.",
    });

    expect(screen.getByTestId("agent-create-provider-error")).toHaveTextContent(
      "Unable to load global provider settings."
    );
    expect(screen.getByTestId("submit-agent-create")).toBeDisabled();
  });

  it("Should fail closed on an off-contract reasoning effort", () => {
    renderScreen({
      draft: validDraft({
        model: "gpt-5.6-sol",
        reasoningEffort: "invalid effort" as AgentCreateDialogDraft["reasoningEffort"],
      }),
    });

    expect(screen.getByTestId("agent-create-provider-error")).toHaveTextContent(
      "Choose a valid reasoning effort."
    );
    expect(screen.getByTestId("submit-agent-create")).toBeDisabled();
  });

  it("Should clear authored runtime overrides back to project defaults", async () => {
    const user = userEvent.setup();
    renderScreen({ draft: validDraft({ model: "gpt-5.6-sol", reasoningEffort: "high" }) });

    await user.click(screen.getByTestId("agent-create-runtime-use-project-defaults"));

    expect(screen.getByTestId("agent-create-runtime-inherited")).toHaveTextContent(
      "Choose a provider, or leave blank to use a configured default."
    );
    expect(screen.queryByTestId("agent-create-runtime-use-project-defaults")).toBeNull();
  });

  it("Should edit the agent Fast default through the shared runtime selector", async () => {
    const user = userEvent.setup();
    renderScreen({ draft: validDraft() });

    await user.click(screen.getByTestId("agent-create-runtime-select"));
    const speed = screen.getByTestId("runtime-selector-speed");
    expect(speed).toHaveAttribute("aria-checked", "false");
    await user.click(speed);
    expect(screen.getByTestId("runtime-selector-speed")).toHaveAttribute("aria-checked", "true");
  });

  it("Should translate the selected permission policy into its contract consequence", async () => {
    const user = userEvent.setup();
    renderScreen({ draft: validDraft() });

    await user.click(advancedToggle());

    expect(screen.getByTestId("agent-create-permissions-consequence")).toHaveTextContent(
      "The agent's provider decides when to ask you."
    );

    await user.click(screen.getByTestId("agent-create-permissions-approve-reads"));

    expect(screen.getByTestId("agent-create-permissions-consequence")).toHaveTextContent(
      "New sessions read freely and ask before changing anything."
    );
  });

  it("Should accept comma and newline token input while preserving de-duped order", async () => {
    const user = userEvent.setup();
    renderScreen({ draft: validDraft() });

    await user.click(advancedToggle());
    const input = screen.getByTestId("agent-create-tools-input");

    await user.type(input, "compozy__skill_view,");
    await user.type(input, "mcp__github__*{enter}");
    await user.type(input, "compozy__skill_view{enter}");

    expect(screen.getByTestId("agent-create-tools-tokens")).toHaveTextContent(
      "compozy__skill_view"
    );
    expect(screen.getByTestId("agent-create-tools-tokens")).toHaveTextContent("mcp__github__*");
    expect(screen.getAllByLabelText("Remove compozy__skill_view")).toHaveLength(1);
  });

  it("Should retain the draft and stay open when the save fails", () => {
    renderScreen({ draft: validDraft(), submitError: "agent definition already exists" });

    expect(screen.getByTestId("agent-create-submit-error")).toHaveTextContent(
      "agent definition already exists"
    );
    expect(screen.getByTestId("agent-create-name")).toHaveValue("Release Captain");
    expect(screen.getByTestId("submit-agent-create")).toBeEnabled();
  });

  it("Should disable controls while submitting", () => {
    renderScreen({ draft: validDraft(), isSubmitting: true });

    expect(screen.getByTestId("submit-agent-create")).toBeDisabled();
    expect(screen.getByTestId("agent-create-cancel")).toBeDisabled();
  });
});
