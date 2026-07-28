type DataSurfaceState = "loading" | "error" | "empty" | "ready";

function resolveDataSurfaceState({
  isLoading = false,
  error = null,
  isEmpty = false,
}: {
  isLoading?: boolean;
  error?: Error | null;
  isEmpty?: boolean;
}): DataSurfaceState {
  if (isLoading) return "loading";
  if (error) return "error";
  if (isEmpty) return "empty";
  return "ready";
}

export { resolveDataSurfaceState };
export type { DataSurfaceState };
