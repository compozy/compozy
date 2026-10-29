/**
 * Reads and writes review comments, one canonical `comments.json` per key under
 * `web/gallery/comments/<key>/`. The dev-server API (`comments-plugin.ts`) and the agent CLI
 * (`proto comments`) share it, so the files stay the single source of truth with or without the
 * dev server. Keys are prototype slugs or `flow-<id>` for flows.
 */
import {
  existsSync,
  mkdirSync,
  readdirSync,
  readFileSync,
  renameSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { dirname, join, relative } from "node:path";

import {
  canonicalComment,
  canonicalFile,
  createComment,
  isMessageBody,
  withEditedMessage,
  withReply,
  withStatus,
  type CommentAuthor,
  type CommentCounts,
  type CommentFile,
  type CommentStatus,
  type NewComment,
  type ReviewComment,
} from "../shell/comments/model.ts";
import { AttachmentStore } from "./attachments-store.ts";

const fileName = "comments.json";

export class CommentError extends Error {
  constructor(
    message: string,
    readonly status: 400 | 404
  ) {
    super(message);
  }
}

export function requireMessageBody(value: unknown): string {
  if (!isMessageBody(value)) throw new CommentError("Invalid message body", 400);
  return value.trim();
}

/** Ids are `<key>-<n>`; the key is everything before the last number. */
export function keyOfId(id: string) {
  const match = /^(.+)-\d+$/.exec(id);
  if (!match?.[1]) throw new CommentError(`Invalid comment id "${id}"`, 400);
  return match[1];
}

export type MessageEdit = {
  body?: string;
  addAttachments?: string[];
  removeAttachments?: string[];
};

type StoreOptions = {
  /** `web/gallery/comments/`. */
  directory: string;
  /** Throws a 404 `CommentError` unless the key names a prototype (or flow) that exists. */
  assertKey: (key: string) => void;
  /** Base for repository-relative paths in messages. */
  repoRoot?: string;
  now?: () => Date;
};

export class CommentStore {
  constructor(private readonly options: StoreOptions) {}

  get directory() {
    return this.options.directory;
  }

  private now() {
    return (this.options.now?.() ?? new Date()).toISOString();
  }

  path(key: string) {
    return join(this.options.directory, key, fileName);
  }

  /** Repository-relative path of a key's file, for messages. */
  displayPath(key: string) {
    const path = this.path(key);
    return this.options.repoRoot ? relative(this.options.repoRoot, path) : path;
  }

  attachments(key: string) {
    this.options.assertKey(key);
    return new AttachmentStore(join(this.options.directory, key));
  }

  /** Every comment of a key, with served attachment URLs. */
  read(key: string): ReviewComment[] {
    this.options.assertKey(key);
    const path = this.path(key);
    if (!existsSync(path)) return [];
    const file = JSON.parse(readFileSync(path, "utf8")) as CommentFile;
    return file.comments.map(comment => {
      const normalized = canonicalComment(comment);
      return {
        ...normalized,
        thread: normalized.thread.map(item => ({
          ...item,
          attachments: item.attachments.map(image => ({
            ...image,
            url: `/__comments/${key}/attachments/${image.id}`,
          })),
        })),
      };
    });
  }

  /** Writes the canonical file atomically; removes it when no comment is left. */
  write(key: string, comments: readonly ReviewComment[]) {
    this.options.assertKey(key);
    const path = this.path(key);
    if (comments.length === 0) {
      rmSync(path, { force: true });
      return;
    }
    mkdirSync(dirname(path), { recursive: true });
    const temporary = `${path}.tmp`;
    writeFileSync(temporary, `${JSON.stringify(canonicalFile(key, comments), null, 2)}\n`);
    renameSync(temporary, path);
  }

  /** Keys that have a comments file, alphabetically. */
  keys() {
    const { directory } = this.options;
    if (!existsSync(directory)) return [];
    return readdirSync(directory, { withFileTypes: true })
      .flatMap(entry =>
        entry.isDirectory() && existsSync(join(directory, entry.name, fileName)) ? [entry.name] : []
      )
      .sort();
  }

  /** Open and resolved counts per key; files of keys that no longer exist are skipped. */
  counts(): CommentCounts {
    const counts: CommentCounts = {};
    for (const key of this.keys()) {
      let comments: ReviewComment[];
      try {
        comments = this.read(key);
      } catch (error) {
        if (error instanceof CommentError) continue;
        throw error;
      }
      const open = comments.filter(comment => comment.status === "open").length;
      counts[key] = { open, resolved: comments.length - open };
    }
    return counts;
  }

  get(id: string) {
    const comment = this.read(keyOfId(id)).find(item => item.id === id);
    if (!comment) throw new CommentError(`Comment "${id}" not found`, 404);
    return comment;
  }

  private update(id: string, change: (comment: ReviewComment) => ReviewComment | null) {
    const key = keyOfId(id);
    const comments = this.read(key);
    const current = comments.find(comment => comment.id === id);
    if (!current) throw new CommentError(`Comment "${id}" not found`, 404);
    const next = change(current);
    const rest = comments.filter(comment => comment.id !== id);
    this.write(key, next ? [...rest, next] : rest);
    return next ?? current;
  }

  add(key: string, input: NewComment) {
    requireMessageBody(input.body);
    const comments = this.read(key);
    const comment = createComment(key, comments, input, this.now());
    this.attachments(key).commit([], input.attachments ?? [], [], images => {
      comment.thread[0] = { ...comment.thread[0]!, attachments: images };
      this.write(key, [...comments, comment]);
    });
    return comment;
  }

  reply(id: string, author: CommentAuthor, body: string, attachments: string[] = []) {
    requireMessageBody(body);
    let result: ReviewComment | undefined;
    this.attachments(keyOfId(id)).commit([], attachments, [], images => {
      result = this.update(id, comment => {
        const next = withReply(comment, author, body, this.now());
        next.thread[next.thread.length - 1] = { ...next.thread.at(-1)!, attachments: images };
        return next;
      });
    });
    return result!;
  }

  /** Changes the status, optionally adding a note in the same write (resolve with a note). */
  setStatus(id: string, status: CommentStatus, note?: { author: CommentAuthor; body: string }) {
    if (note) requireMessageBody(note.body);
    return this.update(id, comment => {
      const at = this.now();
      const replied = note ? withReply(comment, note.author, note.body, at) : comment;
      return withStatus(replied, status, at);
    });
  }

  remove(id: string) {
    const comment = this.update(id, () => null);
    this.attachments(comment.slug).remove(comment.thread.flatMap(item => item.attachments));
    return comment;
  }

  editMessage(id: string, messageId: string, input: MessageEdit) {
    if (
      input.body === undefined &&
      !input.addAttachments?.length &&
      !input.removeAttachments?.length
    ) {
      throw new CommentError("Empty message update", 400);
    }
    const current = this.get(id);
    const message = current.thread.find(item => item.id === messageId);
    if (!message) throw new CommentError("Message not found", 404);
    const body = requireMessageBody(input.body ?? message.body);
    let result: ReviewComment | undefined;
    this.attachments(current.slug).commit(
      message.attachments,
      input.addAttachments ?? [],
      input.removeAttachments ?? [],
      images => {
        result = this.update(id, comment => {
          const next = withEditedMessage(comment, messageId, body, this.now());
          return {
            ...next,
            thread: next.thread.map(item =>
              item.id === messageId ? { ...item, attachments: images } : item
            ),
          };
        });
      }
    );
    return result!;
  }
}
