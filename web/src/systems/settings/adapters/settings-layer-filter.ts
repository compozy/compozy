import { normalizeOptionalText } from "./settings-api-error";

interface LayeredSettingsFilter {
  scope?: "user" | "profile" | "workspace";
  workspace_id?: string;
  profile?: string;
}

/** Canonical user/profile/workspace selector shared by settings endpoints. */
export function normalizeSettingsLayerFilter<T extends LayeredSettingsFilter>(
  filter: T
): {
  scope: T["scope"];
  workspace_id: string | undefined;
  profile: string | undefined;
} {
  return {
    scope: filter.scope,
    workspace_id: normalizeOptionalText(filter.workspace_id),
    profile: normalizeOptionalText(filter.profile),
  };
}
