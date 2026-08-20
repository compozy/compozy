import { useState } from "react";

import type { Meta, StoryObj } from "@storybook/react-vite";
import { Blocks } from "lucide-react";
import { fn, userEvent, within } from "storybook/test";

import { statusTone } from "@/lib/status-tone";

import type { CmdPaletteViewAction } from "../../lib/cmd-palette-types";
import type { PaletteViewContent } from "../../lib/palette-view-registry";
import { OsPaletteDomainChips } from "../os-palette-domain-chips";
import { OsPaletteViewShell } from "../os-palette-view-shell";
import { OsPaletteViewUnavailable } from "../os-palette-view-stack";
import {
  OsPaletteProgramBand,
  OsPaletteProgramFailure,
  OsPaletteProgramReloaded,
} from "../os-palette-program-status";
import { PaletteDetailView } from "../palette-detail-view";
import { PaletteFormView } from "../palette-form-view";
import { PaletteGridView } from "../palette-grid-view";
import { PaletteListRow } from "../palette-list-row";
import { DesktopShell } from "./_desktop";

const ACTION: CmdPaletteViewAction = {
  title: "Open",
  primary: true,
  action: { kind: "tool", tool: "ext.fixture.open" },
};

const definition = {
  id: "ext.fixture.contract",
  title: "Fixture",
  icon: Blocks,
  placeholder: "Search fixture…",
  enterHint: "open",
  description: "Fixture",
};

function ContractFrame({ content }: { content: PaletteViewContent }) {
  const [query, setQuery] = useState("");
  return (
    <DesktopShell wallpaper="ember">
      <div className="absolute inset-0 z-20 flex items-start justify-center bg-overlay-scrim pt-[15vh]">
        <OsPaletteViewShell
          breadcrumb={{ truncated: false, visible: ["Views", "Fixture"] }}
          content={content}
          definition={definition}
          query={query}
          onPop={fn()}
          onQueryChange={setQuery}
        />
      </div>
    </DesktopShell>
  );
}

function body(kind: "detail" | "form" | "grid", node: React.ReactNode): PaletteViewContent {
  return {
    kind,
    rows: [],
    body: node,
    header: null,
    empty: null,
    note: null,
    backHint: "back",
    resetKey: kind,
    onEmptyQueryBackspace: () => false,
  };
}

function listContent(options?: {
  empty?: React.ReactNode;
  header?: React.ReactNode;
  detail?: React.ReactNode;
  rows?: readonly { id: string; title: string; subtitle: string; tone: string }[];
}): PaletteViewContent {
  const rows = options?.rows ?? [
    { id: "one", title: "Review release", subtitle: "COM-142", tone: "in_progress" },
    { id: "two", title: "Prepare rollout", subtitle: "COM-143", tone: "failed" },
  ];
  return {
    kind: "list",
    rows: rows.map(row => ({
      value: row.id,
      testId: `contract-row-${row.id}`,
      node: (
        <PaletteListRow
          row={{
            id: row.id,
            title: row.title,
            subtitle: row.subtitle,
            badge: {
              label: row.tone.replaceAll("_", " "),
              tone: statusTone(row.tone),
            },
          }}
        />
      ),
      onSelect: fn(),
    })),
    header: options?.header ?? null,
    empty: options?.empty ?? null,
    aside: options?.detail,
    note: null,
    backHint: "back",
    resetKey: "contract-list",
    onEmptyQueryBackspace: () => false,
  };
}

const meta = {
  title: "systems/os/components/PaletteViewContract",
  component: ContractFrame,
  parameters: { layout: "fullscreen" },
} satisfies Meta<typeof ContractFrame>;

export default meta;
type Story = StoryObj<typeof meta>;

export const List: Story = {
  args: {
    content: listContent({
      header: (
        <OsPaletteDomainChips
          active="all"
          chips={[
            { id: "all", label: "All", count: 2 },
            { id: "running", label: "Running", count: 1 },
            { id: "failed", label: "Failed", count: 1 },
          ]}
          onChange={fn()}
        />
      ),
    }),
  },
};

export const TasksZeroMatch: Story = {
  args: {
    content: listContent({
      rows: [],
      header: (
        <OsPaletteDomainChips
          active="failed"
          chips={[
            { id: "all", label: "All", count: 4 },
            { id: "failed", label: "Failed", count: 0 },
          ]}
          onChange={fn()}
        />
      ),
      empty: (
        <p className="px-3 py-8 text-center text-small-body text-muted">No tasks are failed.</p>
      ),
    }),
  },
};

export const ColdLoading: Story = {
  args: {
    content: listContent({
      rows: [],
      empty: <p className="px-3 py-8 text-center text-small-body text-muted">Loading tasks…</p>,
    }),
  },
};

export const VaultNamesOnly: Story = {
  args: {
    content: listContent({
      rows: [
        { id: "deploy-token", title: "DEPLOY_TOKEN", subtitle: "production · token", tone: "" },
        { id: "registry-key", title: "REGISTRY_KEY", subtitle: "build · key", tone: "" },
      ],
    }),
  },
};

