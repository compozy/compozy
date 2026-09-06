// Steering provenance (VC-07, BUG-20260906-injected-guidance-missing-history):
// how each authored message reached the turn, bound by the daemon's explicit
// `message_id` on its `prompt_steered` / `prompt_superseded` markers — never by
// the preceding bubble or same-turn proximity, since several steers can share a
// turn. One identity yields exactly one bubble: the real user message when the
// transcript holds it (its latest marker becomes the meta line under it), else
// a receipt bubble from the marker's own `authored_text` for guidance that was
// never dispatched as a normal message (pending, superseded). Pure over the
// loaded thread messages; pagination only widens what is bound.

import type { TranscriptMarkerPayload } from "../types";
import {
  PROMPT_ACCEPTED_MARKER,
  PROMPT_STEERED_MARKER,
  PROMPT_SUPERSEDED_MARKER,
  type SteerMarkerKind,
  type SteerMarkerView,
  steerMarkerView,
} from "./steer-marker";

const PROMPT_QUEUED_MARKER = "transcript_marker.prompt_queued";

export interface SteerProvenance {
  messageId: string;
  /** Latest truth for this identity (pending → injected/fallback, or superseded). */
  kind: SteerMarkerKind;
  /** The daemon's record of what the operator wrote; `null` on legacy markers. */
  authoredText: string | null;
  /** The turn the guidance targeted, when the marker named it. */
  targetTurnId: string | null;
  /** The queue position the daemon recorded for a queued prompt ("was #N"); `null` when it never named one. */
  queuePosition: string | null;
  /** Key of the marker event carrying the latest truth (see `steerMarkerKey`). */
  latestMarker: string;
}

/** What a steer marker row should render for one marker event. */
export type SteerMarkerRender =
  /** The identity's real message is loaded: its meta line owns the fact; the row renders nothing. */
  | { kind: "bound" }
  /** Guidance never dispatched as a message: render its receipt bubble here, once, with the latest truth. */
  | { kind: "receipt"; provenance: SteerProvenance }
  /** Older marker of an identity whose latest marker renders elsewhere: nothing. */
  | { kind: "superseded-marker" }
  /** No identity or text on the wire (legacy): the truthful neutral row. */
  | { kind: "legacy" };

export interface SteerProvenanceIndex {
  /** Keyed by the daemon's authored identity (`metadata.message_id`), the id its markers name. */
  byMessageId: ReadonlyMap<string, SteerProvenance>;
  /** Provenance for a rendered message, resolved through its authored identity. */
  forMessage(message: ThreadMessageLike): SteerProvenance | null;
  /** Render decision per marker event payload; absent for non-steer markers. */
  renderFor(marker: object): SteerMarkerRender | null;
  /**
   * Marker events clustered behind `first` (consecutive same-kind markers of
   * one message render as one row): the receipts to render at that row.
   */
  receiptsForCluster(first: object, count: number): SteerProvenance[];
}

