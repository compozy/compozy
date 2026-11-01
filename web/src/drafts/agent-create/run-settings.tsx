/** The right rail: how it runs, where it works, which model, and the live preview. */
import { Clock3, Eye, Puzzle } from "@compozy/ui/icons";
import { useId, type ReactNode } from "react";

import {
  Button,
  CommandSelect,
  CommandSelectShell,
  CommandSelectTrigger,
  Eyebrow,
  ImmutableIdentity,
  KindIcon,
  RadioCard,
  providerKindIconRegistry,
} from "@compozy/ui";

import { RuntimeSelector } from "@/systems/runtime/components/runtime-selector";

import {
  GlobalMark,
  LaterMark,
  LimitMeter,
  PlaceList,
  ProjectMark,
} from "../stays-on-agents-shared/add-pickers";
import { WORKSPACES } from "../stays-on-agents-shared/fixtures";

import {
  ACTIVE_LIMIT,
  MODELS,
  MORE_PROJECTS,
  PLACE_GLOBAL,
  PLACE_LATER,
  modelLabel,
  placeLabel,
  providersFor,
  type AgentMode,
  type Draft,
  type Place,
} from "./model";

const PLACES = [PLACE_GLOBAL, ...WORKSPACES, ...MORE_PROJECTS];

function PlaceMark({ place }: { place: Place }) {
  if (place === PLACE_GLOBAL) return <GlobalMark />;
  if (place === PLACE_LATER) return <LaterMark />;
  return <ProjectMark name={place} />;
}

function WherePicker({
  value,
  onChange,
  defaultOpen,
  disabled,
  labelId,
}: {
  value: Place;
  onChange: (next: Place) => void;
  defaultOpen?: boolean;
  disabled?: boolean;
  labelId: string;
}) {
  return (
    <CommandSelect defaultOpen={defaultOpen} key={value}>
      <CommandSelectTrigger
        aria-labelledby={labelId}
        className="h-button-lg rounded-pill border-transparent bg-card shadow-card hover:bg-selected"
        disabled={disabled}
        selected
      >
        <span className="flex min-w-0 flex-1 items-center gap-2.5">
          <PlaceMark place={value} />
          <span className="truncate text-small-body font-medium text-fg">{placeLabel(value)}</span>
        </span>
      </CommandSelectTrigger>
      <CommandSelectShell inputPlaceholder="Search projects…">
        <PlaceList later onChange={onChange} places={PLACES} value={value} />
      </CommandSelectShell>
    </CommandSelect>
  );
}

function Labeled({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col gap-2">
      <Eyebrow id={id}>{label}</Eyebrow>
      {children}
    </div>
  );
}

function LimitCard() {
  return (
    <div className="flex flex-col gap-2 rounded-md bg-card p-3 shadow-card">
      <LimitMeter />
      <p className="text-small-body text-muted-foreground">
        <span className="text-fg">
          {ACTIVE_LIMIT} of {ACTIVE_LIMIT} agents are staying on.
        </span>{" "}
        Create this one now, and start it once you've paused another.
      </p>
    </div>
  );
}

function SignInNote({ provider }: { provider: string }) {
  return (
    <div className="flex items-center gap-3 rounded-md bg-card p-3 shadow-card">
      <p className="min-w-0 flex-1 text-small-body text-muted-foreground">
        <span className="text-fg">{provider} isn't signed in,</span> so it can't start yet. Sign in,
        or pick another model.
      </p>
      <Button size="sm" type="button" variant="secondary">
        Sign in
      </Button>
    </div>
  );
}

export interface RunSettingsProps {
  draft: Draft;
  onMode: (next: AgentMode) => void;
  onPlace: (next: Place) => void;
  onRuntime: (next: Draft["runtime"]) => void;
  openPlaces?: boolean;
  atLimit?: boolean;
  signedOut?: string;
}

export function RunSettings({
  draft,
  onMode,
  onPlace,
  onRuntime,
  openPlaces,
  atLimit,
  signedOut,
}: RunSettingsProps) {
  const ids = useId();
  const modeId = `${ids}-mode`;
  const whereId = `${ids}-where`;
  const modelId = `${ids}-model`;
  return (
    <>
      <div aria-labelledby={modeId} className="flex flex-col gap-2" role="radiogroup">
        <Eyebrow id={modeId}>How it works</Eyebrow>
        <RadioCard
          icon={Clock3}
          iconWellSize="lg"
          onSelect={() => onMode("on-demand")}
          selected={draft.mode === "on-demand"}
          title="On demand, runs when you ask"
        />
        <RadioCard
          icon={Eye}
          iconWellSize="lg"
          onSelect={() => onMode("stays-on")}
          selected={draft.mode === "stays-on"}
          title="Stays on and watches for you"
        />
      </div>
      {draft.mode === "stays-on" ? (
        <Labeled id={whereId} label="Where it works">
          {atLimit ? (
            <LimitCard />
          ) : (
            <WherePicker
              defaultOpen={openPlaces}
              labelId={whereId}
              onChange={onPlace}
              value={draft.place}
            />
          )}
        </Labeled>
      ) : null}
      <Labeled id={modelId} label="Model">
        <RuntimeSelector
          ariaLabelledby={modelId}
          className="w-full justify-start bg-card shadow-card [&>svg:last-child]:ml-auto"
          models={MODELS}
          onChange={onRuntime}
          providers={providersFor(signedOut)}
          value={draft.runtime}
        />
        {signedOut ? <SignInNote provider={signedOut} /> : null}
      </Labeled>
    </>
  );
}

function Fact({ mark, children }: { mark: ReactNode; children: ReactNode }) {
  return (
    <span className="flex min-w-0 items-center gap-2">
      {mark}
      <span className="min-w-0 truncate">{children}</span>
    </span>
  );
}

const FACT_ICON = "size-4 shrink-0 text-muted-foreground";

/** The same facts as the choices above, for an agent that can't be changed here. */
export function RunFacts({ draft, owner }: { draft: Draft; owner?: string }) {
  const stays = draft.mode === "stays-on";
  const ModeIcon = stays ? Eye : Clock3;
  return (
    <ImmutableIdentity
      className="[&>div]:items-center"
      rows={[
        ...(owner
          ? [
              {
                label: "Comes from",
                value: (
                  <Fact mark={<Puzzle aria-hidden="true" className={FACT_ICON} />}>{owner}</Fact>
                ),
              },
            ]
          : []),
        {
          label: "How it works",
          value: (
            <Fact mark={<ModeIcon aria-hidden="true" className={FACT_ICON} />}>
              {stays ? "Stays on" : "On demand"}
            </Fact>
          ),
        },
        ...(stays
          ? [
              {
                label: "Where it works",
                value: (
                  <Fact mark={<PlaceMark place={draft.place} />}>{placeLabel(draft.place)}</Fact>
                ),
              },
            ]
          : []),
        {
          label: "Model",
          value: (
            <Fact
              mark={
                <KindIcon
                  className="shrink-0"
                  kind={draft.runtime.provider}
                  registry={providerKindIconRegistry}
                  size="sm"
                  tone="default"
                />
              }
            >
              {modelLabel(draft.runtime)}
            </Fact>
          ),
        },
      ]}
    />
  );
}
