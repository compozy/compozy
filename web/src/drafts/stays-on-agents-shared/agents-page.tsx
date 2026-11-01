/** S2's page: the Agents app content, also the host behind the create wizard. */
import { protoHref } from "@compozy/proto";
import { ListFilter, Plus, Users2 } from "lucide-react";
import { useState, type ReactNode } from "react";

import {
  Button,
  Empty,
  FiltersWithSearch,
  ListingPage,
  ListingToolbar,
  PillGroup,
  SearchInput,
  type Filter,
  type ListingViewMode,
  type TopbarSlotValue,
} from "@compozy/ui";

import {
  buildTeamFilterFields,
  byAttention,
  matchesFilters,
  matchesFocus,
  type TeamFocus,
} from "../stays-on-agents-list/team-filters";
import { StaysOnWelcome, TeamColumns, TeamSection } from "../stays-on-agents-list/team-section";
import type { ScopePickerProps } from "./add-pickers";
import { PROJECTS } from "./agents-fixtures";
import type { AgentState } from "./agent-state";
import { DraftShell } from "./draft-shell";
import type { DraftAgent, PauseReason } from "./fixtures";

export interface AddScopeState {
  agent: string;
  mode: ScopePickerProps["mode"];
}

export interface ListedPlace {
  scope: string;
  state: AgentState;
  pauseReason?: PauseReason;
  /** What it's doing there right now, in plain words. */
  now?: string;
}

export interface ListedAgent extends DraftAgent {
  /** Where a stays-on agent runs, each with its live state. */
  places?: readonly ListedPlace[];
  /** Separates two definitions that share a name. */
  origin?: string;
  /** Daily activity over the last two weeks. */
  activity?: readonly number[];
  recap?: string;
  lastActive?: string;
}

const FOCUS: { value: TeamFocus; label: string }[] = [
  { value: "all", label: "Everyone" },
  { value: "needs-you", label: "Needs you" },
  { value: "working", label: "Working" },
  { value: "paused", label: "Paused" },
];

const FIELDS = buildTeamFilterFields(PROJECTS);
const FILTER_ID = () => `team-filter-${Math.random().toString(36).slice(2, 8)}`;

export interface AgentsPageProps {
  staysOn: readonly ListedAgent[];
  onDemand: readonly ListedAgent[];
  initialView?: ListingViewMode;
  /** `columns` is the variation: rows only, Stays on beside On demand, no Rows/Cards toggle. */
  layout?: "switchable" | "columns";
  query?: string;
  /** Filter chips already on, for review. */
  chips?: readonly Omit<Filter<string>, "id">[];
  overlay?: ReactNode;
  onCreate?: () => void;
  /** Holds one stays-on card's `+` picker open for review. */
  adding?: AddScopeState;
}

/** The Agents window: a team to meet first, a catalog to search second. */
export function AgentsPage({
  staysOn,
  onDemand,
  initialView = "cards",
  layout = "switchable",
  query = "",
  chips: initialChips = [],
  overlay,
  onCreate,
  adding,
}: AgentsPageProps) {
  const [view, setView] = useState<ListingViewMode>(initialView);
  const [draft, setDraft] = useState(query);
  const [focus, setFocus] = useState<TeamFocus>("all");
  const [chips, setChips] = useState<Filter<string>[]>(() =>
    initialChips.map(chip => ({ ...chip, id: FILTER_ID() }))
  );
  const everyone = [...staysOn, ...onDemand];
  const shown = (agents: readonly ListedAgent[]) =>
    agents.filter(agent => matchesFocus(agent, focus) && matchesFilters(agent, draft, chips));
  const shownStaysOn = [...shown(staysOn)].sort(byAttention);
  const shownOnDemand = shown(onDemand);
  const clear = () => {
    setDraft("");
    setChips([]);
    setFocus("all");
  };
  const focusItems = FOCUS.flatMap(item => {
    const count = everyone.filter(agent => matchesFocus(agent, item.value)).length;
    if (item.value === "all") return [item];
    return count > 0 ? [{ ...item, badge: count }] : [];
  });
  const create = onCreate ?? (() => window.location.assign(protoHref("agent-create")));
  const topbar: TopbarSlotValue = {
    count: everyone.length,
    actions: (
      <Button onClick={create} size="sm" type="button" variant="secondary">
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
          <ListingToolbar.Filters>
            <FiltersWithSearch<string>
              allowMultiple={false}
              fields={FIELDS}
              filters={chips}
              onChange={setChips}
              size="sm"
              trigger={
                <Button aria-label="Add filter" size="sm" type="button" variant="quiet">
                  <ListFilter aria-hidden="true" />
                  Filter
                </Button>
              }
            />
          </ListingToolbar.Filters>
        </ListingToolbar.Leading>
        <ListingToolbar.Trailing>
          {focusItems.length > 1 ? (
            <PillGroup items={focusItems} onChange={setFocus} size="sm" value={focus} />
          ) : null}
          {layout === "switchable" ? (
            <ListingToolbar.ViewToggle onChange={setView} value={view} />
          ) : null}
        </ListingToolbar.Trailing>
      </ListingToolbar>
    ),
  };
  const nothing = shownStaysOn.length === 0 && shownOnDemand.length === 0;
  const unfiltered = draft === "" && chips.length === 0 && focus === "all";
  return (
    <DraftShell app="agents" glyph={<Users2 />} overlay={overlay} title="Agents" topbar={topbar}>
      <ListingPage>
        {nothing ? (
          <div className="flex min-h-0 flex-1 items-center justify-center px-6 py-10">
            <Empty
              action={
                <Button onClick={clear} size="sm" type="button" variant="ghost">
                  Clear filters
                </Button>
              }
              description="Try a different search or clear the active filters."
              icon={Users2}
              title="No agents match"
            />
          </div>
        ) : layout === "columns" ? (
          <TeamColumns
            adding={adding}
            onDemand={shownOnDemand}
            staysOn={shownStaysOn}
            welcome={unfiltered ? <StaysOnWelcome onCreate={create} /> : undefined}
          />
        ) : (
          <div className="flex flex-col gap-8">
            {shownStaysOn.length > 0 ? (
              <TeamSection adding={adding} agents={shownStaysOn} label="Stays on" view={view} />
            ) : unfiltered ? (
              <StaysOnWelcome onCreate={create} />
            ) : null}
            {shownOnDemand.length > 0 ? (
              <TeamSection agents={shownOnDemand} label="On demand" pageSize={12} view={view} />
            ) : null}
          </div>
        )}
      </ListingPage>
    </DraftShell>
  );
}
