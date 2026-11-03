import { Alert, AlertDescription } from "@compozy/ui";

import { providerReviewer, useProviders } from "@/systems/providers";
import { useSettingsGeneral } from "@/systems/settings";

import { useAgentSessions } from "../../hooks/use-agent-sessions";
import { useAgentSettingsPage } from "../../hooks/use-agent-settings-page";
import {
  invalidPermissionRule,
  permissionRuleErrorFromDaemon,
  permissionRulesEqual,
} from "../../lib/agent-permission-rules";
import {
  buildSettingsDraftFromAgent,
  type AgentSettingsDraft,
} from "../../lib/agent-settings-draft";
import { formatAgentOwner } from "../../lib/agent-ownership";
import type { AgentEditorProps } from "./agent-what-it-does-editor";
import { EditorLoading } from "./editor-states";
import { AgentPermissionsView } from "./permissions-view";

function countChanges(draft: AgentSettingsDraft, baseline: AgentSettingsDraft): number {
  return (
    (draft.permissions === baseline.permissions ? 0 : 1) +
    (draft.sessionApproval === baseline.sessionApproval ? 0 : 1) +
    (permissionRulesEqual(draft.permissionRules, baseline.permissionRules) ? 0 : 1)
  );
}

/** S5, S6, S14: the agent's Permissions page, saved through the same definition update. */
export function AgentPermissionsEditor({ agent, workspace, readOnly }: AgentEditorProps) {
  const page = useAgentSettingsPage({ name: agent.name, section: "runtime" });
  const providers = useProviders();
  const staysOn = agent.kind === "stays_on";
  const settings = useSettingsGeneral();
  const { sessions } = useAgentSessions(workspace, agent.name);
  const draft = page.draft;
  if (!draft) return <EditorLoading label="Loading permissions" />;

  const providerId = agent.provider || agent.effective_runtime?.provider || "";
  const summary = providers.data?.providers.find(provider => provider.name === providerId);
  const baseline = buildSettingsDraftFromAgent(page.agent ?? agent);
  const changes = countChanges(draft, baseline);
  const invalidRule = invalidPermissionRule(draft.permissionRules);
  const blocked =
    page.saveBlocked && !invalidRule && draft.permissions === null
      ? "Pick a level first"
      : invalidRule
        ? "Fix the exception first"
        : page.saveBlocked
          ? page.saveBlockedCaption
          : undefined;
  return (
    <>
      {page.error ? (
        <Alert data-testid="agent-permissions-error" variant="destructive">
          <AlertDescription>{page.error}</AlertDescription>
        </Alert>
      ) : null}
      <AgentPermissionsView
        agent={agent}
        blocked={blocked}
        changes={changes}
        degraded={sessions.some(
          session =>
            (session.state === "active" || session.state === "starting") &&
            session.permission_mode_degraded
        )}
        // A local problem blocks Save; a daemon refusal of a row is shown on that row.
        invalidRule={
          invalidRule ?? permissionRuleErrorFromDaemon(page.error, draft.permissionRules)
        }
        legacyLevel={draft.legacyPermissions}
        level={draft.permissions}
        onDiscard={page.onDiscard}
        onLevelChange={level => page.patchDraft({ permissions: level, legacyPermissions: null })}
        onRulesChange={rules => page.patchDraft({ permissionRules: rules })}
        onSave={page.onSave}
        onSessionApprovalChange={approval =>
          // `children` is what an unset reach already means; picking it is no change.
          page.patchDraft({
            sessionApproval:
              approval === "children" && baseline.sessionApproval === "" ? "" : approval,
          })
        }
        owner={readOnly ? formatAgentOwner(agent) : undefined}
        providerId={providerId}
        providerName={summary?.display_name || providerId || "This provider"}
        readOnly={readOnly}
        reviewer={providerReviewer(summary)}
        rules={draft.permissionRules}
        saving={page.phase === "saving"}
        sessionApproval={draft.sessionApproval}
        settingsLevel={staysOn ? undefined : settings.data?.config.permissions.mode}
      />
      {page.unsavedGuardDialog}
    </>
  );
}
