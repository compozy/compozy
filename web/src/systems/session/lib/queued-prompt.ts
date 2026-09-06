import type { SessionInputPayload, SessionInputsResponse } from "../types";
import { attachmentExtensionMark, isImageAttachmentMime } from "./attachment-kinds";
import { sessionAttachmentBytesURL } from "./attachment-url";

export type QueuedPromptAttachmentPreview =
  | { kind: "image"; url: string }
  | { kind: "file"; mark: string };

export interface QueuedPromptAttachmentSummary {
  fileCount: number;
  imageCount: number;
  preview?: QueuedPromptAttachmentPreview;
}

/** Closed daemon queue status set; the client never adds to it (Part II Key Decision). */
export type QueuedPromptStatus = "queued" | "dispatching" | "sent" | "failed" | "canceled";

/**
 * Who parked the entry when it was not this operator: the daemon's
 * `owner_kind` (goal, coordinator, supervision, …) and its identifier. An
 * operator-owned row carries no owner and no attribution.
 */
export interface QueuedPromptOwner {
  kind: string;
  id: string | null;
}

/**
 * How an in-row edit ended: the daemon replaced the entry, the daemon refused
 * because the entry had started dispatching and the text went to the composer
 * as a fresh draft, or the request did not land and the editor stays open.
 */
export type QueuedPromptEditOutcome = "replaced" | "handed_off" | "failed";

export interface QueuedPrompt {
  attachments?: QueuedPromptAttachmentSummary;
  id: string;
  mode?: string;
  /** Another actor's entry; `null` for the operator's own rows. */
  owner: QueuedPromptOwner | null;
  /** Run order from the daemon list (1-based); the client never counts on its own. */
  position: number;
  status?: string;
  text: string;
}

export function queuedPromptAttachmentSummary(
  attachments: SessionInputPayload["attachments"],
  workspaceId: string,
  sessionId: string
): QueuedPromptAttachmentSummary | undefined {
  if (!attachments || attachments.length === 0) return undefined;

  const first = attachments[0];
  const imageCount = attachments.filter(attachment =>
    isImageAttachmentMime(attachment.mime_type)
  ).length;
  const fileCount = attachments.length - imageCount;
  const imageURL = isImageAttachmentMime(first.mime_type)
    ? sessionAttachmentBytesURL(workspaceId, sessionId, `compozy://session-attachments/${first.id}`)
    : null;

  return {
    fileCount,
    imageCount,
    preview: imageURL
      ? { kind: "image", url: imageURL }
      : { kind: "file", mark: attachmentExtensionMark(first.name, first.mime_type) },
  };
}

/**
 * Owner attribution from the daemon's `owner_kind` / `owner_id` pair. An
 * absent or blank kind is the operator's own entry (`null`); `user` reads the
 * same way because the strip never names the operator to themselves.
 */
export function queuedPromptOwner(
  ownerKind: string | null | undefined,
  ownerId: string | null | undefined
): QueuedPromptOwner | null {
  const kind = ownerKind?.trim() ?? "";
  if (kind.length === 0 || kind === "user") return null;
  const id = ownerId?.trim() ?? "";
  return { id: id.length > 0 ? id : null, kind };
}

/** A row the operator may still edit, promote, or remove: only `queued` entries. */
export function isQueuedPromptMutable(prompt: Pick<QueuedPrompt, "status">): boolean {
  return prompt.status === undefined || prompt.status === "queued";
}

/**
 * The strip's read model from the daemon queue list, in dispatch order. Every
 * row is a durable entry the daemon accepted; position is its list index and
 * attribution is the daemon's `owner_kind` / `owner_id` pair.
 */
export function queuedPromptsFromInputs(
  inputs: readonly SessionInputPayload[] | undefined,
  workspaceId: string,
  sessionId: string
): QueuedPrompt[] {
  if (!inputs) return [];
  return inputs.map((input, index) => {
    const attachments = queuedPromptAttachmentSummary(input.attachments, workspaceId, sessionId);
    return {
      id: input.id,
      mode: input.mode,
      owner: queuedPromptOwner(input.owner_kind, input.owner_id),
      position: index + 1,
      status: input.status,
      text: input.text,
      ...(attachments ? { attachments } : {}),
    };
  });
}

/**
 * The cap the daemon reports on the queue list (`queue.cap`), the only owner
 * of "full" before any refusal. `null` when the response predates the summary.
 */
export function queueCapFromInputs(response: SessionInputsResponse | undefined): number | null {
  const cap = response?.queue?.cap;
  return typeof cap === "number" && Number.isInteger(cap) && cap > 0 ? cap : null;
}

/**
 * A cache write that changes only the list keeps the daemon's queue summary as
 * it was: the cap is not the client's to invent, and the entry count is
 * reread through the owner's invalidation rather than recounted here.
 */
export function withCachedInputs(
  current: SessionInputsResponse | undefined,
  inputs: SessionInputPayload[]
): SessionInputsResponse {
  return current?.queue !== undefined ? { inputs, queue: current.queue } : { inputs };
}
