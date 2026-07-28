import type { OperationRequestBody, OperationResponse } from "@/lib/api-contract";

export type CreateSupportBundleRequest = OperationRequestBody<"createSupportBundle">;
export type SupportBundleOperation = OperationResponse<"createSupportBundle", 202>["operation"];

export interface SupportBundleDownloadResult {
  operation: SupportBundleOperation;
  fileName: string;
}
