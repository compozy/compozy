import { ModelCatalogApiError } from "../adapters/model-catalog-api";
import type { ProviderModelSourceStatus } from "../types";

export function modelSourceFailureMessage(source: ProviderModelSourceStatus): string | null {
  if (source.refresh_state !== "failed") return null;
  switch (source.failure_reason) {
    case "auth_required":
      return "Sign in to update the model list.";
    case "timeout":
      return "The model list took too long to update. Try refreshing again.";
    default:
      return "Could not update the model list. Try refreshing again.";
  }
}

export function modelRefreshErrorMessage(
  error: unknown,
  displayedSources: readonly ProviderModelSourceStatus[]
): string | null {
  if (!error) return null;
  const displayedFailure =
    error instanceof ModelCatalogApiError &&
    error.sources.some(
      source =>
        source.refresh_state === "failed" &&
        displayedSources.some(
          displayed =>
            displayed.source_id === source.source_id &&
            displayed.provider_id === source.provider_id &&
            displayed.refresh_state === "failed" &&
            displayed.failure_reason === source.failure_reason
        )
    );
  return displayedFailure ? null : "Could not refresh the model list. Try again.";
}
