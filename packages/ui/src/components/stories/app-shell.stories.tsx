import type { Meta, StoryObj } from "@storybook/react-vite";

import {
  AppShell,
  AppShellBrand,
  AppShellContent,
  AppShellHeader,
  AppShellMain,
  AppShellNavItem,
  AppShellNavSection,
  AppShellSidebar,
} from "../app-shell";
import { Button } from "../button";
import { SectionHeading } from "../section-heading";
import { StatusBadge } from "../status-badge";
import { SurfaceCard, SurfaceCardBody, SurfaceCardHeader, SurfaceCardTitle } from "../surface-card";

const meta: Meta<typeof AppShell> = {
  title: "ui/AppShell",
  component: AppShell,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Shared shell frame used by the daemon routes for navigation and operator context.",
      },
    },
  },
  tags: ["autodocs"],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Full shell composition with sidebar navigation and content chrome.
 */
export const Default: Story = {
  args: {},
  render: () => (
    <AppShell>
      <AppShellSidebar>
        <AppShellBrand
          badge={<StatusBadge tone="accent">daemon</StatusBadge>}
          detail="storybook-workspace"
          title="Compozy"
        />
        <AppShellNavSection title="Workspace">
          <AppShellNavItem active badge="7" label="Dashboard" />
          <AppShellNavItem badge="2" label="Workflows" />
          <AppShellNavItem badge="4" label="Runs" />
        </AppShellNavSection>
      </AppShellSidebar>
      <AppShellMain>
        <AppShellHeader>
          <SectionHeading
            actions={<Button size="sm">Sync all</Button>}
            description="Shared shell primitives stay product-agnostic while carrying the daemon theme."
            eyebrow="Overview"
            title="Operator dashboard"
          />
        </AppShellHeader>
        <AppShellContent>
          <SurfaceCard>
            <SurfaceCardHeader>
              <SurfaceCardTitle>Runtime overview</SurfaceCardTitle>
            </SurfaceCardHeader>
            <SurfaceCardBody>
              <p className="text-sm text-muted-foreground">
                Route stories consume this shell frame through the real app router.
              </p>
            </SurfaceCardBody>
          </SurfaceCard>
        </AppShellContent>
      </AppShellMain>
    </AppShell>
  ),
};
