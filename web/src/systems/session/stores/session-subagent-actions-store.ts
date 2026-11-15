import { createStoreLogic } from "@xstate/store";

import type { SubagentView } from "../components/subagents/types";

/** The parent row a subagent verb acts on: its own workspace, session, and profile. */
export interface SubagentActionScope {
  workspaceId: string;
  sessionId: string;
  profile: string;
}

/** What one confirmation holds while its DELETE flies. */
export interface PendingSubagentDelete {
  subagent: SubagentView;
  scope: SubagentActionScope;
}

interface SubagentActionsContext {
  pending: PendingSubagentDelete | null;
  /** The last delete failure, kept beside the open confirmation. */
  failure: string | null;
}

/**
 * The rail's subagent confirmation state. One store per list host, above the
 * threads, so the dialog survives the branch whose row opened it.
 */
export const sessionSubagentActionsLogic = createStoreLogic({
  context: (): SubagentActionsContext => ({ pending: null, failure: null }),
  on: {
    deleteOpened: (_context, { pending }: { pending: PendingSubagentDelete }) => ({
      pending,
      failure: null,
    }),
    deleteFailed: (context, { failure }: { failure: string | null }) => ({ ...context, failure }),
    deleteClosed: () => ({ pending: null, failure: null }),
  },
});
