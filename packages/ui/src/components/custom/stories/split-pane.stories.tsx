import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { InboxIcon } from "lucide-react";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { SplitPane } from "../split-pane";
import { UIProvider } from "../ui-provider";

const meta: Meta<typeof SplitPane> = {
  title: "components/custom/SplitPane",
  component: SplitPane,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Two-column layout primitive: a fixed-width `list` column and a flex `detail` column. The detail slot animates between an empty-state placeholder and the selected entity. On narrow viewports the list and detail stack as full-width views with a Back button.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

interface Row {
  id: string;
  name: string;
  meta: string;
}

const ROWS: Row[] = [
  { id: "1", name: "Refactor tokens", meta: "14m ago" },
  { id: "2", name: "Investigate streaming", meta: "32m ago" },
  { id: "3", name: "Docs rewrite", meta: "1h ago" },
  { id: "4", name: "Perf audit", meta: "2h ago" },
];

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex bg-canvas text-fg" style={{ width: 960, height: 520 }}>
      {children}
    </div>
  );
}

function ListColumn({
  selected,
  onSelect,
}: {
  selected: string | null;
  onSelect: (id: string) => void;
}) {
  return (
    <div className="flex h-full flex-col">
      <div className="flex h-11 items-center justify-between border-b border-border px-3">
        <span className="text-sm font-medium">Runs</span>
        <span className="font-mono text-badge uppercase tracking-badge text-subtle">
          {ROWS.length}
        </span>
      </div>
      <ul className="flex-1 overflow-y-auto">
        {ROWS.map(row => (
          <li key={row.id}>
            <button
              type="button"
              data-active={selected === row.id}
              onClick={() => onSelect(row.id)}
              className="flex w-full flex-col gap-1 border-b border-border p-3 text-left transition-colors hover:bg-hover data-[active=true]:bg-canvas-tint"
            >
              <span className="text-[13px] font-medium text-foreground">{row.name}</span>
              <span className="font-mono text-badge uppercase tracking-badge text-subtle">
                {row.meta}
              </span>
            </button>
          </li>
        ))}
      </ul>
    </div>
  );
}

function DetailView({ row }: { row: Row }) {
  return (
    <div className="flex h-full flex-col">
      <div className="flex h-11 items-center gap-3 border-b border-border px-4">
        <span className="text-sm font-medium">{row.name}</span>
        <span className="font-mono text-badge uppercase tracking-badge text-subtle">{row.id}</span>
      </div>
      <div className="flex-1 overflow-y-auto px-6 py-4 text-sm text-muted-foreground">
        Detail view for run {row.id}, event timeline, artifacts, diff.
      </div>
    </div>
  );
}

function DetailEmpty() {
  return (
    <div className="flex h-full flex-col items-center justify-center gap-3 px-8 text-center">
      <div className="flex size-12 items-center justify-center rounded-xl bg-elevated text-muted-foreground">
        <InboxIcon className="size-5" aria-hidden="true" />
      </div>
      <h3 className="text-[15px] font-medium text-foreground">Select a run</h3>
      <p className="max-w-[380px] text-[13px] text-muted-foreground">
        Pick a run from the list to inspect its timeline, agents, and artifacts.
      </p>
    </div>
  );
}

function Interactive({ initial }: { initial?: string | null }) {
  const [selected, setSelected] = useState<string | null>(initial ?? null);
  const selectedRow = ROWS.find(row => row.id === selected) ?? null;
  return (
    <SplitPane
      list={<ListColumn selected={selected} onSelect={setSelected} />}
      detail={selectedRow ? <DetailView row={selectedRow} /> : null}
      detailEmpty={<DetailEmpty />}
      onDetailClose={() => setSelected(null)}
    />
  );
}

export const Empty: Story = {
  render: () => (
    <Frame>
      <Interactive />
    </Frame>
  ),
};

export const Selected: Story = {
  render: () => (
    <Frame>
      <Interactive initial="2" />
    </Frame>
  ),
  parameters: {
    docs: {
      description: {
        story:
          "When a detail is passed, the detail column renders instead of `detailEmpty`. Motion fades the panel between states.",
      },
    },
  },
};

export const CustomListWidth: Story = {
  render: () => (
    <Frame>
      <SplitPane
        list={<ListColumn selected={null} onSelect={() => undefined} />}
        detail={null}
        detailEmpty={<DetailEmpty />}
        listWidth={420}
      />
    </Frame>
  ),
};

export const NarrowViewport: Story = {
  parameters: {
    viewport: { defaultViewport: "mobile1" },
    docs: {
      description: {
        story:
          "At widths below the narrow breakpoint the list and detail stack as full-width views. Selecting a row hides the list; the Back button returns to it.",
      },
    },
  },
  render: () => (
    <div className="flex bg-canvas text-fg" style={{ width: 420, height: 520 }}>
      <Interactive initial="1" />
    </div>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const back = await canvas.findByRole("button", { name: "Back" });
    await expect(back).toBeInTheDocument();
    await userEvent.click(back);
    await waitFor(() => expect(canvas.getByText("Select a run")).toBeInTheDocument());
  },
};

export const ReducedMotion: Story = {
  parameters: {
    docs: {
      description: {
        story:
          "With `UIProvider reducedMotion='always'` the detail swap is instant, motion drops the opacity transition.",
      },
    },
  },
  render: () => (
    <UIProvider reducedMotion="always">
      <Frame>
        <Interactive initial="2" />
      </Frame>
    </UIProvider>
  ),
};

export const SelectAndRender: Story = {
  render: () => (
    <Frame>
      <Interactive />
    </Frame>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByText("Select a run")).toBeInTheDocument();
    const row = await canvas.findByRole("button", { name: /Refactor tokens/ });
    await userEvent.click(row);
    await waitFor(() => expect(canvas.getByText(/Detail view for run 1/)).toBeInTheDocument());
  },
};
