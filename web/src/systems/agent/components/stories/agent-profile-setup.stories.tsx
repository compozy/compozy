import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import type { UseAgentRuntimeEditorResult } from "../../hooks/use-agent-runtime-editor";
import { AgentProfileSetup } from "../agent-profile/agent-profile-setup";

function modelView(overrides: Partial<UseAgentRuntimeEditorResult> = {}) {
  return {
    value: { provider: "claude", model: "claude-opus-5-5", reasoning_effort: "high" },
    speed: "normal",
    overridden: false,
    onChange: fn(),
    onSpeedChange: fn(),
    onUseDefault: fn(),
    providerOptions: [{ id: "claude", name: "Claude", runtime_provider: "claude" }],
    providersLoading: false,
    providerSourceError: null,
    onRetryProviderSource: fn(),
    runtimeModels: [
      {
        id: "claude-opus-5-5",
        provider: "claude",
        name: "Opus 5.5",
        efforts: ["low", "medium", "high", "xhigh"],
        availability: "live",
        curated: true,
      },
      {
        id: "claude-sonnet-5-5",
        provider: "claude",
        name: "Sonnet 5.5",
        efforts: ["low", "medium", "high"],
        availability: "live",
        curated: true,
      },
    ],
    modelCatalogLoading: false,
    modelCatalogRefreshing: false,
    modelCatalogError: null,
    onRefreshCatalog: fn(),
    onOpenProviderSettings: fn(),
    isPending: false,
    error: null,
    conflictMessage: null,
    onReload: null,
    onDismissError: fn(),
    ...overrides,
  } satisfies UseAgentRuntimeEditorResult;
}

const meta: Meta<typeof AgentProfileSetup> = {
  title: "systems/agent/components/AgentProfileSetup",
  component: AgentProfileSetup,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "How a stays-on agent is set up. The Model row is the runtime selector itself: a pick saves at once.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[22rem]">
        <Story />
      </div>
    ),
  ],
  args: {
    staysOn: true,
    readOnly: false,
    model: modelView(),
    summaries: {
      behavior: "Watch new issues and propose the next step",
      checkIns: "On in 1 project",
      memory: "3 notes",
      tools: "Everything it's allowed",
      permissions: "Edit on its own",
    },
    onOpenBehavior: fn(),
    onOpenCheckIns: fn(),
    onOpenMemory: fn(),
    onOpenTools: fn(),
    onOpenPermissions: fn(),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const UsesDefault: Story = {};

export const Overridden: Story = {
  args: {
    model: modelView({
      overridden: true,
      value: { provider: "claude", model: "claude-sonnet-5-5", reasoning_effort: "medium" },
    }),
  },
};

export const SaveFailed: Story = {
  args: {
    model: modelView({ overridden: true, error: "Couldn't update the model" }),
  },
};

export const ReadOnly: Story = {
  args: { readOnly: true, owner: "Acme kit", model: modelView({ overridden: true }) },
};
