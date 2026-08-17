import { useQuery } from "@tanstack/react-query";

import { useDocumentVisible } from "@/hooks/use-document-visible";

import { sessionAttentionSummaryOptions, type SessionCatalogStreamStatus } from "@/systems/session";

import type { OsAttentionSummary } from "../lib/attention-model";

const ATTENTION_SUMMARY_REFETCH_INTERVAL_MS = 5_000;

export interface OsAttentionSummaryModel {
  summary: OsAttentionSummary | null;
  stale: boolean;
  loading: boolean;
}

/**
 * The daemon's exact cross-workspace counts. Every session number the operator
 * sees — bell badge and tab title — comes from here rather than from counting a
 * cursor-paginated row page, so the count stays true past any page size.
 *
 * The count refetches on catalog wakes like every other read model; the poll is
 * the fallback for a client whose stream has gone quiet, and it stops while the
 * document is hidden.
 */
export function useAttentionSummary(
  streamStatus: SessionCatalogStreamStatus
): OsAttentionSummaryModel {
  const documentVisible = useDocumentVisible();
  const query = useQuery({
    ...sessionAttentionSummaryOptions(),
    refetchInterval: documentVisible ? ATTENTION_SUMMARY_REFETCH_INTERVAL_MS : false,
  });
  const payload = query.data;
  return {
    summary:
      payload === undefined ? null : { needsYou: payload.needs_you, finished: payload.finished },
    stale: streamStatus !== "live" || query.isError || payload === undefined,
    loading: query.isLoading,
  };
}
