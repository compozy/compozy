import { useQueryClient } from "@tanstack/react-query";

import { useUpdateAutomationJob, useUpdateAutomationTrigger } from "@/systems/automation";
import { parseProfileAutomationIdentity } from "@/systems/profiles";
import { invalidateSessionOwnerLabelReads } from "@/systems/session";

/**
 * The cross-system actions the profile lifecycle host needs but cannot import:
 * resuming a paused profile automation through its owning mutation, and
 * rereading the session rows a rename relabels (#1220).
 */
export function useProfileLifecycleHostActions() {
  const queryClient = useQueryClient();
  const updateJob = useUpdateAutomationJob();
  const updateTrigger = useUpdateAutomationTrigger();

  const onSetAutomationEnabled = async (identity: string, profile: string, enabled: boolean) => {
    const automation = parseProfileAutomationIdentity(identity);
    const variables = { id: automation.id, profile, data: { enabled } };
    if (automation.kind === "job") {
      await updateJob.mutateAsync(variables);
      return;
    }
    await updateTrigger.mutateAsync(variables);
  };
  const onProfileRenamed = () => invalidateSessionOwnerLabelReads(queryClient);

  return { onSetAutomationEnabled, onProfileRenamed };
}
