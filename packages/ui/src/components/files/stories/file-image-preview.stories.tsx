import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, waitFor } from "storybook/test";

import { FileImagePreview } from "../file-image-preview";
import { PNG_BASE64, SVG_WITH_SCRIPT } from "./files-fixtures";

const meta: Meta<typeof FileImagePreview> = {
  title: "components/files/FileImagePreview",
  component: FileImagePreview,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          'Image preview for png, jpeg, gif, webp, and svg. Every type renders through `<img src="data:…">`, so SVG markup is never inlined and its scripts never run.',
      },
    },
  },
  decorators: [
    Story => (
      <div className="flex h-[320px] flex-col">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Png: Story = {
  tags: ["play-fn"],
  args: { data: PNG_BASE64, mimeType: "image/png", alt: "checker.png", meta: "86 B" },
  play: async ({ canvasElement }) => {
    const image = canvasElement.querySelector("img");
    await waitFor(() =>
      expect(
        canvasElement.querySelector('[data-slot="file-image-preview-caption"]')
      ).toHaveTextContent(`${image?.naturalWidth} × ${image?.naturalHeight} · 86 B`)
    );
  },
};

export const SvgAsImage: Story = {
  name: "SVG as image only",
  tags: ["play-fn"],
  args: { data: btoa(SVG_WITH_SCRIPT), mimeType: "image/svg+xml", alt: "logo.svg" },
  play: async ({ canvasElement }) => {
    await expect(canvasElement.querySelector("img")?.getAttribute("src")).toMatch(
      /^data:image\/svg\+xml;base64,/
    );
    await expect(canvasElement.querySelector("svg script")).toBeNull();
  },
};

export const Unsupported: Story = {
  args: {
    data: "AAAA",
    mimeType: "image/tiff",
    alt: "scan.tiff",
    fallback: <p className="p-6 text-small-body text-muted-foreground">No preview</p>,
  },
};
