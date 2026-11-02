import { Plus } from "@compozy/ui/icons";

import { Button, CatalogCard, Pill, StatusDot, type AgentAvatarState } from "@compozy/ui";

import type { WorkspacePayload } from "@/systems/workspace";

import {
  AGENT_PLACE_LATER,
  agentCreateAppearance,
  type AgentCreateDialogDraft,
} from "../lib/agent-create-draft";
import { agentPlaceLabel } from "../lib/agent-place";
import { AgentFleetNewSessionButton } from "./agent-fleet-new-session-button";
import { AgentMark } from "./agent-mark";

/** How the agent will read on the Agents page, updated as you type. */
export function AgentCreatePreview({
  draft,
  state = "idle",
  stateLabel,
  workspaces,
}: {
  draft: AgentCreateDialogDraft;
  state?: AgentAvatarState;
  stateLabel?: string;
  workspaces: readonly WorkspacePayload[];
}) {
  const typed = draft.displayName.trim();
  const name = typed || "Your agent";
  const stays = draft.kind === "stays_on";
  return (
    <CatalogCard data-testid="agent-create-preview">
      <div className="flex min-w-0 items-start gap-3">
        <AgentMark
          appearance={agentCreateAppearance(draft)}
          label={name}
          provider={draft.provider || null}
          quietIdle
          size={44}
          state={state}
          surfaceClassName="ring-card"
        />
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <CatalogCard.Title className={typed ? undefined : "text-subtle"}>
            {name}
          </CatalogCard.Title>
          <CatalogCard.Description className="line-clamp-2">
            {draft.description.trim() || "What it does shows here."}
          </CatalogCard.Description>
        </div>
      </div>
      <CatalogCard.Actions className="justify-between">
        {stays ? (
          <Pill size="sm">
            <StatusDot size="sm" tone={state === "failed" ? "destructive" : "faint"} />
            <span className="max-w-40 truncate">
              {stateLabel ??
                (draft.place === AGENT_PLACE_LATER
                  ? "Not working yet"
                  : agentPlaceLabel(draft.place, workspaces))}
            </span>
          </Pill>
        ) : (
          <span className="text-eyebrow tracking-eyebrow text-subtle">Runs when you ask</span>
        )}
        {stays ? (
          <Button
            aria-hidden="true"
            disabled
            size="icon-sm"
            tabIndex={-1}
            type="button"
            variant="quiet"
          >
            <Plus aria-hidden="true" />
          </Button>
        ) : (
          <AgentFleetNewSessionButton agentName={name} disabled onNewSession={() => undefined} />
        )}
      </CatalogCard.Actions>
    </CatalogCard>
  );
}
