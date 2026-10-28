import { createContext } from "react";

export interface SessionMessageEdit {
  active: boolean;
  pending: boolean;
  start: (messageId: string, text: string) => void;
  cancel: () => void;
  submit: () => Promise<void>;
}

export const SessionMessageEditContext = createContext<SessionMessageEdit | null>(null);
