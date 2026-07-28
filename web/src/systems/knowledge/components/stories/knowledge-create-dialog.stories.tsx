import type { Meta, StoryObj } from "@storybook/react-vite";

import { KnowledgeCreateDialog } from "@/systems/knowledge/components/knowledge-create-dialog";

const meta: Meta<typeof KnowledgeCreateDialog> = {
  title: "systems/knowledge/components/KnowledgeCreateDialog",
  component: KnowledgeCreateDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Controller-mediated knowledge entry creation dialog. The user picks a memory `type` (user / feedback / project / reference), supplies a canonical name + optional description, and authors the markdown content. Submission flows through the controller and produces a fresh decision.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Default — empty dialog ready for a fresh entry. Defaults to `feedback` type
 * scope=global. Submission is gated until both name and content have content.
 */
export const Default: Story = {
  args: {},
  render: () => (
    <KnowledgeCreateDialog
      defaultType="feedback"
      isPending={false}
      onConfirm={async () => undefined}
      onOpenChange={() => undefined}
      open
      scope="global"
    />
  ),
};

/**
 * PendingSubmit — busy state: the create button is disabled and shows the
 * pending phase. Use to verify spinner/disabled treatment in flight.
 */
export const PendingSubmit: Story = {
  args: {},
  render: () => (
    <KnowledgeCreateDialog
      defaultType="project"
      isPending
      onConfirm={async () => undefined}
      onOpenChange={() => undefined}
      open
      scope="workspace"
    />
  ),
};

/**
 * RejectedByPolicy — controller rejection surfaces inline in the footer error
 * row. Demonstrates the canonical token-driven `text-danger` error styling
 * (no arbitrary color values).
 */
export const RejectedByPolicy: Story = {
  args: {},
  render: () => (
    <KnowledgeCreateDialog
      defaultType="reference"
      error="Create rejected by policy: agent-tier scope requires explicit agent name"
      isPending={false}
      onConfirm={async () => undefined}
      onOpenChange={() => undefined}
      open
      scope="agent"
    />
  ),
};
