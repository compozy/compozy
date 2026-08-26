export interface AgentActivitySearch {
  root?: string;
  call?: string;
}

function nonBlankString(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() !== "" ? value : undefined;
}

export function validateAgentActivitySearch(search: Record<string, unknown>): AgentActivitySearch {
  const root = nonBlankString(search.root);
  const call = nonBlankString(search.call);
  return {
    ...(root ? { root } : {}),
    ...(call ? { call } : {}),
  };
}
