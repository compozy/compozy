/** Where the agent works: the live cards, searchable once there are too many to scan. */
import { ChevronRight, Plus } from "@compozy/ui/icons";
import { useState } from "react";

import { Alert, AlertDescription, Button, Empty, SearchInput, Section } from "@compozy/ui";

import { agentCanResume, agentPlacesByAttention } from "../../lib/agent-profile";
import type { AgentTeamPlace } from "../../lib/agent-team";
import { ACTIVE_INSTANCE_LIMIT } from "../../lib/agent-instance";
import { AgentScopePicker } from "../agent-add-pickers";
import type { AgentAddPlaceActions } from "../agent-team/agent-team-parts";
import { AgentPlaceCard, type AgentPlaceActions } from "./agent-place-card";

const HOME_CARDS = 4;

export function AgentAddPlace({
  name,
  provider,
  places,
  add,
  primary = false,
}: {
  name: string;
  provider: string | null;
  places: readonly AgentTeamPlace[];
  add: AgentAddPlaceActions;
  primary?: boolean;
}) {
  const taken = new Set(places.map(place => place.scope));
  return (
    <AgentScopePicker
      capped={add.capped}
      onPick={place => add.onAdd(name, place)}
      onSignIn={add.onSignIn}
      scopes={add.places.filter(place => !taken.has(place.id))}
      signedOut={add.signedOut.get(provider ?? "") ?? null}
      trigger={
        <Button
          // Optical: a link in a section head sits on the head's line and the panel's edge.
          className={primary ? undefined : "-my-1 -mr-2"}
          data-testid="agent-add-place"
          size="sm"
          type="button"
          variant={primary ? "default" : "link"}
        >
          <Plus aria-hidden="true" />
          Add to a project
        </Button>
      }
    />
  );
}

/** The home's section: the few places that matter most, with the rest one click away. */
export function AgentWorkingNow({
  name,
  provider,
  places,
  actions,
  add,
  onSeeAll,
}: {
  name: string;
  provider: string | null;
  places: readonly AgentTeamPlace[];
  actions: AgentPlaceActions;
  add: AgentAddPlaceActions;
  onSeeAll: () => void;
}) {
  const shown = agentPlacesByAttention(places).slice(0, HOME_CARDS);
  const more = places.length - shown.length;
  const usable = places.some(place => agentCanResume(place.pauseReason));
  return (
    <Section
      count={places.length}
      label={usable ? "Where it's working" : "Where it worked"}
      right={
        !usable ? null : more > 0 ? (
          <Button className="-my-1 -mr-2" onClick={onSeeAll} size="sm" type="button" variant="link">
            See all {places.length}
            <ChevronRight aria-hidden="true" />
          </Button>
        ) : (
          <AgentAddPlace add={add} name={name} places={places} provider={provider} />
        )
      }
    >
      <div className="grid grid-cols-1 gap-3 @2xl:grid-cols-2">
        {shown.map(place => (
          <AgentPlaceCard key={place.scope} place={place} {...actions} />
        ))}
      </div>
    </Section>
  );
}

/** The full page: every place, searchable. */
export function AgentWhereItWorks({
  places,
  actions,
  capped,
}: {
  places: readonly AgentTeamPlace[];
  actions: AgentPlaceActions;
  capped: boolean;
}) {
  const [query, setQuery] = useState("");
  const needle = query.trim().toLowerCase();
  const shown = agentPlacesByAttention(places).filter(place =>
    place.label.toLowerCase().includes(needle)
  );
  return (
    <div className="@container flex flex-col gap-4 py-2">
      {capped ? (
        <Alert variant="warning">
          <AlertDescription>
            You already have {ACTIVE_INSTANCE_LIMIT} agents running, which is your limit. Pause one
            to start another.
          </AlertDescription>
        </Alert>
      ) : null}
      {places.length > 6 ? (
        <SearchInput
          aria-label="Search projects"
          onChange={setQuery}
          placeholder={`Search ${places.length} projects`}
          value={query}
        />
      ) : null}
      {places.length === 0 ? (
        <Empty
          description="Add it to a project and it starts checking in there by itself."
          icon={Plus}
          title="Not working anywhere yet"
        />
      ) : shown.length === 0 ? (
        <Empty description="Try another project name." title="No project matches" />
      ) : (
        <div className="grid grid-cols-1 gap-3 @2xl:grid-cols-2">
          {shown.map(place => (
            <AgentPlaceCard key={place.scope} place={place} {...actions} />
          ))}
        </div>
      )}
    </div>
  );
}
