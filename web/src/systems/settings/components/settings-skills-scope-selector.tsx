import { NativeSelect, NativeSelectOption, PillGroup } from "@compozy/ui";

import { AgentCommandSelect, type AgentPayload } from "@/systems/agent";
import type { WorkspacePayload } from "@/systems/workspace";

import type { SkillsScopeSelection, SkillsScopeValue } from "../hooks/use-settings-skills-scope";
import type { SettingsScope } from "../types";
import { SettingsFieldRow } from "./settings-field-row";
import { SettingsGroup } from "./settings-group";

interface SettingsSkillsScopeSelectorProps {
  selection: SkillsScopeSelection;
  availableScopes: readonly SettingsScope[];
  /** Personal-lane label: "User", or the acting profile's name. */
  personalLabel: string;
  agents: AgentPayload[];
  workspaces: WorkspacePayload[];
  onSelectUser: () => void;
  onSelectWorkspaceScope: () => void;
  onSelectAgentScope: () => void;
  onSelectAgent: (agentName: string) => void;
  onSelectWorkspace: (workspaceId: string) => void;
}

/**
 * Which layer is being edited. The profile dimension is deliberately absent:
 * it follows the shell's profile lens, so this page moves with the rest of the
 * app instead of offering a second, divergent selector.
 */
export function SettingsSkillsScopeSelector({
  selection,
  availableScopes,
  personalLabel,
  agents,
  workspaces,
  onSelectUser,
  onSelectWorkspaceScope,
  onSelectAgentScope,
  onSelectAgent,
  onSelectWorkspace,
}: SettingsSkillsScopeSelectorProps) {
  const items: Array<{ value: SkillsScopeValue; label: string; testId: string }> = [
    { value: "user", label: personalLabel, testId: "settings-page-skills-scope-user" },
  ];
  if (availableScopes.includes("workspace") && workspaces.length > 0) {
    items.push({
      value: "workspace",
      label: "Workspace",
      testId: "settings-page-skills-scope-workspace",
    });
  }
  if (availableScopes.includes("agent")) {
    items.push({ value: "agent", label: "Agent", testId: "settings-page-skills-scope-agent" });
  }

  const showWorkspacePicker = selection.scope !== "user";
  return (
    <SettingsGroup title="Scope">
      <div
        className="flex flex-wrap items-center gap-2"
        data-testid="settings-page-skills-scope-row"
      >
        <PillGroup<SkillsScopeValue>
          items={items}
          value={selection.scope}
          size="sm"
          aria-label="Skills scope"
          onChange={next => {
            if (next === "user") onSelectUser();
            else if (next === "workspace") onSelectWorkspaceScope();
            else onSelectAgentScope();
          }}
        />
      </div>

      {showWorkspacePicker ? (
        <div className="mt-4 grid gap-4 md:grid-cols-2">
          {selection.scope === "agent" ? (
            <SettingsFieldRow
              data-testid="settings-page-skills-agent-select"
              label="Agent"
              control={
                <AgentCommandSelect
                  agents={agents}
                  value={selection.agentName || null}
                  onChange={next => onSelectAgent(next ?? "")}
                  triggerTestId="settings-agent-select"
                  className="w-56"
                  placeholder="Select an agent"
                />
              }
            />
          ) : null}
          <SettingsFieldRow
            data-testid="settings-page-skills-workspace-context"
            label="Workspace"
            help={
              selection.scope === "agent"
                ? "Optional workspace resolver context for the selected agent"
                : undefined
            }
            control={
              <NativeSelect
                aria-label="Workspace"
                className="w-56"
                data-testid="settings-page-skills-workspace-context-input"
                value={selection.workspaceId ?? ""}
                onChange={event => onSelectWorkspace(event.target.value)}
              >
                {selection.scope === "agent" ? (
                  <NativeSelectOption value="">User resolution</NativeSelectOption>
                ) : null}
                {workspaces.map(workspace => (
                  <NativeSelectOption key={workspace.id} value={workspace.id}>
                    {workspace.name}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            }
          />
        </div>
      ) : null}
    </SettingsGroup>
  );
}

export function SettingsSkillsScopeNotice({ kind }: { kind: "agent" | "repository-profile" }) {
  if (kind === "repository-profile") {
    return (
      <SettingsGroup
        title="Repository profile"
        data-testid="settings-page-skills-repository-profile-note"
      >
        <p className="text-sm text-muted">
          This workspace projection follows the active profile and cannot be edited here.
        </p>
      </SettingsGroup>
    );
  }
  return (
    <SettingsGroup
      title="Marketplace & policy"
      data-testid="settings-page-skills-agent-policy-note"
    >
      <p className="text-sm text-muted">
        Agent scope only supports logical `skills.disabled_skills` tombstones. Registry enablement,
        poll interval, source policy, and marketplace allowlists remain user settings.
      </p>
    </SettingsGroup>
  );
}
