/**
 * Dev-server API for review comments (`/__comments`), backed by the local comment store, plus a
 * watcher that tells the gallery when a comments file changes (UI, CLI or a hand edit) over Vite's
 * HMR socket as `proto:comments`. Serve-only: it never runs in a build.
 *
 * Routes: `GET /` counts per key · `GET /<key>` comments · `POST /<key>` new comment ·
 * `POST /<key>/<id>/replies` · `POST /<key>/<id>/status` · `PATCH /<key>/<id>/messages/<mid>` ·
 * `DELETE /<key>/<id>` · `POST /<key>/attachments` (multipart `file`) ·
 * `GET|DELETE /<key>/attachments/<attachment id>`.
 */
import type { IncomingMessage, ServerResponse } from "node:http";
import { basename, dirname, resolve } from "node:path";
import type { Plugin } from "vite";

import {
  commentsEvent,
  isAttachmentIds,
  maxAttachmentBytes,
  parseNewComment,
  type CommentAuthor,
} from "../../shell/comments/model.ts";
import { AttachmentError } from "../attachments-store.ts";
import { CommentError, requireMessageBody, type CommentStore } from "../comments-store.ts";
import { localCommentStore } from "../local-comments.ts";
import { isTrustedWrite } from "../trusted-write.ts";

const base = "/__comments";

type Result = { status: number; payload: unknown; bytes?: Uint8Array; type?: string };

const readMethods = new Set(["GET", "HEAD"]);

async function readBytes(request: IncomingMessage) {
  const chunks: Buffer[] = [];
  let size = 0;
  for await (const chunk of request) {
    size += (chunk as Buffer).length;
    // Room for the multipart envelope around a file at the limit.
    if (size > maxAttachmentBytes + 65_536) {
      throw new AttachmentError("attachment_too_large", 413);
    }
    chunks.push(chunk as Buffer);
  }
  return Buffer.concat(chunks);
}

async function readBody(request: IncomingMessage): Promise<Record<string, unknown>> {
  const text = (await readBytes(request)).toString("utf8");
  if (!text) return {};
  if (!request.headers["content-type"]?.startsWith("application/json")) {
    throw new CommentError("Expected an application/json body", 400);
  }
  try {
    const value = JSON.parse(text) as unknown;
    return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : {};
  } catch {
    throw new CommentError("Invalid JSON body", 400);
  }
}

const authorOf = (body: Record<string, unknown>): CommentAuthor =>
  body.author === "agent" ? "agent" : "reviewer";

function ids(body: Record<string, unknown>, key: string): string[] {
  const value = body[key];
  if (value === undefined) return [];
  if (!isAttachmentIds(value)) throw new CommentError("Invalid attachment ids", 400);
  return value;
}

async function attachments(
  store: CommentStore,
  request: IncomingMessage,
  key: string,
  id?: string
): Promise<Result | null> {
  const files = store.attachments(key);
  const method = request.method ?? "GET";
  if (!id && method === "POST") {
    const form = await new Request("http://localhost", {
      method: "POST",
      headers: { "Content-Type": request.headers["content-type"] ?? "" },
      body: new Uint8Array(await readBytes(request)),
    }).formData();
    const file = form.get("file");
    if (!(file instanceof File)) throw new CommentError("Missing file", 400);
    const image = files.upload(file.name, new Uint8Array(await file.arrayBuffer()));
    return { status: 201, payload: { ...image, url: `${base}/${key}/attachments/${image.id}` } };
  }
  if (id && method === "GET") {
    const { image, bytes } = files.read(id, store.read(key));
    return { status: 200, payload: null, bytes, type: image.type };
  }
  if (id && method === "DELETE") {
    files.discard(id);
    return { status: 204, payload: null };
  }
  return null;
}

