/** `proto comments attach|detach|attachments`: image attachments from the command line. */
import { mkdirSync, readFileSync, rmSync, statSync, writeFileSync } from "node:fs";
import { basename, resolve } from "node:path";

import { localCommentStore } from "../../node/local-comments.ts";
import {
  maxAttachmentBytes,
  maxAttachments,
  type CommentAttachment,
  type CommentAuthor,
  type ReviewComment,
} from "../../shell/comments/model.ts";
import { stringFlag, UsageError, type ParsedArgs } from "../args.ts";

/**
 * The 0-based message `--message <n>` (1-based) names, else the latest one by `author`; either
 * way the message must be `author`'s, so an agent never edits a reviewer's words.
 */
export function messageIndex(
  comment: ReviewComment,
  position: string | undefined,
  author: CommentAuthor
) {
  const index =
    position === undefined
      ? comment.thread.map(message => message.author).lastIndexOf(author)
      : Number(position) - 1;
  if (!Number.isInteger(index) || index < 0 || !comment.thread[index]) {
    throw new UsageError(`No matching message in ${comment.id}; pass --message <number>`);
  }
  if (comment.thread[index].author !== author) {
    throw new UsageError(`Message ${index + 1} of ${comment.id} is not by ${author}`);
  }
  return index;
}

function download(id: string, args: ParsedArgs) {
  const store = localCommentStore();
  const comment = store.get(id);
  const out = stringFlag(args, "out");
  if (!out) throw new UsageError("comments attachments needs --out <directory>");
  const directory = resolve(out);
  mkdirSync(directory, { recursive: true });
  const files = store.attachments(comment.slug);
  // Files this run wrote are removed if a later one fails; files already there are untouched.
  const saved: { id: string; message: number; path: string }[] = [];
  try {
    for (const [index, message] of comment.thread.entries()) {
      for (const image of message.attachments) {
        const path = resolve(directory, `${image.id}-${basename(image.name) || "image"}`);
        writeFileSync(path, files.read(image.id, [comment]).bytes, { flag: "wx" });
        saved.push({ id: image.id, message: index + 1, path });
      }
    }
  } catch (error) {
    for (const file of saved) rmSync(file.path, { force: true });
    throw error;
  }
  if (args.flags.json === true) console.log(JSON.stringify(saved, null, 2));
  else for (const file of saved) console.log(file.path);
}

export async function commentMedia(
  action: "attach" | "detach" | "attachments",
  id: string,
  rest: readonly string[],
  args: ParsedArgs
) {
  if (action === "attachments") {
    download(id, args);
    return;
  }
  const store = localCommentStore();
  const comment = store.get(id);
  const files = store.attachments(comment.slug);
  if (action === "detach") {
    const attachment = rest[0];
    if (!attachment) throw new UsageError("comments detach needs <attachment id>");
    const message = comment.thread.find(item =>
      item.attachments.some(image => image.id === attachment)
    );
    if (!message) throw new UsageError(`Attachment ${attachment} is not in ${id}`);
    store.editMessage(id, message.id, { removeAttachments: [attachment] });
    console.log(`detach: ${id}`);
    return;
  }
  if (rest.length === 0) throw new UsageError("comments attach needs at least one image file");
  const message = comment.thread[messageIndex(comment, stringFlag(args, "message"), "agent")]!;
  if (message.attachments.length + rest.length > maxAttachments) {
    throw new UsageError(`A message takes up to ${maxAttachments} images`);
  }
  const uploaded: CommentAttachment[] = [];
  try {
    for (const path of rest) {
      if (statSync(path).size > maxAttachmentBytes) {
        throw new UsageError(`${path} is larger than ${maxAttachmentBytes / 1024 / 1024} MB`);
      }
      uploaded.push(files.upload(basename(path), readFileSync(path)));
    }
    store.editMessage(id, message.id, { addAttachments: uploaded.map(image => image.id) });
  } catch (error) {
    // Uploads that were never claimed stay pending; drop them now instead of in 24 hours.
    for (const image of uploaded) files.discard(image.id);
    throw error;
  }
  console.log(`attach: ${id}`);
}
