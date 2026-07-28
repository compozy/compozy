import { toast } from "sonner";

export type UserFeedbackTone = "error" | "info" | "success" | "warning";

export interface UserFeedback {
  message: string;
  tone: UserFeedbackTone;
}

/** Delivers operation feedback independently of the component that started it. */
export function notifyUser({ message, tone }: UserFeedback): void {
  toast[tone](message);
}
