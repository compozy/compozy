import { toast } from "sonner";

export type UserFeedbackTone = "error" | "info" | "success" | "warning";

export interface UserFeedbackAction {
  label: string;
  onClick: () => void;
}

export interface UserFeedback {
  message: string;
  tone: UserFeedbackTone;
  /**
   * One recovery the operator can take from the toast itself. Offer it only
   * where re-running is genuinely safe — a button that repeats a non-idempotent
   * operation is worse than no button at all.
   */
  action?: UserFeedbackAction;
}

/** Delivers operation feedback independently of the component that started it. */
export function notifyUser({ message, tone, action }: UserFeedback): void {
  // A toast without a recovery is called exactly as it always was — handing
  // sonner an explicit `undefined` would be a second, noisier way to say the
  // same thing.
  if (action === undefined) {
    toast[tone](message);
    return;
  }
  toast[tone](message, { action });
}
