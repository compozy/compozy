import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { primaryAgentFixture } from "@/systems/agent/testing";

import {
  buildSettingsDraftFromAgent,
  validateAgentSettingsDraft,
} from "../../lib/agent-settings-draft";
import type { AgentSettingsSection } from "../../lib/agent-settings-search";
import type { AgentPayload } from "../../types";
import { AgentSettingsPanels, type AgentSettingsPanelsProps } from "../agent-settings-panels";

vi.mock("@/systems/runtime", () => ({
  RuntimeSelector: ({
    triggerTestId,
    onChange,
    value,
  }: {
    triggerTestId?: string;
    onChange: (next: { provider: string; model: string; reasoning_effort: string }) => void;
    value: { provider: string; model: string; reasoning_effort: string };
  }) => (
    <button
      type="button"
      data-testid={triggerTestId}
      data-provider={value.provider}
      data-model={value.model}
      data-reasoning-effort={value.reasoning_effort}
      onClick={() => onChange({ provider: "codex", model: "gpt-5", reasoning_effort: "high" })}
    >
      Runtime selector
    </button>
  ),
}));

function agent(overrides: Partial<AgentPayload> = {}): AgentPayload {
  return {
    ...primaryAgentFixture,
    origin: "workspace",
    permissions: "approve-reads",
    tools: ["agh__task_view"],
    toolsets: ["agh__catalog"],
    deny_tools: ["agh__task_delete"],
    skills: { disabled: ["release-notes"] },
    mcp_servers: [
      {
        name: "github",
        transport: "stdio",
        command: "github-mcp",
        env: { GITHUB_ORG: "redacted-value" },
        secret_env: { GITHUB_TOKEN: "secret-value" },
      },
    ],
    ...overrides,
  };
}

function props(section: AgentSettingsSection, overrides: Partial<AgentSettingsPanelsProps> = {}) {
  const currentAgent = agent();
  const draft = buildSettingsDraftFromAgent(currentAgent);
  return {
    section,
    agent: currentAgent,
    draft,
    validation: validateAgentSettingsDraft(draft),
    disabled: false,
    readOnly: false,
    mutationDenied: false,
    saveError: null,
    conflictBanner: null,
    onReloadAndRetry: vi.fn(),
    onPatch: vi.fn(),
    onDelete: vi.fn(),
    isDeleting: false,
    providerOptions: [],
    providersLoading: false,
    runtimeModels: [],
    modelCatalogLoading: false,
    modelCatalogLoaded: true,
    modelCatalogRefreshing: false,
    modelCatalogError: null,
    onRefreshCatalog: vi.fn(),
    onOpenProviderSettings: vi.fn(),
    ...overrides,
  } satisfies AgentSettingsPanelsProps;
}

