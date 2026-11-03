import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { primaryAgentFixture } from "../../mocks/fixtures";
import {
  blankCheckInsModelFixture,
  blankMemoryModelFixture,
  blankSoulDocumentFixture,
  checkInsModelFixture,
  memoryModelFixture,
  soulDocumentFixture,
} from "../../mocks/editor-fixtures";
import type { AgentPayload } from "../../types";
import { AgentCheckInsView } from "../agent-editors/check-ins-view";
import { AgentMemoryView } from "../agent-editors/memory-view";
import {
  AgentPermissionsView,
  type AgentPermissionsViewProps,
} from "../agent-editors/permissions-view";
import { AgentWhatItDoesView } from "../agent-editors/what-it-does-view";

const agent: AgentPayload = {
  ...primaryAgentFixture,
  label: "Triage Lead",
  description: "Watches issues and proposes the next step",
};

const meta: Meta<typeof AgentWhatItDoesView> = {
  title: "systems/agent/components/AgentEditors",
  component: AgentWhatItDoesView,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "The pure views behind Instructions, Heartbeat and Memory. The containers read the files; the page chrome belongs to the route.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="mx-auto w-full max-w-5xl">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const WhatItDoesFilled: Story = {
  render: () => (
    <AgentWhatItDoesView
      agent={agent}
      document={soulDocumentFixture()}
      readOnly={false}
      state="idle"
    />
  ),
};

export const WhatItDoesEmpty: Story = {
  render: () => (
    <AgentWhatItDoesView
      agent={agent}
      document={blankSoulDocumentFixture()}
      readOnly={false}
      state="idle"
    />
  ),
};

export const WhatItDoesExtensionOwned: Story = {
  render: () => (
    <AgentWhatItDoesView
      agent={{ ...agent, package_owned: true }}
      document={soulDocumentFixture()}
      readOnly
      state="idle"
    />
  ),
};

export const CheckInsFilled: Story = {
  render: () => <AgentCheckInsView agent={agent} model={checkInsModelFixture()} state="idle" />,
};

export const CheckInsEmpty: Story = {
  render: () => (
    <AgentCheckInsView agent={agent} model={blankCheckInsModelFixture()} state="idle" />
  ),
};

export const CheckInsNowhere: Story = {
  render: () => (
    <AgentCheckInsView
      agent={agent}
      model={checkInsModelFixture({
        instance: null,
        places: [],
        standing: "off",
        nowhere: true,
        on: false,
      })}
      onAddToProject={fn()}
      state="idle"
    />
  ),
};

export const MemoryFilled: Story = {
  render: () => <AgentMemoryView agent={agent} model={memoryModelFixture()} state="idle" />,
};

export const MemoryEmpty: Story = {
  render: () => <AgentMemoryView agent={agent} model={blankMemoryModelFixture()} state="idle" />,
};

const alwaysOnAgent: AgentPayload = { ...agent, kind: "always_on", provider: "claude" };

function permissionsProps(
  overrides: Partial<AgentPermissionsViewProps> = {}
): AgentPermissionsViewProps {
  return {
    agent: alwaysOnAgent,
    providerId: "claude",
    providerName: "Claude Code",
    level: "",
    sessionApproval: "",
    rules: [],
    invalidRule: null,
    reviewer: "available",
    degraded: false,
    readOnly: false,
    changes: 0,
    saving: false,
    onLevelChange: fn(),
    onSessionApprovalChange: fn(),
    onRulesChange: fn(),
    onSave: fn(),
    onDiscard: fn(),
    ...overrides,
  };
}

const permissionRules: AgentPermissionsViewProps["rules"] = [
  { id: "docs", kind: "edit", path: "docs/**", decision: "allow" },
  { id: "infra", kind: "edit", path: "infra/**", decision: "deny" },
  { id: "migrations", kind: "delete", path: "db/migrations/**", decision: "deny" },
];

export const PermissionsNothingSet: Story = {
  render: () => <AgentPermissionsView {...permissionsProps()} />,
};

export const PermissionsReviewerUnavailable: Story = {
  render: () => (
    <AgentPermissionsView
      {...permissionsProps({ level: "auto", providerName: "Cursor", reviewer: "unavailable" })}
    />
  ),
};

export const PermissionsReviewerDegraded: Story = {
  render: () => <AgentPermissionsView {...permissionsProps({ level: "auto", degraded: true })} />,
};

export const PermissionsNeverAskCodex: Story = {
  render: () => (
    <AgentPermissionsView
      {...permissionsProps({
        agent: { ...alwaysOnAgent, provider: "codex" },
        level: "approve-all",
        providerId: "codex",
        providerName: "Codex",
      })}
    />
  ),
};

export const PermissionsExceptions: Story = {
  render: () => (
    <AgentPermissionsView
      {...permissionsProps({ level: "approve-edits", rules: permissionRules })}
    />
  ),
};

export const PermissionsExceptionInvalid: Story = {
  render: () => (
    <AgentPermissionsView
      {...permissionsProps({
        blocked: "Fix the exception first",
        changes: 1,
        invalidRule: {
          id: "outside",
          message: "Use a path inside the project, such as docs/** or src/app.",
        },
        rules: [
          ...permissionRules,
          { id: "outside", kind: "edit", path: "../secrets", decision: "allow" },
        ],
        sessionApproval: "workspace",
      })}
    />
  ),
};

export const PermissionsOnDemand: Story = {
  render: () => (
    <AgentPermissionsView
      {...permissionsProps({
        agent: { ...agent, kind: "on_demand" },
        settingsLevel: "approve-all",
      })}
    />
  ),
};

export const PermissionsExtensionOwned: Story = {
  render: () => (
    <AgentPermissionsView
      {...permissionsProps({ owner: "Release Kit", readOnly: true, rules: permissionRules })}
    />
  ),
};
