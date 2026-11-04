import { Alert, AlertDescription } from "@compozy/ui";

import { useSettingsGeneral } from "@/systems/settings";

import { useAgentSessions } from "../../hooks/use-agent-sessions";
import { useAgentSettingsPage } from "../../hooks/use-agent-settings-page";
import { usePermissionsProvider } from "../../hooks/use-permissions-provider";
import {
  invalidPermissionRule,
  permissionRuleErrorFromDaemon,
} from "../../lib/agent-permission-rules";
import {
  hasDegradedRunningSession,
  nextSessionApproval,
  permissionsPageBlocked,
  permissionsPageChanges,
} from "../../lib/agent-permissions-page";
import { buildSettingsDraftFromAgent } from "../../lib/agent-settings-draft";
import { formatAgentOwner } from "../../lib/agent-ownership";
import type { AgentEditorProps } from "./agent-what-it-does-editor";
import { EditorLoading } from "./editor-states";
import { AgentPermissionsView } from "./permissions-view";

/** S5, S6, S14: the agent's Permissions page, saved through the same definition update. */
export function AgentPermissionsEditor({ agent, workspace, readOnly }: AgentEditorProps) {
  const page = useAgentSettingsPage({ name: agent.name, section: "runtime" });
  const provider = usePermissionsProvider(agent);
  const settings = useSettingsGeneral();
  const { sessions } = useAgentSessions(workspace, agent.name);
  const draft = page.draft;
  if (!draft) return <EditorLoading label="Loading permissions" />;

  const baseline = buildSettingsDraftFromAgent(page.agent ?? agent);
  const invalidRule = invalidPermissionRule(draft.permissionRules);
  return (
    <>
      {page.error ? (
        <Alert data-testid="agent-permissions-error" variant="destructive">
          <AlertDescription>{page.error}</AlertDescription>
        </Alert>
      ) : null}
      <AgentPermissionsView
        {...provider}
        agent={agent}
        blocked={permissionsPageBlocked(draft, page.saveBlocked, page.saveBlockedCaption)}
        changes={permissionsPageChanges(draft, baseline)}
        degraded={hasDegradedRunningSession(sessions)}
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
          page.patchDraft({ sessionApproval: nextSessionApproval(approval, baseline) })
        }
        owner={readOnly ? formatAgentOwner(agent) : undefined}
        readOnly={readOnly}
        rules={draft.permissionRules}
        saving={page.phase === "saving"}
        sessionApproval={draft.sessionApproval}
        settingsLevel={
          agent.kind === "always_on" ? undefined : settings.data?.config.permissions.mode
        }
      />
      {page.unsavedGuardDialog}
    </>
  );
}