export interface ThreadMessageLike {
  id?: string;
  role?: string;
  content?: unknown;
  /** The thread shape keeps the daemon's message metadata under `custom`. */
  metadata?: { custom?: unknown } | undefined;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/**
 * The identity the daemon authored for a message: its durable
 * `metadata.message_id`, which its markers name. The rendered `id` may be a
 * uniquified projection of it (a duplicate MessageID gets a suffix); rows keep
 * binding by the rendered id, provenance binds by the authored one.
 */
export function authoredMessageId(message: ThreadMessageLike): string | null {
  const custom = message.metadata?.custom;
  if (isRecord(custom)) {
    const authored = custom.message_id;
    if (typeof authored === "string" && authored.trim().length > 0) return authored.trim();
  }
  return typeof message.id === "string" && message.id.length > 0 ? message.id : null;
}

function markerOf(data: Record<string, unknown>): TranscriptMarkerPayload | null {
  const marker = data.marker;
  if (!isRecord(marker) || typeof marker.kind !== "string") return null;
  return marker as unknown as TranscriptMarkerPayload;
}

function evidenceString(marker: TranscriptMarkerPayload, key: string): string | null {
  const value = marker.evidence?.[key];
  return typeof value === "string" && value.trim().length > 0 ? value.trim() : null;
}

function isCompozyEventPart(part: Record<string, unknown>): boolean {
  return (
    part.type === "data-compozy-event" || (part.type === "data" && part.name === "compozy-event")
  );
}

interface MarkerVisit {
  key: string;
  messageIndex: number;
  view: SteerMarkerView;
  marker: TranscriptMarkerPayload;
}

// The runtime hands rows re-shaped copies of the event payload, so a marker is
// identified by its own recorded facts, never by object identity.
function steerMarkerKey(marker: TranscriptMarkerPayload): string {
  const evidence = marker.evidence ?? {};
  return [
    marker.kind,
    marker.occurred_at,
    typeof evidence.message_id === "string" ? evidence.message_id : "",
    typeof evidence.queue_entry_id === "string" ? evidence.queue_entry_id : "",
    typeof evidence.steer_delivery === "string" ? evidence.steer_delivery : "",
    typeof evidence.replacement_entry_id === "string" ? evidence.replacement_entry_id : "",
  ].join("\u001f");
}

/** The key of a marker event payload, or `null` when it is not a steer marker. */
function eventKey(data: Record<string, unknown>): string | null {
  const marker = markerOf(data);
  if (!marker) return null;
  if (
    marker.kind !== PROMPT_STEERED_MARKER &&
    marker.kind !== PROMPT_SUPERSEDED_MARKER &&
    marker.kind !== PROMPT_ACCEPTED_MARKER
  ) {
    return null;
  }
  return steerMarkerKey(marker);
}

// The position a queued prompt held comes only from the daemon: the accepted
// marker when it carries one, else the original `prompt_queued` marker for the
// same queue entry. Never an invented "#1".
function queuedPositions(messages: readonly ThreadMessageLike[]): ReadonlyMap<string, string> {
  const positions = new Map<string, string>();
  for (const message of messages) {
    if (message.role !== "assistant" || !Array.isArray(message.content)) continue;
    for (const part of message.content) {
      if (!isRecord(part) || !isCompozyEventPart(part) || !isRecord(part.data)) continue;
      const marker = markerOf(part.data);
      if (!marker || marker.kind !== PROMPT_QUEUED_MARKER) continue;
      const entryId = evidenceString(marker, "queue_entry_id");
      const position = marker.evidence?.queue_position;
      if (entryId && (typeof position === "number" || typeof position === "string")) {
        positions.set(entryId, String(position));
      }
    }
  }
  return positions;
}

function steerMarkerVisits(messages: readonly ThreadMessageLike[]): MarkerVisit[] {
  const visits: MarkerVisit[] = [];
  messages.forEach((message, messageIndex) => {
    if (message.role !== "assistant" || !Array.isArray(message.content)) return;
    for (const part of message.content) {
      if (!isRecord(part) || !isCompozyEventPart(part) || !isRecord(part.data)) continue;
      const marker = markerOf(part.data);
      if (!marker) continue;
      if (
        marker.kind !== PROMPT_STEERED_MARKER &&
        marker.kind !== PROMPT_SUPERSEDED_MARKER &&
        marker.kind !== PROMPT_ACCEPTED_MARKER
      ) {
        continue;
      }
      const view = steerMarkerView(marker);
      if (!view) continue;
      visits.push({ key: steerMarkerKey(marker), messageIndex, view, marker });
    }
  });
  return visits;
}

const EMPTY_INDEX: SteerProvenanceIndex = {
  byMessageId: new Map(),
  forMessage: () => null,
  renderFor: () => null,
  receiptsForCluster: () => [],
};

export function emptySteerProvenance(): SteerProvenanceIndex {
  return EMPTY_INDEX;
}

/**
 * Bind every steer marker to the message identity it names. Later markers for
 * the same identity replace earlier ones (a pending receipt confirmed as
 * injected, a pending steer superseded); the real user message, when loaded,
 * owns the meta line and the receipt disappears. Markers without an identity
 * stay legacy rows and attach to nothing.
 */
export function deriveSteerProvenance(
  messages: readonly ThreadMessageLike[]
): SteerProvenanceIndex {
  const loadedUserIds = new Set<string>();
  for (const message of messages) {
    if (message.role !== "user") continue;
    const authored = authoredMessageId(message);
    if (authored !== null) loadedUserIds.add(authored);
  }
  const visits = steerMarkerVisits(messages);
  if (visits.length === 0) return EMPTY_INDEX;
  const positions = queuedPositions(messages);

  const byMessageId = new Map<string, SteerProvenance>();
  const identityByMarker = new Map<string, string>();
  const visitKeys = new Set(visits.map(visit => visit.key));
  for (const visit of visits) {
    const messageId = evidenceString(visit.marker, "message_id");
    if (messageId === null) continue;
    identityByMarker.set(visit.key, messageId);
    const previous = byMessageId.get(messageId);
    const entryId = evidenceString(visit.marker, "queue_entry_id");
    byMessageId.set(messageId, {
      messageId,
      kind: visit.view.kind,
      authoredText: evidenceString(visit.marker, "authored_text") ?? previous?.authoredText ?? null,
      targetTurnId:
        evidenceString(visit.marker, "target_turn_id") ?? previous?.targetTurnId ?? null,
      queuePosition:
        evidenceString(visit.marker, "queue_position") ??
        (entryId ? positions.get(entryId) : undefined) ??
        previous?.queuePosition ??
        null,
      latestMarker: visit.key,
    });
  }

  const renderForKey = (key: string): SteerMarkerRender | null => {
    const messageId = identityByMarker.get(key);
    if (messageId === undefined) {
      return visitKeys.has(key) ? { kind: "legacy" } : null;
    }
    const provenance = byMessageId.get(messageId)!;
    if (loadedUserIds.has(messageId)) return { kind: "bound" };
    if (provenance.latestMarker !== key) return { kind: "superseded-marker" };
    if (
      provenance.authoredText === null ||
      provenance.kind === "injected" ||
      provenance.kind === "queued"
    ) {
      // Delivered guidance always has a real message; when it is not loaded the
      // row stays the truthful neutral line rather than a bubble the daemon did
      // not write, and a receipt without text has nothing to show.
      return { kind: "legacy" };
    }
    return { kind: "receipt", provenance };
  };
  const renderFor = (marker: object): SteerMarkerRender | null => {
    const key = eventKey(marker as Record<string, unknown>);
    return key === null ? null : renderForKey(key);
  };

  const receiptsForCluster = (first: object, count: number): SteerProvenance[] => {
    const firstKey = eventKey(first as Record<string, unknown>);
    const start = firstKey === null ? -1 : visits.findIndex(visit => visit.key === firstKey);
    if (start < 0) return [];
    const receipts: SteerProvenance[] = [];
    const messageIndex = visits[start]!.messageIndex;
    for (
      let index = start;
      index < visits.length && index < start + Math.max(1, count);
      index += 1
    ) {
      const visit = visits[index]!;
      if (visit.messageIndex !== messageIndex) break;
      const render = renderForKey(visit.key);
      if (render?.kind === "receipt") receipts.push(render.provenance);
    }
    return receipts;
  };

  const forMessage = (message: ThreadMessageLike): SteerProvenance | null => {
    const authored = authoredMessageId(message);
    return authored === null ? null : (byMessageId.get(authored) ?? null);
  };

  return { byMessageId, forMessage, renderFor, receiptsForCluster };
}
