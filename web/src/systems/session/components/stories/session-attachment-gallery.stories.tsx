import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import type { SessionAttachmentItem } from "@/systems/session";

import { SessionAttachmentGallery } from "../session-attachment-gallery";

const STORY_IMAGE =
  "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==";

function imageItem(id: string, filename: string): SessionAttachmentItem {
  return {
    kind: "image",
    id,
    href: `#${id}`,
    src: STORY_IMAGE,
    filename,
    width: 1280,
    height: 720,
  };
}

function fileItem(
  id: string,
  filename: string,
  extension: string,
  sizeLabel: string
): SessionAttachmentItem {
  return {
    kind: "file",
    id,
    href: `#${id}`,
    filename,
    extension,
    sizeLabel,
  };
}

const meta: Meta<typeof SessionAttachmentGallery> = {
  title: "systems/session/components/SessionAttachmentGallery",
  component: SessionAttachmentGallery,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Right-aligned transcript attachment rail. Frames stay 280px; overflow is a one-row fade, never a wrap.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="flex w-[420px] flex-col items-end bg-canvas p-4">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
  args: {
    items: [imageItem("att-diagram", "diagram.png")],
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** One image frame above a user turn. */
export const OneImage: Story = {
  args: {
    items: [imageItem("att-diagram", "diagram.png")],
  },
};

/** Six frames overflow into the 64px canvas fade. */
export const Overflow: Story = {
  args: {
    items: [
      imageItem("att-1", "one.png"),
      imageItem("att-2", "two.png"),
      imageItem("att-3", "three.png"),
      imageItem("att-4", "four.png"),
      imageItem("att-5", "five.png"),
      imageItem("att-6", "six.png"),
    ],
  },
};

/** Frames and file cards share insert order on the same rail. */
export const Mixed: Story = {
  args: {
    items: [
      imageItem("att-diagram", "diagram.png"),
      fileItem("att-notes", "notes.pdf", "PDF", "48 KB"),
      imageItem("att-shot", "shot.webp"),
    ],
  },
};

/** File-only turn — no image frames. */
export const FilesOnly: Story = {
  args: {
    items: [
      fileItem("att-notes", "notes.pdf", "PDF", "48 KB"),
      fileItem("att-readme", "README.md", "MD", "2.1 KB"),
      fileItem("att-log", "trace.txt", "TXT", "812 B"),
    ],
  },
};
