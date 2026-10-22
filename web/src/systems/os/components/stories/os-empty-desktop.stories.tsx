import type { Meta, StoryObj } from "@storybook/react-vite";

import { OsEmptyDesktopView } from "../os-empty-desktop";
import { DesktopShell } from "./_desktop";
import { EmptyDesktopPreview } from "./_empty-desktop-preview";

/**
 * VC-10 — the empty desktop inside the full shell: the plain question over the
 * real session composer. Sending starts a session with the prompt and opens it
 * on this desktop; here the send is recorded as an action.
 */
const meta: Meta<typeof OsEmptyDesktopView> = {
  title: "systems/os/components/OsEmptyDesktop",
  component: OsEmptyDesktopView,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "An empty desktop is a place to start: the question, the session composer with its agent control, and the ⌘K hint. No card and no modal — the desk panel is the surface. Under All projects the composer drops the agent control: New session asks for the project and agent.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** A project is active: type and press Enter to start a session with the chosen agent. */
export const InProject: Story = {
  render: () => (
    <DesktopShell>
      <EmptyDesktopPreview />
    </DesktopShell>
  ),
};

/** The prompt was sent: the words stay while the session starts, and the status says so. */
export const Starting: Story = {
  render: () => (
    <DesktopShell>
      <EmptyDesktopPreview starting prompt="Fix the flaky login test and open a pull request" />
    </DesktopShell>
  ),
};

/** All projects: the prompt still starts a session; New session asks for the project and agent. */
export const AllProjects: Story = {
  render: () => (
    <DesktopShell workspace={{ name: "All projects", monogram: "∗" }}>
      <EmptyDesktopPreview asksTarget />
    </DesktopShell>
  ),
};

/** Compact presentation: the bottom tab bar replaces the rail; the composer keeps its measure. */
export const Compact: Story = {
  parameters: { viewport: { defaultViewport: "iphone14" } },
  render: () => (
    <DesktopShell compact>
      <EmptyDesktopPreview />
    </DesktopShell>
  ),
};
