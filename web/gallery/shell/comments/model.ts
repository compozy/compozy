/**
 * Review comments anchored to a rendered prototype (element or region targets) or, under the
 * reserved `flow-<id>` key, to a journey flow (node targets). Shared by the gallery (browser), the
 * dev-server API and the agent CLI, so it holds types and pure helpers only: no DOM, no Node APIs.
 *
 * One file per key, `web/gallery/comments/<key>/comments.json` (version 2).
 */

export type CommentStatus = "open" | "resolved";
export type CommentAuthor = "reviewer" | "agent";

/** CSS pixels, or fractions of the canvas in `rectNormalized`/`offset`. */
export type Rect = { x: number; y: number; width: number; height: number };

/**
 * The view the reviewer was looking at. `theme` is the URL value (`light`, `dark`); on a flow,
 * `state` is the selected node id, empty for the whole flow.
 */
export type CommentContext = {
  /** Git commit the gallery ran from, null when unknown. */
  commit: string | null;
  state: string;
  theme: string | null;
  viewport: { width: number; height: number };
};

/** A point of a rendered prototype: what the overlay pins and the CLI maps to source. */
export type ElementTarget = {
  /** `element` for a click, `region` for a dragged rectangle. */
  kind: "element" | "region";
  /** Re-finds the anchor element: `[data-testid=…]` based when possible, else a short DOM path. */
  selector: string;
  testId: string | null;
  tag: string;
  /** Visible text of the anchor, trimmed to 120 characters. */
  text: string;
  /** Repository-relative JSX source (`web/src/…/file.tsx:<line>:<column>`). */
  source: string | null;
  /** Distinct source locations, most specific first (at most 8). */
  sourceChain: string[];
  /** Region in canvas (iframe viewport) pixels at capture time. */
  rect: Rect;
  /** `rect` as fractions of the canvas width and height. */
  rectNormalized: Rect;
  /** Region as fractions of the anchor's box, so the pin follows the element. Null for clicks. */
  offset: Rect | null;
};

/** A node of a flow (its id in the `.flow.ts` file), or the whole flow (`null`). */
export type NodeTarget = { kind: "node"; node: string | null; label: string };

export type CommentTarget = ElementTarget | NodeTarget;

export type CommentAttachment = {
  id: string;
  name: string;
  type: string;
  size: number;
  width: number;
  height: number;
  /** Served URL; added on read, never persisted. */
  url?: string;
};

export type CommentMessage = {
  id: string;
  author: CommentAuthor;
  body: string;
  at: string;
  editedAt: string | null;
  attachments: CommentAttachment[];
};

export type ReviewComment = {
  id: string;
  slug: string;
  status: CommentStatus;
  createdAt: string;
  updatedAt: string;
  resolvedAt: string | null;
  context: CommentContext;
  target: CommentTarget;
  /** The first message is the note; the rest are replies. */
  thread: CommentMessage[];
};

/** `sequence` is the highest comment number ever issued for the key, so deleted ids stay retired. */
export type CommentFile = {
  version: 2;
  slug: string;
  sequence?: number;
  comments: ReviewComment[];
};

export type CommentCounts = Record<string, { open: number; resolved: number }>;

/** Body of `POST /__comments/<slug>`. */
export type NewComment = {
  context: CommentContext;
  target: CommentTarget;
  body: string;
  attachments?: string[];
};

/** Vite HMR event the dev server sends when a comments file changes. */
export const commentsEvent = "proto:comments";

export const keyPattern = /^[a-z0-9]+(?:-[a-z0-9]+)*$/;
export const attachmentIdPattern = /^[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}$/;
const commitPattern = /^[a-f0-9]{40}$/;

export const maxBodyLength = 10_000;
export const maxAttachments = 6;
export const maxAttachmentBytes = 5 * 1024 * 1024;

/** The number after the slug: ids are `<slug>-<n>`, unique across files. */
export function sequence(comment: Pick<ReviewComment, "id" | "slug">) {
  return Number(comment.id.slice(comment.slug.length + 1)) || 0;
}

export const commentNumber = (comment: ReviewComment) => String(sequence(comment));

/** Highest number issued: the saved counter or a surviving comment, whichever is larger. */
export function issuedSequence(comments: readonly ReviewComment[], issued = 0) {
  return Math.max(issued, ...comments.map(sequence));
}

export function nextCommentId(slug: string, comments: readonly ReviewComment[], issued = 0) {
  return `${slug}-${issuedSequence(comments, issued) + 1}`;
}

function round(value: Rect): Rect {
  const r = (n: number) => Math.round(n * 10000) / 10000;
  return { x: r(value.x), y: r(value.y), width: r(value.width), height: r(value.height) };
}

/** Fixed key order and rounding, so files diff cleanly whoever writes them. */
function canonicalTarget(target: CommentTarget): CommentTarget {
  if (target.kind === "node") return { kind: "node", node: target.node, label: target.label };
  return {
    kind: target.kind,
    selector: target.selector,
    testId: target.testId,
    tag: target.tag,
    text: target.text,
    source: target.source,
    sourceChain: [...target.sourceChain],
    rect: round(target.rect),
    rectNormalized: round(target.rectNormalized),
    offset: target.offset ? round(target.offset) : null,
  };
}

const canonicalAttachment = (image: CommentAttachment): CommentAttachment => ({
  id: image.id,
  name: image.name,
  type: image.type,
  size: image.size,
  width: image.width,
  height: image.height,
});