export const ListDetail: Story = {
  args: {
    content: listContent({
      detail: (
        <PaletteDetailView
          detail={{
            markdown: "## Review release\nConfirm the rollout checklist.",
            metadata: [{ label: "State", value: "In progress" }],
            actions: [ACTION],
          }}
          onAction={fn()}
        />
      ),
    }),
  },
};

export const Detail: Story = {
  args: {
    content: body(
      "detail",
      <PaletteDetailView
        detail={{
          markdown: "## Standup follow-ups\n- Check the runtime\n- Publish the notes",
          metadata: [{ label: "Updated", value: "2m ago" }],
          actions: [ACTION],
        }}
        onAction={fn()}
      />
    ),
  },
};

const form = {
  fields: [
    { id: "title", type: "text" as const, label: "Title", required: true },
    { id: "password", type: "password" as const, label: "Password", required: true },
    {
      id: "tag",
      type: "dropdown" as const,
      label: "Tag",
      options: [],
      empty_hint: "No tags are available.",
    },
  ],
  submit: { ...ACTION, title: "Save" },
};

export const FormPristine: Story = {
  args: { content: body("form", <PaletteFormView form={form} onSubmit={async () => {}} />) },
};

export const FormInvalid: Story = {
  args: {
    content: body(
      "form",
      <PaletteFormView
        form={{
          ...form,
          fields: form.fields.map((field, index) =>
            index === 0 ? { ...field, error: "Required" } : field
          ),
        }}
        onSubmit={async () => {}}
      />
    ),
  },
};

export const FormFailed: Story = {
  args: {
    content: body(
      "form",
      <PaletteFormView
        form={form}
        onSubmit={async () => {
          throw new Error("Provider rejected the form");
        }}
      />
    ),
  },
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    await userEvent.type(await canvas.findByLabelText("Title"), "Release notes");
    await userEvent.type(await canvas.findByLabelText("Password"), "top-secret");
    await userEvent.click(await canvas.findByRole("button", { name: "Save" }));
    await canvas.findByText("Provider rejected the form");
  },
};

const grid = {
  sections: [
    {
      title: "Featured",
      tiles: [
        { id: "one", title: "Dunes", image: { url: "/missing-dunes.png" }, actions: [ACTION] },
        { id: "two", title: "Orbit", image: { emoji: "✦" }, actions: [ACTION] },
        { id: "three", title: "Tide", image: { token: "image" }, actions: [ACTION] },
      ],
    },
  ],
};

export const Grid: Story = {
  args: { content: body("grid", <PaletteGridView grid={grid} onAction={fn()} />) },
};

export const GridEmpty: Story = {
  args: {
    content: body(
      "grid",
      <PaletteGridView
        empty={{ title: "No extensions", hint: "Try another source." }}
        grid={{ sections: [] }}
        onAction={fn()}
      />
    ),
  },
};

export const Unavailable: Story = {
  args: { content: listContent() },
  render: () => (
    <DesktopShell wallpaper="ember">
      <div className="absolute inset-0 z-20 flex items-start justify-center bg-overlay-scrim pt-[15vh]">
        <OsPaletteViewUnavailable
          breadcrumb={{ truncated: false, visible: ["Views", "ext.notes.dead"] }}
          viewId="ext.notes.dead"
          onPop={fn()}
        />
      </div>
    </DesktopShell>
  ),
};

export const Timeout: Story = {
  args: {
    content: listContent({
      rows: [],
      empty: (
        <div className="px-3 py-8 text-center text-small-body text-muted">
          <p>This view is taking longer than expected.</p>
          <button type="button" className="mt-2 text-fg underline underline-offset-2">
            Retry
          </button>
        </div>
      ),
    }),
  },
};

export const ProgramBusy: Story = {
  args: {
    content: {
      ...listContent(),
      header: <OsPaletteProgramBand phase="busy" onRetry={fn()} />,
    },
  },
};

export const ProgramDegraded: Story = {
  args: {
    content: {
      ...listContent(),
      header: <OsPaletteProgramBand phase="degraded" onRetry={fn()} />,
    },
  },
};

export const ProgramCircuitBroken: Story = {
  args: {
    content: {
      ...listContent({ rows: [] }),
      empty: (
        <OsPaletteProgramFailure error={null} phase="circuit-open" source="Fixture (ext.fixture)" />
      ),
    },
  },
};

export const ProgramCrashed: Story = {
  args: {
    content: {
      ...listContent({ rows: [] }),
      empty: (
        <OsPaletteProgramFailure
          error="The extension process stopped responding."
          phase="unavailable"
          source="Fixture (ext.fixture)"
        />
      ),
    },
  },
};

export const ProgramReloaded: Story = {
  args: {
    content: {
      ...listContent(),
      header: <OsPaletteProgramReloaded />,
    },
  },
};
