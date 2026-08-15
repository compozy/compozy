import type { Meta, StoryObj } from "@storybook/react-vite";

import { SessionAttachmentCapabilityGate } from "@/components/assistant-ui/session-attachment-strip";
import {
  SessionAttachmentTile,
  type SessionAttachmentTileModel,
} from "@/components/assistant-ui/session-attachment-tile";
import { CenteredSurface } from "@/storybook/story-layout";

const tiles: SessionAttachmentTileModel[] = [
  {
    id: "att-1",
    name: "goal-chip.png",
    mimeType: "image/png",
    bytes: 245_760,
    state: "ready",
    sizeLabel: "240 KB",
    retryable: false,
  },
  {
    id: "att-2",
    name: "permission-dock.png",
    mimeType: "image/png",
    bytes: 98_304,
    state: "ready",
    sizeLabel: "96 KB",
    retryable: false,
  },
  {
    id: "att-3",
    name: "notes.md",
    mimeType: "text/markdown",
    bytes: 4096,
    state: "ready",
    sizeLabel: "4 KB",
    retryable: false,
  },
  {
    id: "att-4",
    name: "flatten-spec.pdf",
    mimeType: "application/pdf",
    bytes: 190_464,
    state: "ready",
    sizeLabel: "186 KB",
    retryable: false,
  },
  {
    id: "att-5",
    name: "empty-session.png",
    mimeType: "image/png",
    bytes: 198_656,
    state: "ready",
    sizeLabel: "194 KB",
    retryable: false,
  },
];

function StripPreview({ showGate = false }: { showGate?: boolean }) {
  return (
    <div className="flex w-[28rem] flex-col gap-1.5 rounded-lg border border-line bg-elevated p-3">
      {showGate ? (
        <SessionAttachmentCapabilityGate
          message="This agent does not accept images."
          onRemove={() => {}}
        />
      ) : null}
      <div data-overflow="end" className="relative min-w-0 isolate">
        <div
          role="list"
          className="flex flex-nowrap items-start gap-2 overflow-x-auto overflow-y-hidden py-0.5 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden"
        >
          {tiles.map(model => (
            <SessionAttachmentTile key={model.id} model={model} onRemove={() => {}} />
          ))}
        </div>
        <span
          aria-hidden="true"
          className="pointer-events-none absolute inset-y-0 end-0 z-2 w-14 bg-linear-to-l from-elevated via-elevated/70 to-transparent rtl:bg-linear-to-r"
        />
      </div>
    </div>
  );
}

const meta: Meta<typeof StripPreview> = {
  title: "systems/session/components/SessionAttachmentStrip",
  component: StripPreview,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Composer attachment strip: one row, host-fill edge fade, capability gate above the track.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <Story />
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Overflow: Story = {};

export const CapabilityGate: Story = {
  args: { showGate: true },
};
