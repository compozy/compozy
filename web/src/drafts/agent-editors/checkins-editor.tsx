/** S7: one switch, then how often, when and what it looks at, beside a live picture of the day. */
import {
  Clock3,
  Coffee,
  ListChecks,
  MessagesSquare,
  Radar,
  Workflow,
  type LucideIcon,
} from "lucide-react";

import { Button, Empty, MarkdownEditor, Switch, toast } from "@compozy/ui";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import { formatClock } from "../persistent-agents-shared/format-clock";
import { AgentAppPage } from "../persistent-agents-shared/agent-page";
import type { AgentState } from "../persistent-agents-shared/agent-state";
import type { DraftAgent } from "../persistent-agents-shared/fixtures";
import {
  DayRangeSlider,
  EditorCard,
  SettingRow,
  StopSlider,
  ToggleRow,
} from "../persistent-agents-shared/rich-controls";
import { NextCard, RecentCard } from "./checkins-activity";
import {
  FLOOR,
  HOW_OFTEN,
  LUNCH,
  NOW,
  composeFile,
  everyLabel,
  scheduleSummary,
  type CheckInSettings,
  type Standing,
} from "./checkins-model";
import { ConflictCard } from "./conflict-card";
import { EditorHead } from "./editor-head";
import { PLACES, TRY, useCheckInsEditor, type CheckInsVariant } from "./hooks/use-checkins-editor";
import { RawFile } from "./raw-file";
import { UnsavedDock } from "./unsaved-dock";

export type { CheckInsVariant };

const LOOKS_AT: readonly { id: string; label: string; icon: LucideIcon }[] = [
  { id: "task", label: "Tasks that got stuck", icon: ListChecks },
  { id: "session_health", label: "Sessions having trouble", icon: MessagesSquare },
  { id: "sessions_idle", label: "Sessions that went quiet", icon: Clock3 },
  { id: "automations_failed", label: "Automations that failed", icon: Workflow },
  { id: "monitors_fired", label: "Things it was asked to watch", icon: Radar },
];

function agentPose(on: boolean, standing: Standing): AgentState {
  if (standing === "failed") return "failed";
  return on && standing !== "paused" ? "idle" : "paused";
}

function SettingsCard({
  settings,
  limit,
  onChange,
  onLimit,
}: {
  settings: CheckInSettings;
  limit: boolean;
  onChange: (patch: Partial<CheckInSettings>) => void;
  onLimit: (reached: boolean) => void;
}) {
  const off = !settings.on;
  const toggle = (id: string, on: boolean) =>
    onChange({
      looksAt: on ? [...settings.looksAt, id] : settings.looksAt.filter(item => item !== id),
    });
  return (
    <EditorCard className="gap-7">
      <SettingRow title="How often" value={everyLabel(settings.stop)}>
        <StopSlider
          atLimit={limit}
          disabled={off}
          floor={FLOOR}
          floorWord="limit"
          label="How often"
          onChange={stop => {
            onLimit(false);
            onChange({ stop });
          }}
          onLimit={() => onLimit(true)}
          stops={HOW_OFTEN.map(item => item.label)}
          value={settings.stop}
        />
        {limit ? (
          <p className="text-small-body text-warning" role="status">
            Every 15 min is as often as it can check in. Your settings set this limit.
          </p>
        ) : null}
      </SettingRow>
      <SettingRow
        title="When"
        value={`${formatClock(settings.hours[0])} to ${formatClock(settings.hours[1])}`}
      >
        <DayRangeSlider
          disabled={off}
          gap={settings.lunch ? LUNCH : undefined}
          label="When"
          now={NOW}
          onChange={hours => onChange({ hours })}
          value={settings.hours}
        />
        <ToggleRow
          checked={settings.lunch}
          disabled={off}
          icon={Coffee}
          label={`Skip lunch, ${formatClock(LUNCH[0])} to ${formatClock(LUNCH[1])}`}
          onChange={lunch => onChange({ lunch })}
        />
      </SettingRow>
      <SettingRow title="What it looks at">
        <div className="flex flex-col divide-y divide-line-soft">
          {LOOKS_AT.map(item => (
            <ToggleRow
              checked={settings.looksAt.includes(item.id)}
              disabled={off}
              icon={item.icon}
              key={item.id}
              label={item.label}
              onChange={on => toggle(item.id, on)}
            />
          ))}
        </div>
      </SettingRow>
      <SettingRow title="Anything else to keep an eye on">
        <MarkdownEditor
          aria-label="Anything else to keep an eye on"
          disabled={off}
          onValueChange={also => onChange({ also })}
          placeholder="Pull requests waiting on me for more than a day"
          value={settings.also}
        />
      </SettingRow>
    </EditorCard>
  );
}

