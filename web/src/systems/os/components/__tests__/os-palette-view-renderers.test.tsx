// Suite: command palette view renderers
// Invariant: list, detail, form, grid, and program degradation share one live stack
// chrome while each body keeps its own focus, validation, sanitization, and activation contract.
// Boundary IN: renderer components and the shared view shell.
// Boundary OUT: TanStack domain fetches, daemon patch transport, and browser E2E stack teardown.

import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { Blocks } from "lucide-react";

import { UIProvider } from "@compozy/ui";

import { compareStatusAttentionFirst, TASK_STATUS_TONE } from "@/lib/status-tone";

import {
  contentForEnvelope,
  viewActionCommandID,
} from "../../hooks/use-cmd-palette-declarative-view";
import type { OsPaletteDomainRow as DomainRow } from "../../hooks/use-os-palette-domain-search";
import type {
  CmdPaletteViewAction,
  CmdPaletteViewDetail,
  CmdPaletteViewForm,
  CmdPaletteViewGrid,
  CmdPaletteViewEnvelope,
} from "../../lib/cmd-palette-types";
import type { PaletteViewContent } from "../../lib/palette-view-registry";
import { OsPaletteDomainChips } from "../os-palette-domain-chips";
import { OsPaletteDomainRow } from "../os-palette-domain-row";
import { OsPaletteViewShell } from "../os-palette-view-shell";
import { OsPaletteViewUnavailable } from "../os-palette-view-stack";
import { OsPaletteProgramBand, OsPaletteProgramFailure } from "../os-palette-program-status";
import { PaletteDetailView } from "../palette-detail-view";
import { PaletteFormView } from "../palette-form-view";
import { PaletteGridView } from "../palette-grid-view";

const ACTION: CmdPaletteViewAction = {
  title: "Open",
  primary: true,
  action: { kind: "tool", tool: "open" },
};

function withUI(node: React.ReactNode) {
  return render(<UIProvider reducedMotion="always">{node}</UIProvider>);
}

function shellContent(kind: "list" | "detail" | "form" | "grid"): PaletteViewContent {
  return {
    kind,
    rows: [],
    body: kind === "list" ? undefined : <div>{kind} body</div>,
    header: null,
    empty: <p>No rows</p>,
    note: null,
    backHint: "back",
    resetKey: kind,
    onEmptyQueryBackspace: () => false,
  };
}

