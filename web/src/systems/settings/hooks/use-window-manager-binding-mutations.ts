import { useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  cmdPaletteKeys,
  updateWindowManagerBindings,
  windowManagerKeys,
  type WindowManagerSettingsSection,
} from "@/systems/os";

export interface WindowManagerBindingCommit {
  shortcuts?: WindowManagerSettingsSection["config"]["shortcuts"];
  globalShortcuts?: WindowManagerSettingsSection["config"]["globalShortcuts"];
  aliases?: Readonly<Record<string, string>>;
  overwrite?: boolean;
}

export interface WindowManagerBindingMutations {
  commit: (update: WindowManagerBindingCommit) => Promise<WindowManagerSettingsSection>;
  saving: boolean;
}

/**
 * The single write path for bindings and aliases.
 *
 * Each edit lands on its own — there is no draft to save — because a rebind has
 * to dispatch the moment it is made (US-022.AC-3) and two surfaces editing at
 * once must converge on whatever the daemon serialized (US-022.EC-4). The reply
 * is the whole section, so it is seeded straight into the cache the shell reads
 * and the palette catalog is re-read for the labels it carries.
 */
export function useWindowManagerBindingMutations(
  workspaceId: string | null,
  clientId?: string
): WindowManagerBindingMutations {
  const queryClient = useQueryClient();
  const queue = useRef<Promise<void> | null>(null);
  const [pendingCount, setPendingCount] = useState(0);
  const mutation = useMutation({
    mutationFn: (update: WindowManagerBindingCommit) => {
      const scope = clientId === undefined ? { workspaceId } : { workspaceId, clientId };
      return updateWindowManagerBindings({ ...update, ...scope });
    },
    onSuccess: async section => {
      queryClient.setQueryData(windowManagerKeys.config(workspaceId, clientId), section);
      await queryClient.invalidateQueries({
        queryKey: cmdPaletteKeys.workspaceCatalogs(workspaceId ?? ""),
      });
    },
  });

  return {
    commit: update => {
      setPendingCount(count => count + 1);
      const run = () => mutation.mutateAsync(update);
      const next = (queue.current ?? Promise.resolve()).then(run, run);
      queue.current = next.then(
        () => undefined,
        () => undefined
      );
      return next.finally(() => {
        setPendingCount(count => Math.max(0, count - 1));
      });
    },
    saving: pendingCount > 0,
  };
}
