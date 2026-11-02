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
          "The pure views behind What it does, Check-ins and Memory. The containers read the files; the page chrome belongs to the route.",
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
