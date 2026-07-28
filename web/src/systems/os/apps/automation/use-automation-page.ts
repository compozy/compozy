// The automation page view-models live in dedicated files (each under the
// production-source line cap); this barrel preserves the original import path.
export { useAutomationJobsPage } from "./use-automation-jobs-page";
export { useAutomationTriggersPage } from "./use-automation-triggers-page";
export { useAutomationJobDetailPage } from "./use-automation-job-detail-page";
export { useAutomationTriggerDetailPage } from "./use-automation-trigger-detail-page";
export {
  automationListLoopFilter,
  validateJobsSearch,
  validateTriggersSearch,
} from "./use-automation-page-base";
export type { AutomationCreateSeed, AutomationRouteSearch } from "./use-automation-page-base";
