import { createContext } from "react";

import type { SteerProvenanceIndex } from "./steer-provenance";

/** The thread's steer provenance index; `null` outside a thread (stories, isolated rows). */
export const SteerProvenanceContext = createContext<SteerProvenanceIndex | null>(null);
