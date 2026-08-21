// Suite: command palette view bodies
// Invariant: form, grid, chip, and list-row bodies honor the declared handlers,
// filtering ownership, icon tokens, and directory/file contracts.
// Owning layer: unit. Canonical companion to os-palette-view-renderers.
// Boundary IN: claimed view renderers and contentForEnvelope.
// Boundary OUT: the shared view shell and command palette root.

import type { ReactNode } from "react";

import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import {
  contentForEnvelope,
  hostFiltersLocally,
  viewActionCommandID,
} from "../../hooks/use-cmd-palette-declarative-view";
import type {
  CmdPaletteViewAction,
  CmdPaletteViewEnvelope,
  CmdPaletteViewForm,
  CmdPaletteViewGrid,
} from "../../lib/cmd-palette-types";
import { PaletteFormView } from "../palette-form-view";
import { virtualGridRows } from "../../lib/cmd-palette-grid";
import { PaletteGridView } from "../palette-grid-view";
import { PaletteListRow } from "../palette-list-row";
import { PaletteViewChips } from "../palette-view-chips";

const ACTION: CmdPaletteViewAction = {
  title: "Open",
  primary: true,
  action: { kind: "tool", tool: "open" },
};

function withUI(node: ReactNode) {
  return render(<UIProvider reducedMotion="always">{node}</UIProvider>);
}

