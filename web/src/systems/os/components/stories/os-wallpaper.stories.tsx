import type { Meta, StoryObj } from "@storybook/react-vite";

import { OsWallpaper } from "../os-wallpaper";
import { DesktopShell } from "./_desktop";

/**
 * Wallpaper Visual Contract stories always render the shared full desktop
 * shell (menubar + dock + desk hint) over the tokenized wallpaper. Capturing
 * `OsWallpaper` alone is not a valid VC-05 target — chrome parity first,
 * then gradient/grid verification on the same frame.
 */
const meta: Meta<typeof OsWallpaper> = {
  title: "systems/os/components/OsWallpaper",
  component: OsWallpaper,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The desktop wallpaper layer. Every story below mounts it under DesktopShell (menubar + full dock + optional desk hint) so Visual Contract rows compare the empty-desktop composition, not a wallpaper-only crop.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Ember — empty desktop: menubar + full dock over ember + dotted grid.
 * VC-05 implementation target.
 */
export const Ember: Story = {
  args: {},
  name: "Ember (full desktop shell)",
  parameters: {
    docs: {
      description: {
        story: "Full DesktopShell over ember. Do not capture a wallpaper-only crop for VC-05.",
      },
    },
  },
  render: () => <DesktopShell wallpaper="ember" deskHint menubar dock />,
};

/**
 * Mesh — success-tint low-right with teal depth top-left (full shell).
 */
export const Mesh: Story = {
  args: {},
  name: "Mesh (full desktop shell)",
  render: () => <DesktopShell wallpaper="mesh" deskHint menubar dock />,
};

/**
 * Carbon — near-flat warm ramp with a faint top sheen (full shell).
 */
export const Carbon: Story = {
  args: {},
  name: "Carbon (full desktop shell)",
  render: () => <DesktopShell wallpaper="carbon" deskHint menubar dock />,
};
