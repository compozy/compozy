import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, within } from "storybook/test";

import type { EntityMode } from "@agh/ui";

import {
  AgentCreateDialog,
  createDefaultAgentCreateDraft,
  type AgentCreateDialogDraft,
} from "@/systems/agent";
import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";
import { workspaceDetailFixture } from "@/systems/workspace/mocks";

const providerOptions: RuntimeProviderOption[] = (workspaceDetailFixture.providers ?? []).map(
  provider => ({
    id: provider.name,
    name: provider.display_name?.trim() || provider.name,
    ...(provider.harness?.trim() ? { harness: provider.harness.trim() } : {}),
    runtime_provider: provider.runtime_provider?.trim() || provider.name,
  })
);

const modelProvider = providerOptions[0]?.id ?? "codex";

const runtimeModels: RuntimeModelOption[] = [
  {
    id: "gpt-5.6-sol",
    provider: modelProvider,
    name: "GPT-5.6 Sol",
    efforts: ["low", "medium", "high", "xhigh", "max"],
    availability: "live",
    curated: true,
    featured: true,
    context_window: 1_050_000,
    cost_input: 5,
    cost_output: 30,
    supports_tools: true,
    reasoning_source: "acp",
  },
  {
    id: "gpt-5.6-luna",
    provider: modelProvider,
    name: "GPT-5.6 Luna",
    efforts: ["low", "high"],
    availability: "live",
    curated: true,
    context_window: 1_050_000,
    cost_input: 1,
    cost_output: 6,
    supports_tools: true,
  },
];

const validDraft: AgentCreateDialogDraft = {
  ...createDefaultAgentCreateDraft(true),
  name: "incident-triage",
  provider: modelProvider,
  model: "gpt-5.6-sol",
  reasoningEffort: "high",
  prompt:
    "You triage production incidents for the checkout platform. Gather evidence from logs and recent deploys, propose a likely root cause, and escalate to the on-call human when customer impact is confirmed. Never modify infrastructure directly.",
  permissions: "approve-reads",
  command: "",
  categoryPath: "operations/incident",
  tools: ["agh__catalog"],
  toolsets: [],
  denyTools: [],
  disabledSkills: [],
};

const meta: Meta<typeof AgentCreateDialog> = {
  title: "systems/agent/components/AgentCreateDialog",
  component: AgentCreateDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Create-only agent editor on the shared entity-dialog shell. Simple carries the definition, runtime, and permission policy; Advanced adds runtime overrides and the tool/skill allowlists.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function AgentCreateDialogHarness({
  initialDraft,
  initialMode,
  isSubmitting = false,
  submitError = null,
  modelCatalogError = null,
  modelCatalogLoading = false,
  modelCatalogLoaded = true,
  providersError = null,
  providersLoading = false,
  providers = providerOptions,
  models = runtimeModels,
  hasActiveWorkspace = true,
}: {
  initialDraft?: AgentCreateDialogDraft;
  initialMode?: EntityMode;
  isSubmitting?: boolean;
  submitError?: string | null;
  modelCatalogError?: string | null;
  modelCatalogLoading?: boolean;
  modelCatalogLoaded?: boolean;
  providersError?: string | null;
  providersLoading?: boolean;
  providers?: RuntimeProviderOption[];
  models?: RuntimeModelOption[];
  hasActiveWorkspace?: boolean;
}) {
  const [draft, setDraft] = useState(initialDraft ?? createDefaultAgentCreateDraft(true));

  return (
    <AgentCreateDialog
      draft={draft}
      hasActiveWorkspace={hasActiveWorkspace}
      initialMode={initialMode}
      isSubmitting={isSubmitting}
      modelCatalogError={modelCatalogError}
      modelCatalogLoading={modelCatalogLoading}
      modelCatalogLoaded={modelCatalogLoaded}
      modelCatalogRefreshing={false}
      onDraftChange={setDraft}
      onOpenChange={() => undefined}
      onOpenProviderSettings={fn()}
      onRefreshCatalog={fn()}
      onSubmit={fn()}
      open
      providerOptions={providers}
      providersError={providersError}
      providersLoading={providersLoading}
      runtimeModels={models}
      submitError={submitError}
      workspaceId={hasActiveWorkspace ? workspaceDetailFixture.workspace.id : null}
      workspaceName={hasActiveWorkspace ? workspaceDetailFixture.workspace.name : null}
    />
  );
}

/** Simple tier: definition, runtime, and permission policy. */
export const Default: Story = {
  args: {},
  render: () => <AgentCreateDialogHarness initialDraft={validDraft} />,
};

/** Advanced tier: runtime overrides plus the tool and skill allowlists. */
export const Advanced: Story = {
  args: {},
  render: () => <AgentCreateDialogHarness initialDraft={validDraft} initialMode="advanced" />,
};

/**
 * Advanced tier scrolled to its tail — the runtime-override and allowlist blocks
 * sit below the fold, so a capture at `scrollTop = 0` cannot show them.
 */
export const AdvancedTail: Story = {
  args: {},
  tags: ["play-fn"],
  render: () => <AgentCreateDialogHarness initialDraft={validDraft} initialMode="advanced" />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const tail = await canvas.findByTestId("agent-create-disabled-skills-input");
    tail.scrollIntoView({ block: "end" });
    await expect(canvas.getByTestId("agent-create-category-path")).toBeVisible();
  },
};

