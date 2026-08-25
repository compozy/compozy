import { useAtom } from "@xstate/store-react";

import { useDesktopChrome } from "./use-desktop-chrome";
import { useDesktopShellModel } from "./use-desktop-shell-model";
import { useWorkspaceSetupDefaults } from "./use-workspace-setup-defaults";
import { useWorktreeDialogTargets } from "./use-worktree-dialog-targets";
import {
  backgroundStreamsWithinConnectionBudget,
  continuityStreamsWithinConnectionBudget,
} from "../lib/background-stream-budget";
import { useActiveWorkspace } from "@/systems/workspace";

/** Owns the state and supporting models required to mount the desktop chrome. */
export function useDesktopChromeController() {
  const activeWorkspace = useActiveWorkspace();
  const chrome = useDesktopChrome(activeWorkspace.desktopWorkspaceId);
  const backgroundStreamsEnabled = useAtom(
    chrome.shell.projection,
    backgroundStreamsWithinConnectionBudget
  );
  const continuityStreamsEnabled = useAtom(
    chrome.shell.projection,
    continuityStreamsWithinConnectionBudget
  );
  const model = useDesktopShellModel(activeWorkspace, {
    backgroundStreamsEnabled,
    continuityStreamsEnabled,
  });
  const workspaceSetupDefaults = useWorkspaceSetupDefaults();
  const worktreeDialogs = useWorktreeDialogTargets();

  return {
    chrome,
    continuityStreamsEnabled,
    model,
    workspaceSetupDefaults,
    worktreeDialogs,
  };
}
