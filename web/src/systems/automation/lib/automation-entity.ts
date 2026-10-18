import type { AutomationJob, AutomationTrigger } from "../types";

/** Either daemon entity behind one automation. */
export type AutomationEntity = AutomationJob | AutomationTrigger;

/** Triggers carry an `event`; jobs never do. */
export function isAutomationTrigger(entity: AutomationEntity): entity is AutomationTrigger {
  return "event" in entity;
}
