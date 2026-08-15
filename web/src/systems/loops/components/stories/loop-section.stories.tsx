import type { Meta, StoryObj } from "@storybook/react-vite";
import { History } from "lucide-react";

import { CenteredSurface } from "@/storybook/story-layout";

import { LoopSection } from "../loop-section";

const meta: Meta<typeof LoopSection> = {
  title: "systems/loops/components/LoopSection",
  component: LoopSection,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Collapsible main-column section for loop pages.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Open by default, with a one-line gist on the header.
 */
export const Default: Story = {
  args: {},
  render: () => (
    <CenteredSurface>
      <div className="w-full max-w-2xl">
        <LoopSection gist="12 events" icon={<History aria-hidden="true" />} title="What happened">
          <div className="rounded-lg border border-line bg-canvas-soft px-4 py-3 text-small-body text-fg">
            generation_started
          </div>
        </LoopSection>
      </div>
    </CenteredSurface>
  ),
};

/**
 * Folded: the gist stays on the header.
 */
export const Closed: Story = {
  args: {},
  render: () => (
    <CenteredSurface>
      <div className="w-full max-w-2xl">
        <LoopSection
          defaultOpen={false}
          gist="12 events"
          icon={<History aria-hidden="true" />}
          title="What happened"
        >
          <div className="rounded-lg border border-line bg-canvas-soft px-4 py-3 text-small-body text-fg">
            generation_started
          </div>
        </LoopSection>
      </div>
    </CenteredSurface>
  ),
};
