/**
 * Event vocabulary the editor offers: the four event cards, friendly condition
 * fields, and the click-to-insert detail chips for event and link messages.
 * `memory.consolidated` is intentionally absent (memory-removal).
 */

import {
  availableDataFields,
  ENVELOPE_KEYS,
  filterKeyOptions,
  getEventDef,
} from "./trigger-catalog";
import type { EventDef } from "./trigger-catalog";
import { composeEventId, parseEventSelection, type EventSelection } from "./trigger-event-id";

export type EditorEventCardId = "session.created" | "session.stopped" | "hook.completed" | "ext";

export interface EditorEventCard {
  id: EditorEventCardId;
  label: string;
  description: string;
}

export const EDITOR_EVENT_CARDS: readonly EditorEventCard[] = [
  {
    id: "session.created",
    label: "A session starts",
    description: "An agent session begins in this project.",
  },
  {
    id: "session.stopped",
    label: "A session stops",
    description: "It finished, was canceled, or failed.",
  },
  {
    id: "hook.completed",
    label: "A hook finishes",
    description: "A named hook succeeds or fails.",
  },
  {
    id: "ext",
    label: "An extension sends an event",
    description: "A custom event from an installed extension.",
  },
];

/** Runtime event id for a chosen card, keeping the hook/extension parts already typed. */
export function editorEventIdFor(card: EditorEventCardId, current: EventSelection): string {
  const def = getEventDef(card);
  return composeEventId({
    family: def?.family ?? "fixed",
    catalogId: card,
    hookName: current.hookName,
    extExt: current.extExt,
    extEvent: current.extEvent,
  });
}

/** The event definition behind a start: the webhook definition for links. */
export function editorEventDef(event: string): EventDef | undefined {
  return getEventDef(parseEventSelection(event).catalogId);
}

/** `data.stop_reason` → `Stop reason`; envelope keys keep their own name. */
export function conditionFieldName(path: string): string {
  const bare = path.replace(/^data\./, "").replaceAll(/[._]/g, " ");
  return bare.charAt(0).toUpperCase() + bare.slice(1);
}

/** Field picker label: the friendly name with its path (`Stop reason (data.stop_reason)`). */
export function conditionFieldLabel(path: string): string {
  return `${conditionFieldName(path)} (${path})`;
}

export function conditionFieldOptions(def: EventDef | undefined): string[] {
  return def ? filterKeyOptions(def) : [...ENVELOPE_KEYS];
}

export interface DetailChip {
  label: string;
  value: string;
}

const PREFERRED_DETAIL_FIELDS = [
  "session_id",
  "agent_name",
  "stop_reason",
  "hook_name",
  "hook_outcome",
  "payload",
  "endpoint_slug",
] as const;
const DETAIL_CHIP_LIMIT = 3;

/**
 * Click-to-insert Go template details for an event or link message:
 * up to three event fields plus the project, which links never have.
 */
export function detailChipsFor(def: EventDef | undefined): DetailChip[] {
  if (!def) return [];
  const fields = availableDataFields(def).map(path => path.replace(/^data\./, ""));
  const preferred = PREFERRED_DETAIL_FIELDS.filter(field => fields.includes(field));
  const picked = (preferred.length > 0 ? preferred : fields).slice(0, DETAIL_CHIP_LIMIT);
  const chips = picked.map(field => ({ label: field, value: `{{ .Data.${field} }}` }));
  return def.family === "webhook"
    ? chips
    : [...chips, { label: "workspace", value: "{{ .WorkspaceID }}" }];
}
