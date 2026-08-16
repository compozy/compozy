import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";

import { SessionAttachmentFileCard } from "../session-attachment-file-card";

const meta: Meta<typeof SessionAttachmentFileCard> = {
  title: "systems/session/components/SessionAttachmentFileCard",
  component: SessionAttachmentFileCard,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Sent file card in the transcript gallery. 36×36 extension well, name + mono size, no dismiss control.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="bg-canvas p-6">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
  args: {
    href: "#att-notes",
    filename: "notes.pdf",
    extension: "PDF",
    sizeLabel: "48 KB",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** PDF with a recorded size. */
export const Pdf: Story = {
  args: {},
};

/** Markdown card. */
export const Markdown: Story = {
  args: {
    href: "#att-readme",
    filename: "README.md",
    extension: "MD",
    sizeLabel: "2.1 KB",
  },
};

/** Size omitted when metadata does not carry bytes. */
export const WithoutSize: Story = {
  args: {
    href: "#att-log",
    filename: "trace.txt",
    extension: "TXT",
  },
};
