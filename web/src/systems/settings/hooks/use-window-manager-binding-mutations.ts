import { useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  cmdPaletteKeys,
  updateWindowManagerBindings,
  windowManagerKeys,
  type WindowManagerBindingUpdate,
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
  const queue = useRef(Promise.resolve());
  const inflight = useRef(0);
  const [saving, setSaving] = useState(false);
  const mutation = useMutation({
    mutationFn: (update: WindowManagerBindingCommit) =>
      updateWindowManagerBindings({
        ...update,
        workspaceId,
        clientId,
      } as WindowManagerBindingUpdate),
    onSuccess: async section => {
      queryClient.setQueryData(windowManagerKeys.config(workspaceId, clientId), section);
      await queryClient.invalidateQueries({
        queryKey: cmdPaletteKeys.workspaceCatalogs(workspaceId ?? ""),
      });
    },
  });

  return {
    commit: update => {
      inflight.current += 1;
      setSaving(true);
      const run = () => mutation.mutateAsync(update);
      const next = queue.current.then(run, run);
      queue.current = next.then(
        () => undefined,
        () => undefined
      );
      return next.finally(() => {
        inflight.current -= 1;
        if (inflight.current === 0) setSaving(false);
      });
    },
    saving,
  };
}
