import type { Meta, StoryObj } from "@storybook/react-vite";

import { Field, FieldDescription, FieldError, FieldLabel } from "../field";
import { NativeSelect, NativeSelectOptGroup, NativeSelectOption } from "../native-select";

const meta: Meta<typeof NativeSelect> = {
  title: "components/ui/NativeSelect",
  component: NativeSelect,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Thin wrapper around the native `<select>` element for constrained environments where the Base UI Select is overkill.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const WithLabelAndHelper: Story = {
  args: {},
  render: () => (
    <div className="w-[18rem]">
      <Field>
        <FieldLabel htmlFor="native-select-env">Environment</FieldLabel>
        <NativeSelect id="native-select-env" defaultValue="dev" className="w-full">
          <NativeSelectOption value="dev">Development</NativeSelectOption>
          <NativeSelectOption value="staging">Staging</NativeSelectOption>
          <NativeSelectOption value="prod">Production</NativeSelectOption>
        </NativeSelect>
        <FieldDescription>Picks which daemon config AGH should boot with.</FieldDescription>
      </Field>
    </div>
  ),
};

export const ErrorState: Story = {
  args: {},
  render: () => (
    <div className="w-[18rem]">
      <Field data-invalid>
        <FieldLabel htmlFor="native-select-error">Environment</FieldLabel>
        <NativeSelect
          id="native-select-error"
          aria-invalid
          aria-describedby="native-select-error-message"
          defaultValue=""
          className="w-full"
        >
          <NativeSelectOption value="" disabled>
            Select an environment
          </NativeSelectOption>
          <NativeSelectOption value="dev">Development</NativeSelectOption>
          <NativeSelectOption value="staging">Staging</NativeSelectOption>
          <NativeSelectOption value="prod">Production</NativeSelectOption>
        </NativeSelect>
        <FieldError id="native-select-error-message">Environment is required.</FieldError>
      </Field>
    </div>
  ),
};

export const Grouped: Story = {
  args: {},
  render: () => (
    <div className="w-[18rem]">
      <NativeSelect defaultValue="claude" className="w-full">
        <NativeSelectOptGroup label="Local">
          <NativeSelectOption value="claude">Claude Code</NativeSelectOption>
          <NativeSelectOption value="codex">Codex CLI</NativeSelectOption>
        </NativeSelectOptGroup>
        <NativeSelectOptGroup label="Remote">
          <NativeSelectOption value="gemini">Gemini CLI</NativeSelectOption>
        </NativeSelectOptGroup>
      </NativeSelect>
    </div>
  ),
};
