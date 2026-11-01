/** S2's page: the Agents app content, also the host behind the create wizard. */
import { Plus, Users2 } from "lucide-react";
import { Fragment, useState, type ReactNode } from "react";

import {
  Button,
  CatalogCard,
  Empty,
  ListGroup,
  ListingPage,
  ListingRow,
  ListingToolbar,
  Pill,
  SearchInput,
  StatusDot,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  cn,
  type ListingViewMode,
  type StatusDotTone,
  type TopbarSlotValue,
} from "@compozy/ui";

import { AgentFleetNewSessionButton } from "@/systems/agent/components/agent-fleet-new-session-button";
import {
  FLEET_CARDS_CLASS,
  FLEET_ROWS_CLASS,
} from "@/systems/agent/components/agent-fleet-classes";

import { AgentAvatar } from "./agent-avatar";
import type { AgentState } from "./agent-state";
import { DraftShell } from "./draft-shell";
import { SCOPE_LABEL, type DraftAgent } from "./fixtures";

export interface ListedAgent extends DraftAgent {
  /** Where a persistent agent runs, each with its live state. */
  places?: readonly { scope: string; state: AgentState }[];
  /** Separates two definitions that share a slug. */
  origin?: string;
}

const PLACE_DOT: Partial<Record<AgentState, StatusDotTone>> = {
  working: "success",
  "needs-you": "accent",
  failed: "danger",
  paused: "faint",
  idle: "faint",
  starting: "faint",
};

function overallState(agent: ListedAgent): AgentState {
  const states = (agent.places ?? []).map(place => place.state);
  const order: AgentState[] = ["needs-you", "failed", "working", "paused", "starting"];
  return order.find(state => states.includes(state)) ?? "idle";
}

function PlaceChip({ scope, state }: { scope: string; state: AgentState }) {
  const tone = PLACE_DOT[state];
  return (
    <Pill size="sm" tone={state === "needs-you" ? "accent" : "neutral"}>
      {tone ? <StatusDot size="sm" tone={tone} /> : null}
      {SCOPE_LABEL(scope)}
      {state === "paused" ? <span className="text-subtle">Paused</span> : null}
    </Pill>
  );
}

function AddPlaceButton({ agent }: { agent: ListedAgent }) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            aria-label={`Add ${agent.label} to a workspace`}
            size="icon-sm"
            type="button"
            variant="quiet"
          >
            <Plus aria-hidden="true" />
          </Button>
        }
      />
      <TooltipContent>Add to a workspace</TooltipContent>
    </Tooltip>
  );
}

function scopeSummary(scopes: readonly string[]) {
  const named = scopes.map(SCOPE_LABEL);
  return named.length > 3
    ? `${named.slice(0, 3).join(" · ")} +${named.length - 3}`
    : named.join(" · ");
}

function AgentPills({ agent }: { agent: ListedAgent }) {
  return (
    <>
      {agent.origin ? <Pill size="xs">{agent.origin}</Pill> : null}
      {agent.invalid ? (
        <Pill size="sm" tone="warning">
          Invalid
        </Pill>
      ) : null}
    </>
  );
}

function CardFoot({ agent }: { agent: ListedAgent }) {
  if (agent.kind === "persistent") {
    const places = agent.places ?? [];
    return (
      <CatalogCard.Actions className="justify-between">
        <div className="flex min-w-0 flex-wrap items-center gap-1.5">
          {places.length > 0 ? (
            places.map(place => <PlaceChip key={place.scope} {...place} />)
          ) : (
            <span className="text-eyebrow tracking-eyebrow text-subtle">Not running</span>
          )}
        </div>
        <AddPlaceButton agent={agent} />
      </CatalogCard.Actions>
    );
  }
  return (
    <CatalogCard.Actions className="justify-between">
      <span className="min-w-0 truncate text-eyebrow tracking-eyebrow text-subtle">
        {scopeSummary(agent.scopes)}
        {agent.active > 0 ? ` · ${agent.active} active` : ""}
      </span>
      <AgentFleetNewSessionButton agentName={agent.label} onNewSession={() => undefined} />
    </CatalogCard.Actions>
  );
}

function AgentCard({ agent }: { agent: ListedAgent }) {
  return (
    <CatalogCard actionable>
      <div className="flex min-w-0 items-start gap-3">
        <AgentAvatar
          appearance={agent.appearance}
          className="m-0.5"
          label={agent.label}
          provider={agent.provider}
          quietIdle
          size={36}
          state={agent.kind === "persistent" ? overallState(agent) : "idle"}
          surfaceClassName="ring-card"
        />
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <div className="flex min-w-0 items-center gap-2">
            <CatalogCard.Title className="min-w-0 flex-1">{agent.label}</CatalogCard.Title>
            <AgentPills agent={agent} />
          </div>
          <CatalogCard.Description className="line-clamp-2">
            {agent.description}
          </CatalogCard.Description>
        </div>
      </div>
      <CardFoot agent={agent} />
    </CatalogCard>
  );
}

