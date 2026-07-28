import { createContext } from "react";

export type SessionComposerPrefill = (text: string) => void;

export const SessionComposerPrefillContext = createContext<SessionComposerPrefill | null>(null);