async function route(store: CommentStore, request: IncomingMessage, segments: string[]) {
  const [key, id, action, messageId] = segments;
  const method = request.method ?? "GET";
  if (!key) {
    if (method === "GET") return { status: 200, payload: { counts: store.counts() } };
  } else if (id === "attachments") {
    const result = await attachments(store, request, key, action);
    if (result) return result;
  } else if (!id) {
    if (method === "GET") return { status: 200, payload: { slug: key, comments: store.read(key) } };
    if (method === "POST") {
      const input = parseNewComment(await readBody(request));
      if (!input) throw new CommentError("Invalid comment", 400);
      return { status: 201, payload: store.add(key, input) };
    }
  } else if (!id.startsWith(`${key}-`)) {
    throw new CommentError(`Comment "${id}" is not from "${key}"`, 400);
  } else if (!action && method === "DELETE") {
    return { status: 200, payload: store.remove(id) };
  } else if (action === "messages" && messageId && segments.length === 4 && method === "PATCH") {
    const body = await readBody(request);
    return {
      status: 200,
      payload: store.editMessage(id, messageId, {
        ...(body.body === undefined ? {} : { body: requireMessageBody(body.body) }),
        addAttachments: ids(body, "addAttachments"),
        removeAttachments: ids(body, "removeAttachments"),
      }),
    };
  } else if (action === "replies" && method === "POST") {
    const body = await readBody(request);
    const reply = requireMessageBody(body.body);
    return {
      status: 200,
      payload: store.reply(id, authorOf(body), reply, ids(body, "attachments")),
    };
  } else if (action === "status" && method === "POST") {
    const body = await readBody(request);
    if (body.status !== "open" && body.status !== "resolved") {
      throw new CommentError("Invalid status", 400);
    }
    const note =
      body.note === undefined
        ? undefined
        : { author: authorOf(body), body: requireMessageBody(body.note) };
    return { status: 200, payload: store.setStatus(id, body.status, note) };
  }
  return { status: 405, payload: { error: `${method} not allowed` } };
}

function send(response: ServerResponse, result: Result) {
  response.statusCode = result.status;
  if (result.bytes && result.type) {
    response.setHeader("Content-Type", result.type);
    response.setHeader("X-Content-Type-Options", "nosniff");
    response.setHeader("Cache-Control", "private, max-age=31536000, immutable");
    response.end(result.bytes);
    return;
  }
  if (result.status === 204) {
    response.end();
    return;
  }
  response.setHeader("Content-Type", "application/json; charset=utf-8");
  response.end(JSON.stringify(result.payload));
}

export function commentsApi(): Plugin {
  return {
    name: "compozy:prototype-comments",
    apply: "serve",
    configureServer(server) {
      const store = localCommentStore();
      const directory = resolve(store.directory);
      // Only `comments/<key>/comments.json` changes are review changes.
      const notify = (path: string) => {
        if (basename(path) !== "comments.json") return;
        const folder = dirname(resolve(path));
        if (dirname(folder) !== directory) return;
        server.ws.send({ type: "custom", event: commentsEvent, data: { slug: basename(folder) } });
      };
      server.watcher.add(directory);
      server.watcher.on("add", notify);
      server.watcher.on("change", notify);
      server.watcher.on("unlink", notify);

      server.middlewares.use(base, (request, response) => {
        const path = new URL(request.url ?? "/", "http://localhost").pathname;
        const segments = path.split("/").filter(Boolean).map(decodeURIComponent);
        if (!readMethods.has(request.method ?? "GET") && !isTrustedWrite(request.headers)) {
          send(response, { status: 403, payload: { error: "Cross-origin writes are refused" } });
          return;
        }
        route(store, request, segments).then(
          result => send(response, result),
          (error: unknown) => {
            const status =
              error instanceof CommentError || error instanceof AttachmentError
                ? error.status
                : 500;
            send(response, {
              status,
              payload: { error: error instanceof Error ? error.message : String(error) },
            });
          }
        );
      });
    },
  };
}
