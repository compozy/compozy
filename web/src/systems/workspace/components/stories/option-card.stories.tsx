import { FolderPlus, Home } from "lucide-react";
import type { Meta, StoryObj } from "@storybook/react-vite";

import { Button, Pill } from "@agh/ui";

import { CenteredSurface } from "@/storybook/story-layout";

import { OptionCard } from "../option-card";

const meta: Meta<typeof OptionCard> = {
  title: "systems/workspace/components/OptionCard",
  component: OptionCard,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "System-local compound primitive that replaces the legacy `SetupOptionCard` 9-prop bag. Composes `Section` for the eyebrow/right chrome and exposes named slots for the icon, title, description, meta, and action.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-full max-w-md">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Comfortable size used by the first-run onboarding rail.
 */
export const Comfortable: Story = {
  args: {},
  render: () => (
    <OptionCard size="comfortable" data-testid="option-card-comfortable">
      <OptionCard.Header eyebrow="Global" right={<Pill tone="accent">HOME</Pill>} />
      <OptionCard.Body>
        <OptionCard.Icon tone="accent">
          <Home className="size-4" />
        </OptionCard.Icon>
        <OptionCard.Content>
          <OptionCard.Title>Use global workspace</OptionCard.Title>
          <OptionCard.Description>
            Resolve the daemon's $HOME workspace and skip a per-project path.
          </OptionCard.Description>
          <OptionCard.Meta>/Users/pedro</OptionCard.Meta>
        </OptionCard.Content>
      </OptionCard.Body>
      <OptionCard.Action>
        <Button className="w-full justify-between text-accent-ink">Use this workspace</Button>
      </OptionCard.Action>
    </OptionCard>
  ),
};

/**
 * Compact size used inside the ruled workspace setup dialog.
 */
export const Compact: Story = {
  args: {},
  render: () => (
    <OptionCard size="compact" data-testid="option-card-compact">
      <OptionCard.Header eyebrow="Path" right={<Pill>MANUAL</Pill>} />
      <OptionCard.Body>
        <OptionCard.Icon tone="neutral">
          <FolderPlus className="size-4" />
        </OptionCard.Icon>
        <OptionCard.Content>
          <OptionCard.Title>Pick a workspace path</OptionCard.Title>
          <OptionCard.Description>
            Provide an absolute path AGH will register and watch.
          </OptionCard.Description>
        </OptionCard.Content>
      </OptionCard.Body>
      <OptionCard.Action>
        <Button className="w-full justify-between text-accent-ink">Register path</Button>
      </OptionCard.Action>
    </OptionCard>
  ),
};
