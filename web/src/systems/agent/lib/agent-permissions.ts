import type { AgentCreatePermissionChoice } from "./agent-create-draft";

/** One plain-language vocabulary for permissions across create, settings, and detail views. */
export const AGENT_CREATE_PERMISSION_OPTIONS: readonly {
  value: AgentCreatePermissionChoice;
  label: string;
  description: string;
}[] = [
  {
    value: "",
    label: "Use the provider's setting",
    description: "CompozyOS won't change how the agent asks for approval.",
  },
  {
    value: "deny-all",
    label: "Ask before every action",
    description: "Every action waits for your approval first.",
  },
  {
    value: "approve-reads",
    label: "Ask only before changes",
    description: "Reading is automatic. Changes and commands still ask.",
  },
  {
    value: "approve-all",
    label: "Never ask",
    description: "The agent acts without asking. Use it for agents you trust.",
  },
] as const;

/** Plain label for a stored permission value; unknown modes print as-is. */
export function permissionLabel(value: string | null | undefined): string {
  const trimmed = value?.trim() ?? "";
  const option = AGENT_CREATE_PERMISSION_OPTIONS.find(candidate => candidate.value === trimmed);
  return option ? option.label : trimmed;
}
