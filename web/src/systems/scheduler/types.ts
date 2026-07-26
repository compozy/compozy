import type { OperationQuery, OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type SchedulerStatus = OperationResponse<"getScheduler", 200>["scheduler"];
export type SchedulerPauseRequest = OperationRequestBody<"pauseScheduler">;
export type SchedulerResumeRequest = OperationRequestBody<"resumeScheduler">;
export type SchedulerDrainRequest = OperationRequestBody<"drainScheduler">;
export type SchedulerDrainResult = OperationResponse<"drainScheduler", 200>;
export type SchedulerBacklogQuery = OperationQuery<"getSchedulerBacklog">;
export type SchedulerBacklog = OperationResponse<"getSchedulerBacklog", 200>["backlog"];
export type SchedulerBacklogRun = SchedulerBacklog["runs"][number];
