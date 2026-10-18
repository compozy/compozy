export interface ProfileFlowSearch {
  flow: string;
  profile?: string;
  name?: string;
  new_name?: string;
}

export interface ProfilesSettingsSearch extends Record<string, unknown> {
  flow?: string;
  profile?: string;
  name?: string;
  new_name?: string;
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
  const name = optionalSearchText(search.name);
  const newName = optionalSearchText(search.new_name);
  return {
    ...(flow === undefined ? {} : { flow }),
    ...(profile === undefined ? {} : { profile }),
    ...(name === undefined ? {} : { name }),
    ...(newName === undefined ? {} : { new_name: newName }),
  };
}

export function profileFlowFromSearch(
  search: Record<string, unknown>
): ProfileFlowSearch | undefined {
  const normalized = validateProfilesSettingsSearch(search);
  if (normalized.flow === undefined) return undefined;
  return { ...normalized, flow: normalized.flow };
}
