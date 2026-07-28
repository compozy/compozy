import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";
import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";
import {
  rolesStatusFixture,
  rolesStatusWithDiagnosticFixture,
  settingsRolesConfigFixture,
  settingsRolesConfigWithFallbackFixture,
} from "@/systems/settings/mocks";

import { useRolesDisclosure } from "../../hooks/use-roles-disclosure";
import type { RolesRuntimeOptions } from "../../hooks/use-roles-runtime-options";
import { buildRolesViewModel } from "../../lib/roles-view-model";
import type { RoleStatus, SettingsRolesConfig } from "../../types";
import { RoleList } from "../role-list";

const providerFixtures: RuntimeProviderOption[] = [
  { id: "anthropic", name: "Anthropic", runtime_provider: "anthropic" },
  { id: "openai", name: "OpenAI", runtime_provider: "openai" },
];

const modelFixtures: RuntimeModelOption[] = [
  {
    id: "claude-haiku-4-5",
    provider: "anthropic",
    name: "claude-haiku-4-5",
    efforts: ["low", "medium", "high"],
    default_effort: "medium",
    availability: "live",
    curated: true,
  },
  {
    id: "claude-sonnet-5",
    provider: "anthropic",
    name: "claude-sonnet-5",
    efforts: ["low", "medium", "high", "xhigh"],
    default_effort: "high",
    availability: "live",
    curated: true,
  },
  {
    id: "gpt-5",
    provider: "openai",
    name: "gpt-5",
    efforts: ["minimal", "low", "medium", "high"],
    default_effort: "medium",
    availability: "live",
    curated: true,
  },
];

function runtimeOptions(overrides: Partial<RolesRuntimeOptions> = {}): RolesRuntimeOptions {
  return {
    providers: providerFixtures,
    models: modelFixtures,
    agents: [],
    loading: false,
    catalogLoaded: true,
    catalogError: null,
    catalogStale: false,
    refresh: fn(),
    refreshing: false,
    ...overrides,
  };
}

interface PanelArgs {
  statuses: readonly RoleStatus[];
  config: SettingsRolesConfig;
  validationErrors: Record<string, string>;
  options: RolesRuntimeOptions;
  expanded?: boolean;
}

function RolesPanelHarness({ statuses, config, validationErrors, options, expanded }: PanelArgs) {
  const roles = buildRolesViewModel(statuses, config);
  const disclosure = useRolesDisclosure({ roles, validationErrors });
  const view = expanded ? { ...disclosure, isOpen: () => true, anyOpen: true } : disclosure;
  return (
    <StorySurface>
      <div className="mx-auto flex w-full max-w-settings-page-wide flex-col gap-6">
        <RoleList
          roles={roles}
          options={options}
          disclosure={view}
          validationErrors={validationErrors}
          setRoleEnabled={fn()}
          setRoleAgent={fn()}
          setRoleField={fn()}
          setRoleRuntime={fn()}
          clearRuntime={fn()}
          setNumberFieldValidity={() => fn()}
          addFallback={fn()}
          removeFallback={fn()}
          updateFallback={fn()}
          registerFieldRef={() => fn()}
        />
      </div>
    </StorySurface>
  );
}

const meta: Meta<typeof RolesPanelHarness> = {
  title: "systems/settings/components/RolesPanel",
  component: RolesPanelHarness,
  parameters: { layout: "fullscreen" },
  args: {
    statuses: rolesStatusFixture.roles,
    config: settingsRolesConfigFixture,
    validationErrors: {},
    options: runtimeOptions(),
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Default read: six collapsed rows carrying name, resolution, route and switch. */
export const Collapsed: Story = {};

/** Every role expanded, showing the two routing decisions and role policy fields. */
export const Expanded: Story = { args: { expanded: true } };

/** Dream routed at a missing catalog agent — the row opens itself on the warning. */
export const Diagnostics: Story = {
  args: {
    statuses: rolesStatusWithDiagnosticFixture.roles,
    // The draft names the ghost agent too, so the picker shows it as configured
    // rather than collapsing an out-of-catalog value into the default.
    config: {
      ...settingsRolesConfigFixture,
      dream: { ...settingsRolesConfigFixture.dream, agent: "ghost" },
    },
  },
};

/** A fallback route missing its model, so the role and its Advanced fold both open. */
export const FallbackEditor: Story = {
  args: {
    config: {
      ...settingsRolesConfigWithFallbackFixture,
      dream: {
        ...settingsRolesConfigWithFallbackFixture.dream,
        fallback_chain: [
          settingsRolesConfigWithFallbackFixture.dream.fallback_chain[0]!,
          { provider: "", model: "", reasoning_effort: "" },
        ],
      },
    },
    validationErrors: { "dream.fallback.1": "Choose a provider and model." },
  },
};

/** No providers configured yet — the runtime row says so instead of offering a dead control. */
export const NoProviders: Story = {
  args: {
    expanded: true,
    options: runtimeOptions({ providers: [], models: [] }),
  },
};
