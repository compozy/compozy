import { normalizeListingSearchValue } from "@/lib/listing-search";
import type { AutomationScope, AutomationSource } from "../types";
import type { AutomationEditorSection } from "./automation-form-draft";
import type { AutomationDoes, AutomationStart } from "./automation-sentence";

/** Start view on the listing; `webhook` is only an editor preselection (`create=1`). */
export type AutomationsStartParam = "schedule" | "event" | "webhook";

export interface AutomationsRouteSearch {
  start?: AutomationsStartParam;
  q?: string;
  enabled?: boolean;
  scope?: AutomationScope;
  source?: AutomationSource;
  target?: AutomationDoes;
  loop?: string;
  view?: "cards";
  /**
   * One-shot editor deep links, stripped once consumed:
   * `?create=1[&start=schedule|event|webhook]` opens New automation with Starts chosen;
   * `?create=loop&loop=<name>[&start=…]` also fixes Does to that Loop;
   * `?edit=1|options` on a detail route opens Edit (at Options for "Set up retries").
   */
  /** Numeric so the router keeps `create=1` (a string "1" would serialize as `"1"`). */
  create?: 1 | "loop";
  edit?: 1 | AutomationEditorSection;
}

/** What a `create` deep link asks the editor to open with. */
export interface AutomationEditorSeed {
  /** Identity of the request, so a re-render never opens it twice. */
  key: string;
  start: AutomationStart;
  loop?: string;
}

/** The editor request a `create` deep link carries, or `null`. */
export function automationEditorSeed(search: AutomationsRouteSearch): AutomationEditorSeed | null {
  const start: AutomationStart = search.start ?? "schedule";
  if (search.create === 1) return { key: `create:${start}`, start };
  if (search.create === "loop" && search.loop) {
    return { key: `loop:${search.loop}:${start}`, start, loop: search.loop };
  }
  return null;
}

/** Listing Start view; an editor-only `webhook` preselection is not a view. */
export function automationsStartView(
  search: AutomationsRouteSearch
): "schedule" | "event" | undefined {
  return search.start === "schedule" || search.start === "event" ? search.start : undefined;
}

/** `loop` filters the list unless it is the one-shot `create=loop` seed. */
export function automationListLoopFilter(search: AutomationsRouteSearch): string | undefined {
  return search.create === "loop" ? undefined : search.loop;
}

/** Any search, facet or Start view: a zero result is "filtered empty", not first run. */
export function automationRouteHasActiveFilters(search: AutomationsRouteSearch): boolean {
  return (
    (search.q?.trim() ?? "") !== "" ||
    automationListLoopFilter(search) !== undefined ||
    search.scope !== undefined ||
    search.source !== undefined ||
    search.enabled !== undefined ||
    search.target !== undefined ||
    automationsStartView(search) !== undefined
  );
}

export function parseAutomationScope(value: unknown): AutomationScope | undefined {
  return value === "global" || value === "workspace" ? value : undefined;
}

export function parseAutomationSource(value: unknown): AutomationSource | undefined {
  return value === "config" || value === "package" || value === "dynamic" ? value : undefined;
}

export function parseAutomationEnabled(value: unknown): boolean | undefined {
  if (value === true || value === "true") return true;
  if (value === false || value === "false") return false;
  return undefined;
}

export function parseAutomationTarget(value: unknown): AutomationDoes | undefined {
  return value === "agent" || value === "loop" || value === "task" ? value : undefined;
}

function parseCreate(value: unknown): AutomationsRouteSearch["create"] {
  if (value === "loop") return "loop";
  if (value === "1" || value === 1 || value === true) return 1;
  return undefined;
}

function parseEdit(value: unknown): AutomationsRouteSearch["edit"] {
  if (value === "options") return "options";
  if (value === "1" || value === 1 || value === true) return 1;
  return undefined;
}

function parseStart(
  value: unknown,
  create: AutomationsRouteSearch["create"]
): AutomationsStartParam | undefined {
  if (value === "schedule" || value === "event") return value;
  return value === "webhook" && create !== undefined ? "webhook" : undefined;
}

/** Unknown or malformed values normalize to absent (`?start=bogus` → `/automations`). */
export function validateAutomationsSearch(raw: Record<string, unknown>): AutomationsRouteSearch {
  const create = parseCreate(raw.create);
  const search: AutomationsRouteSearch = {
    start: parseStart(raw.start, create),
    q: normalizeListingSearchValue(raw.q),
    enabled: parseAutomationEnabled(raw.enabled),
    scope: parseAutomationScope(raw.scope),
    source: parseAutomationSource(raw.source),
    target: parseAutomationTarget(raw.target),
    loop: normalizeListingSearchValue(raw.loop),
    view: raw.view === "cards" ? "cards" : undefined,
    create,
    edit: parseEdit(raw.edit),
  };
  for (const key of Object.keys(search) as (keyof AutomationsRouteSearch)[]) {
    if (search[key] === undefined) delete search[key];
  }
  return search;
}

/** One-shot editor deep link on a detail route: `edit=options` opens Edit at Options. */
export type AutomationDetailEditParam = 1 | "options";

/**
 * Detail route search: the listing state Back restores (inherited from the
 * parent `/automations` route) plus the one-shot editor deep link.
 */
export interface AutomationDetailRouteSearch extends Omit<AutomationsRouteSearch, "create"> {
  edit?: AutomationDetailEditParam;
}

export function validateAutomationDetailSearch(
  raw: Record<string, unknown>
): AutomationDetailRouteSearch {
  const { create: _create, ...listing } = validateAutomationsSearch(raw);
  const edit =
    raw.edit === "options" ? "options" : raw.edit === "1" || raw.edit === 1 ? 1 : undefined;
  return edit ? { ...listing, edit } : listing;
}

/** The listing search Back returns to: everything but the detail-only deep link. */
export function automationListingSearch(
  search: AutomationDetailRouteSearch
): AutomationsRouteSearch {
  const { edit: _edit, ...listing } = search;
  return listing;
}

/** The listing state a row carries into its detail route so Back can restore it (UT-062). */
export function automationDetailSearchFrom(
  search: AutomationsRouteSearch
): AutomationDetailRouteSearch {
  const { create: _create, loop, ...listing } = search;
  const loopFilter = automationListLoopFilter({ ...search, loop });
  return loopFilter === undefined ? listing : { ...listing, loop: loopFilter };
}
