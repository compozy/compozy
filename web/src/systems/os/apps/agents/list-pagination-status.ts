export type ListPaginationStatus = "loading" | "available" | undefined;

/** Load-more footer state: busy while fetching, offered while more pages remain. */
export function listPaginationStatus(
  isLoadingMore: boolean,
  hasMore: boolean
): ListPaginationStatus {
  if (isLoadingMore) return "loading";
  return hasMore ? "available" : undefined;
}
