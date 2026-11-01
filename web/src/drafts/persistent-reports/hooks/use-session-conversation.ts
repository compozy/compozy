import { createContext, use, type ReactNode } from "react";

export interface SessionConversation {
  /** Sends an ordinary prompt: a picked proposal and a typed message are the same act. */
  send: (text: string) => void;
  /** Leaves a line in the transcript, the way a settled decision does. */
  record: (line: ReactNode) => void;
}

const SessionConversationContext = createContext<SessionConversation | null>(null);

export const SessionConversationProvider = SessionConversationContext.Provider;

export function useSessionConversation() {
  const conversation = use(SessionConversationContext);
  if (!conversation) throw new Error("Report entries render inside MainSession");
  return conversation;
}
