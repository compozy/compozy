import type { Meta, StoryObj } from "@storybook/react-vite";

import { FormSection } from "../form-section";
import { ImmutableIdentity } from "../immutable-identity";

const meta: Meta<typeof ImmutableIdentity> = {
  title: "components/custom/ImmutableIdentity",
  component: ImmutableIdentity,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Readable summary of fields an update contract cannot mutate. A disabled input is forbidden as a data display — it reads as a control that happens to be off rather than a value that cannot change.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    rows: [
      { label: "Platform", value: "slack" },
      { label: "Extension", value: "agh-slack" },
      { label: "Scope", value: "workspace" },
    ],
  },
  render: args => (
    <div className="w-(--width-modal-sm) max-w-full p-6">
      <ImmutableIdentity {...args} />
    </div>
  ),
};

export const WithHintInSection: Story = {
  args: {
    hint: "Platform and scope are set at creation — the update contract omits them.",
    rows: [
      { label: "Reference", mono: true, value: "vault:bridges/slack-bot-token" },
      { label: "Kind", value: "token" },
    ],
  },
  render: args => (
    <div className="w-(--width-modal-sm) max-w-full p-6">
      <FormSection title="Identity">
        <ImmutableIdentity {...args} />
      </FormSection>
    </div>
  ),
};
