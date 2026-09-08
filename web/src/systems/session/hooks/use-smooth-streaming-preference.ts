import { useSelector } from "@xstate/store-react";

import { sessionStreamingPreferenceStore } from "../stores/session-streaming-preference-store";

/** The operator's client-local "Smooth streaming" preference and its setter (S10). */
export function useSmoothStreamingPreference(): {
  enabled: boolean;
  setEnabled: (enabled: boolean) => void;
} {
  const enabled = useSelector(
    sessionStreamingPreferenceStore,
    snapshot => snapshot.context.smoothStreaming
  );
  return {
    enabled,
    setEnabled: (next: boolean) =>
      sessionStreamingPreferenceStore.trigger.smoothStreamingSet({ enabled: next }),
  };
}
