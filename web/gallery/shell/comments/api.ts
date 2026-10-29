/**
 * Client of the dev-server comments API (`node/plugins/comments.ts`). A gallery-only Query cache
 * keeps one list per key plus the open/resolved counts; the server's `proto:comments` HMR event
 * invalidates them when a file changes (gallery, CLI or hand edit). Prototype frames never mount
 * this cache.
 */
import { QueryClient, queryOptions } from "@tanstack/react-query";

import {
  commentsEvent,
  type CommentCounts,
  type CommentStatus,
  type NewComment,
  type ReviewComment,
} from "#gallery/shell/comments/model";

export const commentsBase = "/__comments";

export const galleryQueryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
});

export class CommentsRequestError extends Error {}

async function request<T>(
  path: string,
  init?: { method?: string; body?: unknown; signal?: AbortSignal }
) {
  let response: Response;
  try {
    response = await fetch(`${commentsBase}${path}`, {
      signal: init?.signal,
      method: init?.method ?? "GET",
      headers: init?.body ? { "Content-Type": "application/json" } : undefined,
      body: init?.body ? JSON.stringify(init.body) : undefined,
    });
  } catch (error) {
    if (error instanceof Error && error.name === "AbortError") throw error;
    throw new CommentsRequestError("The gallery dev server is not answering");
  }
  if (!response.ok) {
    const payload = (await response.json().catch(() => ({}))) as { error?: string };
    throw new CommentsRequestError(payload.error ?? `HTTP ${response.status}`);
  }
  return response.json() as Promise<T>;
}

const commentsKey = (slug: string) => ["prototype-comments", slug] as const;
const countsKey = ["prototype-comment-counts"] as const;

export const commentsOptions = (slug: string) =>
  queryOptions({
    queryKey: commentsKey(slug),
    queryFn: ({ signal }) =>
      request<{ comments: ReviewComment[] }>(`/${slug}`, { signal }).then(value => value.comments),
  });

export const countsOptions = queryOptions({
  queryKey: countsKey,
  queryFn: ({ signal }) =>
    request<{ counts: CommentCounts }>("", { signal }).then(value => value.counts),
});

function invalidate(slug: string) {
  return Promise.all([
    galleryQueryClient.invalidateQueries({ queryKey: commentsKey(slug) }),
    galleryQueryClient.invalidateQueries({ queryKey: countsKey }),
  ]);
}

if (import.meta.hot) {
  import.meta.hot.on(commentsEvent, (data: { slug: string }) => void invalidate(data.slug));
}

async function mutate<T>(slug: string, path: string, method: string, body?: unknown) {
  const result = await request<T>(`/${slug}${path}`, { method, body });
  await invalidate(slug);
  return result;
}

export type MessageEdit = {
  body?: string;
  addAttachments?: string[];
  removeAttachments?: string[];
};

export const commentsApi = {
  create: (slug: string, input: NewComment) => mutate<ReviewComment>(slug, "", "POST", input),
  reply: (slug: string, id: string, body: string, attachments: string[] = []) =>
    mutate<ReviewComment>(slug, `/${id}/replies`, "POST", {
      body,
      author: "reviewer",
      attachments,
    }),
  setStatus: (slug: string, id: string, status: CommentStatus) =>
    mutate<ReviewComment>(slug, `/${id}/status`, "POST", { status, author: "reviewer" }),
  edit: (slug: string, id: string, messageId: string, input: MessageEdit) =>
    mutate<ReviewComment>(slug, `/${id}/messages/${messageId}`, "PATCH", input),
  remove: (slug: string, id: string) => mutate<ReviewComment>(slug, `/${id}`, "DELETE"),
};

/** The commit the gallery runs from (injected by `web/gallery/vite.config.ts`). */
export const galleryCommit: string | null = import.meta.env.VITE_PROTO_COMMIT ?? null;
