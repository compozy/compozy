import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";

import { ClarificationCard } from "../clarification-card";

const meta: Meta<typeof ClarificationCard> = {
  title: "systems/session/components/ClarificationCard",
  component: ClarificationCard,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Operator answer surface for a live clarification — bounded choice buttons or a free-text form. Inline and action-signaled (accent StatusDot), deliberately distinct from the permission prompt.",
      },
    },
  },
  args: {
    question: "Which environment should the migration target first?",
    choices: ["Staging", "Production"],
    deadlineHint: "3:45 PM",
    isSubmitting: false,
    errorMessage: null,
    onAnswerChoice: fn(),
    onAnswerText: fn(),
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-full max-w-xl">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Bounded choices — up to four single-click answers submitting a zero-based index. */
export const PendingChoices: Story = { args: {} };

/** Free-text question — a labeled input that submits trimmed text. */
export const PendingFreeText: Story = {
  args: { choices: undefined, question: "Describe the rollback plan you would prefer." },
};

/** Mid-submit — every answer path is disabled so a duplicate submission is impossible. */
export const Submitting: Story = { args: { isSubmitting: true } };

/** Retryable failure — an accessible inline error while the controls stay answerable. */
export const ErrorRetry: Story = {
  args: { errorMessage: "Couldn't send your answer. Try again." },
};
