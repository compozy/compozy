import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type AutomationJobsListResponse = OperationResponse<"listAutomationJobs", 200>;
export type AutomationTriggersListResponse = OperationResponse<"listAutomationTriggers", 200>;
export type AutomationSuggestionsListResponse = OperationResponse<"listAutomationSuggestions", 200>;
export type AutomationSuggestionAcceptanceResponse = OperationResponse<
  "acceptAutomationSuggestion",
  200
>;
export type AutomationSuggestionDismissalResponse = OperationResponse<
  "dismissAutomationSuggestion",
  200
>;
export type AutomationJob = AutomationJobsListResponse["jobs"][number];
export type AutomationTrigger = AutomationTriggersListResponse["triggers"][number];
export type AutomationSuggestion = AutomationSuggestionsListResponse["suggestions"][number];
export type AutomationRun = OperationResponse<"getAutomationRun", 200>["run"];
export type AutomationSchedulerState = NonNullable<AutomationJob["scheduler"]>;

export type AutomationJobListFilter = OperationQuery<"listAutomationJobs">;
export type AutomationTriggerListFilter = OperationQuery<"listAutomationTriggers">;
export type AutomationJobStableFilter = Omit<AutomationJobListFilter, "cursor">;
export type AutomationTriggerStableFilter = Omit<AutomationTriggerListFilter, "cursor">;
export type AutomationRunListFilter = OperationQuery<"listAutomationRuns">;
export type AutomationRunHistoryFilter = OperationQuery<"listAutomationJobRuns">;
export type AutomationSuggestionStatus = NonNullable<
  OperationQuery<"listAutomationSuggestions">["status"]
>;

export type CreateAutomationJobRequest = OperationRequestBody<"createAutomationJob">;
export type UpdateAutomationJobRequest = OperationRequestBody<"updateAutomationJob">;
export type CreateAutomationTriggerRequest = OperationRequestBody<"createAutomationTrigger">;
export type UpdateAutomationTriggerRequest = OperationRequestBody<"updateAutomationTrigger">;

export type AutomationScope = AutomationJob["scope"];
export type AutomationSource = AutomationJob["source"];
export type AutomationRunStatus = AutomationRun["status"];
export type AutomationSchedule = NonNullable<AutomationJob["schedule"]>;
export type AutomationScheduleMode = AutomationSchedule["mode"];
export type AutomationCatchUpPolicy = NonNullable<AutomationSchedule["catch_up_policy"]>;
export type AutomationRetry = AutomationJob["retry"];
export type AutomationFireLimit = AutomationJob["fire_limit"];
export type AutomationTriggerFilter = NonNullable<AutomationTrigger["filter"]>;

export type AutomationKind = "jobs" | "triggers";
export type AutomationScopeFilter = "all" | AutomationScope;
