/**
 * `proto comments <action>`: read and answer review comments. It reads and writes
 * `web/gallery/comments/<slug>/comments.json` directly, so it works with or without the dev server
 * (a running gallery picks changes up live).
 *
 *   comments list [--slug <slug>] [--status open|resolved|all] [--json]
 *   comments show <id> [--json]
 *   comments reply <id> --note "…" [--author agent|reviewer]
 *   comments resolve <id> --note "what changed"
 *   comments reopen <id> [--note "why"]
 *   comments edit <id> --note "…" [--message <n>]
 *   comments delete <id>
 *   comments attach <id> <file…> [--message <n>] · detach <id> <attachment id>
 *   comments attachments <id> --out <dir> [--json]
 */
import { execFileSync } from "node:child_process";

import { prototypeParams } from "../../contract.ts";
import { flowOfKey } from "../../flow.ts";
import { CommentError } from "../../node/comments-store.ts";
import { localCommentStore } from "../../node/local-comments.ts";
import { galleryOrigin } from "../../node/paths.ts";
import {
  sourceFile,
  targetSummary,
  type CommentAuthor,
  type ReviewComment,
} from "../../shell/comments/model.ts";
import { oneOf, stringFlag, UsageError, type ParsedArgs } from "../args.ts";
import { commentMedia, messageIndex } from "./comment-media.ts";

export const commentActions = [
  "list",
  "show",
  "reply",
  "resolve",
  "reopen",
  "edit",
  "delete",
  "attach",
  "detach",
  "attachments",
] as const;

function headCommit() {
  try {
    return execFileSync("git", ["rev-parse", "HEAD"], {
      encoding: "utf8",
      stdio: ["ignore", "pipe", "ignore"],
    }).trim();
  } catch {
    return null;
  }
}

/**
 * The gallery view that reopens a comment: a flow comment opens its flow on the commented node;
 * a prototype comment its state, theme and width. Either selects the comment.
 */
function galleryUrl(comment: ReviewComment) {
  const url = new URL("/", galleryOrigin);
  const flow = flowOfKey(comment.slug);
  if (flow !== null) {
    url.searchParams.set("flow", flow);
    if (comment.context.state) url.searchParams.set("node", comment.context.state);
    url.searchParams.set("comments", "1");
    url.searchParams.set("comment", comment.id);
    return url.href;
  }
  url.searchParams.set("p", comment.slug);
  url.searchParams.set(prototypeParams.state, comment.context.state);
  if (comment.context.theme) url.searchParams.set(prototypeParams.theme, comment.context.theme);
  url.searchParams.set("w", String(comment.context.viewport.width));
  url.searchParams.set("comments", "1");
  url.searchParams.set("comment", comment.id);
  return url.href;
}

function previewUrl(comment: ReviewComment) {
  const url = new URL(`/${comment.slug}`, galleryOrigin);
  url.searchParams.set(prototypeParams.chrome, "0");
  url.searchParams.set(prototypeParams.state, comment.context.state);
  if (comment.context.theme) url.searchParams.set(prototypeParams.theme, comment.context.theme);
  return url.href;
}

function details(comment: ReviewComment) {
  const { context, target } = comment;
  const lines = [`  commit:   ${context.commit ?? "unknown"}`];
  const file = target.kind === "node" ? null : sourceFile(target.source);
  if (context.commit && file && context.commit !== headCommit()) {
    lines.push(`  inspect:  git show ${context.commit}:${file}`);
  }
  if (target.kind !== "node") {
    if (target.sourceChain.length > 1) lines.push(`  chain:    ${target.sourceChain.join(" < ")}`);
    const r = target.rect;
    lines.push(`  rect:     x ${r.x} y ${r.y} w ${r.width} h ${r.height} (canvas px)`);
    lines.push(`  preview:  ${previewUrl(comment)}`);
  }
  lines.push(`  gallery:  ${galleryUrl(comment)}`);
  for (const [index, message] of comment.thread.entries()) {
    const edited = message.editedAt ? ` · edited ${message.editedAt}` : "";
    lines.push(`${index + 1} ${message.author} · ${message.at}${edited}`, `  ${message.body}`);
    for (const image of message.attachments) {
      lines.push(`  ${image.id}  ${image.name}  ${image.type}  ${image.size} bytes`);
    }
  }
  return lines;
}

