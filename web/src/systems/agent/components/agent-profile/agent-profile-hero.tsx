/** The agent's header: who it is, edited in place, and what it's doing right now. */
import type { ReactNode } from "react";

import { Pill, StatusDot, type AgentAvatarAppearance, type AgentAvatarState } from "@compozy/ui";

import { AGENT_STATE_DOT } from "../../lib/agent-team";
import { AgentAvatarEditor, AgentInlineText } from "./agent-identity-editor";

export interface AgentHeroIdentity {
  name: string;
  label: string;
  description: string;
  appearance: AgentAvatarAppearance;
  provider: string | null;
  staysOn: boolean;
  owner?: string;
  readOnly: boolean;
}

export function AgentProfileHero({
  identity,
  state,
  line,
  action,
  onSave,
}: {
  identity: AgentHeroIdentity;
  state: AgentAvatarState;
  line: string;
  action: ReactNode;
  onSave: (patch: {
    display_name?: string;
    description?: string;
    appearance?: AgentAvatarAppearance;
  }) => void;
}) {
  const { readOnly } = identity;
  const live = state === "working" || state === "needs-you";
  return (
    <header className="flex flex-wrap items-center gap-5" data-testid="agent-profile-hero">
      <AgentAvatarEditor
        appearance={identity.appearance}
        label={identity.label}
        name={identity.name}
        onSave={appearance => onSave({ appearance })}
        provider={identity.provider}
        readOnly={readOnly}
        size={80}
        state={state}
      />
      <div className="flex min-w-0 flex-1 basis-80 flex-col gap-1">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h2 className="flex min-w-0 flex-col">
            <AgentInlineText
              label="Name"
              max={80}
              onSave={display_name => onSave({ display_name })}
              readOnly={readOnly}
              required
              textClassName="text-ui-title-lg font-medium text-fg"
              value={identity.label}
            />
          </h2>
          <Pill data-testid="agent-profile-kind" size="sm" tone="neutral">
            {identity.staysOn ? "Stays on" : "On demand"}
          </Pill>
          {identity.owner ? (
            <Pill form="plain" size="sm">
              From {identity.owner}
            </Pill>
          ) : null}
        </div>
        <AgentInlineText
          label="Description"
          max={280}
          multiline
          onSave={description => onSave({ description })}
          placeholder="Add what it does"
          readOnly={readOnly}
          textClassName="text-body text-muted-foreground"
          value={identity.description}
        />
        <p
          className={`flex items-center gap-2 text-body ${live ? "text-fg" : "text-muted-foreground"}`}
          data-testid="agent-profile-now"
        >
          <StatusDot size="sm" tone={AGENT_STATE_DOT[state] ?? "faint"} />
          {line}
        </p>
      </div>
      <div className="flex shrink-0 items-center gap-2">{action}</div>
    </header>
  );
}
