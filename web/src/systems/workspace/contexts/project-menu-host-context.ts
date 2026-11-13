import { createContext } from "react";

import type { ProjectMenuVerbs } from "../lib/project-menu";
import type { WorkspacePayload } from "../types";

/**
 * The verbs the shell reaches for any project — New session, New folder, Open files, Move to a
 * folder — so every project menu offers them without its surface wiring each one. A surface's
 * own verbs win over these. Absent outside the shell, where those verbs are hidden.
 */
export interface ProjectMenuHost {
  verbsFor: (workspace: WorkspacePayload) => ProjectMenuVerbs;
  /** A project menu opened: the host starts the reads its verbs need (the folder layout). */
  menuOpened: () => void;
}

export const ProjectMenuHostContext = createContext<ProjectMenuHost | null>(null);
