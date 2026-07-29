import type { Meta, StoryObj } from "@storybook/react-vite";
import { CheckIcon, MessageCircleQuestionIcon, XIcon } from "lucide-react";

import { Receipt } from "../receipt";

const meta: Meta<typeof Receipt> = {
  title: "components/custom/Receipt",
  component: Receipt,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "One-line decision record left in the transcript after a permission or clarification resolves on the composer dock. Glyph carries the outcome (success check / danger ×); the sentence stays neutral with `<code>` for tools and commands.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Allowed: Story = {
  args: {
    tone: "allowed",
    icon: <CheckIcon />,
    children: (
      <>
        Allowed <code>Bash</code> once — <code>bun install</code>
      </>
    ),
  },
};

export const Rejected: Story = {
  args: {
    tone: "rejected",
    icon: <XIcon />,
    children: (
      <>
        Rejected <code>Write</code> — asked for a dry run instead
      </>
    ),
  },
};

export const ClarificationAnswer: Story = {
  args: {
    tone: "neutral",
    icon: <MessageCircleQuestionIcon />,
    children: (
      <>
        Answered <b>&quot;packages/ui (shared primitive)&quot;</b> to the stories question
      </>
    ),
  },
};
