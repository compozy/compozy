/** Where the agent works: the live cards, searchable once there are too many to scan. */
import { ChevronRight, Plus } from "@compozy/ui/icons";
import { useState } from "react";

import { Alert, AlertDescription, Button, Empty, SearchInput, Section } from "@compozy/ui";

import { ScopePicker, type ScopePickerProps } from "../stays-on-agents-shared/add-pickers";
import { SCOPE_LABEL, WORKSPACES } from "../stays-on-agents-shared/fixtures";
import type { PlaceActions } from "./hooks/use-places";
import { usePlaces } from "./hooks/use-places";
import { PlaceCard } from "./place-card";
import { canResume } from "./place-edits";
import type { ProfilePlace } from "./profile-fixtures";
import { goToProfile } from "./profile-nav";

const ATTENTION: Record<string, number> = { "needs-you": 0, failed: 1, working: 2, paused: 3 };
const HOME_CARDS = 4;

export function AddPlace({
  places,
  mode = "scopes",
  open = false,
  provider = "Claude",
  primary = false,
}: {
  places: readonly ProfilePlace[];
  mode?: ScopePickerProps["mode"];
  open?: boolean;
  provider?: string;
  primary?: boolean;
}) {
  const taken = new Set(places.map(place => place.scope));
  const scopes = ["@user", ...WORKSPACES].filter(scope => !taken.has(scope));
  return (
    <ScopePicker
      defaultOpen={open}
      mode={mode}
      provider={provider}
      scopes={mode === "none" ? [] : scopes}
      trigger={
        <Button
          // Optical: a link in a section head sits on the head's line and the panel's edge.
          className={primary ? undefined : "-my-1 -mr-2"}
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

function sorted(places: readonly ProfilePlace[]) {
  return [...places].sort(
    (left, right) => (ATTENTION[left.state] ?? 4) - (ATTENTION[right.state] ?? 4)
  );
}

/** The home's section: the few places that matter most, with the rest one click away. */
export function WorkingNow({
  places,
  actions,
  allState = "where-many",
}: {
  places: readonly ProfilePlace[];
  actions: PlaceActions;
  allState?: string;
}) {
  const shown = sorted(places).slice(0, HOME_CARDS);
  const more = places.length - shown.length;
  return (
    <Section
      count={places.length}
      label={
        places.some(place => canResume(place.pauseReason))
          ? "Where it's working"
          : "Where it worked"
      }
      right={
        !places.some(place => canResume(place.pauseReason)) ? null : more > 0 ? (
          <Button
            className="-my-1 -mr-2"
            onClick={() => goToProfile(allState)}
            size="sm"
            type="button"
            variant="link"
          >
            See all {places.length}
            <ChevronRight aria-hidden="true" />
          </Button>
        ) : (
          <AddPlace places={places} />
        )
      }
    >
      <div className="grid grid-cols-1 gap-3 @2xl:grid-cols-2">
        {shown.map(place => (
          <PlaceCard key={place.scope} place={place} {...actions} />
        ))}
      </div>
    </Section>
  );
}

/** The full page: every place, searchable. */
export function WhereItWorks({
  places,
  capped = false,
}: {
  places: readonly ProfilePlace[];
  capped?: boolean;
}) {
  const [query, setQuery] = useState("");
  const { places: current, actions } = usePlaces(places);
  const needle = query.trim().toLowerCase();
  const shown = sorted(current).filter(place =>
    SCOPE_LABEL(place.scope).toLowerCase().includes(needle)
  );
  return (
    <div className="@container flex flex-col gap-4 py-2">
      {capped ? (
        <Alert variant="warning">
          <AlertDescription>
            You already have 10 agents running, which is your limit. Pause one to start another.
          </AlertDescription>
        </Alert>
      ) : null}
      {current.length > 6 ? (
        <SearchInput
          aria-label="Search projects"
          onChange={setQuery}
          placeholder={`Search ${current.length} projects`}
          value={query}
        />
      ) : null}
      {current.length === 0 ? (
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
            <PlaceCard key={place.scope} place={place} {...actions} />
          ))}
        </div>
      )}
    </div>
  );
}
