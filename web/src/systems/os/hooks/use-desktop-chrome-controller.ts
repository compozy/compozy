import { useState } from "react";

import { useDesktopChrome } from "./use-desktop-chrome";
import { useDesktopShellModel } from "./use-desktop-shell-model";
import { useWorkspaceSetupDefaults } from "./use-workspace-setup-defaults";
import { useWorktreeDialogTargets } from "./use-worktree-dialog-targets";

/** Owns the state and supporting models required to mount the desktop chrome. */
export function useDesktopChromeController() {
  const [backgroundStreamsEnabled, setBackgroundStreamsEnabled] = useState(true);
  const [continuityStreamsEnabled, setContinuityStreamsEnabled] = useState(true);
  const model = useDesktopShellModel({ backgroundStreamsEnabled, continuityStreamsEnabled });
  const chrome = useDesktopChrome(model.desktopWorkspaceId);
  const workspaceSetupDefaults = useWorkspaceSetupDefaults();
  const worktreeDialogs = useWorktreeDialogTargets();

  return {
    chrome,
    continuityStreamsEnabled,
    model,
    setBackgroundStreamsEnabled,
    setContinuityStreamsEnabled,
    workspaceSetupDefaults,
    worktreeDialogs,
  };
}
