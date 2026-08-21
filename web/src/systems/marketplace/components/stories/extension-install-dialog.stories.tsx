import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import type { ExtensionInstallPreview } from "@/systems/extensions";

import { ExtensionInstallDialog } from "../extension-install-dialog";

const PREVIEW = {
  name: "growth-kit",
  declared_profiles: [
    {
      create: true,
      name: "growth",
      credentials: [
        {
          missing: true,
          provider: "openai",
          slot: "api_key",
          source_extension: "growth-kit",
        },
      ],
    },
    { create: false, name: "operations", credentials: [] },
  ],
  placements: [
    { dormant: false, kind: "skill", profile: "growth", resource: "campaign-brief" },
    { dormant: false, kind: "agent", resource: "release-reviewer" },
  ],
} satisfies ExtensionInstallPreview;

const meta: Meta<typeof ExtensionInstallDialog> = {
  title: "systems/marketplace/components/ExtensionInstallDialog",
  component: ExtensionInstallDialog,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** The confirmation names every declared profile and the credentials still required. */
export const DeclaredProfilesReview: Story = {
  args: {
    open: true,
    pending: false,
    preview: PREVIEW,
    onFormChange: fn(),
    onOpenChange: fn(),
    onSubmit: fn(),
  },
};
