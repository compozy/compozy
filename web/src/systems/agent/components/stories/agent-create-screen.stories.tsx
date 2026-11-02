import type { AgentInstanceCapacity } from "../../lib/agent-instance";
import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import {
  AgentCreateScreen,
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
    runtime_strategy: provider.runtime_strategy,
  })
);

const workspaces = [workspaceDetailFixture.workspace];

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
  displayName: "Incident Triage",
  provider: modelProvider,
  model: "gpt-5.6-sol",
  reasoningEffort: "high",
  prompt:
    "You triage production incidents for the checkout platform. Gather evidence from logs and recent deploys, propose a likely root cause, and escalate to the on-call human when customer impact is confirmed. Never modify infrastructure directly.",
  permissions: "approve-reads",
  command: "",
  categoryPath: "operations/incident",
  tools: ["compozy__catalog"],
  toolsets: [],
  denyTools: [],
  disabledSkills: [],
};

const meta: Meta<typeof AgentCreateScreen> = {
  title: "systems/agent/components/AgentCreateScreen",
  component: AgentCreateScreen,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "One screen for every agent: who it is and what it follows on the left; how it works, where, its model and a live preview on the right. Advanced holds the Short name and what it can use.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function AgentCreateScreenHarness({
  initialDraft,
  isSubmitting = false,
  submitError = null,
  instanceCapacity = { limit: 10, active: 2, reached: false },
  startFailure = null,
  modelCatalogError = null,
  modelCatalogLoading = false,
  providersError = null,
  providersLoading = false,
  providers = providerOptions,
  models = runtimeModels,
  hasActiveWorkspace = true,
}: {
  initialDraft?: AgentCreateDialogDraft;
  isSubmitting?: boolean;
  submitError?: string | null;
  instanceCapacity?: AgentInstanceCapacity;
  startFailure?: string | null;
  modelCatalogError?: string | null;
  modelCatalogLoading?: boolean;
  providersError?: string | null;
  providersLoading?: boolean;
  providers?: RuntimeProviderOption[];
  models?: RuntimeModelOption[];
  hasActiveWorkspace?: boolean;
}) {
  const [draft, setDraft] = useState(initialDraft ?? createDefaultAgentCreateDraft(true));

  return (
    <AgentCreateScreen
      draft={draft}
      hasActiveWorkspace={hasActiveWorkspace}
      isSubmitting={isSubmitting}
      modelCatalogError={modelCatalogError}
      modelCatalogLoading={modelCatalogLoading}
      modelCatalogRefreshing={false}
      onDraftChange={setDraft}
      onOpenChange={() => undefined}
      onOpenCreatedAgent={fn()}
      onOpenProviderSettings={fn()}
      onRefreshCatalog={fn()}
      onSubmit={fn()}
      open
      providerOptions={providers}
      providersError={providersError}
      providersLoading={providersLoading}
      runtimeModels={models}
      instanceCapacity={instanceCapacity}
      startFailure={startFailure}
      submitError={submitError}
      takenNames={new Set(["reviewer"])}
      workspaceName={hasActiveWorkspace ? workspaceDetailFixture.workspace.name : null}
      workspaces={workspaces}
    />
  );
}

/** Simple tier: definition, runtime, and permission policy. */
export const Default: Story = {
  args: {},
  render: () => <AgentCreateScreenHarness initialDraft={validDraft} />,
};

/** Stays on: Where it works joins the side panel. */
export const StaysOn: Story = {
  args: {},
  render: () => <AgentCreateScreenHarness initialDraft={{ ...validDraft, kind: "stays_on" }} />,
};

/** The limit shows before submitting; the agent is created unstarted. */
export const LimitReached: Story = {
  args: {},
  render: () => (
    <AgentCreateScreenHarness
      instanceCapacity={{ limit: 10, active: 10, reached: true }}
      initialDraft={{ ...validDraft, kind: "stays_on" }}
    />
  ),
};

/** The agent exists but couldn't start: nothing is lost, and its page offers a retry. */
export const FailedToStart: Story = {
  args: {},
  render: () => (
    <AgentCreateScreenHarness
      initialDraft={{ ...validDraft, kind: "stays_on" }}
      startFailure="This agent can't reach that workspace."
    />
  ),
};

/** Advanced open: the Short name, permissions, tools and skills. */
export const Advanced: Story = {
  args: {},
  tags: ["play-fn"],
  render: () => <AgentCreateScreenHarness initialDraft={validDraft} />,
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(body.getByRole("button", { name: "Advanced" }));
    await expect(await body.findByTestId("agent-create-short-name")).toBeVisible();
  },
};

/** Pristine draft — both required fields empty and the primary gated. */
export const Empty: Story = {
  args: {},
  render: () => <AgentCreateScreenHarness />,
};

/** A name already in use, with a free one offered. */
export const NameTaken: Story = {
  args: {},
  render: () => (
    <AgentCreateScreenHarness initialDraft={{ ...validDraft, displayName: "Reviewer", name: "" }} />
  ),
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(body.getByTestId("agent-create-name-error")).toHaveTextContent(
      "You already have an agent with this name."
    );
  },
};

/** Authored runtime overrides with the project-default reset available. */
export const RuntimeOverride: Story = {
  args: {},
  render: () => <AgentCreateScreenHarness initialDraft={validDraft} />,
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(body.getByTestId("agent-create-runtime-use-project-defaults")).toBeVisible();
  },
};

/** Model catalog still loading — the draft survives, only the selector waits. */
export const CatalogLoading: Story = {
  args: {},
  render: () => <AgentCreateScreenHarness initialDraft={validDraft} modelCatalogLoading />,
};

/** Model catalog failed — catalog truth stays visible in Simple. */
export const CatalogError: Story = {
  args: {},
  render: () => (
    <AgentCreateScreenHarness
      initialDraft={validDraft}
      modelCatalogError="Model catalog refresh failed. Showing the last known models."
    />
  ),
};

/** No provider is configured for the selected scope. */
export const CatalogEmpty: Story = {
  args: {},
  render: () => (
    <AgentCreateScreenHarness
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
    <AgentCreateScreenHarness
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
    <AgentCreateScreenHarness
      hasActiveWorkspace={false}
      initialDraft={{ ...validDraft, scope: "global" }}
    />
  ),
};

/** Disabled submission state while the daemon validates the definition. */
export const Submitting: Story = {
  args: {},
  render: () => <AgentCreateScreenHarness initialDraft={validDraft} isSubmitting />,
};

/** Durable duplicate-name submission error; every entered value is retained. */
export const DuplicateError: Story = {
  args: {},
  render: () => (
    <AgentCreateScreenHarness
      initialDraft={validDraft}
      submitError="agent definition already exists"
    />
  ),
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await expect(body.getByTestId("agent-create-submit-error")).toHaveTextContent(
      "agent definition already exists"
    );
  },
};
