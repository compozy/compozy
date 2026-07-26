import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import { Field, FieldHeader, FieldLabel, HelpTip, Input, RequiredMark } from "@agh/ui";

const meta: Meta<typeof HelpTip> = {
  title: "components/custom/HelpTip",
  component: HelpTip,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "The `(?)` beside a label: explanatory prose on demand instead of a permanent description line under every field. A real 24 px button (WCAG 2.2 SC 2.5.8) around a 14 px glyph, opening on hover, focus, and click — the click path is what makes the prose reachable on touch. Always a sibling of the `<label>`, never a child.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[420px] bg-background p-6">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** The canonical pairing: label, required mark, then the help trigger. */
export const BesideFieldLabel: Story = {
  args: {},
  render: () => (
    <Field>
      <FieldHeader>
        <FieldLabel htmlFor="category-path">
          Category path
          <RequiredMark />
        </FieldLabel>
        <HelpTip label="About category path">
          Slash-separated catalog grouping, stored as one segment per level.
        </HelpTip>
      </FieldHeader>
      <Input className="font-mono" id="category-path" placeholder="operations/incident" />
    </Field>
  ),
};

/** Long prose wraps inside the 288 px ceiling instead of stretching the popup. */
export const LongProse: Story = {
  args: {},
  render: () => (
    <Field>
      <FieldHeader>
        <FieldLabel htmlFor="runtime">Runtime</FieldLabel>
        <HelpTip label="About runtime">
          Leave this unchanged to inherit the project defaults. Selecting a provider, model, or
          reasoning effort creates agent-level overrides that every session launched from this
          definition will carry.
        </HelpTip>
      </FieldHeader>
      <Input id="runtime" placeholder="Select model" />
    </Field>
  ),
};

/** Opened state, for visual capture — headless runs cannot hover. */
export const Open: Story = {
  args: {},
  tags: ["play-fn"],
  render: () => (
    <Field>
      <FieldHeader>
        <FieldLabel htmlFor="scope">Visibility</FieldLabel>
        <HelpTip label="About visibility" side="bottom">
          A workspace agent is reachable only inside that project.
        </HelpTip>
      </FieldHeader>
      <Input id="scope" placeholder="workspace" />
    </Field>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "About visibility" }));
    await expect(
      await within(document.body).findByText(
        "A workspace agent is reachable only inside that project."
      )
    ).toBeInTheDocument();
  },
};
