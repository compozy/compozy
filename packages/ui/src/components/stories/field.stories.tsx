import type { Meta, StoryObj } from "@storybook/react-vite";

import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from "../field";
import { Input } from "../input";

const meta: Meta<typeof Field> = {
  title: "components/ui/Field",
  component: Field,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Form field wrapper that composes Label, Input, helper text, and error messaging. Pair with @agh/ui primitives for the control.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  render: () => (
    <div className="w-88">
      <Field>
        <FieldLabel htmlFor="field-default">Workspace name</FieldLabel>
        <Input id="field-default" defaultValue="Latency triage" />
      </Field>
    </div>
  ),
};

export const WithHelperText: Story = {
  args: {},
  render: () => (
    <div className="w-88">
      <Field>
        <FieldLabel htmlFor="field-helper">Agent slug</FieldLabel>
        <Input id="field-helper" defaultValue="claude-orchestrator" />
        <FieldDescription>Lowercase identifier used in CLI commands and logs.</FieldDescription>
      </Field>
    </div>
  ),
};

export const ErrorState: Story = {
  args: {},
  render: () => (
    <div className="w-88">
      <Field data-invalid>
        <FieldLabel htmlFor="field-error">API token</FieldLabel>
        <Input
          id="field-error"
          aria-invalid
          aria-describedby="field-error-message"
          defaultValue=""
        />
        <FieldError id="field-error-message">API token is required.</FieldError>
      </Field>
    </div>
  ),
};

export const GroupedFieldset: Story = {
  args: {},
  render: () => (
    <FieldSet className="w-88">
      <FieldLegend>Session metadata</FieldLegend>
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor="field-session-name">Name</FieldLabel>
          <Input id="field-session-name" defaultValue="Incident 2026-04-17" />
        </Field>
        <Field>
          <FieldLabel htmlFor="field-session-tag">Tag</FieldLabel>
          <Input id="field-session-tag" defaultValue="oncall" />
          <FieldDescription>Used to group runs in the sessions index.</FieldDescription>
        </Field>
      </FieldGroup>
    </FieldSet>
  ),
};
