import type { Meta, StoryObj } from "@storybook/react-vite";
import {
  BookOpenIcon,
  NetworkIcon,
  PlusIcon,
  SearchIcon,
  SettingsIcon,
  SparklesIcon,
  WaypointsIcon,
  WrenchIcon,
  ZapIcon,
} from "lucide-react";
import { useState } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { UIProvider } from "../custom/ui-provider";
import { Sidebar } from "../sidebar";

const meta: Meta<typeof Sidebar> = {
  title: "components/ui/Sidebar",
  component: Sidebar,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Workspace rail + panel shell. Slots are content-agnostic: pass workspace switchers into `rail`, the wordmark/search into `header`, nav tree into `nav`, and connection/settings into `footer`. Collapse is animated via motion and respects the `UIProvider` reduced-motion setting.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex min-h-[520px] bg-canvas text-fg" style={{ width: 960 }}>
      {children}
      <div className="flex min-h-0 flex-1 items-center justify-center px-10 text-sm text-muted-foreground">
        Main content area
      </div>
    </div>
  );
}

const WORKSPACES = [
  { id: "A", name: "agh-core" },
  { id: "C", name: "compozy" },
  { id: "R", name: "research" },
];

function RailContent({ active = "A" }: { active?: string }) {
  return (
    <>
      <div
        aria-hidden
        className="flex size-7 items-center justify-center rounded-md bg-accent font-mono text-badge font-medium text-accent-ink"
      >
        a
      </div>
      {WORKSPACES.map(ws => (
        <button
          key={ws.id}
          type="button"
          title={ws.name}
          data-active={ws.id === active}
          className="inline-flex size-7 items-center justify-center rounded-full border border-border bg-canvas-soft font-mono text-eyebrow text-muted-foreground transition-colors hover:text-foreground data-[active=true]:border-accent data-[active=true]:bg-elevated data-[active=true]:text-foreground"
        >
          {ws.id}
        </button>
      ))}
      <button
        type="button"
        aria-label="Add workspace"
        className="inline-flex size-7 items-center justify-center rounded-full border border-dashed border-border text-muted-foreground transition-colors hover:text-foreground"
      >
        <PlusIcon className="size-3" />
      </button>
    </>
  );
}

function HeaderContent() {
  return (
    <>
      <span className="flex-1 truncate text-sm font-medium tracking-tight">agh-core</span>
      <button
        type="button"
        aria-label="Search"
        className="inline-flex size-6 items-center justify-center rounded-md text-muted-foreground hover:bg-hover hover:text-foreground"
      >
        <SearchIcon className="size-3" />
      </button>
    </>
  );
}

const NAV_ITEMS = [
  { label: "Tasks", icon: SparklesIcon, active: true },
  { label: "Automation", icon: ZapIcon },
  { label: "Bridges", icon: WaypointsIcon },
  { label: "Network", icon: NetworkIcon },
  { label: "Knowledge", icon: BookOpenIcon },
  { label: "Skills", icon: WrenchIcon },
];

function NavContent() {
  return (
    <div className="flex flex-col gap-1 px-2 py-3">
      <span className="eyebrow px-2 pb-1 text-subtle">Workspace</span>
      {NAV_ITEMS.map(item => (
        <button
          key={item.label}
          type="button"
          data-active={item.active}
          className="flex items-center gap-2 rounded-md px-2 py-1.5 text-left text-[13px] text-muted-foreground transition-colors hover:bg-hover hover:text-foreground data-[active=true]:bg-elevated data-[active=true]:text-foreground"
        >
          <item.icon className="size-3" aria-hidden="true" />
          <span className="flex-1 truncate">{item.label}</span>
        </button>
      ))}
    </div>
  );
}

function FooterContent() {
  return (
    <div className="flex flex-col gap-2 text-[12px] text-muted-foreground">
      <div className="flex items-center gap-2">
        <span aria-hidden className="size-1.5 rounded-full bg-success" />
        <span className="font-mono text-badge uppercase tracking-badge">connected</span>
        <span className="ml-auto font-mono text-badge text-subtle">v0.4.1</span>
      </div>
      <button
        type="button"
        className="flex items-center gap-2 rounded-md px-1.5 py-1 text-left text-[13px] text-muted-foreground hover:bg-hover hover:text-foreground"
      >
        <SettingsIcon className="size-3" />
        <span>Settings</span>
      </button>
    </div>
  );
}

function StoryShell({
  collapsed,
  onCollapse,
}: {
  collapsed?: boolean;
  onCollapse?: (next: boolean) => void;
}) {
  return (
    <Sidebar
      rail={<RailContent />}
      header={<HeaderContent />}
      nav={<NavContent />}
      footer={<FooterContent />}
      collapsed={collapsed}
      onCollapse={onCollapse}
    />
  );
}

export const Expanded: Story = {
  render: () => (
    <Frame>
      <StoryShell collapsed={false} />
    </Frame>
  ),
};

export const Collapsed: Story = {
  render: () => (
    <Frame>
      <StoryShell collapsed={true} />
    </Frame>
  ),
  parameters: {
    docs: {
      description: {
        story:
          "With `collapsed` set to true, the panel animates to 0 width while the rail stays fully visible.",
      },
    },
  },
};

export const Uncontrolled: Story = {
  render: () => (
    <Frame>
      <Sidebar
        rail={<RailContent />}
        header={<HeaderContent />}
        nav={<NavContent />}
        footer={<FooterContent />}
      />
    </Frame>
  ),
};

export const ReducedMotion: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "With `UIProvider reducedMotion='always'` the collapse transition is instant , motion drops the width animation.",
      },
    },
  },
  render: () => (
    <UIProvider reducedMotion="always">
      <Frame>
        <StoryShell collapsed={false} />
      </Frame>
    </UIProvider>
  ),
};

function InteractiveShell() {
  const [collapsed, setCollapsed] = useState(false);
  return <StoryShell collapsed={collapsed} onCollapse={setCollapsed} />;
}

export const TogglesCollapse: Story = {
  render: () => (
    <Frame>
      <InteractiveShell />
    </Frame>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const trigger = await canvas.findByRole("button", { name: "Toggle sidebar" });
    await expect(trigger).toHaveAttribute("aria-expanded", "true");

    const sidebar = canvasElement.querySelector<HTMLElement>("[data-slot=sidebar]");
    const rail = canvasElement.querySelector<HTMLElement>("[data-slot=sidebar-rail]");
    await expect(sidebar).not.toBeNull();
    await expect(rail).not.toBeNull();

    await userEvent.click(trigger);
    await waitFor(() => expect(trigger).toHaveAttribute("aria-expanded", "false"));
    await expect(sidebar).toHaveAttribute("data-state", "collapsed");
    await expect(rail?.offsetWidth).toBeGreaterThan(0);
  },
};
