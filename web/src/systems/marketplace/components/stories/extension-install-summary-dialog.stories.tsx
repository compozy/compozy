import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { storyCatalog } from "../../routes/marketplace-story-data";

import { ExtensionInstallSummaryDialog } from "../extension-install-summary-dialog";

const meta = {
  title: "systems/marketplace/components/ExtensionInstallSummaryDialog",
  component: ExtensionInstallSummaryDialog,
  parameters: { layout: "fullscreen" },
  args: {
    action: "install",
    open: true,
    pending: false,
    onConfirm: fn(),
    onOpenChange: fn(),
  },
} satisfies Meta<typeof ExtensionInstallSummaryDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

export const OptionalSecret: Story = {
  args: {
    action: "install",
    entry: storyCatalog.context7,
    destination: { scope: "global", profile: "default" },
    preview: {
      name: "context7",
      declared_profiles: [],
      placements: [],
      inputs: [
        {
          id: "api_key",
          prompt: "Context7 API key",
          type: "secret",
          required: false,
          binding: { type: "env", name: "CONTEXT7_API_KEY" },
        },
      ],
    },
  },
};

export const RequiredIdentifier: Story = {
  args: {
    action: "install",
    entry: {
      ...storyCatalog.context7,
      name: "Supabase",
      entry_id: "supabase",
      description: "Connect your Supabase project to inspect its database and services.",
    },
    destination: { scope: "workspace", profile: "default" },
    preview: {
      name: "supabase",
      declared_profiles: [],
      placements: [],
      inputs: [
        {
          id: "project_ref",
          prompt: "Project reference",
          type: "identifier",
          required: true,
          binding: { type: "url_query", name: "project_ref" },
        },
      ],
    },
  },
  play: async () => {
    const body = within(document.body);
    await userEvent.type(await body.findByLabelText("Project reference"), "acme-production");
    await userEvent.tab();
    await expect(body.getByRole("button", { name: "Install" })).toBeEnabled();
  },
};

export const RequiredMissing: Story = {
  args: RequiredIdentifier.args,
  play: async () => {
    const body = within(document.body);
    await userEvent.click(await body.findByLabelText("Project reference"));
    await userEvent.tab();
    await expect(body.getByTestId("extension-input-project_ref-error")).toBeVisible();
    await expect(body.getByRole("button", { name: "Install" })).toBeDisabled();
  },
};

export const TooLong: Story = {
  args: RequiredIdentifier.args,
  play: async () => {
    const body = within(document.body);
    await userEvent.click(await body.findByLabelText("Project reference"));
    await userEvent.paste("x".repeat(8193));
    await userEvent.tab();
    await expect(body.getByText("Too long (max 8 KB)")).toBeVisible();
    await expect(body.getByRole("button", { name: "Install" })).toBeDisabled();
  },
};
