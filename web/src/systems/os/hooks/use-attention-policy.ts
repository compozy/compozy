import { settingsAttentionFilterForProfile, useSettingsAttention } from "@/systems/settings";
import { useProfileReadScope } from "@/systems/profiles";

export interface OsAttentionPolicy {
  toasts: boolean;
  sound: boolean;
  system: boolean;
  mutedWorkspaceIds: ReadonlySet<string>;
}

/**
 * Live `[attention]` delivery policy, shared by the bell (which marks muted
 * rows) and the notifier (which honours the toggles). Defaults mirror the
 * daemon's — toasts on, sound on, system off — so the shell behaves correctly
 * during the first load instead of going silent.
 *
 * A muted workspace silences delivery only. Its rows and its counts stay,
 * which is why this hook feeds rendering and never filters the summary.
 */
export function useAttentionPolicy(): OsAttentionPolicy {
  const profile = useProfileReadScope();
  const query = useSettingsAttention(settingsAttentionFilterForProfile(profile.destination));
  const config = query.data?.config;
  return {
    toasts: config?.toasts ?? true,
    sound: config?.sound ?? true,
    system: config?.system ?? false,
    mutedWorkspaceIds: new Set(config?.muted_workspaces ?? []),
  };
}
