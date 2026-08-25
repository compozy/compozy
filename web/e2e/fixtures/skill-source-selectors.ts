import type { Locator, Page } from "@playwright/test";

/**
 * Locators for the Settings > Skills sources section and the skill expose panel.
 *
 * These live beside `selectors.ts` rather than inside it: that file already
 * carries the whole app's surface, and one more section belongs in its own file.
 */

export interface SkillSourceSettingsSelectors {
  scopeUser: Locator;
  scopeWorkspace: Locator;
  scopeAgent: Locator;
  workspaceSelect: Locator;
  list: Locator;
  save: Locator;
  message: Locator;
  saveError: Locator;
  defaultsOnly: Locator;
  readOnlyNotice: Locator;
  row(slug: string): Locator;
  toggle(slug: string): Locator;
  count(slug: string): Locator;
  disclosure(slug: string): Locator;
  remove(slug: string): Locator;
  truncated(slug: string): Locator;
  unreadable(slug: string): Locator;
  customInput: Locator;
  customAdd: Locator;
  customError: Locator;
  keyPosture(key: string): Locator;
  keyCustomize(key: string): Locator;
  keyUseInherited(key: string): Locator;
}

export interface SkillExposeSelectors {
  panel: Locator;
  list: Locator;
  failure: Locator;
  row(target: string): Locator;
  rowStatus(target: string): Locator;
  exposeAgain(target: string): Locator;
  unexpose(target: string): Locator;
  result(target: string): Locator;
  pickerTrigger: Locator;
  pickerOption(slug: string): Locator;
  pickerConfirm: Locator;
  pickerNone: Locator;
  card: Locator;
}

const SOURCES = "settings-page-skills-sources";
const SOURCE = "settings-page-skills-source";
const CUSTOM = "settings-page-skills-custom-sources";
const PANEL = "skill-expose-panel";
const PICKER = "skill-expose-target-picker";

export function skillSourceSettingsSelectors(
  page: Pick<Page, "getByTestId">
): SkillSourceSettingsSelectors {
  return {
    scopeUser: page.getByTestId("settings-page-skills-scope-user"),
    scopeWorkspace: page.getByTestId("settings-page-skills-scope-workspace"),
    scopeAgent: page.getByTestId("settings-page-skills-scope-agent"),
    workspaceSelect: page.getByTestId("settings-page-skills-workspace-context-input"),
    list: page.getByTestId(`${SOURCES}-list`),
    save: page.getByTestId(`${SOURCES}-save`),
    message: page.getByTestId(`${SOURCES}-message`),
    saveError: page.getByTestId(`${SOURCES}-save-error`),
    defaultsOnly: page.getByTestId(`${SOURCES}-defaults-only`),
    readOnlyNotice: page.getByTestId(`${SOURCES}-read-only`),
    row: slug => page.getByTestId(`${SOURCE}-${slug}`),
    toggle: slug => page.getByTestId(`${SOURCE}-${slug}-toggle`),
    count: slug => page.getByTestId(`${SOURCE}-${slug}-count`),
    disclosure: slug => page.getByTestId(`${SOURCE}-${slug}-disclosure`),
    remove: slug => page.getByTestId(`${SOURCE}-${slug}-remove`),
    truncated: slug => page.getByTestId(`${SOURCE}-${slug}-truncated`),
    unreadable: slug => page.getByTestId(`${SOURCE}-${slug}-unreadable`),
    customInput: page.getByTestId(`${CUSTOM}-input`),
    customAdd: page.getByTestId(`${CUSTOM}-add`),
    customError: page.getByTestId(`${CUSTOM}-error`),
    keyPosture: key => page.getByTestId(`${SOURCES}-key-${key}-posture`),
    keyCustomize: key => page.getByTestId(`${SOURCES}-key-${key}-customize`),
    keyUseInherited: key => page.getByTestId(`${SOURCES}-key-${key}-use-inherited`),
  };
}

export function skillExposeSelectors(page: Pick<Page, "getByTestId">): SkillExposeSelectors {
  return {
    panel: page.getByTestId(PANEL),
    list: page.getByTestId(`${PANEL}-list`),
    failure: page.getByTestId(`${PANEL}-failure`),
    row: target => page.getByTestId(`${PANEL}-row-${target}`),
    rowStatus: target => page.getByTestId(`${PANEL}-row-${target}-status`),
    exposeAgain: target => page.getByTestId(`${PANEL}-row-${target}-expose-again`),
    unexpose: target => page.getByTestId(`${PANEL}-row-${target}-unexpose`),
    result: target => page.getByTestId(`${PANEL}-result-${target}`),
    pickerTrigger: page.getByTestId(`${PICKER}-trigger`),
    pickerOption: slug => page.getByTestId(`${PICKER}-option-${slug}`),
    pickerConfirm: page.getByTestId(`${PICKER}-confirm`),
    pickerNone: page.getByTestId(`${PICKER}-none`),
    card: page.getByTestId("skill-exposures-card"),
  };
}
