/**
 * Image uploads of one message being written: each file uploads as soon as it is added (paste,
 * drop or pick) and stays pending on the server until the message is saved. Removed, cancelled or
 * late uploads are discarded, even after unmount.
 */
import { useEffect, useRef, useState } from "react";

import { commentsBase } from "#gallery/shell/comments/api";
import {
  maxAttachmentBytes,
  maxAttachments,
  type CommentAttachment,
} from "#gallery/shell/comments/model";

export type Upload = {
  key: string;
  file: File;
  preview: string;
  progress: number;
  image?: CommentAttachment;
  error?: string;
};

const tooLarge = "Images can be up to 5 MB.";

const endpoint = (slug: string) => `${commentsBase}/${slug}/attachments`;

function discard(slug: string, id: string) {
  void fetch(`${endpoint(slug)}/${id}`, { method: "DELETE" }).catch(() => undefined);
}

export function useAttachments(slug: string) {
  const [items, setItems] = useState<Upload[]>([]);
  const active = useRef(false);
  const pending = useRef(new Map<string, Upload>());

  useEffect(() => {
    active.current = true;
    const uploads = pending.current;
    return () => {
      active.current = false;
      for (const item of uploads.values()) {
        URL.revokeObjectURL(item.preview);
        if (item.image) discard(slug, item.image.id);
      }
      uploads.clear();
    };
  }, [slug]);

  function publish() {
    if (active.current) setItems([...pending.current.values()].map(item => ({ ...item })));
  }

  function upload(previous: Upload) {
    const item: Upload = { ...previous, error: undefined, progress: 0 };
    pending.current.set(item.key, item);
    const form = new FormData();
    form.set("file", item.file);
    const xhr = new XMLHttpRequest();
    xhr.open("POST", endpoint(slug));
    xhr.upload.onprogress = event => {
      if (!event.lengthComputable) return;
      item.progress = Math.round((event.loaded / event.total) * 100);
      publish();
    };
    xhr.onerror = () => {
      item.error = "Upload failed. Try again.";
      publish();
    };
    xhr.onload = () => {
      if (xhr.status < 200 || xhr.status >= 300) {
        item.error = xhr.status === 413 ? tooLarge : "Use a valid PNG, JPEG, WebP or GIF image.";
        publish();
        return;
      }
      const image = JSON.parse(xhr.responseText) as CommentAttachment;
      if (!active.current || !pending.current.has(item.key)) {
        discard(slug, image.id);
        return;
      }
      item.image = image;
      item.progress = 100;
      publish();
    };
    xhr.send(form);
    publish();
  }

  function add(files: readonly File[], capacity = maxAttachments) {
    for (const file of files.slice(0, Math.max(0, capacity - pending.current.size))) {
      const item: Upload = {
        key: crypto.randomUUID(),
        file,
        preview: URL.createObjectURL(file),
        progress: 0,
      };
      pending.current.set(item.key, item);
      if (file.size > maxAttachmentBytes) {
        item.error = tooLarge;
        publish();
      } else {
        upload(item);
      }
    }
  }

  function remove(key: string) {
    const item = pending.current.get(key);
    if (!item) return;
    pending.current.delete(key);
    URL.revokeObjectURL(item.preview);
    if (item.image) discard(slug, item.image.id);
    publish();
  }

  /** The message was saved: its uploads are no longer pending. */
  function committed() {
    for (const item of pending.current.values()) URL.revokeObjectURL(item.preview);
    pending.current.clear();
    publish();
  }

  return {
    items,
    add,
    remove,
    retry: upload,
    committed,
    cancel: () => {
      // Deleting the visited entry while iterating a Map is safe.
      for (const key of pending.current.keys()) remove(key);
    },
    ids: items.flatMap(item => (item.image ? [item.image.id] : [])),
    ready: items.every(item => item.image !== undefined),
  };
}

export type Uploads = ReturnType<typeof useAttachments>;
