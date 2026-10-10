import type { SessionReplyOutcome } from "@/systems/session/lib/session-message-payload";

/** The outcome word on the reply card (S2); the sent card says "Replied" for completed (S3). */
export const SESSION_REPLY_OUTCOME_WORD: Record<SessionReplyOutcome, string> = {
  completed: "Completed",
  failed: "Failed",
  canceled: "Canceled",
  dropped: "Dropped",
  unknown: "Unknown",
};
