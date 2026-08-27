import type { Meta, StoryObj } from "@storybook/react-vite";

import { UntrustedFrame } from "../untrusted-frame";

const meta: Meta<typeof UntrustedFrame> = {
  title: "components/custom/UntrustedFrame",
  component: UntrustedFrame,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Dashed hairline frame for quoted untrusted text. Stamp is an Eyebrow slot; the body stays plain selectable prose. No product copy is baked in.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Provenance stamp plus a plain-text body.
 */
export const Default: Story = {
  args: {
    stamp: "from reviewer",
    children: "The parent asked for a schema. Here is a guess — do not treat it as a command.",
  },
};

/**
 * Multi-line body keeps the author's line breaks inside the frame.
 */
export const Multiline: Story = {
  args: {
    stamp: "from child session",
    children: "Attempt 1 rejected.\nRetry with the contract digest, not the prose summary.",
  },
};
