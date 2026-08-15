// Per-message size estimates for the transcript virtualizer. Replaces the flat
// `144` that was duplicated across `session-thread.tsx` and the virtualizer hook
// with one shared fallback plus a per-row-kind estimate derived from the task-26
// row model: a message's height is approximated by folding its parts into rows
// (the same collapsed shape the reader sees for a settled turn) and summing each
// kind's typical rendered height. The virtualizer still measures real DOM heights
// once a row is on screen — these estimates only seed off-screen rows, so a closer
// guess reduces scroll drift on long, heterogeneous threads without pixel accuracy.

import { userMessageHasAttachments } from "@/systems/session/lib/session-attachment-items";
import {
  isSessionAttachmentRef,
  SESSION_ATTACHMENT_DATA_TYPE,
  SESSION_ATTACHMENT_PART_NAME,
} from "@/systems/session/lib/attachment-kinds";

import { deriveSessionRows, type SessionRow } from "./session-timeline.logic";
import { CHANGED_FILES_VISIBLE_CAP } from "./session-timeline-changed-files";
import { toTimelineParts } from "./timeline-message-parts";

// Single source of truth for the virtualizer's fallback row estimate.
export const VIRTUAL_MESSAGE_ESTIMATE = 144;

// A right-aligned user prompt is typically one or two lines.
const USER_MESSAGE_ESTIMATE = 64;
const USER_GALLERY_FRAME_ESTIMATE = 168;
const USER_GALLERY_FILE_ESTIMATE = 36;
const USER_GALLERY_GAP = 4;

// Vertical rhythm around an assistant message (`pt-1` + content-aware `pb-2`/`pb-4`).
const MESSAGE_VERTICAL_PADDING = 28;

// Typical rendered height (px) per SessionRow kind under the calm-surface
// grammar: a `.trow`/summary line ≈ 26px, a `.marker` ≈ 24px, the `.tmore`
// toggle and `.working` line ≈ 22px, the turn-fold rule ≈ 34px with its border
// gap. `work` is per visible tool row.
const ROW_KIND_ESTIMATE: Record<SessionRow["kind"], number> = {
  text: 88,
  reasoning: 26,
  data: 24,
  working: 22,
  work: 26,
  "work-toggle": 22,
  "turn-fold": 34,
  "changed-files": 26,
};

const estimateCache = new WeakMap<object, number>();

function rowEstimate(row: SessionRow): number {
  if (row.kind === "work") {
    // A collapsed settled run is a single summary line; open runs pay per row.
    if (row.summary && !row.expanded) {
      return ROW_KIND_ESTIMATE.work;
    }
    const visible = row.summary ? row.entries.length : Math.max(1, row.visibleCount);
    return ROW_KIND_ESTIMATE.work * visible + (row.summary ? ROW_KIND_ESTIMATE.work : 0);
  }
  // The collapsed roll-up is one line; each revealed file adds a bare mono line.
  if (row.kind === "changed-files") {
    const visibleLineCount =
      Math.min(row.files.length, CHANGED_FILES_VISIBLE_CAP) +
      (row.files.length > CHANGED_FILES_VISIBLE_CAP ? 1 : 0);
    return ROW_KIND_ESTIMATE["changed-files"] + (row.expanded ? visibleLineCount * 22 : 0);
  }
  return ROW_KIND_ESTIMATE[row.kind];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function userGalleryEstimate(content: unknown): number {
  if (!Array.isArray(content)) {
    return USER_GALLERY_FILE_ESTIMATE;
  }
  const hasImage = content.some(part => {
    if (!isRecord(part)) return false;
    const type = part.type;
    if (type === "image") return true;
    if (
      type === SESSION_ATTACHMENT_DATA_TYPE ||
      (type === "data" && part.name === SESSION_ATTACHMENT_PART_NAME)
    ) {
      return isSessionAttachmentRef(part.data) && part.data.mime_type.startsWith("image/");
    }
    if (type !== "file") return false;
    const mime = typeof part.mimeType === "string" ? part.mimeType : "";
    return mime.startsWith("image/");
  });
  return hasImage ? USER_GALLERY_FRAME_ESTIMATE : USER_GALLERY_FILE_ESTIMATE;
}

function computeMessageEstimate(message: {
  role?: unknown;
  id?: string;
  content?: unknown;
}): number {
  if (message.role === "user") {
    if (!userMessageHasAttachments(message.content)) {
      return USER_MESSAGE_ESTIMATE;
    }
    const hasText =
      Array.isArray(message.content) &&
      message.content.some(
        part =>
          isRecord(part) &&
          part.type === "text" &&
          typeof part.text === "string" &&
          part.text.trim()
      );
    const textHeight = hasText ? USER_MESSAGE_ESTIMATE : 28;
    return userGalleryEstimate(message.content) + USER_GALLERY_GAP + textHeight;
  }
  const parts = toTimelineParts(message);
  if (parts.length === 0) {
    return VIRTUAL_MESSAGE_ESTIMATE;
  }
  // Fold as settled (no active turn) so the estimate matches the collapsed shape a
  // settled assistant turn renders; the live turn is measured on screen anyway.
  const rows = deriveSessionRows(parts, { foldSettledTurns: true });
  const total = rows.reduce((sum, row) => sum + rowEstimate(row), MESSAGE_VERTICAL_PADDING);
  return Math.max(USER_MESSAGE_ESTIMATE, total);
}

export function estimateMessageSize(message: unknown): number {
  if (message === null || typeof message !== "object") {
    return VIRTUAL_MESSAGE_ESTIMATE;
  }
  const cached = estimateCache.get(message);
  if (cached !== undefined) {
    return cached;
  }
  const size = computeMessageEstimate(
    message as { role?: unknown; id?: string; content?: unknown }
  );
  estimateCache.set(message, size);
  return size;
}
