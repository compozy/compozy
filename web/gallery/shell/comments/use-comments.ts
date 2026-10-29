import { useQuery } from "@tanstack/react-query";

import { commentsOptions, countsOptions } from "#gallery/shell/comments/api";
import type { CommentCounts, ReviewComment } from "#gallery/shell/comments/model";

const noComments: ReviewComment[] = [];

/** Comments of a prototype, plus the API error when the dev server cannot answer. */
export function useComments(slug: string | null) {
  const query = useQuery({ ...commentsOptions(slug ?? ""), enabled: slug !== null });
  return { comments: query.data ?? noComments, error: query.error?.message ?? null };
}

const noCounts: CommentCounts = {};

export function useCommentCounts() {
  return useQuery(countsOptions).data ?? noCounts;
}
