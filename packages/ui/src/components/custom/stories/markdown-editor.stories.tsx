import type { Meta, StoryObj } from "@storybook/react-vite";
import * as React from "react";
import { expect, userEvent, within } from "storybook/test";

import { Field, FieldError, FieldLabel } from "../../field";
import { MarkdownEditor } from "../markdown-editor";

const meta: Meta<typeof MarkdownEditor> = {
  title: "components/custom/MarkdownEditor",
  component: MarkdownEditor,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Tiptap rich-text field that reads and writes Markdown: bold, italic, strike, inline code, lists, and links. Cmd/Ctrl+Enter calls `onSubmit`.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function EditorField({
  initial = "",
  error,
  disabled,
}: {
  initial?: string;
  error?: string;
  disabled?: boolean;
}) {
  const [value, setValue] = React.useState(initial);
  return (
    <Field className="w-96" data-invalid={Boolean(error)}>
      <FieldLabel id="story-note-label">Reason</FieldLabel>
      <MarkdownEditor
        aria-invalid={Boolean(error)}
        aria-labelledby="story-note-label"
        disabled={disabled}
        onValueChange={setValue}
        placeholder="Optional"
        value={value}
      />
      {error ? <FieldError>{error}</FieldError> : null}
      <pre className="mt-2 font-mono text-eyebrow whitespace-pre-wrap text-subtle">{value}</pre>
    </Field>
  );
}

export const Empty: Story = {
  render: () => <EditorField />,
};

export const Formatted: Story = {
  render: () => (
    <EditorField
      initial={
        "Waiting on **billing review** before the rollout.\n\n- re-run `make gate`\n- check the [spec](https://compozy.com)"
      }
    />
  ),
};

export const Invalid: Story = {
  render: () => <EditorField error="Use 1000 characters or fewer." initial="A long note" />,
};

export const Disabled: Story = {
  render: () => <EditorField disabled initial="Parked while the release ships." />,
};

export const BoldSelection: Story = {
  tags: ["play-fn"],
  render: () => <EditorField initial="ship it" />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("textbox", { name: "Reason" }));
    await userEvent.keyboard("{Control>}a{/Control}");
    await userEvent.keyboard("{Meta>}a{/Meta}");
    await userEvent.click(canvas.getByRole("button", { name: "Bold" }));
    await expect(canvasElement.querySelector("pre")).toHaveTextContent("**ship it**");
  },
};