describe("command palette view bodies", () => {
  it("Should map a local view tool to the daemon-canonical id [RD0083]", () => {
    expect(
      viewActionCommandID("ext.notes.capture", {
        title: "Save",
        action: { kind: "tool", tool: "capture" },
      })
    ).toBe("ext__notes__capture");
  });

  it("Should honor an explicit filtering override [RD0103]", () => {
    expect(hostFiltersLocally({ filtering: false, complete: true })).toBe(false);
    expect(hostFiltersLocally({ complete: true })).toBe(true);
    const envelope = viewEnvelope();
    expect(
      contentForEnvelope({
        activeChip: null,
        envelope,
        query: "alpha",
        runAction: vi.fn(),
        selectedRow: "",
        setActiveChip: vi.fn(),
        setSelectedRow: vi.fn(),
        filterLocally: false,
      }).rows.map(row => row.value)
    ).toEqual(["alpha", "beta"]);
  });

  it("Should fire form field events, block provider errors, and use directory mode [RD0043, RD0110]", async () => {
    const user = userEvent.setup();
    const onEvent = vi.fn();
    const onSubmit = vi.fn();
    const form: CmdPaletteViewForm = {
      fields: [
        {
          id: "title",
          type: "text",
          label: "Title",
          error: "Too short",
          on_change: "title.change",
          on_blur: "title.blur",
        },
        {
          id: "folder",
          type: "file",
          label: "Folder",
          directories: true,
          default: ["notes"],
        },
      ],
      on_submit: "form.submit",
      submit: { ...ACTION, title: "Save" },
    };
    withUI(<PaletteFormView form={form} onEvent={onEvent} onSubmit={onSubmit} />);
    await user.type(screen.getByLabelText("Title"), "A");
    expect(onEvent).toHaveBeenCalledWith("title.change", ["A"], true);
    await user.tab();
    expect(onEvent).toHaveBeenCalledWith("title.blur", [], false);
    await user.click(screen.getByRole("button", { name: "Save" }));
    expect(onSubmit).not.toHaveBeenCalled();
    expect(screen.getByText("Too short")).toBeVisible();
    expect(document.querySelector("#palette-field-folder")).toHaveAttribute("webkitdirectory");
  });

  it("Should route the provider submit handler with the current form values [RD0110]", async () => {
    const user = userEvent.setup();
    const runHandler = vi.fn();
    const content = contentForEnvelope({
      activeChip: null,
      envelope: {
        ...viewEnvelope(),
        kind: "form",
        payload: {
          view: "v1",
          form: {
            fields: [{ id: "title", type: "text", label: "Title", default: "Draft" }],
            on_submit: "form.submit",
          },
        },
      },
      query: "",
      runAction: vi.fn(),
      runHandler,
      selectedRow: "",
      setActiveChip: vi.fn(),
      setSelectedRow: vi.fn(),
    });
    withUI(content.body);

    await user.click(screen.getByRole("button", { name: "Submit" }));

    expect(runHandler).toHaveBeenCalledWith("form.submit", [{ title: "Draft" }], false);
  });

  it("Should dispatch the declared pagination handler [RD0109]", async () => {
    const user = userEvent.setup();
    const runHandler = vi.fn();
    const content = contentForEnvelope({
      activeChip: null,
      envelope: {
        ...viewEnvelope(),
        payload: {
          ...viewEnvelope().payload,
          chrome: {
            complete: false,
            pagination: { has_more: true },
            on_load_more: "list.more",
          },
        },
      },
      query: "",
      runAction: vi.fn(),
      runHandler,
      selectedRow: "",
      setActiveChip: vi.fn(),
      setSelectedRow: vi.fn(),
    });
    withUI(content.header);

    await user.click(screen.getByRole("button", { name: "Load more" }));

    expect(runHandler).toHaveBeenCalledWith("list.more", [], false);
  });

  it("Should notify grid selection and honor token images [RD0047, RD0052]", async () => {
    const user = userEvent.setup();
    const onSelectionChange = vi.fn();
    const grid: CmdPaletteViewGrid = {
      sections: [
        {
          title: "First",
          tiles: [{ id: "one", title: "One", image: { token: "folder" }, actions: [ACTION] }],
        },
        {
          title: "First",
          tiles: [{ id: "two", title: "Two", image: { emoji: "✦" }, actions: [ACTION] }],
        },
      ],
    };
    withUI(
      <PaletteGridView
        columns={1}
        grid={grid}
        onAction={vi.fn()}
        onSelectionChange={onSelectionChange}
      />
    );
    screen.getByRole("grid").focus();
    await user.keyboard("{ArrowDown}");
    expect(onSelectionChange).toHaveBeenCalledWith("two");
    expect(document.querySelector("[data-icon-token='folder']")).not.toBeNull();
  });

  it("Should name view chips and render emoji row icons [RD0045, RA0194]", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    withUI(
      <>
        <PaletteViewChips
          active={null}
          allCount={2}
          chips={[{ id: "open", label: "Open", count: 1 }]}
          onChange={onChange}
        />
        <PaletteListRow row={{ id: "note", title: "Ship", icon: "📝" }} />
      </>
    );
    expect(screen.getByRole("toolbar", { name: "View filters" })).toBeVisible();
    await user.click(screen.getByRole("button", { name: "All, 2" }));
    expect(onChange).toHaveBeenCalledWith(null);
    expect(screen.getByText("📝")).toBeVisible();
  });

  it("Should keep unique virtual grid keys across repeated section titles [RD0051]", () => {
    const tiles = [
      { id: "a", title: "A", image: { token: "folder" }, actions: [ACTION] },
      { id: "b", title: "B", image: { token: "folder" }, actions: [ACTION] },
    ];
    const rows = virtualGridRows(
      [
        { title: "First", tiles: [tiles[0]!] },
        { title: "First", tiles: [tiles[1]!] },
      ],
      1,
      tiles
    );
    expect(rows.map(row => row.key)).toEqual(["0:0", "1:0"]);
  });

  it("Should not treat a loading list or grid as idle empty [RD0108]", () => {
    const content = contentForEnvelope({
      activeChip: null,
      envelope: {
        ...viewEnvelope(),
        payload: {
          view: "v1",
          chrome: { complete: true, is_loading: true },
          sections: [{ rows: [] }],
        },
      },
      query: "",
      runAction: vi.fn(),
      selectedRow: "",
      setActiveChip: vi.fn(),
      setSelectedRow: vi.fn(),
      filterLocally: true,
    });
    expect(content.empty).toBeNull();
    withUI(<PaletteGridView loading grid={{ sections: [] }} onAction={vi.fn()} />);
    expect(screen.queryByText("No items yet")).toBeNull();
    expect(screen.getByText("updating")).toBeVisible();
  });
});

function viewEnvelope(): CmdPaletteViewEnvelope {
  return {
    view_id: "ext.notes.browser",
    title: "Browse notes",
    kind: "list",
    revision: "vr_1",
    stream_epoch: "session:vs_test",
    payload: {
      view: "v1",
      chrome: { complete: true, filtering: false },
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
