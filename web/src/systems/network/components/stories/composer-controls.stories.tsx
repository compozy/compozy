import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { storyDefaultWorkspaceId } from "@/storybook/fintech-scenario";
import { PanelSurface } from "@/storybook/story-layout";

import { ChannelThreadComposer } from "../composer/channel-thread-composer";
import { Composer } from "../composer/composer";
import { ComposerSlashPopover } from "../composer/composer-slash-popover";
import { ComposerToolbar } from "../composer/composer-toolbar";
import { DetailComposer } from "../composer/detail-composer";

const meta: Meta<typeof Composer> = {
  title: "systems/network/components/ComposerControls",
  component: Composer,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Network composer primitives: textarea composer, toolbar, and slash popover.",
      },
    },
  },
  decorators: [
    Story => (
      <PanelSurface className="min-h-[260px] p-0">
        <Story />
      </PanelSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Channel composer with toolbar and send action.
 */
export const Default: Story = {
  args: {
    placeholder: "Start a new thread...",
    testIdSuffix: "story",
    sendLabel: "Send to #launch-war-room",
    onSubmit: fn(),
  },
};

/**
 * Disabled composer explains why no active network session can author messages.
 */
export const Disabled: Story = {
  args: {
    placeholder: "Start a new thread...",
    testIdSuffix: "story",
    sendLabel: "Send to #launch-war-room",
    disabled: true,
    disabledReason: "Start a network-capable session to write here.",
    onSubmit: fn(),
  },
};

/**
 * Toolbar actions are icon-only controls with stable test ids.
 */
export const Toolbar: Story = {
  args: {},
  render: () => (
    <div className="p-4">
      <ComposerToolbar
        testIdSuffix="story"
        onAttach={fn()}
        onFormat={fn()}
        onMention={fn()}
        onSlash={fn()}
      />
    </div>
  ),
};

/**
 * Slash popover lists available commands and disabled post-MVP affordances.
 */
export const SlashPopover: Story = {
  args: {},
  render: () => (
    <div className="relative mt-36 p-4">
      <ComposerSlashPopover open filterValue="" onSelect={fn()} onClose={fn()} />
    </div>
  ),
};

/**
 * ChannelThreadComposer wires the generic composer to the create-thread network action.
 */
export const ChannelThread: Story = {
  args: {},
  render: () => (
    <ChannelThreadComposer
      workspaceId={storyDefaultWorkspaceId}
      channel="launch-war-room"
      sessionId="session_launch_coordination"
      peerFrom="northstar-local"
      displayName="Northstar Launch Control"
    />
  ),
};

/**
 * DetailComposer renders thread and direct reply variants.
 */
export const DetailVariants: Story = {
  args: {},
  render: () => (
    <div className="grid gap-4">
      <DetailComposer
        surface="thread"
        channel="launch-war-room"
        threadId="thread_story_launch_cutover"
        sessionId="session_launch_coordination"
        peerFrom="northstar-local"
        displayName="Northstar Launch Control"
      />
      <DetailComposer
        surface="direct"
        channel="launch-war-room"
        directId="direct_story_launch_corridor"
        sessionId="session_launch_coordination"
        peerFrom="northstar-local"
        peerTo="partner-settlement"
        peerLabel="@partner-settlement"
        displayName="Northstar Launch Control"
      />
    </div>
  ),
};
