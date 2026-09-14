import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import type { ExtensionInstallPreview } from "@/systems/extensions";

import { ExtensionInstallDialog } from "../extension-install-dialog";

const PREVIEW = {
  name: "growth-kit",
  inputs: [],
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

export const PackagedInputs: Story = {
  args: {
    ...DeclaredProfilesReview.args,
    preview: {
      ...PREVIEW,
      name: "durable-input-kit",
      digest_sha256: "a".repeat(64),
      inputs: [
        {
          id: "token",
          prompt: "API key",
          type: "secret",
          required: true,
          binding: { type: "env", name: "TOKEN" },
        },
        {
          id: "workspace_id",
          prompt: "Workspace",
          type: "identifier",
          required: true,
          binding: { type: "url_query", name: "workspace" },
        },
        {
          id: "read_only",
          prompt: "Read only",
          type: "boolean",
          required: false,
          default: false,
          binding: { type: "url_query", name: "read_only" },
        },
      ],
    },
  },
};

export const OptionalInput: Story = {
  args: {
    ...DeclaredProfilesReview.args,
    preview: {
      ...PREVIEW,
      name: "context7",
      inputs: [
        {
          id: "context7_api_key",
          prompt: "Context7 API key",
          type: "secret",
          required: false,
          binding: { type: "env", name: "CONTEXT7_API_KEY" },
        },
      ],
    },
  },
};
