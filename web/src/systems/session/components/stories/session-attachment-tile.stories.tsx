import type { Meta, StoryObj } from "@storybook/react-vite";

import {
  SessionAttachmentTile,
  type SessionAttachmentTileModel,
} from "@/components/assistant-ui/session-attachment-tile";
import { CenteredSurface } from "@/storybook/story-layout";

const readyImage: SessionAttachmentTileModel = {
  id: "att-ready",
  name: "goal-chip.png",
  mimeType: "image/png",
  bytes: 245_760,
  state: "ready",
  sizeLabel: "240 KB",
  retryable: false,
};

const meta: Meta<typeof SessionAttachmentTile> = {
  title: "systems/session/components/SessionAttachmentTile",
  component: SessionAttachmentTile,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Composer draft attachment tile. States are data-state on the list item; Retry appears only for persist failure.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-80 rounded-lg border border-line bg-elevated p-2">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
  args: {
    model: readyImage,
    onRemove: () => {},
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const ReadyImage: Story = {};

export const Uploading: Story = {
  args: {
    model: {
      ...readyImage,
      id: "att-uploading",
      state: "uploading",
      sizeLabel: "Saving…",
    },
  },
};

export const PersistError: Story = {
  args: {
    model: {
      ...readyImage,
      id: "att-error",
      name: "scan-desk.jpg",
      mimeType: "image/jpeg",
      state: "error",
      sizeLabel: "Couldn't save",
      retryable: true,
    },
    onRetry: () => {},
  },
};

export const Oversize: Story = {
  args: {
    model: {
      ...readyImage,
      id: "att-oversize",
      name: "mockup-4k.png",
      bytes: 19_083_264,
      state: "error",
      sizeLabel: "18.2 MB · over 10 MB",
      retryable: false,
    },
  },
};

export const Rejected: Story = {
  args: {
    model: {
      id: "att-rejected",
      name: "diagram.svg",
      mimeType: "image/svg+xml",
      bytes: 12_288,
      state: "rejected",
      sizeLabel: "PNG, JPEG, WebP, PDF, Markdown, or text",
      retryable: false,
    },
  },
};

export const FilePdf: Story = {
  args: {
    model: {
      id: "att-pdf",
      name: "flatten-spec.pdf",
      mimeType: "application/pdf",
      bytes: 190_464,
      state: "ready",
      sizeLabel: "186 KB",
      retryable: false,
    },
  },
};

export const FileMarkdown: Story = {
  args: {
    model: {
      id: "att-md",
      name: "notes.md",
      mimeType: "text/markdown",
      bytes: 4096,
      state: "ready",
      sizeLabel: "4 KB",
      retryable: false,
    },
  },
};

export const FileText: Story = {
  args: {
    model: {
      id: "att-txt",
      name: "error-log.txt",
      mimeType: "text/plain",
      bytes: 12_288,
      state: "ready",
      sizeLabel: "12 KB",
      retryable: false,
    },
  },
};
