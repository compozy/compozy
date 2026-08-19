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
  aliases?: Readonly<Record<string, string>>;
  overwrite?: boolean;
}

export interface WindowManagerBindingMutations {
  /** Resolves with the section the daemon produced, or throws its refusal. */
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
  workspaceId: string | null
): WindowManagerBindingMutations {
  const queryClient = useQueryClient();
  const mutation = useMutation({
    mutationFn: (update: WindowManagerBindingCommit) =>
      updateWindowManagerBindings({ ...update, workspaceId } as WindowManagerBindingUpdate),
    onSuccess: async section => {
      queryClient.setQueryData(windowManagerKeys.config(workspaceId), section);
      // Chord badges and "Title (alias)" rows read the catalog, not this
      // section, so they would otherwise keep the previous binding on screen.
      await queryClient.invalidateQueries({
        queryKey: cmdPaletteKeys.workspaceCatalogs(workspaceId ?? ""),
      });
    },
  });

  return {
    commit: update => mutation.mutateAsync(update),
    saving: mutation.isPending,
  };
}
