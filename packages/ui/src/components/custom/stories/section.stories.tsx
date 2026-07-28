import type { Meta, StoryObj } from "@storybook/react-vite";

import { Button } from "../../button";
import { Pill } from "../pill";
import { Section } from "../section";

const meta: Meta<typeof Section> = {
  title: "components/custom/Section",
  component: Section,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Section shell with 13 px H2 label + optional right-aligned slot + body. Bottom border is opt-in via `bordered` (default `false`)",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Basic: Story = {
  args: {},
  render: () => (
    <div className="w-[520px]">
      <Section label="Routes">
        <ul className="divide-y divide-line text-sm text-muted">
          <li className="py-2">/runtime/sessions</li>
          <li className="py-2">/runtime/memory</li>
          <li className="py-2">/runtime/skills</li>
        </ul>
      </Section>
    </div>
  ),
};

export const WithRightSlot: Story = {
  args: {},
  render: () => (
    <div className="w-[520px]">
      <Section
        label="Recent runs"
        right={
          <>
            <Pill tone="success">Live</Pill>
            <Button size="xs" variant="outline" type="button">
              View all
            </Button>
          </>
        }
      >
        <p className="text-sm text-muted">
          Dense operational rows go here: replay, inspect, or open detail.
        </p>
      </Section>
    </div>
  ),
};

export const WithNote: Story = {
  args: {},
  render: () => (
    <div className="w-[520px]">
      <Section label="Runtime" note="Read-only daemon state" divided>
        <p className="text-sm text-muted">
          Use note for compact section context without creating a local wrapper.
        </p>
      </Section>
    </div>
  ),
};

export const BodyOnly: Story = {
  args: {},
  render: () => (
    <div className="w-[520px]">
      <Section>
        <p className="text-sm text-muted">
          Section with no eyebrow; the surrounding layout supplies the heading.
        </p>
      </Section>
    </div>
  ),
};

export const Bordered: Story = {
  args: {},
  render: () => (
    <div className="w-[520px]">
      <Section label="Members" bordered>
        <p className="text-sm text-muted">
          Opt-in `bordered` paints a single `--line` hairline under the head when separation is
          required. Default is borderless.
        </p>
      </Section>
    </div>
  ),
};