function NotStartedCard({
  agent,
  nowhere,
  onTurnOn,
}: {
  agent: DraftAgent;
  nowhere: boolean;
  onTurnOn: () => void;
}) {
  return (
    <EditorCard>
      <Empty
        action={
          <Button onClick={nowhere ? undefined : onTurnOn} type="button">
            {nowhere ? "Add it to a project" : "Turn on check-ins"}
          </Button>
        }
        description={
          nowhere
            ? `${agent.label} isn't working in a project yet, so there's nothing to look at.`
            : "Turn this on and it looks around by itself, and only speaks up when something needs you."
        }
        fill={false}
        icon={
          <AgentAvatar
            appearance={agent.appearance}
            label={agent.label}
            quietIdle
            size={40}
            state="paused"
          />
        }
        title={nowhere ? "Nowhere to check in yet" : "No check-ins yet"}
      />
    </EditorCard>
  );
}

export function CheckInsEditor({
  agent,
  variant,
}: {
  agent: DraftAgent;
  variant: CheckInsVariant;
}) {
  const editor = useCheckInsEditor(variant);
  const {
    settings,
    raw,
    limit,
    conflict,
    standing,
    place,
    never,
    nowhere,
    paused,
    next,
    changes,
    activity,
  } = editor;

  return (
    <AgentAppPage agent={agent} hubState="checkins" leaf="Check-ins" width="max-w-5xl">
      <div className="flex flex-col gap-6 py-2">
        <EditorHead
          agent={agent}
          state={agentPose(settings.on, standing)}
          subtitle={paused ? "Waiting for you" : scheduleSummary(settings)}
          title="Check-ins"
        >
          <label className="flex cursor-pointer items-center gap-2.5 text-body text-fg">
            {settings.on && !paused ? "On" : "Off"}
            <Switch
              aria-label="Check-ins"
              checked={settings.on && !paused}
              disabled={nowhere || paused}
              onCheckedChange={editor.turnOn}
            />
          </label>
        </EditorHead>
        {conflict ? (
          <ConflictCard
            agentLabel={agent.label}
            mine={<p>Every 15 min, 08:00 to 18:00</p>}
            onKeepMine={editor.keepMine}
            onUseTheirs={() => {
              editor.useTheirs();
              toast(`Using ${agent.label}'s version`);
            }}
            theirs={<p>Every 1 hr, 09:00 to 18:00</p>}
            title={`${agent.label} changed its schedule while you were editing`}
          />
        ) : null}
        <div className="grid grid-cols-[minmax(0,1fr)_23rem] items-start gap-5">
          <div className="flex min-w-0 flex-col gap-4">
            <SettingsCard
              limit={limit}
              onChange={editor.patch}
              onLimit={editor.setLimit}
              settings={settings}
            />
            <RawFile
              defaultOpen={variant === "advanced"}
              onChange={editor.setRaw}
              value={raw ?? composeFile(settings)}
            />
          </div>
          <div className="sticky top-4 flex min-w-0 flex-col gap-4">
            {never || nowhere ? (
              <NotStartedCard
                agent={agent}
                nowhere={nowhere}
                onTurnOn={() => editor.turnOn(true)}
              />
            ) : (
              <>
                <NextCard
                  activity={activity}
                  initialTry={TRY[variant] ?? "idle"}
                  next={next}
                  onPlace={editor.setPlace}
                  onResume={editor.resume}
                  place={place}
                  places={PLACES}
                  settings={editor.effective}
                  standing={standing}
                />
                <RecentCard activity={activity} />
              </>
            )}
          </div>
        </div>
        <UnsavedDock count={changes} onDiscard={editor.discard} onSave={editor.save} />
      </div>
    </AgentAppPage>
  );
}