describe("command palette view renderers", () => {
  it("Should mount every kind under the same stack chrome and pop on empty Backspace [UT-130]", async () => {
    const user = userEvent.setup();
    const onPop = vi.fn();
    const view = withUI(
      <OsPaletteViewShell
        breadcrumb={{ truncated: false, visible: ["Example"] }}
        content={shellContent("list")}
        definition={{
          id: "example",
          title: "Example",
          icon: Blocks,
          placeholder: "Search example…",
          enterHint: "open",
          description: "Example",
        }}
        query=""
        onPop={onPop}
        onQueryChange={vi.fn()}
      />
    );
    const frame = screen.getByTestId("os-command-palette");
    for (const kind of ["list", "detail", "form", "grid"] as const) {
      view.rerender(
        <UIProvider reducedMotion="always">
          <OsPaletteViewShell
            breadcrumb={{ truncated: false, visible: ["Example"] }}
            content={shellContent(kind)}
            definition={{
              id: "example",
              title: "Example",
              icon: Blocks,
              placeholder: "Search example…",
              enterHint: "open",
              description: "Example",
            }}
            query=""
            onPop={onPop}
            onQueryChange={vi.fn()}
          />
        </UIProvider>
      );
      expect(frame).toHaveAttribute("data-palette-kind", kind);
      expect(screen.getByPlaceholderText("Search example…")).toBeVisible();
    }
    await user.type(screen.getByPlaceholderText("Search example…"), "{Backspace}");
    expect(onPop).toHaveBeenCalledOnce();
  });

  it("Should name an unavailable extension and retain the pop path [UT-131]", async () => {
    const user = userEvent.setup();
    const onPop = vi.fn();
    withUI(
      <OsPaletteViewUnavailable
        breadcrumb={{ truncated: false, visible: ["ext.notes.dead"] }}
        onPop={onPop}
        viewId="ext.notes.dead"
      />
    );
    expect(screen.getByText(/notes view is not available/i)).toBeVisible();
    await user.type(screen.getByPlaceholderText("Search…"), "{Backspace}");
    expect(onPop).toHaveBeenCalledOnce();
  });

  it("Should use shared task tones and single-select truthful chips [UT-133]", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const row: DomainRow = {
      key: "task:1",
      label: "Review release",
      status: "in_progress",
      app: "tasks",
      route: { pathname: "/tasks", search: {} },
    };
    withUI(
      <>
        <OsPaletteDomainRow row={row} />
        <OsPaletteDomainChips
          active="all"
          chips={[
            { id: "all", label: "All", count: 4 },
            { id: "running", label: "Running", count: 1 },
          ]}
          onChange={onChange}
        />
      </>
    );
    expect(screen.getByTestId("os-palette-domain-status")).toHaveAttribute(
      "data-tone",
      TASK_STATUS_TONE.in_progress
    );
    expect(screen.getByRole("button", { name: "All, 4" })).toHaveAttribute("aria-pressed", "true");
    await user.click(screen.getByRole("button", { name: "Running, 1" }));
    expect(onChange).toHaveBeenCalledWith("running");
    expect(compareStatusAttentionFirst("failed", "completed")).toBeLessThan(0);
  });

  it("Should keep hostile markdown inert and render metadata [UT-135, UT-136]", () => {
    const detail: CmdPaletteViewDetail = {
      markdown: [
        "<script>globalThis.pwned=true</script>",
        "",
        "[bad](javascript:alert(1)) **Safe**",
      ].join("\n"),
      metadata: [{ label: "State", value: "Ready" }],
    };
    withUI(<PaletteDetailView detail={detail} />);
    expect(document.querySelector("script")).toBeNull();
    expect(screen.getByText("Safe")).toBeVisible();
    expect(screen.getByText("Ready")).toBeVisible();
    expect(document.querySelector('a[href^="javascript:"]')).toBeNull();
    expect(screen.getByText(/bad \[blocked\]/i)).toBeVisible();
  });

  it("Should validate fields in order and preserve masked values after failure [UT-137, UT-138]", async () => {
    const user = userEvent.setup();
    const form: CmdPaletteViewForm = {
      fields: [
        { id: "title", type: "text", label: "Title", required: true },
        { id: "password", type: "password", label: "Password", required: true },
        {
          id: "kind",
          type: "dropdown",
          label: "Kind",
          required: false,
          options: [],
          empty_hint: "No kinds are available.",
        },
      ],
      submit: { ...ACTION, title: "Save" },
    };
    const onSubmit = vi.fn().mockRejectedValue(new Error("Provider rejected the form"));
    expect(
      viewActionCommandID("ext.notes.capture", {
        title: "Save",
        action: { kind: "tool", tool: "capture" },
      })
    ).toBe("ext.notes.capture");
    withUI(<PaletteFormView form={form} onSubmit={onSubmit} />);
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(screen.getByLabelText("Title")).toHaveFocus();
    expect(screen.getAllByText("Required")).toHaveLength(2);
    expect(screen.getByText("No kinds are available.")).toBeVisible();

    await user.type(screen.getByLabelText("Title"), "Release notes");
    await user.type(screen.getByLabelText("Password"), "top-secret");
    expect(screen.getByLabelText("Password")).toHaveAttribute("type", "password");
    await user.click(screen.getByRole("button", { name: "Save" }));
    await waitFor(() => expect(onSubmit).toHaveBeenCalledOnce());
    expect(onSubmit).toHaveBeenCalledWith(form.submit, {
      title: "Release notes",
      password: "top-secret",
      kind: "",
    });
    expect(screen.getByTestId("palette-form-error")).toHaveTextContent(
      "Provider rejected the form"
    );
    expect(screen.getByLabelText("Title")).toHaveValue("Release notes");
    expect(screen.getByLabelText("Password")).toHaveValue("top-secret");
  });

  it("Should navigate a grid in two dimensions, activate, and degrade broken media [UT-139]", async () => {
    const user = userEvent.setup();
    const onAction = vi.fn();
    const grid: CmdPaletteViewGrid = {
      sections: [
        {
          title: "Featured",
          tiles: [
            { id: "one", title: "One", image: { url: "/one.png" }, actions: [ACTION] },
            { id: "two", title: "Two", image: { emoji: "✦" }, actions: [ACTION] },
            { id: "three", title: "Three", image: { token: "box" }, actions: [ACTION] },
          ],
        },
      ],
    };
    withUI(<PaletteGridView columns={2} grid={grid} onAction={onAction} />);
    const control = screen.getByRole("grid");
    control.focus();
    await user.keyboard("{ArrowDown}{Enter}");
    expect(screen.getByTestId("palette-grid-tile-three")).toHaveAttribute("aria-selected", "true");
    expect(onAction).toHaveBeenCalledWith(ACTION);
    const image = document.querySelector("img");
    expect(image).not.toBeNull();
    fireEvent.error(image!);
    expect(document.querySelector("img")).toBeNull();
  });

  it("Should use the list empty grammar for an empty grid [UT-139]", () => {
    withUI(
      <PaletteGridView
        empty={{ title: "No extensions", hint: "Try another source." }}
        grid={{ sections: [] }}
        onAction={vi.fn()}
      />
    );
    expect(screen.getByText("No extensions")).toBeVisible();
    expect(screen.getByText("Try another source.")).toBeVisible();
  });

  it("Should virtualize grids beyond the mount threshold [UT-139]", () => {
    const tiles = Array.from({ length: 151 }, (_, index) => ({
      id: `tile-${index}`,
      title: `Tile ${index}`,
      image: { token: "image" },
      actions: [ACTION],
    }));
    withUI(<PaletteGridView grid={{ sections: [{ title: "Large", tiles }] }} onAction={vi.fn()} />);
    expect(screen.getByRole("grid")).toHaveAttribute("data-virtualized", "true");
    expect(screen.queryAllByRole("gridcell").length).toBeLessThan(tiles.length);
  });

  it("Should keep the shell and last-good rows live across busy and degraded bands [UT-165]", async () => {
    const user = userEvent.setup();
    const onPop = vi.fn();
    const onQueryChange = vi.fn();
    const content = {
      ...shellContent("list"),
      rows: [
        {
          value: "last-good",
          testId: "last-good",
          node: <span>Last good row</span>,
          onSelect: vi.fn(),
        },
      ],
      header: <OsPaletteProgramBand phase="busy" onRetry={vi.fn()} />,
    } satisfies PaletteViewContent;
    const view = withUI(
      <OsPaletteViewShell
        breadcrumb={{ truncated: false, visible: ["Notes"] }}
        content={content}
        definition={{
          id: "ext.notes.browser",
          title: "Browse notes",
          icon: Blocks,
          placeholder: "Search notes…",
          enterHint: "open",
          description: "Browse notes from Notes",
        }}
        query=""
        onPop={onPop}
        onQueryChange={onQueryChange}
      />
    );
    expect(screen.getByText("updating")).toBeVisible();
    expect(screen.getByText("Last good row")).toBeVisible();
    await user.type(screen.getByPlaceholderText("Search notes…"), "n");
    expect(onQueryChange).toHaveBeenCalledWith("n");

    view.rerender(
      <UIProvider reducedMotion="always">
        <OsPaletteViewShell
          breadcrumb={{ truncated: false, visible: ["Notes"] }}
          content={{
            ...content,
            header: <OsPaletteProgramBand phase="degraded" onRetry={vi.fn()} />,
          }}
          definition={{
            id: "ext.notes.browser",
            title: "Browse notes",
            icon: Blocks,
            placeholder: "Search notes…",
            enterHint: "open",
            description: "Browse notes from Notes",
          }}
          query=""
          onPop={onPop}
          onQueryChange={onQueryChange}
        />
      </UIProvider>
    );
    expect(screen.getByText("degraded")).toBeVisible();
    expect(screen.getByRole("button", { name: "Retry" })).toBeVisible();
    expect(screen.getByText("Last good row")).toBeVisible();
    await user.type(screen.getByPlaceholderText("Search notes…"), "{Backspace}");
    expect(onPop).toHaveBeenCalledOnce();
  });

  it("Should render circuit and crash frames without trapping the shared chrome [UT-165]", () => {
    withUI(
      <OsPaletteViewShell
        breadcrumb={{ truncated: false, visible: ["Notes", "Browse notes"] }}
        content={{
          ...shellContent("list"),
          empty: (
            <OsPaletteProgramFailure
              error={null}
              phase="circuit-open"
              source="Browse notes (ext.notes)"
            />
          ),
        }}
        definition={{
          id: "ext.notes.browser",
          title: "Browse notes",
          icon: Blocks,
          placeholder: "Search notes…",
          enterHint: "open",
          description: "Browse notes from Notes",
        }}
        query=""
        onPop={vi.fn()}
        onQueryChange={vi.fn()}
      />
    );
    expect(screen.getByText("view broken")).toBeVisible();
    expect(screen.getByText("Browse notes (ext.notes)")).toBeVisible();
    expect(screen.getByText("until reopen")).toBeVisible();
    expect(screen.getByPlaceholderText("Search notes…")).toBeVisible();
  });

  it("Should filter complete frames locally and leave incomplete frames provider-owned [UT-163]", () => {
    const complete = viewEnvelope(true);
    const incomplete = viewEnvelope(false);
    const shared = {
      activeChip: "all",
      query: "alpha",
      runAction: vi.fn(),
      selectedRow: "",
      setActiveChip: vi.fn(),
      setSelectedRow: vi.fn(),
    };
    expect(
      contentForEnvelope({ ...shared, envelope: complete, filterLocally: true }).rows.map(
        row => row.value
      )
    ).toEqual(["alpha"]);
    expect(
      contentForEnvelope({ ...shared, envelope: incomplete, filterLocally: false }).rows.map(
        row => row.value
      )
    ).toEqual(["alpha", "beta"]);
  });
});

function viewEnvelope(complete: boolean): CmdPaletteViewEnvelope {
  return {
    view_id: "ext.notes.browser",
    title: "Browse notes",
    kind: "list",
    revision: "vr_1",
    stream_epoch: "session:vs_test",
    payload: {
      view: "v1",
      chrome: { complete },
      sections: [
        {
          rows: [
            { id: "alpha", title: "Alpha" },
            { id: "beta", title: "Beta" },
          ],
        },
      ],
    },
  };
}
