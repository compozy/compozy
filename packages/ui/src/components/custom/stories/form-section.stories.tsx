import type { Meta, StoryObj } from "@storybook/react-vite";
import { Bot, Clock, ShieldCheck } from "lucide-react";

import {
  Field,
  FieldDescription,
  FieldHeader,
  FieldLabel,
  FormSection,
  HelpTip,
  Input,
  Textarea,
} from "@agh/ui";

const meta: Meta<typeof FormSection> = {
  title: "components/custom/FormSection",
  component: FormSection,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "One titled block of a modal body or settings form. Follows `.sec` (modal-system.css:221): a hairline rule and 22 px top padding, no card surface and no horizontal padding of its own, so fields align with the body gutter instead of stepping 16 px further in. Title is 12.5/600/-0.012em. Explanatory prose lives in `help`; `description` is reserved for runtime truth.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[720px] bg-background p-6">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Explanatory prose sits behind the `(?)`, keeping the default read quiet. */
export const WithHelp: Story = {
  args: {},
  render: () => (
    <FormSection
      help="Sessions launched from this definition inherit the visibility you pick here."
      icon={ShieldCheck}
      rightLabel="Required"
      title="Scope"
    >
      <Field>
        <FieldHeader>
          <FieldLabel htmlFor="scope">Visibility</FieldLabel>
          <HelpTip label="About visibility">
            A workspace agent is reachable only inside that project; a global agent is reachable
            everywhere.
          </HelpTip>
        </FieldHeader>
        <Input id="scope" placeholder="workspace" />
      </Field>
      <Field>
        <FieldLabel htmlFor="notes">Notes</FieldLabel>
        <Textarea id="notes" placeholder="Context this agent should carry into every session" />
      </Field>
    </FormSection>
  ),
};

/** `description` carries what the runtime will do — never an explanation. */
export const WithRuntimeTruth: Story = {
  args: {},
  render: () => (
    <FormSection description="Project runtime defaults will be used." icon={Bot} title="Runtime">
      <Field>
        <FieldLabel htmlFor="model">Model</FieldLabel>
        <Input id="model" placeholder="Select model" />
        <FieldDescription>Backed by the live provider catalog.</FieldDescription>
      </Field>
    </FormSection>
  ),
};

/** Stacked sections — the first drops its rule, the rest carry a hairline. */
export const Stacked: Story = {
  args: {},
  render: () => (
    <div className="flex flex-col">
      <FormSection help="What this job is called in the catalog." icon={Bot} title="The definition">
        <Input placeholder="daily-code-review" />
      </FormSection>
      <FormSection help="All times evaluate in UTC." icon={Clock} title="On this schedule">
        <Input className="font-mono" placeholder="0 9 * * *" />
      </FormSection>
    </div>
  ),
};

/** No icon, no help, no right label — verifies the head collapses cleanly. */
export const HeadOnly: Story = {
  args: {},
  render: () => (
    <FormSection title="Schedule">
      <Input placeholder="cron expression" />
    </FormSection>
  ),
};
