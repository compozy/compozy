/** S5, S6 and S14 on one screen: what it may do, whose requests it answers, and the exceptions. */
import { Settings2, Users, UsersRound } from "@compozy/ui/icons";
import { useState } from "react";

import { Alert, AlertDescription, AlertTitle, FormSection, Pill, RadioCard } from "@compozy/ui";

import { EditorHead } from "../agent-editors/editor-head";
import { UnsavedDock } from "../agent-editors/unsaved-dock";
import { AgentAppPage } from "../stays-on-agents-shared/agent-page";
import type { DraftAgent } from "../stays-on-agents-shared/fixtures";
import { LEVELS, type PermissionLevel, type PermissionRule } from "./levels";
import { RulesSection } from "./rules-section";

export type ApprovalReach = "children" | "workspace";

export interface PermissionsEditorProps {
  agent: DraftAgent;
  level?: PermissionLevel;
  /** Nothing set: stays-on agents run at `approve-edits`. */
  unset?: boolean;
  reach?: ApprovalReach;
  rules?: readonly PermissionRule[];
  invalidRuleId?: string;
  unsaved?: boolean;
  readOnly?: boolean;
  /** Provider has no AI reviewer. */
  noReviewer?: boolean;
  /** A running session fell back from the reviewer. */
  degraded?: boolean;
  /** On-demand agents can leave the choice to Settings. */
  followsSettings?: boolean;
}

function providerName(provider: string) {
  if (provider === "codex") return "Codex";
  if (provider === "cursor") return "Cursor";
  return "Claude";
}

function LevelNote({
  agent,
  level,
  noReviewer,
  degraded,
}: Pick<PermissionsEditorProps, "agent" | "noReviewer" | "degraded"> & {
  level: PermissionLevel;
}) {
  const provider = providerName(agent.provider);
  if (level === "auto" && noReviewer) {
    return (
      <Alert variant="warning">
        <AlertTitle>{provider} has no AI reviewer</AlertTitle>
        <AlertDescription>
          This agent will use Edit on its own instead. It never gets more access than that.
        </AlertDescription>
      </Alert>
    );
  }
  if (level === "auto" && degraded) {
    return (
      <Alert variant="warning">
        <AlertTitle>The reviewer isn't available right now</AlertTitle>
        <AlertDescription>
          The current session is running as Edit on its own. It keeps your choice for new sessions.
        </AlertDescription>
      </Alert>
    );
  }
  if (level === "approve-all" && agent.provider === "codex") {
    return (
      <Alert>
        <AlertTitle>Codex keeps its own reviewer here</AlertTitle>
        <AlertDescription>
          Compozy won't ask you, but Codex still reviews risky actions, inside the project and with
          no network. Turning that off is a separate setting.
        </AlertDescription>
      </Alert>
    );
  }
  return null;
}

export function PermissionsEditor({
  agent,
  level: savedLevel = "approve-edits",
  unset = false,
  reach: savedReach = "children",
  rules: savedRules = [],
  invalidRuleId,
  unsaved = false,
  readOnly = false,
  noReviewer,
  degraded,
  followsSettings = false,
}: PermissionsEditorProps) {
  const [level, setLevel] = useState<PermissionLevel | "settings">(
    followsSettings ? "settings" : savedLevel
  );
  const [reach, setReach] = useState(savedReach);
  const [rules, setRules] = useState(savedRules);
  const changed =
    (unsaved ? 1 : 0) +
    (level === (followsSettings ? "settings" : savedLevel) ? 0 : 1) +
    (reach === savedReach ? 0 : 1) +
    (rules === savedRules ? 0 : 1);
  const reset = () => {
    setLevel(followsSettings ? "settings" : savedLevel);
    setReach(savedReach);
    setRules(savedRules);
  };
  const stays = agent.kind === "stays_on";
  return (
    <AgentAppPage agent={agent} leaf="Permissions">
      <div className="flex flex-col gap-8">
        <EditorHead
          agent={agent}
          subtitle="Choose how much it can do without you."
          title="Permissions"
        />
        {readOnly ? (
          <Alert>
            <AlertDescription>
              This agent comes from the {agent.owner} extension. Duplicate it to change these.
            </AlertDescription>
          </Alert>
        ) : null}
        <FormSection
          help="Applies to new sessions with this agent."
          title="What can it do on its own?"
        >
          <div aria-label="Permission level" className="flex flex-col gap-2" role="radiogroup">
            {stays ? null : (
              <RadioCard
                description="Uses your Settings. Right now: Never ask."
                disabled={readOnly}
                icon={Settings2}
                onSelect={() => setLevel("settings")}
                selected={level === "settings"}
                title="Follow Settings"
              />
            )}
            {LEVELS.map(option => (
              <RadioCard
                badge={
                  unset && option.value === "approve-edits" ? (
                    <Pill size="sm" tone="neutral">
                      Default
                    </Pill>
                  ) : undefined
                }
                description={option.description}
                disabled={readOnly}
                icon={option.icon}
                key={option.value}
                onSelect={() => setLevel(option.value)}
                selected={level === option.value}
                title={option.label}
              />
            ))}
          </div>
          {level === "settings" ? null : (
            <LevelNote agent={agent} degraded={degraded} level={level} noReviewer={noReviewer} />
          )}
        </FormSection>
        {stays ? (
          <FormSection
            help="Only you can change this. An agent can't widen it for itself."
            title="Whose requests can it answer?"
          >
            <div aria-label="Approval reach" className="flex flex-col gap-2" role="radiogroup">
              <RadioCard
                description="It can approve what its own child sessions ask."
                disabled={readOnly}
                icon={Users}
                onSelect={() => setReach("children")}
                selected={reach === "children"}
                title="Only its own sessions"
              />
              <RadioCard
                description="For a coordinator you trust. It can approve work in any session it can reach."
                disabled={readOnly}
                icon={UsersRound}
                onSelect={() => setReach("workspace")}
                selected={reach === "workspace"}
                title="Any session it can reach"
              />
            </div>
          </FormSection>
        ) : null}
        <RulesSection
          invalidId={invalidRuleId}
          onChange={setRules}
          readOnly={readOnly}
          rules={rules}
        />
        <p className="text-small-body text-subtle">
          Child sessions don't inherit any of this. They follow their own agent or your Settings.
        </p>
        <UnsavedDock
          blocked={invalidRuleId ? "Fix the exception first" : undefined}
          count={changed}
          onDiscard={reset}
          onSave={reset}
        />
      </div>
    </AgentAppPage>
  );
}
