import type { Meta, StoryObj } from "@storybook/react-vite";

import { Field, FieldLabel } from "../../field";
import { Input } from "../../input";
import { RequiredMark } from "../required-mark";

const meta: Meta<typeof RequiredMark> = {
  title: "components/custom/RequiredMark",
  component: RequiredMark,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Accent affix marking a field the contract rejects when empty. The designed entity editors spell the requirement out next to the label instead of leaning on an asterisk, so the rule never depends on a glyph alone.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const OnFieldLabel: Story = {
  render: () => (
    <div className="w-(--width-modal-sm) max-w-full p-6">
      <Field>
        <FieldLabel htmlFor="required-mark-story-name">
          Agent name
          <RequiredMark />
        </FieldLabel>
        <Input id="required-mark-story-name" placeholder="incident-triage" />
      </Field>
    </div>
  ),
};

export const CustomAffix: Story = {
  render: () => (
    <div className="w-(--width-modal-sm) max-w-full p-6">
      <Field>
        <FieldLabel htmlFor="required-mark-story-slot">
          Bot token
          <RequiredMark>required by provider</RequiredMark>
        </FieldLabel>
        <Input id="required-mark-story-slot" placeholder="xoxb-…" type="password" />
      </Field>
    </div>
  ),
};
