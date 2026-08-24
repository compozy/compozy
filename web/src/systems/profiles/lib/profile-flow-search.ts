export interface ProfileFlowSearch {
  flow: string;
  profile?: string;
}

export interface ProfilesSettingsSearch extends Record<string, unknown> {
  flow?: string;
  profile?: string;
}

function optionalSearchText(value: unknown): string | undefined {
  if (typeof value !== "string") return undefined;
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

export function validateProfilesSettingsSearch(
  search: Record<string, unknown>
): ProfilesSettingsSearch {
  const flow = optionalSearchText(search.flow);
  const profile = optionalSearchText(search.profile);
  return {
    ...(flow === undefined ? {} : { flow }),
    ...(profile === undefined ? {} : { profile }),
  };
}

export function profileFlowFromSearch(
  search: Record<string, unknown>
): ProfileFlowSearch | undefined {
  const normalized = validateProfilesSettingsSearch(search);
  if (normalized.flow === undefined) return undefined;
  return normalized.profile === undefined
    ? { flow: normalized.flow }
    : { flow: normalized.flow, profile: normalized.profile };
}
