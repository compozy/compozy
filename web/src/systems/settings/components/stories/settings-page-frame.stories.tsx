import type { Meta, StoryObj } from "@storybook/react-vite";

import { Input } from "@compozy/ui";

import { PanelSurface } from "@/storybook/story-layout";

import { SettingsFieldRow } from "../settings-field-row";
import { SettingsGroup } from "../settings-group";
import { SettingsPageFrame } from "../settings-page-frame";
import { SettingsSaveBar } from "../settings-save-bar";

const meta = {
  title: "systems/settings/components/SettingsPageFrame",
  component: SettingsPageFrame,
  parameters: { layout: "fullscreen" },
  decorators: [
    Story => (
      <PanelSurface className="min-h-[720px]">
        <Story />
      </PanelSurface>
    ),
  ],
} satisfies Meta<typeof SettingsPageFrame>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Form: Story = {
  args: {
    description: "Changes here apply to new sessions on this machine.",
    meta: [{ key: "sessions", content: "3 active sessions" }],
    slug: "general",
    children: (
      <SettingsGroup title="Defaults">
        <SettingsFieldRow
          control={<Input defaultValue="codex" />}
          help="Agent used when a session does not choose another."
          label="Default agent"
        />
      </SettingsGroup>
    ),
    saveBar: (
      <SettingsSaveBar
        onReset={() => undefined}
        onSave={() => undefined}
        slug="general"
        state={{ kind: "dirty", warnings: [] }}
      />
    ),
  },
};

export const Listing: Story = {
  args: {
    children: (
      <SettingsGroup title="Providers">
        <p className="px-4 py-3 text-small-body text-muted">Codex · ready</p>
        <p className="border-t border-line-soft px-4 py-3 text-small-body text-muted">
          Claude Code · needs sign-in
        </p>
      </SettingsGroup>
    ),
    meta: [{ key: "ready", content: "1 ready" }],
    slug: "providers",
    width: "wide",
  },
};
