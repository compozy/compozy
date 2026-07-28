import type { OperationResponse } from "@/lib/api-contract";

export type StatusPayload = OperationResponse<"getStatus", 200>;
export type DoctorPayload = OperationResponse<"getDoctor", 200>;
export type HealthPayload = StatusPayload["health"];
export type MemoryHealthPayload = StatusPayload["memory"];
export type DaemonStatusPayload = StatusPayload["daemon"];
