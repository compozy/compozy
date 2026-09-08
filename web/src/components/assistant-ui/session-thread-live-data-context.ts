import { createContext } from "react";

/**
 * Whether the thread's window is applying live frames (US-018.EC-2). A paused
 * background window keeps its last frame: nothing in the transcript moves.
 */
export const SessionThreadLiveDataContext = createContext(true);
