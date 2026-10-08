// The automation page view-models live in dedicated files (each under the
// production-source line cap); this barrel is their one import path.
export { useAutomationsPage } from "./use-automations-page";
export { useAutomationJobDetailPage } from "./use-automation-job-detail-page";
export { useAutomationTriggerDetailPage } from "./use-automation-trigger-detail-page";
export type { AutomationCreateSeed } from "./use-automation-page-base";
