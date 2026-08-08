import type { Meta, StoryObj } from "@storybook/react-vite";

import { QrCode } from "../qr-code";

const meta: Meta<typeof QrCode> = {
  title: "components/custom/QrCode",
  component: QrCode,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Scannable QR matrix rendered as a single SVG path. Dark modules on a light plate so phone cameras decode it; always pair it with the same payload as copyable text.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Default 176 px matrix. */
export const Default: Story = {
  args: {
    value: "https://gateway.example.test/#pair=cpz_gwp_2f8c1d5a9b",
    label: "Scan to pair this device",
  },
};

/** Compact 128 px matrix for inline placement beside the copyable artifact. */
export const Small: Story = {
  args: {
    value: "https://gateway.example.test/#pair=cpz_gwp_2f8c1d5a9b",
    label: "Scan to pair this device",
    size: "sm",
  },
};

/** Long payloads pick a higher version automatically and stay the same rendered size. */
export const LongPayload: Story = {
  args: {
    value: `https://gateway.example.test/#pair=${"cpz_gwp_".padEnd(120, "a1b2c3d4")}`,
    label: "Scan to pair this device",
    size: "lg",
  },
};
