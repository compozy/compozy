import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent, within } from "storybook/test";

import { FieldError } from "../../field";
import { DirectoryPickerInput, type DirectoryPickerSuggestion } from "../directory-picker-input";

const TREE: Record<string, string[]> = {
  "~/": ["code", "Desktop", "Documents", "notes"],
  "~/code/": ["checkout-platform", "compozy", "website"],
};

function suggestionsFor(value: string): DirectoryPickerSuggestion[] {
  const cut = value.lastIndexOf("/") + 1;
  const dir = value.slice(0, cut);
  const prefix = value.slice(cut).toLowerCase();
  return (TREE[dir] ?? [])
    .filter(name => name.toLowerCase().startsWith(prefix))
    .map(name => ({ value: `${dir}${name}/`, label: name }));
}

const meta: Meta<typeof DirectoryPickerInput> = {
  title: "components/custom/DirectoryPickerInput",
  component: DirectoryPickerInput,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Mono path field with keyboard folder completion and an optional Browse… button for a native chooser. The caller supplies suggestions and validation.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[420px]">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

function Harness({
  initial,
  browse,
  error,
}: {
  initial: string;
  browse?: boolean;
  error?: string;
}) {
  const [value, setValue] = useState(initial);
  return (
    <div className="flex flex-col gap-1.5">
      <DirectoryPickerInput
        aria-label="Project folder"
        invalid={Boolean(error)}
        onBrowse={browse ? () => setValue("~/code/compozy") : undefined}
        onValueChange={setValue}
        placeholder="~/code/my-project"
        suggestions={suggestionsFor(value)}
        value={value}
      />
      {error ? <FieldError>{error}</FieldError> : null}
    </div>
  );
}

const noop = () => {};

/** Web without a desktop shell: typing completes folders; Tab drills into the highlighted one. */
export const Autocomplete: Story = {
  tags: ["play-fn"],
  args: { value: "", onValueChange: noop, suggestions: [] },
  render: () => <Harness initial="~/" />,
  play: async ({ canvasElement }) => {
    const input = within(canvasElement).getByRole("combobox", { name: "Project folder" });
    await userEvent.click(input);
    await userEvent.type(input, "co");
    await userEvent.keyboard("{Tab}");
    await expect(input).toHaveValue("~/code/");
  },
};

/** Desktop shell: Browse… opens the OS folder dialog. */
export const WithBrowse: Story = {
  args: { value: "", onValueChange: noop, suggestions: [] },
  render: () => <Harness browse initial="" />,
};

/** The caller's validation renders below the field. */
export const Invalid: Story = {
  args: { value: "", onValueChange: noop, suggestions: [] },
  render: () => <Harness browse error="No folder at this path." initial="~/code/missing" />,
};
