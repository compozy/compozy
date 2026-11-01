/** How the agent will read on the Agents page, updated as you type. */
import { Plus } from "lucide-react";

import { Button, CatalogCard, Pill, StatusDot } from "@compozy/ui";

import { AgentFleetNewSessionButton } from "@/systems/agent/components/agent-fleet-new-session-button";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import type { AgentState } from "../stays-on-agents-shared/agent-state";

import { PLACE_LATER, placeLabel, type Draft } from "./model";

export function AgentPreview({
  draft,
  state = "idle",
  stateLabel,
}: {
  draft: Draft;
  state?: AgentState;
  stateLabel?: string;
}) {
  const name = draft.name.trim() || "Your agent";
  const stays = draft.mode === "stays-on";
  return (
    <CatalogCard>
      <div className="flex min-w-0 items-start gap-3">
        <AgentAvatar
          appearance={draft.appearance}
          label={name}
          provider={draft.runtime.provider}
          quietIdle
          size={44}
          state={state}
          surfaceClassName="ring-card"
        />
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <CatalogCard.Title className={draft.name.trim() ? undefined : "text-subtle"}>
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
            <StatusDot size="sm" tone={state === "failed" ? "danger" : "faint"} />
            <span className="max-w-40 truncate">
              {stateLabel ??
                (draft.place === PLACE_LATER ? "Not working yet" : placeLabel(draft.place))}
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
