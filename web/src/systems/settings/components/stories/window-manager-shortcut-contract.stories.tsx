import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { userEvent, within } from "storybook/test";

import type { ShortcutMap, WindowManagerSettingsSection } from "@/systems/os";
import { CenteredSurface } from "@/storybook/story-layout";
import { cmdPaletteStoryShortcuts } from "@/systems/os/mocks/cmd-palette-fixtures";

import type { AliasEditorModel } from "../../hooks/use-window-manager-alias-editor";
import type { ShortcutRecorderModel } from "../../hooks/use-window-manager-shortcut-recorder";
import { settingsWindowManagerSectionFixture } from "../../mocks/window-manager-fixtures";
import { ShortcutPresetCard } from "../layouts/shortcut-preset-card";
import { WindowManagerShortcutTable } from "../layouts/window-manager-shortcut-table";
import {
  parseSettingsWindowManagerSection,
  type WindowManagerSettingsWire,
} from "@/systems/os/lib/window-manager-settings-section";

/**
 * The table renders daemon truth, so a story state is produced by handing it the
 * section a daemon would have served — never by reaching inside the component.
 */
function sectionFrom(
  mutate: (wire: WindowManagerSettingsWire) => void
): WindowManagerSettingsSection {
  const wire = structuredClone(settingsWindowManagerSectionFixture) as WindowManagerSettingsWire;
  mutate(wire);
  return parseSettingsWindowManagerSection(wire);
}

const BASE_SECTION = sectionFrom(() => {});

/** The loser of an overwrite: still listed, holding nothing (US-022.AC-2). */
const LOSER_SECTION = sectionFrom(wire => {
  wire.config.shortcuts = { ...wire.config.shortcuts, "session.new": [] };
  wire.effective_shortcuts = {
    ...wire.effective_shortcuts,
    "session.new": [],
    "ext.notes.capture": ["meta+KeyN"],
  };
  wire.aliases = { ...wire.aliases, "ext.notes.capture": "cap" };
});

/** An extension default the daemon withheld (US-029.AC-2). */
const DORMANT_SECTION = sectionFrom(wire => {
  wire.extension_defaults = [
    {
      command: "ext.notes.capture",
      binding: ["meta+KeyN"],
      dormant: true,
      conflict_with: "session.new",
    },
  ];
  wire.effective_shortcuts = { ...wire.effective_shortcuts, "ext.notes.capture": [] };
});

function recorderStub(overrides: Partial<ShortcutRecorderModel> = {}): ShortcutRecorderModel {
  return {
    recording: null,
    recordingMode: null,
    announcement: "",
    conflict: null,
    conflicts: [],
    error: null,
    saving: false,
    start: () => {},
    cancel: () => {},
    overwrite: () => {},
    dismissConflict: () => {},
    reset: () => {},
    resetAll: () => {},
    applyShortcuts: () => {},
    ...overrides,
  };
}

function aliasStub(overrides: Partial<AliasEditorModel> = {}): AliasEditorModel {
  return {
    cell: () => ({ value: "", problem: null, saving: false }),
    conflict: null,
    change: () => {},
    commit: () => {},
    cancel: () => {},
    overwrite: () => {},
    dismissConflict: () => {},
    ...overrides,
  };
}

function TableFixture({
  section = BASE_SECTION,
  recorder = recorderStub(),
  aliases = aliasStub(),
}: {
  section?: WindowManagerSettingsSection;
  recorder?: ShortcutRecorderModel;
  aliases?: AliasEditorModel;
}) {
  return (
    <div className="max-h-190 w-240 overflow-y-auto rounded-lg border border-line bg-canvas-soft">
      <WindowManagerShortcutTable aliases={aliases} recorder={recorder} section={section} />
    </div>
  );
}

function PresetFixture({ initial }: { initial: ShortcutMap }) {
  const [overrides, setOverrides] = useState(initial);
  return (
    <div className="w-240 overflow-hidden rounded-lg border border-line bg-canvas-soft">
      <ShortcutPresetCard
        defaults={cmdPaletteStoryShortcuts}
        overrides={overrides}
        onChange={setOverrides}
      />
    </div>
  );
}

const meta: Meta<typeof TableFixture> = {
  title: "systems/settings/components/WindowManagerShortcutContract",
  component: TableFixture,
  parameters: { layout: "fullscreen" },
  decorators: [
    Story => (
      <CenteredSurface>
        <Story />
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-01 — the whole registry with its source filter. */
export const WholeRegistry: Story = {};

/** VC-01 — narrowed to one extension's contributions. */
export const SourceFiltered: Story = {
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByTestId("shortcut-source-ext.notes"));
  },
};

/** VC-02 — an alias with whitespace, rejected in place. */
export const InvalidAlias: Story = {
  args: {
    aliases: aliasStub({
      cell: commandId =>
        commandId === "ext.notes.capture"
          ? { value: "ca p", problem: "grammar", saving: false }
          : { value: "", problem: null, saving: false },
    }),
  },
};

/** VC-03 — a contested chord, named and transferable. */
export const ChordConflict: Story = {
  args: {
    recorder: recorderStub({
      conflict: {
        commandId: "ext.notes.capture",
        chord: "meta+KeyN",
        owner: "session.new",
        ownerTitle: "New session",
        desired: {},
      },
    }),
  },
};

/** VC-04 — after the transfer: the loser keeps its row, holding nothing. */
export const OverwrittenLoser: Story = {
  args: { section: LOSER_SECTION },
};

/** US-029.AC-2 — an extension default the daemon refused to apply. */
export const DormantExtensionDefault: Story = {
  args: { section: DORMANT_SECTION },
};

export const Recording: Story = {
  args: { recorder: recorderStub({ recording: "session.new", recordingMode: "replace" }) },
};

export const PresetPreview: Story = {
  render: () => <PresetFixture initial={{}} />,
  play: async ({ canvasElement }) => {
    await userEvent.click(within(canvasElement).getByRole("button", { name: "Preview" }));
  },
};

export const PresetApplied: Story = {
  render: () => <PresetFixture initial={{}} />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Preview" }));
    await userEvent.click(canvas.getByRole("button", { name: "Apply preset" }));
  },
};
