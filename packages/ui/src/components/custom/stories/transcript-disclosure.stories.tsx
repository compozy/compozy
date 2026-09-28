import type { Meta, StoryObj } from "@storybook/react-vite";
import { Brain, Terminal } from "lucide-react";
import { useState } from "react";
import { expect, userEvent, within } from "storybook/test";

import { TranscriptDisclosure } from "../transcript-disclosure";

const meta: Meta<typeof TranscriptDisclosure> = {
  title: "components/custom/TranscriptDisclosure",
  component: TranscriptDisclosure,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "The transcript's controlled disclosure trigger. The caller owns the revealed body and points at it with `aria-controls`; `row` is a tool/work line, `turn` the quiet turn-fold sentence.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Controlled({
  variant,
  icon,
  label,
  trailing,
}: Pick<
  React.ComponentProps<typeof TranscriptDisclosure>,
  "variant" | "icon" | "label" | "trailing"
>) {
  const [expanded, setExpanded] = useState(false);
  return (
    <div className="flex max-w-md flex-col">
      <TranscriptDisclosure
        aria-controls="story-transcript-body"
        expanded={expanded}
        icon={icon}
        label={label}
        onToggle={() => setExpanded(value => !value)}
        trailing={trailing}
        variant={variant}
      />
      {expanded ? (
        <p
          className="ml-transcript-detail-indent text-transcript-body text-muted"
          id="story-transcript-body"
        >
          The revealed body belongs to the caller.
        </p>
      ) : null}
    </div>
  );
}

/** A tool/work line: icon well, label, rotating chevron. */
export const Row: Story = {
  args: { expanded: false, onToggle: () => {}, icon: null, label: "" },
  render: () => (
    <Controlled
      icon={<Terminal aria-hidden="true" className="size-3 text-subtle" />}
      label="Ran command"
      trailing={<span className="text-transcript-meta text-subtle">+12 −3</span>}
    />
  ),
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const trigger = canvas.getByRole("button", { name: /Ran command/ });
    await userEvent.click(trigger);
    await expect(trigger).toHaveAttribute("aria-expanded", "true");
  },
};

/** Settled reasoning: verb plus a faint preview. */
export const RowWithPreview: Story = {
  args: Row.args,
  render: () => (
    <Controlled
      icon={<Brain aria-hidden="true" className="size-3 text-subtle" />}
      label={
        <>
          Thought <span className="font-normal text-subtle">Check the retry fence first</span>
        </>
      }
    />
  ),
};

/** The turn fold: leading chevron, quiet sentence. */
export const Turn: Story = {
  args: Row.args,
  render: () => <Controlled icon={null} label="Worked for 2m 14s · 6 tool calls" variant="turn" />,
};
