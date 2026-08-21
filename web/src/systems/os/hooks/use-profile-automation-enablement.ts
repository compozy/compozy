import { useUpdateAutomationJob, useUpdateAutomationTrigger } from "@/systems/automation";
import { parseProfileAutomationIdentity } from "@/systems/profiles";

/** Routes a paused profile automation identity through its owning mutation. */
export function useProfileAutomationEnablement() {
  const updateJob = useUpdateAutomationJob();
  const updateTrigger = useUpdateAutomationTrigger();

  return async (identity: string, profile: string, enabled: boolean) => {
    const automation = parseProfileAutomationIdentity(identity);
    const variables = { id: automation.id, profile, data: { enabled } };
    if (automation.kind === "job") {
      await updateJob.mutateAsync(variables);
      return;
    }
    await updateTrigger.mutateAsync(variables);
  };
}
