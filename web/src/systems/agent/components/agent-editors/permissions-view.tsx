/** S5, S6 and S14 on one page: what it may do, whose requests it answers, and the exceptions. */
import { Settings2, Users, UsersRound } from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  FormSection,
  Pill,
  RadioCardGroup,
  type RadioCardGroupOption,
} from "@compozy/ui";

import type { ProviderReviewer } from "@/systems/providers";

import type { PermissionRuleDraft } from "../../lib/agent-permission-rules";
import {
  AGENT_PERMISSION_LEVELS,
  FOLLOW_SETTINGS_LABEL,
  isAgentPermissionLevel,
  permissionLabel,
  STAYS_ON_DEFAULT_PERMISSION,
  type AgentPermissionLevel,
} from "../../lib/agent-permissions";
import type { AgentSessionApproval } from "../../lib/agent-settings-draft";
import type { AgentPayload } from "../../types";
import { EditorHead } from "./editor-head";
import { PermissionRulesSection } from "./permission-rules-section";
import { UnsavedDock } from "./unsaved-dock";

/** Codex keeps its own Auto review under `approve-all` unless full access is opted in. */
const PROVIDER_KEEPS_REVIEWER_ON_APPROVE_ALL = new Set(["codex"]);

const EDITS_FILES = new Set<AgentPermissionLevel>(["approve-edits", "auto", "approve-all"]);

export interface AgentPermissionsViewProps {
  agent: AgentPayload;
  /** The provider the agent resolves to: its own, else the inherited runtime provider. */
  providerId: string;
  providerName: string;
  /** `""` is nothing set; `null` is a stored value this version doesn't know. */
  level: AgentPermissionLevel | "" | null;
  legacyLevel?: string | null;
  sessionApproval: AgentSessionApproval;
  rules: readonly PermissionRuleDraft[];
  invalidRule: { id: string; message: string } | null;
  reviewer: ProviderReviewer;
  /** A running session asked for the reviewer and fell back. */
  degraded: boolean;
  /** The Settings level an on-demand agent follows when nothing is set. */
  settingsLevel?: string;
  readOnly: boolean;
  owner?: string;
  changes: number;
  saving: boolean;
  blocked?: string;
  onLevelChange: (level: AgentPermissionLevel | "") => void;
  onSessionApprovalChange: (approval: AgentSessionApproval) => void;
  onRulesChange: (rules: PermissionRuleDraft[]) => void;
  onSave: () => void;
  onDiscard: () => void;
}

function LevelNote({
  providerId,
  providerName,
  level,
  reviewer,
  degraded,
}: Pick<AgentPermissionsViewProps, "providerId" | "providerName" | "reviewer" | "degraded"> & {
  level: AgentPermissionLevel;
}) {
  if (level === "auto" && reviewer === "unavailable") {
    return (
      <Alert data-testid="agent-permissions-no-reviewer" variant="warning">
        <AlertTitle>{providerName} has no AI reviewer</AlertTitle>
        <AlertDescription>
          This agent will use Edit on its own instead. It never gets more access than that.
        </AlertDescription>
      </Alert>
    );
  }
  if (level === "auto" && degraded) {
    return (
      <Alert data-testid="agent-permissions-degraded" variant="warning">
        <AlertTitle>The reviewer isn't available right now</AlertTitle>
        <AlertDescription>
          The current session is running as Edit on its own. It keeps your choice for new sessions.
        </AlertDescription>
      </Alert>
    );
  }
  if (level === "approve-all" && PROVIDER_KEEPS_REVIEWER_ON_APPROVE_ALL.has(providerId)) {
    return (
      <Alert data-testid="agent-permissions-keeps-reviewer">
        <AlertTitle>{providerName} keeps its own reviewer here</AlertTitle>
        <AlertDescription>
          CompozyOS won't ask you, but {providerName} still reviews risky actions, inside the
          project and with no network. Turning that off is a separate setting.
        </AlertDescription>
      </Alert>
    );
  }
  return null;
}

/** The radio value for "Follow Settings", which stores no `permissions`. */
const FOLLOW_SETTINGS = "follow-settings";

