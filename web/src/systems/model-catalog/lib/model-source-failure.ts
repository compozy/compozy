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
