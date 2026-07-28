import { use } from "react";

import { OsShellContext, type OsShellHandle } from "../contexts/os-shell-context";

export function useOsShell(): OsShellHandle {
  const handle = use(OsShellContext);
  if (handle === null) {
    throw new Error("useOsShell requires an <OsShellContext.Provider> above (DesktopShell).");
  }
  return handle;
}