describe("AgentSettingsPanels", () => {
  it("Should render effective project runtime without authoring an agent override", () => {
    const currentAgent = agent({
      provider: "",
      model: undefined,
      reasoning_effort: undefined,
      effective_runtime: {
        provider: "codex",
        model: "gpt-5.4",
        reasoning_effort: "high",
        sources: {
          provider: "project_default",
          model: "provider_default",
          reasoning_effort: "model_default",
        },
      },
    });
    const draft = buildSettingsDraftFromAgent(currentAgent);

    render(
      <AgentSettingsPanels
        {...props("runtime", {
          agent: currentAgent,
          draft,
          validation: validateAgentSettingsDraft(draft),
        })}
      />
    );

    const selector = screen.getByTestId("agent-settings-runtime-select");
    expect(selector).toHaveAttribute("data-provider", "codex");
    expect(selector).toHaveAttribute("data-model", "gpt-5.4");
    expect(selector).toHaveAttribute("data-reasoning-effort", "high");
    expect(screen.getByTestId("agent-settings-runtime-inherited")).toHaveTextContent(
      "provider, model, reasoning"
    );
  });

  it("Should render the Basics, Runtime, Instructions, and Access sections from one draft", async () => {
    const user = userEvent.setup();
    const onPatch = vi.fn();
    const view = render(<AgentSettingsPanels {...props("basics", { onPatch })} />);
    expect(screen.getByTestId("agent-settings-name")).toHaveAttribute("readonly");
    await user.type(screen.getByTestId("agent-settings-category"), "/release");
    expect(onPatch).toHaveBeenCalled();

    view.rerender(<AgentSettingsPanels {...props("runtime", { onPatch })} />);
    await user.click(screen.getByTestId("agent-settings-runtime-select"));
    expect(onPatch).toHaveBeenCalledWith({
      provider: "codex",
      model: "gpt-5",
      reasoningEffort: "high",
    });
    await user.click(screen.getByTestId("agent-settings-runtime-use-project-defaults"));
    expect(onPatch).toHaveBeenCalledWith({ provider: "", model: "", reasoningEffort: "" });
    await user.click(screen.getByTestId("agent-settings-permissions-approve-all"));
    expect(onPatch).toHaveBeenCalledWith({ permissions: "approve-all", legacyPermissions: null });

    view.rerender(<AgentSettingsPanels {...props("instructions", { onPatch })} />);
    expect(screen.getByTestId("agent-settings-prompt")).toHaveValue(primaryAgentFixture.prompt);
    fireEvent.change(screen.getByTestId("agent-settings-prompt"), {
      target: { value: `${primaryAgentFixture.prompt} updated` },
    });
    expect(onPatch).toHaveBeenCalledWith({ prompt: expect.stringContaining("updated") });

    view.rerender(<AgentSettingsPanels {...props("access", { onPatch })} />);
    expect(screen.getByTestId("agent-settings-tools-tokens")).toHaveTextContent("agh__task_view");
    expect(screen.getByTestId("agent-settings-disabled-skills-tokens")).toHaveTextContent(
      "release-notes"
    );
    for (const field of ["tools", "toolsets", "deny-tools", "disabled-skills"]) {
      await user.type(screen.getByTestId(`agent-settings-${field}-input`), `new-${field}`);
      await user.click(screen.getByTestId(`agent-settings-${field}-add`));
    }
    expect(onPatch).toHaveBeenCalledWith({ tools: ["agh__task_view", "new-tools"] });
    expect(onPatch).toHaveBeenCalledWith({ toolsets: ["agh__catalog", "new-toolsets"] });
    expect(onPatch).toHaveBeenCalledWith({
      denyTools: ["agh__task_delete", "new-deny-tools"],
    });
    expect(onPatch).toHaveBeenCalledWith({
      disabledSkills: ["release-notes", "new-disabled-skills"],
    });
  });

  it("Should show only MCP key names and truthful global Danger copy", async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    const currentAgent = agent({ origin: "global" });
    const view = render(
      <AgentSettingsPanels
        {...props("mcp", {
          agent: currentAgent,
          draft: buildSettingsDraftFromAgent(currentAgent),
        })}
      />
    );
    expect(screen.getByText("GITHUB_ORG")).toBeVisible();
    expect(screen.getByText("GITHUB_TOKEN")).toBeVisible();
    expect(screen.queryByText("redacted-value")).toBeNull();
    expect(screen.queryByText("secret-value")).toBeNull();

    view.rerender(
      <AgentSettingsPanels
        {...props("danger", {
          agent: currentAgent,
          draft: buildSettingsDraftFromAgent(currentAgent),
          onDelete,
        })}
      />
    );
    expect(screen.getByTestId("agent-settings-danger")).toHaveTextContent(
      "from the global agent home"
    );
    await user.click(screen.getByTestId("agent-settings-delete"));
    expect(onDelete).toHaveBeenCalledTimes(1);
  });

  it("Should keep fields visible and read-only after mutation denial", () => {
    const view = render(
      <AgentSettingsPanels
        {...props("instructions", {
          mutationDenied: true,
          readOnly: true,
          saveError: null,
        })}
      />
    );
    expect(screen.getByTestId("agent-settings-mutation-denied")).toBeVisible();
    const prompt = screen.getByTestId("agent-settings-prompt");
    expect(prompt).toHaveAttribute("readonly");
    expect(prompt).toHaveAttribute("aria-readonly", "true");
    expect(prompt).not.toHaveAttribute("aria-disabled");
    expect(prompt).not.toBeDisabled();

    view.rerender(
      <AgentSettingsPanels
        {...props("basics", {
          mutationDenied: true,
          readOnly: true,
          saveError: null,
        })}
      />
    );
    const category = screen.getByTestId("agent-settings-category");
    expect(category).toHaveAttribute("readonly");
    expect(category).toHaveAttribute("aria-readonly", "true");
    expect(category).not.toHaveAttribute("aria-disabled");
    expect(category).not.toBeDisabled();
  });

  it("Should disable delete when read-only or mutation-denied", () => {
    const onDelete = vi.fn();
    const view = render(<AgentSettingsPanels {...props("danger", { onDelete, readOnly: true })} />);
    expect(screen.getByTestId("agent-settings-delete")).toBeDisabled();

    view.rerender(<AgentSettingsPanels {...props("danger", { onDelete, mutationDenied: true })} />);
    expect(screen.getByTestId("agent-settings-delete")).toBeDisabled();
    expect(onDelete).not.toHaveBeenCalled();
  });
});