function LevelGroup(props: AgentPermissionsViewProps & { staysOn: boolean }) {
  const { level, staysOn, readOnly, onLevelChange, settingsLevel } = props;
  const unset = level === "";
  const shown = staysOn && unset ? STAYS_ON_DEFAULT_PERMISSION : level;
  const follow: RadioCardGroupOption<string>[] = staysOn
    ? []
    : [
        {
          value: FOLLOW_SETTINGS,
          title: FOLLOW_SETTINGS_LABEL,
          description: settingsLevel
            ? `Uses your Settings. Right now: ${permissionLabel(settingsLevel)}.`
            : "Uses your Settings.",
          icon: Settings2,
          "data-testid": "agent-permissions-level-settings",
        },
      ];
  const levels: RadioCardGroupOption<string>[] = AGENT_PERMISSION_LEVELS.map(option => ({
    value: option.value,
    title: option.label,
    description: option.description,
    icon: option.icon,
    "data-testid": `agent-permissions-level-${option.value}`,
    badge:
      staysOn && unset && option.value === STAYS_ON_DEFAULT_PERMISSION ? (
        <Pill size="sm" tone="neutral">
          Default
        </Pill>
      ) : option.value === "auto" && props.reviewer === "unavailable" ? (
        <Pill size="sm" tone="warning">
          Not available
        </Pill>
      ) : undefined,
  }));
  return (
    <RadioCardGroup
      aria-label="Permission level"
      className="flex flex-col"
      data-testid="agent-permissions-levels"
      disabled={readOnly}
      onValueChange={next => onLevelChange(isAgentPermissionLevel(next) ? next : "")}
      options={[...follow, ...levels]}
      value={unset && !staysOn ? FOLLOW_SETTINGS : shown}
    />
  );
}

const REACH_OPTIONS: RadioCardGroupOption<"children" | "workspace">[] = [
  {
    value: "children",
    title: "Only its own sessions",
    description: "It can approve what its own child sessions ask.",
    icon: Users,
    "data-testid": "agent-permissions-reach-children",
  },
  {
    value: "workspace",
    title: "Any session it can reach",
    description: "For a coordinator you trust. It can approve work in any session it can reach.",
    icon: UsersRound,
    "data-testid": "agent-permissions-reach-workspace",
  },
];

export function AgentPermissionsView(props: AgentPermissionsViewProps) {
  const { agent, level, legacyLevel, sessionApproval, readOnly, owner } = props;
  const staysOn = agent.kind === "stays_on";
  const effective = level === "" && staysOn ? STAYS_ON_DEFAULT_PERMISSION : level;
  const reach = sessionApproval || "children";
  return (
    <div className="flex flex-col gap-8" data-testid="agent-permissions-page">
      <EditorHead
        agent={agent}
        subtitle="Choose how much it can do without you."
        title="Permissions"
      />
      {readOnly ? (
        <Alert data-testid="agent-permissions-read-only">
          <AlertDescription>
            This agent comes from {owner ?? "an extension"}. Duplicate it to change these.
          </AlertDescription>
        </Alert>
      ) : null}
      <FormSection
        help="Applies to new sessions with this agent."
        title="What can it do on its own?"
      >
        {legacyLevel ? (
          <p className="text-small-body text-warning" data-testid="agent-permissions-legacy">
            Unrecognized permission mode: {legacyLevel}. Pick one to save.
          </p>
        ) : null}
        <LevelGroup {...props} staysOn={staysOn} />
        {effective ? (
          <LevelNote
            providerId={props.providerId}
            degraded={props.degraded}
            level={effective}
            providerName={props.providerName}
            reviewer={props.reviewer}
          />
        ) : null}
        {effective && EDITS_FILES.has(effective) ? (
          <p className="text-small-body text-subtle">
            It can edit files, so it can also change its own definition.
          </p>
        ) : null}
      </FormSection>
      {staysOn ? (
        <FormSection
          help="Only you can change this. An agent can't widen it for itself."
          title="Whose requests can it answer?"
        >
          <RadioCardGroup
            aria-label="Approval reach"
            className="flex flex-col"
            data-testid="agent-permissions-reach"
            disabled={readOnly}
            onValueChange={props.onSessionApprovalChange}
            options={REACH_OPTIONS}
            value={reach}
          />
        </FormSection>
      ) : null}
      <PermissionRulesSection
        invalid={props.invalidRule}
        onChange={props.onRulesChange}
        readOnly={readOnly}
        rules={props.rules}
      />
      <p className="text-small-body text-subtle">
        Child sessions don't inherit any of this. They follow their own agent or your Settings.
      </p>
      {readOnly ? null : (
        <UnsavedDock
          blocked={props.blocked}
          count={props.changes}
          onDiscard={props.onDiscard}
          onSave={props.onSave}
          saving={props.saving}
        />
      )}
    </div>
  );
}
