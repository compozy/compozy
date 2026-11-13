import type { ReactNode } from "react";

import { ProjectMenuHostContext } from "@/systems/workspace";

import { useProjectMenuHost } from "../hooks/use-project-menu-host";
import { SessionListGroupFolderDialog } from "./session-list/session-list-group-folder-dialog";

/**
 * The shell's verbs for every project menu, so the overview tile, Settings row, menubar
 * row and board lane offer what the session list's project header does: New session and
 * New folder in the project, Open files, and Move to a project folder of the acting
 * profile. Mount it inside the session-create provider and the OS shell. The folder layout
 * is read only while a project menu is open (on a first open, Move ▸ appears once it lands); under All profiles no single profile owns
 * the folders, so Move stays hidden.
 */
export function ProjectMenuHostProvider({ children }: { children: ReactNode }) {
  const { host, folder, folderProject } = useProjectMenuHost();
  return (
    <ProjectMenuHostContext.Provider value={host}>
      {children}
      <SessionListGroupFolderDialog
        folder={folder}
        name={folderProject?.name ?? ""}
        testId="project-menu-folder"
      />
    </ProjectMenuHostContext.Provider>
  );
}
