import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";
import { WorkBanner, WorkChip, WorkInspector, type OpenWorkEntry } from "@/systems/network";

const meta: Meta<typeof WorkChip> = {
  title: "systems/network/components/Work",
  component: WorkChip,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Three-layer work surfacing per `_design.md` §5.8: inline chip + auto-hiding banner + Work Inspector right-rail tab.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const sampleEntries: OpenWorkEntry[] = [
  {
    workId: "work_review_42",
    state: "working",
    messageId: "msg_review_42",
    targetPeerId: "reviewer.sess-xyz",
    openedAt: new Date(Date.now() - 12_000).toISOString(),
    lastActivityAt: new Date().toISOString(),
  },
  {
    workId: "work_review_43",
    state: "needs_input",
    messageId: "msg_review_43",
    targetPeerId: "reviewer.sess-xyz",
    openedAt: new Date(Date.now() - 240_000).toISOString(),
    lastActivityAt: new Date(Date.now() - 60_000).toISOString(),
  },
];

export const ChipStates: Story = {
  name: "Chip states",
  render: () => (
    <PanelSurface className="flex min-h-[120px] items-center justify-start gap-3 p-6">
      <WorkChip state="working" startedAt={new Date(Date.now() - 12_000).toISOString()} />
      <WorkChip state="needs_input" />
      <WorkChip state="failed" />
      <WorkChip state="canceled" />
    </PanelSurface>
  ),
};

export const BannerInfoTint: Story = {
  name: "Banner - info tint",
  render: () => (
    <PanelSurface className="min-h-[80px] p-0">
      <WorkBanner hasNeedsInput={false} openCount={2} />
    </PanelSurface>
  ),
};

export const BannerWarningTint: Story = {
  name: "Banner - warning tint (needs_input)",
  render: () => (
    <PanelSurface className="min-h-[80px] p-0">
      <WorkBanner hasNeedsInput needsInputCount={2} openCount={3} workingCount={1} />
    </PanelSurface>
  ),
};

export const BannerDangerTint: Story = {
  name: "Banner - danger tint (hard-stop threshold)",
  render: () => (
    <PanelSurface className="min-h-[80px] p-0">
      <WorkBanner hasNeedsInput needsInputCount={5} openCount={6} workingCount={1} />
    </PanelSurface>
  ),
};

export const Inspector: Story = {
  name: "Work Inspector - populated",
  render: () => (
    <PanelSurface className="min-h-[280px] p-0">
      <WorkInspector entries={sampleEntries} />
    </PanelSurface>
  ),
};

export const InspectorEmpty: Story = {
  name: "Work Inspector - empty",
  render: () => (
    <PanelSurface className="min-h-[280px] p-0">
      <WorkInspector entries={[]} />
    </PanelSurface>
  ),
};
