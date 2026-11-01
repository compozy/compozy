/** S4: the agent's home. Who it is, what it's doing, and what it has done for you. */
import { protoHref } from "@compozy/proto";
import { ArrowRight, MessageSquarePlus, MessagesSquare, Moon, Play } from "lucide-react";
import { useState } from "react";

import { Alert, AlertDescription, AlertTitle, Button, Empty, Panel, Section } from "@compozy/ui";

import type { AgentState } from "../stays-on-agents-shared/agent-state";
import { PAUSE_PLAIN } from "../stays-on-agents-shared/agents-fixtures";
import { SCOPE_LABEL } from "../stays-on-agents-shared/fixtures";
import { usePlaces } from "./hooks/use-places";
import { canResume } from "./place-edits";
import { Hero, type HeroEdit } from "./profile-hero";
import { ActivitySection, PlanCard, RhythmSection } from "./profile-activity";
import type { ProfileData, ProfilePlace } from "./profile-fixtures";
import { ProfilePanels } from "./profile-panels";
import { SetupPanel, type ProfilePanel } from "./profile-setup";
import { FindingsPanel, RecentWork } from "./profile-work";
import { AddPlace, WorkingNow } from "./where-it-works";

const ORDER: AgentState[] = ["needs-you", "failed", "working", "paused"];

function overall(data: ProfileData, places: readonly ProfilePlace[]): AgentState {
  if (data.agent.kind === "on_demand") return data.agent.active > 0 ? "working" : "idle";
  return ORDER.find(state => places.some(place => place.state === state)) ?? "idle";
}

function headline(data: ProfileData, places: readonly ProfilePlace[], state: AgentState) {
  if (data.agent.kind === "on_demand") {
    return state === "working" ? `${data.agent.active} running now` : "Starts when you ask";
  }
  if (places.length === 0) return "Waiting for its first project";
  const place = places.find(entry => entry.state === state);
  const where = place ? SCOPE_LABEL(place.scope) : "";
  if (state === "needs-you") return `${place?.now} in ${where}`;
  if (state === "working") return `${place?.now ?? "Working"} in ${where}`;
  if (state === "paused") {
    const stopped = places.filter(entry => entry.state === "paused");
    return stopped.length > 1 ? `Paused in ${stopped.length} projects` : `Paused in ${where}`;
  }
  return `Resting in ${places.length} ${places.length === 1 ? "project" : "projects"}`;
}

function MainAction({
  data,
  places,
  state,
  onResume,
}: {
  data: ProfileData;
  places: readonly ProfilePlace[];
  state: AgentState;
  onResume: () => void;
}) {
  if (data.agent.kind === "on_demand") {
    return (
      <Button size="sm" type="button" variant="secondary">
        <MessageSquarePlus aria-hidden="true" />
        New session
      </Button>
    );
  }
  if (places.length === 0) return <AddPlace places={places} primary />;
  if (state === "needs-you") {
    return (
      <Button
        onClick={() => window.location.assign(protoHref("stays-on-reports", "proposal"))}
        size="sm"
        type="button"
      >
        See what it needs
        <ArrowRight aria-hidden="true" />
      </Button>
    );
  }
  if (state === "paused") {
    if (!places.some(place => canResume(place.pauseReason))) return null;
    return (
      <Button onClick={onResume} size="sm" type="button" variant="secondary">
        <Play aria-hidden="true" />
        Resume
      </Button>
    );
  }
  return (
    <Button size="sm" type="button" variant="secondary">
      <MessagesSquare aria-hidden="true" />
      Open session
    </Button>
  );
}

function Stopped({ places }: { places: readonly ProfilePlace[] }) {
  const stopped = places.find(place => place.pauseReason && place.pauseReason !== "manual");
  if (!stopped?.pauseReason) return null;
  const resumable = canResume(stopped.pauseReason);
  return (
    <Alert variant="warning">
      <AlertTitle>{resumable ? "Check-ins kept failing" : "It can't resume"}</AlertTitle>
      <AlertDescription>{PAUSE_PLAIN[stopped.pauseReason]}</AlertDescription>
    </Alert>
  );
}

function Invalid() {
  return (
    <Alert variant="warning">
      <AlertTitle>This agent can't start</AlertTitle>
      <AlertDescription>
        Line 6 of its setup file has a mistake. Fix it in How it behaves.
      </AlertDescription>
    </Alert>
  );
}

function FirstDay({ name }: { name: string }) {
  return (
    <Section label="How it's been going">
      <Panel bodyClassName="px-6 py-14">
        <Empty
          description="Add it to a project and everything it does, finds and costs will show up here."
          fill={false}
          icon={Moon}
          title={`${name} hasn't started yet`}
        />
      </Panel>
    </Section>
  );
}

export function ProfileHome({
  data,
  invalid = false,
  panel: initialPanel = null,
  edit,
}: {
  data: ProfileData;
  invalid?: boolean;
  panel?: ProfilePanel | null;
  edit?: HeroEdit;
}) {
  const { places, actions, resumeAll } = usePlaces(data.places);
  const [panel, setPanel] = useState<ProfilePanel | null>(initialPanel);
  const staysOn = data.agent.kind === "stays_on";
  const state = overall(data, places);
  return (
    <div className="@container flex flex-col gap-6">
      <Hero
        action={<MainAction data={data} onResume={resumeAll} places={places} state={state} />}
        data={data}
        edit={edit}
        line={headline(data, places, state)}
        state={state}
      />
      {invalid ? <Invalid /> : null}
      <Stopped places={places} />
      <div className="grid grid-cols-1 items-start gap-5 @5xl:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="flex min-w-0 flex-col gap-6">
          {staysOn && places.length > 0 ? <WorkingNow actions={actions} places={places} /> : null}
          {data.fresh ? (
            <FirstDay name={data.agent.label} />
          ) : (
            <>
              <ActivitySection data={data} />
              <RhythmSection data={data} />
            </>
          )}
        </div>
        <aside className="flex min-w-0 flex-col gap-6">
          <FindingsPanel data={data} />
          <RecentWork data={data} />
          {data.fresh ? null : <PlanCard />}
          <SetupPanel agent={data.agent} onOpen={setPanel} readOnly={data.readOnly} />
        </aside>
      </div>
      <ProfilePanels
        agent={data.agent}
        onClose={() => setPanel(null)}
        panel={panel}
        readOnly={data.readOnly ?? false}
      />
    </div>
  );
}
