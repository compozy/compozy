import type { AutomationTrigger } from "../types";

export type TriggerFilterEntry = readonly [path: string, value: string];

/** Normalized exact-match entries shared by trigger prose and Inspect projections. */
export function triggerFilterEntries(
  trigger: Pick<AutomationTrigger, "filter">
): TriggerFilterEntry[] {
  return Object.entries(trigger.filter ?? {}).flatMap(([rawPath, rawValue]) => {
    const path = rawPath.trim();
    const value = rawValue.trim();
    return path === "" || value === "" ? [] : [[path, value] as const];
  });
}
