import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";

import { LOCAL_COMPUTER_FIXTURE, makeComputerFixture } from "../../mocks/remote-computers-fixtures";
import { ComputerScopePicker, type ComputerScopeCount } from "../computer-scope-picker";

const DEVBOX = makeComputerFixture({ id: "cmp_devbox", name: "devbox", status: "connected" });
const STUDIO = makeComputerFixture({
  id: "cmp_studio",
  name: "studio",
  platform: "macos",
  status: "connected",
});
const WORKSTATION = makeComputerFixture({
  id: "cmp_workstation",
  name: "workstation",
  status: "offline",
  lastConnectedLabel: "2h ago",
});
const NEEDS_PRO = makeComputerFixture({
  id: "cmp_mini",
  name: "mini-build",
  status: "needs_pro",
});

const COUNTS = new Map<string, ComputerScopeCount>([
  ["local", { count: 4, needsYou: 1 }],
  ["cmp_devbox", { count: 7, needsYou: 2 }],
  ["cmp_studio", { count: 3, needsYou: 0 }],
  ["cmp_workstation", { count: 2, needsYou: 0 }],
  ["cmp_mini", { count: null, needsYou: 0 }],
]);

const meta: Meta<typeof ComputerScopePicker> = {
  title: "systems/computers/components/ComputerScopePicker",
  component: ComputerScopePicker,
  parameters: {
    layout: "fullscreen",
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="flex min-h-[420px] items-start justify-center p-6">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const AllComputersOpen: Story = {
  args: {
    local: LOCAL_COMPUTER_FIXTURE,
    computers: [DEVBOX, STUDIO, WORKSTATION, NEEDS_PRO],
    countsByComputer: COUNTS,
    value: "all",
    onChange: fn(),
    onReconnect: fn(),
    onUpgrade: fn(),
    onManageComputers: fn(),
    defaultOpen: true,
  },
};

export const ScopedToRemote: Story = {
  args: {
    ...AllComputersOpen.args,
    value: "cmp_devbox",
    defaultOpen: false,
  },
};

export const ScopedOffline: Story = {
  args: {
    ...AllComputersOpen.args,
    value: "cmp_workstation",
    defaultOpen: false,
  },
};

export const VolumeWithSearch: Story = {
  args: {
    ...AllComputersOpen.args,
    computers: [
      DEVBOX,
      STUDIO,
      WORKSTATION,
      NEEDS_PRO,
      makeComputerFixture({ id: "cmp_eu", name: "hetzner-eu", status: "connected" }),
      makeComputerFixture({ id: "cmp_gpu", name: "gpu-a100", status: "connected" }),
      makeComputerFixture({ id: "cmp_ci", name: "ci-runner", status: "connecting" }),
      makeComputerFixture({ id: "cmp_lab", name: "homelab", status: "offline" }),
    ],
    defaultOpen: true,
  },
};
