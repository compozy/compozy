import type { ContextMenuParams, MenuItemConstructorOptions, WebContents } from "electron";

export interface SpellcheckSettings {
  readonly enabled: boolean;
  readonly languages: readonly string[];
}

type SpellcheckSession = Pick<
  WebContents["session"],
  | "isSpellCheckerEnabled"
  | "setSpellCheckerEnabled"
  | "getSpellCheckerLanguages"
  | "setSpellCheckerLanguages"
  | "availableSpellCheckerLanguages"
  | "addWordToSpellCheckerDictionary"
>;

type Context = Pick<
  ContextMenuParams,
  | "isEditable"
  | "selectionText"
  | "linkURL"
  | "editFlags"
  | "misspelledWord"
  | "dictionarySuggestions"
>;

export function productContextMenu(
  params: Context,
  contents: { session: SpellcheckSession; replaceMisspelling(word: string): void },
  copyLink: (url: string) => void,
  settingsChanged: (settings: SpellcheckSettings) => void,
  platform: NodeJS.Platform
): MenuItemConstructorOptions[] {
  const items: MenuItemConstructorOptions[] = [];
  const session = contents.session;
  const changed = () =>
    settingsChanged({
      enabled: session.isSpellCheckerEnabled(),
      languages: session.getSpellCheckerLanguages(),
    });
  if (params.isEditable && session.isSpellCheckerEnabled() && params.misspelledWord) {
    for (const suggestion of params.dictionarySuggestions) {
      items.push({ label: suggestion, click: () => contents.replaceMisspelling(suggestion) });
    }
    if (params.dictionarySuggestions.length === 0) {
      items.push({ label: "No spelling suggestions", enabled: false });
    }
    items.push({
      label: "Add to dictionary",
      click: () => {
        session.addWordToSpellCheckerDictionary(params.misspelledWord);
      },
    });
    items.push({ type: "separator" });
  }
  if (params.isEditable) {
    items.push(
      { role: "undo", enabled: params.editFlags.canUndo },
      { role: "redo", enabled: params.editFlags.canRedo },
      { type: "separator" },
      { role: "cut", enabled: params.editFlags.canCut },
      { role: "copy", enabled: params.editFlags.canCopy },
      { role: "paste", enabled: params.editFlags.canPaste },
      { role: "selectAll", enabled: params.editFlags.canSelectAll }
    );
  } else if (params.selectionText) {
    items.push({ role: "copy", enabled: params.editFlags.canCopy });
  }
  if (params.linkURL) {
    if (items.length) items.push({ type: "separator" });
    items.push({ label: "Copy link", click: () => copyLink(params.linkURL) });
  }
  if (params.isEditable) {
    items.push({ type: "separator" });
    const enabled = session.isSpellCheckerEnabled();
    const languages = session.getSpellCheckerLanguages();
    const selected = languages.length ? languages : ["en-US"];
    const spelling: MenuItemConstructorOptions[] = [
      {
        label: "Check spelling",
        type: "checkbox",
        checked: enabled,
        click: item => {
          session.setSpellCheckerEnabled(item.checked);
          changed();
        },
      },
    ];
    if (platform === "darwin") {
      spelling.push({ label: "Languages are managed by macOS", enabled: false });
    } else {
      spelling.push({
        label: "Languages",
        submenu: session.availableSpellCheckerLanguages.map(language => ({
          label: language,
          type: "checkbox",
          checked: selected.includes(language),
          enabled: enabled && !(selected.length === 1 && selected.includes(language)),
          click: item => {
            const current = session.getSpellCheckerLanguages();
            const active = current.length ? current : ["en-US"];
            const next = item.checked
              ? [...new Set([...active, language])]
              : active.filter(value => value !== language);
            if (next.length === 0) return;
            session.setSpellCheckerLanguages(next);
            changed();
          },
        })),
      });
    }
    items.push({ label: "Spelling", submenu: spelling });
  }
  return items;
}