export function describe(comment: ReviewComment, full: boolean) {
  const { context, target } = comment;
  const view = [
    `state ${context.state}`,
    context.theme ?? "dark",
    `${context.viewport.width}×${context.viewport.height}`,
  ];
  const lines = [`${comment.id}  [${comment.status}]  ${view.join(" · ")}`];
  if (!full) lines.push(`  note:     ${comment.thread[0]?.body ?? ""}`);
  if (target.kind === "node") {
    lines.push(`  target:   ${targetSummary(target)}`);
  } else {
    lines.push(
      `  source:   ${target.source ?? "(none: rendered outside tagged source)"}`,
      `  target:   ${target.kind} <${target.tag}> ${target.selector}`
    );
    if (target.text) lines.push(`  text:     "${target.text}"`);
  }
  if (full) lines.push(...details(comment));
  else if (comment.thread.length > 1) lines.push(`  replies:  ${comment.thread.length - 1}`);
  return lines.join("\n");
}

function requireNote(args: ParsedArgs, action: string) {
  const note = stringFlag(args, "note");
  if (!note?.trim()) throw new UsageError(`comments ${action} needs --note "<text>"`);
  return note;
}

function list(args: ParsedArgs) {
  const store = localCommentStore();
  const status = oneOf(stringFlag(args, "status"), ["open", "resolved", "all"], "status") ?? "open";
  const slug = stringFlag(args, "slug");
  const comments = (slug ? [slug] : store.keys())
    .flatMap(key => {
      if (slug) return store.read(key);
      try {
        return store.read(key);
      } catch (error) {
        // A prototype or flow deleted or renamed since: its saved threads stay on disk.
        if (!(error instanceof CommentError)) throw error;
        console.warn(`Skipping ${store.displayPath(key)}: ${error.message}`);
        return [];
      }
    })
    .filter(comment => status === "all" || comment.status === status);
  if (args.flags.json === true) {
    console.log(JSON.stringify(comments, null, 2));
  } else if (comments.length === 0) {
    console.log(`No ${status === "all" ? "" : `${status} `}comments.`);
  } else {
    console.log(comments.map(comment => describe(comment, false)).join("\n\n"));
  }
}

/** `proto comments <action> …`. */
export async function comments(args: ParsedArgs) {
  const [action, id, ...rest] = args.positionals;
  if (!action || !(commentActions as readonly string[]).includes(action)) {
    throw new UsageError(`comments needs an action: ${commentActions.join(", ")}`);
  }
  if (action === "list") {
    list(args);
    return;
  }
  if (!id) throw new UsageError(`comments ${action} needs a comment id`);
  if (action === "attach" || action === "detach" || action === "attachments") {
    await commentMedia(action, id, rest, args);
    return;
  }
  const store = localCommentStore();
  const author: CommentAuthor =
    oneOf(stringFlag(args, "author"), ["agent", "reviewer"], "author") ?? "agent";
  let comment: ReviewComment;
  if (action === "show") {
    comment = store.get(id);
  } else if (action === "reply") {
    comment = store.reply(id, author, requireNote(args, action));
  } else if (action === "resolve") {
    comment = store.setStatus(id, "resolved", { author, body: requireNote(args, action) });
  } else if (action === "reopen") {
    const note = stringFlag(args, "note")?.trim();
    comment = store.setStatus(id, "open", note ? { author, body: note } : undefined);
  } else if (action === "edit") {
    const current = store.get(id);
    const index = messageIndex(current, stringFlag(args, "message"), author);
    comment = store.editMessage(id, current.thread[index]!.id, { body: requireNote(args, action) });
  } else {
    comment = store.remove(id);
  }
  if (args.flags.json === true) {
    console.log(JSON.stringify(comment, null, 2));
    return;
  }
  console.log(describe(comment, true));
  if (action !== "show") console.log(`\n${action}: ${store.displayPath(comment.slug)}`);
}
