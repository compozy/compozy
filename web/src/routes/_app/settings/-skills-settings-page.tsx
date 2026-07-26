import { AlertCircle } from "lucide-react";
import type { Dispatch, SetStateAction } from "react";
import { Link } from "@tanstack/react-router";

import {
  useSettingsSkillsPage,
  type SkillsScopeSelection,
} from "@/systems/settings/hooks/use-settings-skills-page";
import { AgentCommandSelect, type AgentPayload } from "@/systems/agent";
import {
  SettingLinkRow,
  SettingsAdvancedFold,
  SettingsDisabledSkillsSection,
  SettingsFieldRow,
  SettingsGroup,
  SettingsPageFrame,
  SettingsProvChip,
  SettingsRuntimeUnavailable,
  SettingsInlineSaveControls,
  SettingsSaveBar,
  SettingsTaglistField,
  useSettingsSaveBarState,
  useSettingsTopbar,
  type SettingsScope,
  type SettingsSkillsSection,
} from "@/systems/settings";
import type { WorkspacePayload } from "@/systems/workspace";
import {
  Button,
  Input,
  NativeSelect,
  NativeSelectOption,
  PillGroup,
  Spinner,
  Switch,
} from "@agh/ui";

type SkillsConfig = SettingsSkillsSection["config"];

export function SkillsSettingsPage() {
  const page = useSettingsSkillsPage();
  useSettingsTopbar("skills");
  const policySaveState = useSettingsSaveBarState({
    isDirty: page.isPolicyDirty,
    isSaving: page.isSavingPolicy,
    error: page.savePolicyError,
    warnings: page.policyWarnings,
    lastAppliedLabel: page.lastPolicyLabel,
  });

  if (page.isLoading) {
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-skills-loading"
      >
        <Spinner className="size-5 text-subtle" />
      </div>
    );
  }

  if (page.error || !page.envelope || !page.draft) {
    return (
      <div
        className="flex flex-1 items-center justify-center"
        data-testid="settings-page-skills-error"
      >
        <div className="flex flex-col items-center gap-2 text-center">
          <AlertCircle className="size-6 text-danger" />
          <p className="text-sm text-subtle">
            {page.error?.message ?? "Failed to load skills settings"}
          </p>
          <Button onClick={page.handleRetry} size="sm" type="button" variant="outline">
            Retry
          </Button>
        </div>
      </div>
    );
  }

  const { envelope, draft, setDraft, restart } = page;
  const isGlobalScope = page.selection.scope === "global";
  const scopeLabel =
    page.selection.scope === "global"
      ? "global"
      : `agent ${page.selectedAgent?.name ?? page.selection.agentName}`;

  return (
    <SettingsPageFrame
      description="Which skills your agents can use, and where new ones may come from."
      meta={[
        ...(envelope.runtime_available
          ? [
              {
                key: "discovered",
                content: (
                  <span>
                    <span className="font-medium text-muted">{envelope.discovered_count}</span>{" "}
                    discovered
                  </span>
                ),
              },
              {
                key: "disabled",
                content: (
                  <span>
                    <span className="font-medium text-muted">{envelope.disabled_count}</span>{" "}
                    disabled
                  </span>
                ),
              },
            ]
          : [{ key: "runtime", content: <span>runtime unavailable</span> }]),
        {
          key: "scope",
          content: <span data-testid="settings-page-skills-scope-label">scope {scopeLabel}</span>,
        },
      ]}
      restart={restart}
      saveBar={
        isGlobalScope ? (
          <SettingsSaveBar
            onReset={page.handleResetPolicy}
            onSave={page.handleSavePolicy}
            slug="skills"
            state={policySaveState}
          />
        ) : undefined
      }
      slug="skills"
    >
      {!envelope.runtime_available ? (
        <SettingsRuntimeUnavailable
          slug="skills"
          description="Skill discovery counts could not be measured. Policy settings remain editable."
        />
      ) : null}
      {isGlobalScope ? (
        <>
          <EngineSection draft={draft} setDraft={setDraft} />
          <MarketplaceSection draft={draft} setDraft={setDraft} />
          <ManageSection />
        </>
      ) : null}
      <ScopeSelector
        selection={page.selection}
        availableScopes={page.availableScopes}
        agents={page.agents}
        workspaces={page.workspaces}
        onSelectGlobal={page.selectGlobal}
        onSelectAgentScope={page.selectAgentScope}
        onSelectAgent={page.selectAgent}
        onSelectWorkspaceContext={page.selectWorkspaceContext}
      />
      <SettingsDisabledSkillsSection
        baselineDisabled={envelope.config.disabled_skills ?? []}
        disabled={draft.disabled_skills ?? []}
        note={
          page.selection.scope === "agent"
            ? `applies immediately · scoped to ${page.selectedAgent?.name ?? page.selection.agentName}${page.selectedWorkspaceContext ? ` via ${page.selectedWorkspaceContext.name}` : ""}`
            : "applies immediately · no restart required"
        }
        emptyTitle={
          page.selection.scope === "agent" ? "No agent-local tombstones" : "No skills installed"
        }
        emptyDescription={
          page.selection.scope === "agent"
            ? "This agent is currently inheriting the effective skill set without disabled logical names."
            : "Manage availability from the Skills operational page; nothing has been disabled yet."
        }
        onToggle={page.toggleDisabled}
        controls={
          <SettingsInlineSaveControls
            controlTestIdPrefix="settings-page-skills-disabled"
            testId="settings-page-skills-disabled-controls"
            saveLabel="Apply"
            isDirty={page.isDisabledDirty}
            isSaving={page.isSavingDisabled}
            error={page.saveDisabledError}
            warnings={page.disabledWarnings}
            lastAppliedLabel={page.lastDisabledLabel}
            onSave={page.handleSaveDisabled}
            onReset={page.handleResetDisabled}
          />
        }
      />
      {isGlobalScope ? (
        <SettingsAdvancedFold
          data-testid="settings-page-skills-advanced"
          label="Advanced — endpoint & install policy"
          padded
        >
          <InstallPolicySection draft={draft} setDraft={setDraft} />
        </SettingsAdvancedFold>
      ) : (
        <AgentScopePolicyNotice />
      )}
    </SettingsPageFrame>
  );
}

