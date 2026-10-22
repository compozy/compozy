import { createContext } from "react";

import type { WorkspaceBadgeIdentity } from "@/systems/workspace";

import type { SessionPayload } from "../types";

/**
 * Resolves a row's project badge while the list reads every project (BR-10).
 * Provided by the list rather than threaded through every row container;
 * absent — no badges — in a focused list and inside a project group.
 */
export const SessionListBadgeContext = createContext<
  ((session: SessionPayload) => WorkspaceBadgeIdentity) | undefined
>(undefined);
