import { createContext } from "react";

/**
 * Sends one ordinary prompt to the open session. `null` where nothing can be sent, such as a
 * read-only transcript: report actions simply aren't offered there.
 */
export const SessionReportSendContext = createContext<((text: string) => void) | null>(null);
