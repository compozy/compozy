import type { Meta, StoryObj } from "@storybook/react-vite";

import { Field, FieldDescription, FieldError, FieldLabel } from "../field";
import { Textarea } from "../textarea";

const meta: Meta<typeof Textarea> = {
  title: "components/ui/Textarea",
  component: Textarea,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Multi-line input used for long-form prompts, notes, and agent instructions. Grows with content via `field-sizing-content`.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const WithLabelAndHelper: Story = {
  args: {},
  render: () => (
    <div className="w-104">
      <Field>
        <FieldLabel htmlFor="textarea-notes">Session notes</FieldLabel>
        <Textarea
          id="textarea-notes"
          rows={4}
          defaultValue="Agent should prioritize latency telemetry before touching code."
        />
        <FieldDescription>Shared with every agent turn in this session.</FieldDescription>
      </Field>
    </div>
  ),
};

export const ErrorState: Story = {
  args: {},
  render: () => (
    <div className="w-104">
      <Field data-invalid>
        <FieldLabel htmlFor="textarea-error">Session notes</FieldLabel>
        <Textarea
          id="textarea-error"
          rows={4}
          aria-invalid
          aria-describedby="textarea-error-message"
          defaultValue=""
        />
        <FieldError id="textarea-error-message">Notes cannot be empty.</FieldError>
      </Field>
    </div>
  ),
};

export const Disabled: Story = {
  args: {},
  render: () => (
    <div className="w-104">
      <Field>
        <FieldLabel htmlFor="textarea-disabled">Archived notes</FieldLabel>
        <Textarea
          id="textarea-disabled"
          rows={3}
          disabled
          defaultValue="Session closed on 2026-04-12."
        />
      </Field>
    </div>
  ),
};

/** Mono variant — `font-mono` + 12 px, for prompt/code-style inputs. */
export const MonoVariant: Story = {
  args: {},
  render: () => (
    <div className="w-104">
      <Field>
        <FieldLabel htmlFor="textarea-mono">System prompt</FieldLabel>
        <Textarea
          id="textarea-mono"
          rows={5}
          variant="mono"
          defaultValue="You are an AGH operator. Stay terse. Honor SD-007."
        />
        <FieldDescription>Sent verbatim as the agent's first user message.</FieldDescription>
      </Field>
    </div>
  ),
};
