import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import {
  storyAgentNames,
  storyChannels,
  storyDefaultWorkspaceName,
} from "@/storybook/fintech-scenario";
import { createNetworkChannelDraft } from "@/systems/network";
import { CenteredSurface } from "@/storybook/story-layout";
import { agentFixtures } from "@/systems/agent/mocks";

import { NetworkCreateChannelDialog } from "../network-create-channel-dialog";

const meta: Meta<typeof NetworkCreateChannelDialog> = {
  title: "systems/network/components/NetworkCreateChannelDialog",
  component: NetworkCreateChannelDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Dialog used by the network route to create a materialized channel from selected local agents.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function NetworkCreateChannelDialogHarness({ conflictMessage }: { conflictMessage?: string }) {
  const [draft, setDraft] = useState<ReturnType<typeof createNetworkChannelDraft>>({
    ...createNetworkChannelDraft(),
    channelName: storyChannels.merchantEscalations,
    purpose: "Coordinate VIP merchant escalations between risk, support, and settlement partners.",
    selectedAgentNames: [storyAgentNames.support, storyAgentNames.compliance],
  });

  return (
    <CenteredSurface className="flex-col gap-4">
      {conflictMessage ? (
        <div className="w-full max-w-md rounded-md border border-danger bg-danger-tint px-4 py-3 text-sm text-danger">
          {conflictMessage}
        </div>
      ) : null}
      <NetworkCreateChannelDialog
        agents={agentFixtures}
        canSubmit
        draft={draft}
        isSubmitting={false}
        onChannelNameChange={channelName => setDraft(current => ({ ...current, channelName }))}
        onFanoutPolicyChange={fanoutPolicy => setDraft(current => ({ ...current, fanoutPolicy }))}
        onOpenChange={() => undefined}
        onPurposeChange={purpose => setDraft(current => ({ ...current, purpose }))}
        onAgentSelectionChange={selectedAgentNames =>
          setDraft(current => ({ ...current, selectedAgentNames }))
        }
        onSubmit={() => undefined}
        open
        workspaceName={storyDefaultWorkspaceName}
      />
    </CenteredSurface>
  );
}

/**
 * Default create-channel dialog with two local agents selected.
 */
export const Default: Story = {
  args: {},
  render: () => <NetworkCreateChannelDialogHarness />,
};

/**
 * Duplicate-name validation message shown above the dialog.
 */
export const Error: Story = {
  name: "DuplicateNameError",
  args: {},
  render: () => (
    <NetworkCreateChannelDialogHarness conflictMessage="Channel name already exists in this workspace." />
  ),
};