function AgentRow({ agent }: { agent: ListedAgent }) {
  const places = agent.places ?? [];
  return (
    <ListingRow>
      <ListingRow.Link render={<button aria-label={`Open ${agent.label}`} type="button" />}>
        <ListingRow.Icon className="bg-transparent">
          <AgentAvatar
            appearance={agent.appearance}
            label={agent.label}
            provider={agent.provider}
            quietIdle
            size={24}
            state={agent.kind === "persistent" ? overallState(agent) : "idle"}
            surfaceClassName="ring-card"
          />
        </ListingRow.Icon>
        <ListingRow.Main>
          <ListingRow.Name>
            <ListingRow.Title>{agent.label}</ListingRow.Title>
            <AgentPills agent={agent} />
          </ListingRow.Name>
          <ListingRow.Meta>
            <span className="min-w-0 truncate">{agent.description}</span>
          </ListingRow.Meta>
        </ListingRow.Main>
      </ListingRow.Link>
      <ListingRow.Trail className="gap-2">
        {agent.kind === "persistent" ? (
          <>
            {places.length > 0 ? (
              places.map(place => <PlaceChip key={place.scope} {...place} />)
            ) : (
              <span className="text-eyebrow tracking-eyebrow text-subtle">Not running</span>
            )}
            <AddPlaceButton agent={agent} />
          </>
        ) : (
          <>
            <span className="text-eyebrow tracking-eyebrow text-subtle">
              {scopeSummary(agent.scopes)}
            </span>
            <AgentFleetNewSessionButton agentName={agent.label} onNewSession={() => undefined} />
          </>
        )}
      </ListingRow.Trail>
    </ListingRow>
  );
}

function Group({
  label,
  count,
  agents,
  view,
}: {
  label: string;
  count: number;
  agents: readonly ListedAgent[];
  view: ListingViewMode;
}) {
  if (agents.length === 0) return null;
  return (
    <ListGroup count={count} label={label}>
      {view === "cards" ? (
        <div className={cn(FLEET_CARDS_CLASS, "pt-2.5")}>
          {agents.map(agent => (
            <Fragment key={`${agent.name}-${agent.origin ?? ""}`}>
              <AgentCard agent={agent} />
            </Fragment>
          ))}
        </div>
      ) : (
        <div className={cn(FLEET_ROWS_CLASS, "mt-2.5")}>
          {agents.map(agent => (
            <Fragment key={`${agent.name}-${agent.origin ?? ""}`}>
              <AgentRow agent={agent} />
            </Fragment>
          ))}
        </div>
      )}
    </ListGroup>
  );
}

export interface AgentsPageProps {
  persistent: readonly ListedAgent[];
  ephemeral: readonly ListedAgent[];
  counts?: { persistent: number; ephemeral: number };
  initialView?: ListingViewMode;
  query?: string;
  footer?: ReactNode;
  overlay?: ReactNode;
  onCreate?: () => void;
}

/** The Agents window with its head, toolbar and sections. */
export function AgentsPage({
  persistent,
  ephemeral,
  counts,
  initialView = "cards",
  query = "",
  footer,
  overlay,
  onCreate,
}: AgentsPageProps) {
  const [view, setView] = useState<ListingViewMode>(initialView);
  const [draft, setDraft] = useState(query);
  const total = counts
    ? counts.persistent + counts.ephemeral
    : persistent.length + ephemeral.length;
  const empty = persistent.length === 0 && ephemeral.length === 0;
  const topbar: TopbarSlotValue = {
    count: total,
    actions: (
      <Button onClick={onCreate} size="sm" type="button" variant="secondary">
        <Plus aria-hidden="true" />
        New agent
      </Button>
    ),
    toolbar: (
      <ListingToolbar>
        <ListingToolbar.Leading>
          <SearchInput
            aria-label="Search agents"
            kbd="/"
            onChange={setDraft}
            placeholder="Search agents"
            value={draft}
          />
        </ListingToolbar.Leading>
        <ListingToolbar.Trailing>
          <ListingToolbar.ViewToggle onChange={setView} value={view} />
        </ListingToolbar.Trailing>
      </ListingToolbar>
    ),
  };
  return (
    <DraftShell app="agents" glyph={<Users2 />} overlay={overlay} title="Agents" topbar={topbar}>
      <ListingPage>
        {empty ? (
          <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
            <Empty
              action={
                <Button size="sm" type="button" variant="ghost">
                  Clear filters
                </Button>
              }
              description="Try a different search or clear the active filters."
              icon={Users2}
              title="No agents match"
            />
          </div>
        ) : (
          <div className={cn("flex flex-col gap-6")}>
            <Group
              agents={persistent}
              count={counts?.persistent ?? persistent.length}
              label="Persistent"
              view={view}
            />
            <Group
              agents={ephemeral}
              count={counts?.ephemeral ?? ephemeral.length}
              label="Ephemeral"
              view={view}
            />
            {footer}
          </div>
        )}
      </ListingPage>
    </DraftShell>
  );
}
