/**
 * Image attachments of one comment key, in `web/gallery/comments/<key>/attachments/`. Uploads land
 * in `pending/` with a metadata file and expire after 24 hours unless a message claims them;
 * `commit` moves claimed files into place before the comments file (the commit point) is written.
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
import { join } from "node:path";

import {
  attachmentIdPattern,
  maxAttachments,
  type CommentAttachment,
  type ReviewComment,
} from "../shell/comments/model.ts";
import { ImageError, inspectImage } from "./image.ts";

const extensions: Record<string, string> = {
  "image/png": "png",
  "image/jpeg": "jpg",
  "image/gif": "gif",
  "image/webp": "webp",
};

export const pendingLifetime = 24 * 60 * 60 * 1000;

type Pending = CommentAttachment & { createdAt: number };

export class AttachmentError extends Error {
  constructor(
    message: string,
    readonly status: 400 | 404 | 409 | 413 | 415
  ) {
    super(message);
  }
}

export class AttachmentStore {
  readonly directory: string;
  readonly pending: string;

  /** `keyDirectory` is `comments/<key>/`; callers validate the key first. */
  constructor(
    keyDirectory: string,
    private readonly now: () => number = Date.now
  ) {
    this.directory = join(keyDirectory, "attachments");
    this.pending = join(this.directory, "pending");
  }

  private path(image: CommentAttachment, pending = false) {
    const extension = extensions[image.type];
    if (!attachmentIdPattern.test(image.id) || !extension) {
      throw new AttachmentError("invalid_input", 400);
    }
    return join(pending ? this.pending : this.directory, `${image.id}.${extension}`);
  }

  private metadata(id: string): Pending {
    const file = join(this.pending, `${id}.json`);
    if (!attachmentIdPattern.test(id) || !existsSync(file)) {
      throw new AttachmentError("not_found", 404);
    }
    return JSON.parse(readFileSync(file, "utf8")) as Pending;
  }

  private expired(image: Pending) {
    return image.createdAt < this.now() - pendingLifetime;
  }

  /** Drops pending uploads older than 24 hours. */
  expire() {
    if (!existsSync(this.pending)) return;
    for (const file of readdirSync(this.pending).filter(name => name.endsWith(".json"))) {
      const image = this.metadata(file.slice(0, -".json".length));
      if (this.expired(image)) this.discard(image.id);
    }
  }

  /** Validates the bytes and stores them as a pending upload. */
  upload(name: string, bytes: Uint8Array): CommentAttachment {
    let info: ReturnType<typeof inspectImage>;
    try {
      info = inspectImage(bytes);
    } catch (error) {
      const code = error instanceof ImageError ? error.code : "unsupported_media_type";
      throw new AttachmentError(code, code === "attachment_too_large" ? 413 : 415);
    }
    mkdirSync(this.pending, { recursive: true });
    this.expire();
    const image: CommentAttachment = { id: crypto.randomUUID(), name: name.slice(0, 200), ...info };
    writeFileSync(this.path(image, true), bytes, { flag: "wx" });
    try {
      writeFileSync(
        join(this.pending, `${image.id}.json`),
        JSON.stringify({ ...image, createdAt: this.now() }),
        { flag: "wx" }
      );
    } catch (error) {
      rmSync(this.path(image, true), { force: true });
      throw error;
    }
    return image;
  }

  discard(id: string) {
    const image = this.metadata(id);
    rmSync(this.path(image, true), { force: true });
    rmSync(join(this.pending, `${id}.json`), { force: true });
  }

  /** Bytes of an attached image, or of a pending upload that has not expired. */
  read(id: string, comments: readonly ReviewComment[]) {
    let attached: CommentAttachment | undefined;
    for (const comment of comments) {
      for (const item of comment.thread) {
        attached ??= item.attachments.find(image => image.id === id);
      }
    }
    const image = attached ?? this.metadata(id);
    if (!attached && this.expired(image as Pending)) throw new AttachmentError("not_found", 404);
    const path = this.path(image, !attached);
    if (!existsSync(path)) throw new AttachmentError("not_found", 404);
    return { image, bytes: readFileSync(path) };
  }

  /**
   * Claims pending uploads for one message and drops removed ones: validates everything first,
   * moves files with rollback, then calls `persist` (the commit point) with the final list.
   */
  commit(
    current: readonly CommentAttachment[],
    add: readonly string[],
    remove: readonly string[],
    persist: (images: CommentAttachment[]) => void
  ) {
    if (new Set(add).size !== add.length || new Set(remove).size !== remove.length) {
      throw new AttachmentError("invalid_input", 400);
    }
    const currentIds = new Set(current.map(image => image.id));
    const removedIds = new Set(remove);
    if (remove.some(id => !currentIds.has(id))) throw new AttachmentError("invalid_input", 400);
    const kept = current.filter(image => !removedIds.has(image.id));
    if (kept.length + add.length > maxAttachments) {
      throw new AttachmentError("attachment_limit", 409);
    }
    const claimed = add.map(id => {
      let image: Pending;
      try {
        image = this.metadata(id);
      } catch {
        throw new AttachmentError("invalid_input", 400);
      }
      if (
        this.expired(image) ||
        !existsSync(this.path(image, true)) ||
        existsSync(this.path(image))
      ) {
        throw new AttachmentError("invalid_input", 400);
      }
      return image;
    });
    const moved: Pending[] = [];
    try {
      for (const image of claimed) {
        renameSync(this.path(image, true), this.path(image));
        moved.push(image);
      }
      persist([...kept, ...claimed.map(({ createdAt: _createdAt, ...image }) => image)]);
    } catch (error) {
      for (const image of moved.reverse()) renameSync(this.path(image), this.path(image, true));
      throw error;
    }
    for (const image of claimed) rmSync(join(this.pending, `${image.id}.json`), { force: true });
    for (const image of current) {
      if (removedIds.has(image.id)) rmSync(this.path(image), { force: true });
    }
  }

  remove(images: readonly CommentAttachment[]) {
    for (const image of images) rmSync(this.path(image), { force: true });
  }
}
