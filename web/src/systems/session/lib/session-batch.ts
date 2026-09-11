export interface SessionBatchResult {
  id: string;
  status: "pending" | "running" | "done" | "failed";
  error?: string;
}