type SkillsScopeValue = "global" | "agent";
interface ScopeSelectorProps {
  selection: SkillsScopeSelection;
  availableScopes: readonly SettingsScope[];
  agents: AgentPayload[];
  workspaces: WorkspacePayload[];
  onSelectGlobal: () => void;
  onSelectAgentScope: () => void;
  onSelectAgent: (agentName: string) => void;
  onSelectWorkspaceContext: (workspaceId: string) => void;
}

function ScopeSelector({
  selection,
  availableScopes,
  agents,
  workspaces,
  onSelectGlobal,
  onSelectAgentScope,
  onSelectAgent,
  onSelectWorkspaceContext,
}: ScopeSelectorProps) {
  const agentScopeAvailable = availableScopes.includes("agent");
  const items: Array<{ value: SkillsScopeValue; label: string; testId: string }> = [
    {
      value: "global",
      label: "Global",
      testId: "settings-page-skills-scope-global",
    },
  ];
  if (agentScopeAvailable) {
    items.push({
      value: "agent",
      label: "Agent",
      testId: "settings-page-skills-scope-agent",
    });
  }

  return (
    <SettingsGroup
      title="Scope"
      description="agent scope only changes logical disabled skills for one effective agent"
    >
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
            if (next === "global") {
              onSelectGlobal();
              return;
            }
            onSelectAgentScope();
          }}
        />
      </div>

      {selection.scope === "agent" ? (
        <div className="mt-4 grid gap-4 md:grid-cols-2">
          <SettingsFieldRow
            data-testid="settings-page-skills-agent-select"
            label="Agent"
            description="Select the logical agent that receives the tombstone list"
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
          <SettingsFieldRow
            data-testid="settings-page-skills-workspace-context"
            label="Workspace context"
            description="Optional workspace resolver context for the selected agent"
            control={
              <NativeSelect
                className="w-56"
                data-testid="settings-page-skills-workspace-context-input"
                value={selection.workspaceId ?? ""}
                onChange={event => onSelectWorkspaceContext(event.target.value)}
              >
                <NativeSelectOption value="">Global resolution</NativeSelectOption>
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

interface DraftSectionProps {
  draft: SkillsConfig;
  setDraft: Dispatch<SetStateAction<SkillsConfig | null>>;
}

function EngineSection({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingsGroup title="Skills engine" description="restart required to apply">
      <SettingsFieldRow
        data-testid="settings-page-skills-enabled"
        label="Use skills"
        description="Enable discovery and task resolution"
        control={
          <Switch
            data-testid="settings-page-skills-enabled-switch"
            checked={draft.enabled}
            onCheckedChange={checked =>
              setDraft(prev => {
                const current = prev ?? draft;
                return { ...current, enabled: checked };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}

function MarketplaceSection({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingsGroup title="Marketplace" description="where new skills come from">
      <SettingsFieldRow
        data-testid="settings-page-skills-marketplace-registry"
        label="Skills come from"
        description="Identifier of the marketplace publisher"
        control={
          <Input
            className="w-56"
            data-testid="settings-page-skills-marketplace-registry-input"
            value={draft.marketplace.registry ?? ""}
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  marketplace: { ...current.marketplace, registry: event.target.value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid="settings-page-skills-poll-interval"
        label="Check for updates every"
        description="How often the registry re-scans sources"
        control={
          <Input
            className="w-32 font-mono"
            data-testid="settings-page-skills-poll-interval-input"
            value={draft.poll_interval ?? ""}
            placeholder="5m"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return { ...current, poll_interval: event.target.value };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}

function ManageSection() {
  return (
    <SettingsGroup title="Manage" description="manage runtime state outside of settings">
      <SettingLinkRow
        data-testid="settings-page-skills-link-skills"
        description="Install, update, and disable skills from the marketplace view."
        label="Manage installed skills"
        render={<Link to="/marketplace/skills" search={{ tab: "installed" }} />}
      />
    </SettingsGroup>
  );
}

function AgentScopePolicyNotice() {
  return (
    <SettingsGroup
      title="Marketplace & policy"
      description="read-only in agent scope"
      data-testid="settings-page-skills-agent-policy-note"
    >
      <p className="text-sm text-muted">
        Agent scope only supports logical `skills.disabled_skills` tombstones. Registry enablement,
        poll interval, and marketplace allowlists remain global settings.
      </p>
    </SettingsGroup>
  );
}

function InstallPolicySection({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingsGroup title="Endpoint & install policy" description="restart required to apply">
      <SettingsFieldRow
        data-testid="settings-page-skills-marketplace-base-url"
        label="Marketplace URL"
        description={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Override the registry&apos;s default endpoint
            <SettingsProvChip>skills.marketplace.base_url</SettingsProvChip>
          </span>
        }
        control={
          <Input
            className="w-72 font-mono"
            data-testid="settings-page-skills-marketplace-base-url-input"
            value={draft.marketplace.base_url ?? ""}
            placeholder="https://"
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return {
                  ...current,
                  marketplace: { ...current.marketplace, base_url: event.target.value },
                };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid="settings-page-skills-allowed-mcp"
        label="Allowed MCP installs"
        description={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Marketplace MCP packages that may be installed
            <SettingsProvChip>skills.allowed_marketplace_mcp</SettingsProvChip>
          </span>
        }
        control={
          <SettingsTaglistField
            data-testid="settings-page-skills-allowed-mcp-input"
            label="Allowed MCP installs"
            value={draft.allowed_marketplace_mcp ?? []}
            onChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return { ...current, allowed_marketplace_mcp: value };
              })
            }
          />
        }
      />
      <SettingsFieldRow
        data-testid="settings-page-skills-allowed-hooks"
        label="Allowed hook installs"
        description={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Marketplace hook packages that may be installed
            <SettingsProvChip>skills.allowed_marketplace_hooks</SettingsProvChip>
          </span>
        }
        control={
          <SettingsTaglistField
            data-testid="settings-page-skills-allowed-hooks-input"
            label="Allowed hook installs"
            value={draft.allowed_marketplace_hooks ?? []}
            onChange={value =>
              setDraft(prev => {
                const current = prev ?? draft;
                return { ...current, allowed_marketplace_hooks: value };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}
