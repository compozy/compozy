import { Clock3, Eye } from "@compozy/ui/icons";
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

import type { WorkspacePayload } from "@/systems/workspace";

import { AGENT_PLACE_LATER, type AgentCreateDialogDraft } from "../lib/agent-create-draft";
import { ACTIVE_INSTANCE_LIMIT, GLOBAL_SCOPE } from "../lib/agent-instance";
import { agentPlaceLabel } from "../lib/agent-place";
import {
  AgentGlobalMark,
  AgentLaterMark,
  AgentLimitMeter,
  AgentPlaceList,
  AgentProjectMark,
} from "./agent-add-pickers";
import {
  AgentCreateRuntimeFields,
  type AgentCreateRuntimeFieldsProps,
} from "./agent-create-runtime-fields";

function PlaceMark({ place, label }: { place: string; label: string }) {
  if (place === GLOBAL_SCOPE) return <AgentGlobalMark />;
  if (place === AGENT_PLACE_LATER) return <AgentLaterMark />;
  return <AgentProjectMark name={label} />;
}

function WherePicker({
  value,
  onChange,
  workspaces,
  labelId,
}: {
  value: string;
  onChange: (next: string) => void;
  workspaces: readonly WorkspacePayload[];
  labelId: string;
}) {
  const places = [{ id: GLOBAL_SCOPE, name: "Global" }, ...workspaces];
  const label = agentPlaceLabel(value, workspaces);
  return (
    <CommandSelect key={value}>
      <CommandSelectTrigger
        aria-labelledby={labelId}
        className="h-button-lg rounded-pill border-transparent bg-card shadow-card hover:bg-selected"
        data-testid="agent-create-where"
        selected
      >
        <span className="flex min-w-0 flex-1 items-center gap-2.5">
          <PlaceMark label={label} place={value} />
          <span className="truncate text-small-body font-medium text-fg">{label}</span>
        </span>
      </CommandSelectTrigger>
      <CommandSelectShell inputPlaceholder="Search projects…">
        <AgentPlaceList
          laterValue={AGENT_PLACE_LATER}
          onChange={onChange}
          places={places}
          value={value}
        />
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
    <div
      className="flex flex-col gap-2 rounded-md bg-card p-3 shadow-card"
      data-testid="agent-create-limit"
    >
      <AgentLimitMeter />
      <p className="text-small-body text-muted-foreground">
        <span className="text-fg">
          {ACTIVE_INSTANCE_LIMIT} of {ACTIVE_INSTANCE_LIMIT} agents are staying on.
        </span>{" "}
        Create this one now, and start it once you've paused another.
      </p>
    </div>
  );
}

function SignInNote({ provider, onSignIn }: { provider: string; onSignIn: () => void }) {
  return (
    <div className="flex items-center gap-3 rounded-md bg-card p-3 shadow-card">
      <p className="min-w-0 flex-1 text-small-body text-muted-foreground">
        <span className="text-fg">{provider} isn't signed in,</span> so it can't start yet. Sign in,
        or pick another model.
      </p>
      <Button onClick={onSignIn} size="sm" type="button" variant="secondary">
        Sign in
      </Button>
    </div>
  );
}

export interface AgentCreateRunSettingsProps extends AgentCreateRuntimeFieldsProps {
  atInstanceLimit: boolean;
  signedOut: string | null;
  workspaces: readonly WorkspacePayload[];
}

/** The right rail: how it runs, where it works, and which model. */
export function AgentCreateRunSettings({
  atInstanceLimit,
  signedOut,
  workspaces,
  ...runtime
}: AgentCreateRunSettingsProps) {
  const { draft, onDraftChange, onOpenProviderSettings } = runtime;
  const ids = useId();
  const modeId = `${ids}-mode`;
  const whereId = `${ids}-where`;
  return (
    <>
      <div aria-labelledby={modeId} className="flex flex-col gap-2" role="radiogroup">
        <Eyebrow id={modeId}>How it works</Eyebrow>
        <RadioCard
          data-testid="agent-create-kind-on-demand"
          icon={Clock3}
          iconWellSize="lg"
          onSelect={() => onDraftChange({ ...draft, kind: "on_demand" })}
          selected={draft.kind === "on_demand"}
          title="On demand, runs when you ask"
        />
        <RadioCard
          data-testid="agent-create-kind-stays-on"
          icon={Eye}
          iconWellSize="lg"
          onSelect={() => onDraftChange({ ...draft, kind: "stays_on" })}
          selected={draft.kind === "stays_on"}
          title="Stays on and watches for you"
        />
      </div>
      {draft.kind === "stays_on" ? (
        <Labeled id={whereId} label="Where it works">
          {atInstanceLimit ? (
            <LimitCard />
          ) : (
            <WherePicker
              labelId={whereId}
              onChange={place => onDraftChange({ ...draft, place })}
              value={draft.place}
              workspaces={workspaces}
            />
          )}
        </Labeled>
      ) : null}
      <div className="flex min-w-0 flex-col gap-2">
        <AgentCreateRuntimeFields {...runtime} />
        {signedOut ? <SignInNote onSignIn={onOpenProviderSettings} provider={signedOut} /> : null}
      </div>
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

/** The same facts as the choices above, for an agent the screen no longer edits. */
export function AgentCreateRunFacts({
  draft,
  workspaces,
}: {
  draft: AgentCreateDialogDraft;
  workspaces: readonly WorkspacePayload[];
}) {
  const stays = draft.kind === "stays_on";
  const ModeIcon = stays ? Eye : Clock3;
  const place = agentPlaceLabel(draft.place, workspaces);
  return (
    <ImmutableIdentity
      className="[&>div]:items-center"
      rows={[
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
                value: <Fact mark={<PlaceMark label={place} place={draft.place} />}>{place}</Fact>,
              },
            ]
          : []),
        ...(draft.provider
          ? [
              {
                label: "Model",
                value: (
                  <Fact
                    mark={
                      <KindIcon
                        className="shrink-0"
                        kind={draft.provider}
                        registry={providerKindIconRegistry}
                        size="sm"
                        tone="default"
                      />
                    }
                  >
                    {draft.model || draft.provider}
                  </Fact>
                ),
              },
            ]
          : []),
      ]}
    />
  );
}
