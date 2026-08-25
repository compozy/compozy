import { AlertCircle } from "lucide-react";

import { useSettingsSkillsPage } from "@/systems/settings/hooks/use-settings-skills-page";
import {
  SettingsAdvancedFold,
  SettingsDisabledSkillsSection,
  SettingsPageFrame,
  SettingsRuntimeUnavailable,
  SettingsInlineSaveControls,
  SettingsSaveBar,
  SettingsSkillsScopeNotice,
  SettingsSkillsEngineSection,
  SettingsSkillsInstallPolicySection,
  SettingsSkillsManageSection,
  SettingsSkillsMarketplaceSection,
  SettingsSkillSourcesSection,
  SettingsSkillsScopeSelector,
  useSettingsSaveBarState,
  useSettingsTopbar,
} from "@/systems/settings";
import { Button, Spinner } from "@compozy/ui";

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
  const isUserLayer = page.selection.scope === "user";
  const scopeLabel =
    page.selection.scope === "user"
      ? page.personalLabel.toLowerCase()
      : page.selection.scope === "workspace"
        ? `workspace ${page.selectedWorkspace?.name ?? page.selection.workspaceId}`
        : `agent ${page.selectedAgent?.name ?? page.selection.agentName}`;

  return (
    <SettingsPageFrame
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
        isUserLayer ? (
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
          description="Compozy isn't reachable right now. Skill counts are hidden until it's back. You can still change these settings."
        />
      ) : null}
      {isUserLayer ? <SettingsSkillsEngineSection draft={draft} onChange={setDraft} /> : null}
      <SettingsSkillsScopeSelector
        selection={page.selection}
        availableScopes={page.availableScopes}
        personalLabel={page.personalLabel}
        agents={page.agents}
        workspaces={page.workspaces}
        onSelectUser={page.selectUser}
        onSelectWorkspaceScope={page.selectWorkspaceScope}
        onSelectAgentScope={page.selectAgentScope}
        onSelectAgent={page.selectAgent}
        onSelectWorkspace={page.selectWorkspace}
      />
      <SettingsSkillSourcesSection model={page.sources} />
      {isUserLayer ? (
        <>
          <SettingsSkillsMarketplaceSection draft={draft} onChange={setDraft} />
          <SettingsSkillsManageSection />
        </>
      ) : null}
      <SettingsDisabledSkillsSection
        baselineDisabled={envelope.config.disabled_skills ?? []}
        disabled={draft.disabled_skills ?? []}
        note={
          page.isRepositoryProfile
            ? "read only · active profile projection"
            : page.selection.scope === "agent"
              ? `applies immediately · scoped to ${page.selectedAgent?.name ?? page.selection.agentName}${page.selectedWorkspace ? ` via ${page.selectedWorkspace.name}` : ""}`
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
        readOnly={page.isRepositoryProfile}
        controls={
          page.isRepositoryProfile ? undefined : (
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
          )
        }
      />
      {isUserLayer ? (
        <SettingsAdvancedFold
          data-testid="settings-page-skills-advanced"
          label="Advanced — endpoint & install policy"
          padded
        >
          <SettingsSkillsInstallPolicySection draft={draft} onChange={setDraft} />
        </SettingsAdvancedFold>
      ) : page.selection.scope === "agent" ? (
        <SettingsSkillsScopeNotice kind="agent" />
      ) : page.isRepositoryProfile ? (
        <SettingsSkillsScopeNotice kind="repository-profile" />
      ) : null}
    </SettingsPageFrame>
  );
}