export function canonicalComment(comment: ReviewComment): ReviewComment {
  const { context } = comment;
  return {
    id: comment.id,
    slug: comment.slug,
    status: comment.status,
    createdAt: comment.createdAt,
    updatedAt: comment.updatedAt,
    resolvedAt: comment.resolvedAt,
    context: {
      commit: context.commit ?? null,
      state: context.state,
      theme: context.theme,
      viewport: { width: context.viewport.width, height: context.viewport.height },
    },
    target: canonicalTarget(comment.target),
    thread: comment.thread.map(message => ({
      id: message.id,
      author: message.author,
      body: message.body,
      at: message.at,
      editedAt: message.editedAt ?? null,
      attachments: (message.attachments ?? []).map(canonicalAttachment),
    })),
  };
}

export function canonicalFile(
  slug: string,
  comments: readonly ReviewComment[],
  issued = 0
): CommentFile {
  const ordered = [...comments].sort((a, b) => sequence(a) - sequence(b));
  return {
    version: 2,
    slug,
    sequence: issuedSequence(comments, issued),
    comments: ordered.map(canonicalComment),
  };
}

function message(author: CommentAuthor, body: string, at: string): CommentMessage {
  return {
    id: crypto.randomUUID(),
    author,
    body: body.trim(),
    at,
    editedAt: null,
    attachments: [],
  };
}

export function createComment(
  slug: string,
  existing: readonly ReviewComment[],
  input: NewComment,
  at: string,
  issued = 0
): ReviewComment {
  return canonicalComment({
    id: nextCommentId(slug, existing, issued),
    slug,
    status: "open",
    createdAt: at,
    updatedAt: at,
    resolvedAt: null,
    context: input.context,
    target: input.target,
    thread: [message("reviewer", input.body, at)],
  });
}

export function withReply(
  comment: ReviewComment,
  author: CommentAuthor,
  body: string,
  at: string
): ReviewComment {
  return { ...comment, updatedAt: at, thread: [...comment.thread, message(author, body, at)] };
}

export function withStatus(
  comment: ReviewComment,
  status: CommentStatus,
  at: string
): ReviewComment {
  return { ...comment, status, updatedAt: at, resolvedAt: status === "resolved" ? at : null };
}

/** Edits one message in place, keeping thread order, timestamps and resolution. */
export function withEditedMessage(
  comment: ReviewComment,
  id: string,
  body: string,
  at: string
): ReviewComment {
  if (!isMessageBody(body)) throw new Error("Invalid message body");
  if (!comment.thread.some(item => item.id === id)) throw new Error("Message not found");
  return {
    ...comment,
    updatedAt: at,
    thread: comment.thread.map(item =>
      item.id === id ? { ...item, body: body.trim(), editedAt: at } : item
    ),
  };
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null;
const isNumber = (value: unknown): value is number =>
  typeof value === "number" && Number.isFinite(value);
const isRect = (value: unknown): value is Rect =>
  isRecord(value) &&
  isNumber(value.x) &&
  isNumber(value.y) &&
  isNumber(value.width) &&
  isNumber(value.height);
const isNullableString = (value: unknown) => value === null || typeof value === "string";

export function isMessageBody(value: unknown): value is string {
  return (
    typeof value === "string" && value.trim().length > 0 && value.trim().length <= maxBodyLength
  );
}

export function isAttachmentIds(value: unknown): value is string[] {
  return (
    Array.isArray(value) &&
    value.every(id => typeof id === "string" && attachmentIdPattern.test(id))
  );
}

function isContext(context: unknown): context is CommentContext {
  if (!isRecord(context) || !isRecord(context.viewport)) return false;
  const commit = context.commit;
  return (
    (commit === undefined ||
      commit === null ||
      (typeof commit === "string" && commitPattern.test(commit))) &&
    typeof context.state === "string" &&
    isNullableString(context.theme) &&
    isNumber(context.viewport.width) &&
    isNumber(context.viewport.height)
  );
}

function isTarget(target: unknown): target is CommentTarget {
  if (!isRecord(target)) return false;
  if (target.kind === "node") {
    return (
      (target.node === null ||
        (typeof target.node === "string" &&
          keyPattern.test(target.node) &&
          target.node.length <= 120)) &&
      typeof target.label === "string" &&
      target.label.length <= 200
    );
  }
  return (
    (target.kind === "element" || target.kind === "region") &&
    typeof target.selector === "string" &&
    isNullableString(target.testId) &&
    typeof target.tag === "string" &&
    typeof target.text === "string" &&
    isNullableString(target.source) &&
    Array.isArray(target.sourceChain) &&
    target.sourceChain.every(item => typeof item === "string") &&
    isRect(target.rect) &&
    isRect(target.rectNormalized) &&
    (target.offset === null || isRect(target.offset))
  );
}

/** Validates an untrusted `NewComment` (dev-server API input). */
export function parseNewComment(value: unknown): NewComment | null {
  if (!isRecord(value) || !isMessageBody(value.body)) return null;
  if (!isContext(value.context) || !isTarget(value.target)) return null;
  const attachments = value.attachments;
  if (attachments !== undefined && !isAttachmentIds(attachments)) return null;
  return {
    context: { ...value.context, commit: value.context.commit ?? null },
    target: value.target,
    body: value.body,
    ...(attachments ? { attachments } : {}),
  };
}

/** One line naming what a comment points at, for the panel and the CLI. */
export function targetSummary(target: CommentTarget) {
  if (target.kind === "node") {
    return target.node ? `node ${target.node} · ${target.label}` : "whole flow";
  }
  return `<${target.tag}> ${target.text ? `“${target.text}”` : target.selector}`;
}

/** The repository file of a source tag (`web/src/….tsx:12:3` → `web/src/….tsx`), when safe. */
export function sourceFile(source: string | null): string | null {
  const file = source?.replace(/:\d+:\d+$/, "");
  return file && /^web\/[\w./-]+$/.test(file) && !file.split("/").includes("..") ? file : null;
}
