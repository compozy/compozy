/** S4: the agent's home. Who it is, what it's doing, and what it has done for you. */
import { protoHref } from "@compozy/proto";
import { ArrowRight, MessageSquarePlus, MessagesSquare, Moon, Play } from "lucide-react";
import { useState, type ComponentProps, type ReactNode } from "react";

import { Alert, AlertDescription, AlertTitle, Button, Empty, Pill, StatusDot } from "@compozy/ui";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import { KindPill } from "../persistent-agents-shared/agent-page";
import type { AgentState } from "../persistent-agents-shared/agent-state";
import { PAUSE_PLAIN } from "../persistent-agents-shared/agents-fixtures";
import { SCOPE_LABEL } from "../persistent-agents-shared/fixtures";
import type { IdentityEditor } from "./identity-editor";
import { usePlaces } from "./hooks/use-places";
import { canResume } from "./place-edits";
import { ActivitySection, PlanCard, RhythmSection } from "./profile-activity";
import type { ProfileData, ProfilePlace } from "./profile-fixtures";
import { ProfilePanels } from "./profile-panels";
import { SetupPanel, type ProfilePanel } from "./profile-setup";
import { FindingsPanel, RecentWork } from "./profile-work";
import { AddPlace, WorkingNow } from "./where-it-works";

const ORDER: AgentState[] = ["needs-you", "failed", "working", "paused"];

function overall(data: ProfileData, places: readonly ProfilePlace[]): AgentState {
  if (data.agent.kind === "ephemeral") return data.agent.active > 0 ? "working" : "idle";
  return ORDER.find(state => places.some(place => place.state === state)) ?? "idle";
}

function headline(data: ProfileData, places: readonly ProfilePlace[], state: AgentState) {
  if (data.agent.kind === "ephemeral") {
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
  if (data.agent.kind === "ephemeral") {
    return (
      <Button type="button">
        <MessageSquarePlus aria-hidden="true" />
        New session
      </Button>
    );
  }
  if (places.length === 0) return <AddPlace places={places} primary />;
  if (state === "needs-you") {
    return (
      <Button
        onClick={() => window.location.assign(protoHref("persistent-reports", "proposal"))}
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
      <Button onClick={onResume} type="button">
        <Play aria-hidden="true" />
        Resume
      </Button>
    );
  }
  return (
    <Button type="button">
      <MessagesSquare aria-hidden="true" />
      Open session
    </Button>
  );
}

function Hero({
  data,
  places,
  action,
}: {
  data: ProfileData;
  places: readonly ProfilePlace[];
  action: ReactNode;
}) {
  const state = overall(data, places);
  const live = state === "working" || state === "needs-you";
  return (
    <header className="flex flex-wrap items-center gap-6 py-2">
      <AgentAvatar
        appearance={data.agent.appearance}
        label={data.agent.label}
        provider={data.agent.provider}
        quietIdle
        size={112}
        state={state}
      />
      <div className="flex min-w-0 flex-1 flex-col gap-1.5">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <h1 className="min-w-0 text-display-2xl font-semibold text-fg-strong">
            {data.agent.label}
          </h1>
          <KindPill agent={data.agent} />
          {data.readOnly ? (
            <Pill form="plain" size="sm">
              From {data.agent.owner}
            </Pill>
          ) : null}
        </div>
        <p className="text-body text-muted">{data.agent.description}</p>
        <p className={`flex items-center gap-2 text-body ${live ? "text-fg" : "text-muted"}`}>
          <StatusDot
            size="sm"
            tone={state === "working" ? "success" : state === "needs-you" ? "accent" : "faint"}
          />
          {headline(data, places, state)}
        </p>
      </div>
      <div className="flex shrink-0 items-center gap-2">{action}</div>
    </header>
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
    <div className="flex items-center justify-center rounded-lg bg-card px-6 py-14 shadow-card">
      <Empty
        description="Add it to a project and everything it does, finds and costs will show up here."
        icon={Moon}
        title={`${name} hasn't started yet`}
      />
    </div>
  );
}

export function ProfileHome({
  data,
  invalid = false,
  panel: initialPanel = null,
  identity,
}: {
  data: ProfileData;
  invalid?: boolean;
  panel?: ProfilePanel | null;
  identity?: Partial<ComponentProps<typeof IdentityEditor>>;
}) {
  const { places, actions, resumeAll } = usePlaces(data.places);
  const [panel, setPanel] = useState<ProfilePanel | null>(initialPanel);
  const stayOn = data.agent.kind === "persistent";
  return (
    <div className="@container flex flex-col gap-6 pt-2 pb-16">
      <Hero
        action={
          <MainAction
            data={data}
            onResume={resumeAll}
            places={places}
            state={overall(data, places)}
          />
        }
        data={data}
        places={places}
      />
      {invalid ? <Invalid /> : null}
      <Stopped places={places} />
      <div className="grid grid-cols-1 gap-6 @5xl:grid-cols-[minmax(0,1fr)_22rem]">
        <div className="flex min-w-0 flex-col gap-6">
          {stayOn && places.length > 0 ? <WorkingNow actions={actions} places={places} /> : null}
          {data.fresh ? (
            <FirstDay name={data.agent.label} />
          ) : (
            <>
              <ActivitySection data={data} />
              <RhythmSection data={data} />
            </>
          )}
        </div>
        <aside className="flex min-w-0 flex-col gap-4">
          <FindingsPanel data={data} />
          <RecentWork data={data} />
          {data.fresh ? null : <PlanCard />}
          <SetupPanel agent={data.agent} onOpen={setPanel} readOnly={data.readOnly} />
        </aside>
      </div>
      <ProfilePanels
        agent={data.agent}
        identity={identity}
        onClose={() => setPanel(null)}
        panel={panel}
        readOnly={data.readOnly ?? false}
      />
    </div>
  );
}
