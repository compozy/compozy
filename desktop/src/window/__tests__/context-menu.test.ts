import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { describe, expect, it, vi } from "vitest";
import type { MenuItem, MenuItemConstructorOptions } from "electron";

import { productContextMenu } from "../context-menu";
import { readWindowState, writeWindowState } from "../window-state-store";

const editable: Parameters<typeof productContextMenu>[0] = {
  isEditable: true,
  selectionText: "text",
  linkURL: "",
  misspelledWord: "",
  dictionarySuggestions: [],
  editFlags: {
    canUndo: true,
    canRedo: false,
    canCut: true,
    canCopy: true,
    canPaste: true,
    canDelete: true,
    canSelectAll: true,
    canEditRichly: false,
  },
};

function harness(params = editable, platform: NodeJS.Platform = "linux") {
  let enabled = true;
  let languages = ["en-US"];
  const contents = {
    replaceMisspelling: vi.fn(),
    session: {
      isSpellCheckerEnabled: () => enabled,
      setSpellCheckerEnabled: (value: boolean) => {
        enabled = value;
      },
      getSpellCheckerLanguages: () => languages,
      setSpellCheckerLanguages: (value: string[]) => {
        languages = value;
      },
      availableSpellCheckerLanguages: ["en-US", "pt-BR"],
      addWordToSpellCheckerDictionary: vi.fn(() => true),
    },
  };
  const copy = vi.fn();
  const changed = vi.fn();
  const menu = () => productContextMenu(params, contents, copy, changed, platform);
  return { contents, copy, changed, menu };
}

function submenu(items: MenuItemConstructorOptions[], label: string): MenuItemConstructorOptions[] {
  const value = items.find(item => item.label === label)?.submenu;
  if (!Array.isArray(value)) throw new Error(`Missing submenu ${label}`);
  return value;
}

function click(item: MenuItemConstructorOptions | undefined, checked = false): void {
  if (!item?.click) throw new Error("Missing menu action");
  item.click({ checked } as MenuItem, undefined, {} as Electron.KeyboardEvent);
}

// Invariant: native text actions follow Electron's edit capabilities and spellchecker state.
describe("product context menu", () => {
  it("Should expose edit roles with their native capability flags", () => {
    const menu = harness().menu();
    expect(menu.filter(item => item.role).map(item => [item.role, item.enabled])).toEqual([
      ["undo", true],
      ["redo", false],
      ["cut", true],
      ["copy", true],
      ["paste", true],
      ["selectAll", true],
    ]);
  });

  it("Should copy a transcript selection and link without editable actions", () => {
    const h = harness({ ...editable, isEditable: false, linkURL: "https://compozy.com/" });
    expect(
      h
        .menu()
        .filter(item => item.role)
        .map(item => item.role)
    ).toEqual(["copy"]);
    click(h.menu().find(item => item.label === "Copy link"));
    expect(h.copy).toHaveBeenCalledWith("https://compozy.com/");
    expect(harness({ ...editable, isEditable: false, selectionText: "" }).menu()).toEqual([]);
  });

  it("Should replace misspellings and add the original word to the dictionary", () => {
    const h = harness({ ...editable, misspelledWord: "helo", dictionarySuggestions: ["hello"] });
    click(h.menu().find(item => item.label === "hello"));
    click(h.menu().find(item => item.label === "Add to dictionary"));
    expect(h.contents.replaceMisspelling).toHaveBeenCalledWith("hello");
    expect(h.contents.session.addWordToSpellCheckerDictionary).toHaveBeenCalledWith("helo");
  });

  it("Should choose multiple languages and persist disable separately from languages", () => {
    const h = harness();
    const spelling = () => submenu(h.menu(), "Spelling");
    const languages = () => submenu(spelling(), "Languages");
    expect(languages()[0]?.enabled).toBe(false);
    click(
      languages().find(item => item.label === "pt-BR"),
      true
    );
    expect(h.changed).toHaveBeenLastCalledWith({ enabled: true, languages: ["en-US", "pt-BR"] });
    click(
      languages().find(item => item.label === "en-US"),
      false
    );
    expect(h.changed).toHaveBeenLastCalledWith({ enabled: true, languages: ["pt-BR"] });
    click(
      spelling().find(item => item.label === "Check spelling"),
      false
    );
    expect(h.changed).toHaveBeenLastCalledWith({ enabled: false, languages: ["pt-BR"] });
  });

  it("Should explain macOS language ownership without offering unsupported controls", () => {
    const spelling = submenu(harness(editable, "darwin").menu(), "Spelling");
    expect(spelling.some(item => item.label === "Languages")).toBe(false);
    expect(spelling.find(item => item.label === "Languages are managed by macOS")?.enabled).toBe(
      false
    );
  });
});

// Invariant: existing window state upgrades without losing geometry, and spelling preferences survive reopening.
describe("persisted spelling preferences", () => {
  it("Should preserve existing state and round-trip disabled multilingual preferences", async () => {
    const directory = await mkdtemp(join(tmpdir(), "compozy-spelling-"));
    try {
      const path = join(directory, "window.json");
      const geometry = { x: 0, y: 0, width: 1280, height: 800, maximized: false, zoom_level: 1 };
      await writeFile(path, JSON.stringify(geometry));
      expect(await readWindowState(path)).toEqual(geometry);
      const state = { ...geometry, spellcheck: { enabled: false, languages: ["en-US", "pt-BR"] } };
      await writeWindowState(path, state);
      expect(await readWindowState(path)).toEqual(state);
      expect(JSON.parse(await readFile(path, "utf8"))).toEqual(state);
    } finally {
      await rm(directory, { recursive: true, force: true });
    }
  });
});
