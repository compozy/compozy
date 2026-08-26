import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";
import { settingsSkillSourcesFixture } from "@/systems/settings/mocks";
import type { SettingsSkillSource } from "@/systems/settings";

import type { SettingsSkillSourcesModel } from "../../hooks/use-settings-skill-sources";
import { groupSkillSources } from "../../lib/skill-sources-view";
import { SettingsSkillSourcesSection } from "../settings-skill-sources-section";

const noop = () => undefined;

function model(overrides: Partial<SettingsSkillSourcesModel> = {}): SettingsSkillSourcesModel {
  return {
    groups: groupSkillSources(settingsSkillSourcesFixture),
    enabledPresets: ["agents"],
    customEntries: ["~/team-skills"],
    readOnly: false,
    readOnlyReason: null,
    postures: null,
    isDirty: false,
    isSaving: false,
    saveError: null,
    saveErrorCode: null,
    lastLabel: "Saved · applied immediately",
    togglePreset: noop,
    addCustom: noop,
    removeCustom: noop,
    validateEntry: () => null,
    customize: noop,
    useInherited: noop,
    inheritPendingKey: null,
    inheritError: null,
    save: noop,
    reset: noop,
    ...overrides,
  };
}

function mutated(mutate: (sources: SettingsSkillSource[]) => void): SettingsSkillSourcesModel {
  const sources = structuredClone(settingsSkillSourcesFixture) as SettingsSkillSource[];
  mutate(sources);
  return model({ groups: groupSkillSources(sources) });
}

const meta = {
  title: "systems/settings/components/SettingsSkillSourcesSection",
  component: SettingsSkillSourcesSection,
  parameters: { layout: "padded" },
  render: args => (
    <PanelSurface className="w-full max-w-3xl p-6">
      <SettingsSkillSourcesSection {...args} />
    </PanelSurface>
  ),
} satisfies Meta<typeof SettingsSkillSourcesSection>;

export default meta;
type Story = StoryObj<typeof meta>;

/** CompozyOS always on, the universal folders on, Claude available but off. */
export const Default: Story = { args: { model: model() } };

/**
 * A folder that isn't there yet, one that can't be read, and one too big to
 * finish — three states that must never render as a measured zero.
 */
export const DegradedRoots: Story = {
  args: {
    model: mutated(sources => {
      sources[3].roots[0].readable = false;
      delete sources[3].roots[0].scanned_count;
      delete sources[3].roots[0].skill_count;
      sources[1].roots[1].truncated = true;
      sources[1].roots[1].scanned_count = 300;
      sources[1].roots[1].skill_count = 214;
    }),
  },
};

/** Nothing optional is on and no folder was added by hand. */
export const DefaultsOnly: Story = {
  args: {
    model: model({
      enabledPresets: [],
      customEntries: [],
      groups: groupSkillSources(
        structuredClone(settingsSkillSourcesFixture)
          .filter(source => source.kind !== "custom")
          .map(source =>
            source.always_on ? source : { ...source, enabled: false }
          ) as SettingsSkillSource[]
      ),
    }),
  },
};

/** One key customized for this workspace, the other still following the layer above. */
export const WorkspaceScope: Story = {
  args: {
    model: model({
      postures: [
        { key: "sources", inherited: false, armed: true },
        { key: "custom_sources", inherited: true, armed: false },
      ],
    }),
  },
};

/** The daemon refused the save; its sentence and code render verbatim. */
export const SaveRejected: Story = {
  args: {
    model: model({
      isDirty: true,
      saveError: 'unknown skill source preset "cluade" (did you mean "claude"?)',
      saveErrorCode: "unknown_skill_source",
    }),
  },
};

/** Agent scope reads source policy and never writes it. */
export const AgentScope: Story = {
  args: { model: model({ readOnly: true, readOnlyReason: "agent" }) },
};

/** The exact workspace + profile projection is visible but repository-owned. */
export const RepositoryProfileScope: Story = {
  args: { model: model({ readOnly: true, readOnlyReason: "repository-profile" }) },
};