/** Pristine draft — both required fields empty and the primary gated. */
export const Empty: Story = {
  args: {},
  render: () => <AgentCreateDialogHarness />,
};

/** Canonical validation failures on the required and advanced fields. */
export const ValidationError: Story = {
  args: {},
  render: () => (
    <AgentCreateDialogHarness
      initialDraft={{
        ...createDefaultAgentCreateDraft(true),
        name: "../release",
        categoryPath: "Engineering//Release",
      }}
      initialMode="advanced"
    />
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByTestId("agent-create-name-error")).toHaveTextContent(
      "Agent names cannot be . or .."
    );
    await expect(canvas.getByTestId("agent-create-category-path-error")).toHaveTextContent(
      "Category path cannot contain blank segments."
    );
  },
};

/** Authored runtime overrides with the project-default reset available. */
export const RuntimeOverride: Story = {
  args: {},
  render: () => <AgentCreateDialogHarness initialDraft={validDraft} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByTestId("agent-create-runtime-use-project-defaults")).toBeVisible();
  },
};

/** Model catalog still loading — the draft survives, only the selector waits. */
export const CatalogLoading: Story = {
  args: {},
  render: () => (
    <AgentCreateDialogHarness
      initialDraft={validDraft}
      modelCatalogLoading
      modelCatalogLoaded={false}
    />
  ),
};

/** Model catalog failed — catalog truth stays visible in Simple. */
export const CatalogError: Story = {
  args: {},
  render: () => (
    <AgentCreateDialogHarness
      initialDraft={validDraft}
      modelCatalogError="Model catalog refresh failed. Showing the last known models."
    />
  ),
};

/** No provider is configured for the selected scope. */
export const CatalogEmpty: Story = {
  args: {},
  render: () => (
    <AgentCreateDialogHarness
      initialDraft={{ ...validDraft, provider: "", model: "", reasoningEffort: "" }}
      models={[]}
      providers={[]}
    />
  ),
};

/** Provider settings could not be read — submit stays gated with the cause shown. */
export const ProvidersError: Story = {
  args: {},
  render: () => (
    <AgentCreateDialogHarness
      initialDraft={{ ...validDraft, provider: "", model: "", reasoningEffort: "" }}
      providers={[]}
      providersError="Unable to load global provider settings."
    />
  ),
};

/** No active workspace — workspace scope is unavailable, global still works. */
export const ScopeUnavailable: Story = {
  args: {},
  render: () => (
    <AgentCreateDialogHarness
      hasActiveWorkspace={false}
      initialDraft={{ ...validDraft, scope: "global" }}
    />
  ),
};

/** Disabled submission state while the daemon validates the definition. */
export const Submitting: Story = {
  args: {},
  render: () => <AgentCreateDialogHarness initialDraft={validDraft} isSubmitting />,
};

/** Durable duplicate-name submission error; every entered value is retained. */
export const DuplicateError: Story = {
  args: {},
  render: () => (
    <AgentCreateDialogHarness
      initialDraft={validDraft}
      submitError="agent definition already exists"
    />
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByTestId("agent-create-submit-error")).toHaveTextContent(
      "agent definition already exists"
    );
  },
};
