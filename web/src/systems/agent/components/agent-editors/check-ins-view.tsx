/** S7: one switch, then how often, when and what it looks at, beside a live picture of the day. */
import {
  Clock3,
  Coffee,
  ListChecks,
  MessagesSquare,
  Radar,
  Workflow,
  type LucideIcon,
} from "@compozy/ui/icons";
import type { AgentAvatarState } from "@compozy/ui";

import { Button, Empty, MarkdownEditor, Switch, toast } from "@compozy/ui";

import type { CheckInsEditorModel } from "../../hooks/use-check-ins-editor";
import {
  HOW_OFTEN,
  LUNCH,
  everyLabel,
  formatClock,
  scheduleSummary,
  type CheckInSettings,
} from "../../lib/agent-check-in-form";
import { agentEditorMark } from "../../lib/agent-editor-mark";
import type { AgentPayload } from "../../types";
import { AgentMark } from "../agent-mark";
import { NextCard, RecentCard } from "./check-ins-activity";
import { ConflictCard } from "./conflict-card";
import { EditorHead } from "./editor-head";
import { FileBlockNotice, ReadOnlyNotice } from "./editor-states";
import { RawFile } from "./raw-file";
import { DayRangeSlider, EditorCard, SettingRow, StopSlider, ToggleRow } from "./rich-controls";
import { UnsavedDock } from "./unsaved-dock";

const LOOKS_AT: readonly { id: string; label: string; icon: LucideIcon }[] = [
  { id: "task", label: "Tasks that got stuck", icon: ListChecks },
  { id: "session_health", label: "Sessions having trouble", icon: MessagesSquare },
  { id: "sessions_idle", label: "Sessions that went quiet", icon: Clock3 },
  { id: "automations_failed", label: "Automations that failed", icon: Workflow },
  { id: "monitors_fired", label: "Things it was asked to watch", icon: Radar },
];

function SettingsCard({ model, disabled }: { model: CheckInsEditorModel; disabled: boolean }) {
  const { settings, limit, floor, document, nowHour } = model;
  const patch = (next: Partial<CheckInSettings>) => document.setForm({ ...document.form, ...next });
  const toggle = (id: string, on: boolean) =>
    patch({
      looksAt: on ? [...settings.looksAt, id] : settings.looksAt.filter(item => item !== id),
    });
  return (
    <EditorCard className="gap-7">
      <SettingRow title="How often" value={everyLabel(settings.stop)}>
        <StopSlider
          atLimit={limit}
          disabled={disabled}
          floor={floor}
          floorWord="limit"
          label="How often"
          onChange={stop => {
            model.setLimit(false);
            patch({ stop });
          }}
          onLimit={() => model.setLimit(true)}
          stops={HOW_OFTEN.map(item => item.label)}
          value={settings.stop}
        />
        {limit ? (
          <p className="text-small-body text-warning" role="status">
            {`${everyLabel(floor)} is as often as it can check in. Your settings set this limit.`}
          </p>
        ) : null}
      </SettingRow>
      <SettingRow
        title="When"
        value={`${formatClock(settings.hours[0])} to ${formatClock(settings.hours[1])}`}
      >
        <DayRangeSlider
          disabled={disabled}
          gap={settings.lunch ? LUNCH : undefined}
          label="When"
          now={nowHour}
          onChange={hours => patch({ hours })}
          value={settings.hours}
        />
        <ToggleRow
          checked={settings.lunch}
          disabled={disabled}
          icon={Coffee}
          label={`Skip lunch, ${formatClock(LUNCH[0])} to ${formatClock(LUNCH[1])}`}
          onChange={lunch => patch({ lunch })}
        />
      </SettingRow>
      <SettingRow title="What it looks at">
        <div className="flex flex-col divide-y divide-line-soft">
          {LOOKS_AT.map(item => (
            <ToggleRow
              checked={settings.looksAt.includes(item.id)}
              disabled={disabled}
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
          disabled={disabled}
          onValueChange={also => patch({ also })}
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
  onAddToProject,
}: {
  agent: AgentPayload;
  nowhere: boolean;
  onTurnOn: () => void;
  onAddToProject?: () => void;
}) {
  const label = agent.label || agent.name;
  const action = nowhere ? onAddToProject : onTurnOn;
  return (
    <EditorCard>
      <Empty
        action={
          action ? (
            <Button onClick={action} type="button">
              {nowhere ? "Add it to a project" : "Turn on check-ins"}
            </Button>
          ) : undefined
        }
        description={
          nowhere
            ? `${label} isn't working in a project yet, so there's nothing to look at.`
            : "Turn this on and it looks around by itself, and only speaks up when something needs you."
        }
        fill={false}
        icon={<AgentMark {...agentEditorMark(agent)} quietIdle size={40} state="paused" />}
        title={nowhere ? "Nowhere to check in yet" : "No check-ins yet"}
      />
    </EditorCard>
  );
}

function pose(model: CheckInsEditorModel, state: AgentAvatarState): AgentAvatarState {
  if (model.standing === "failed") return "failed";
  return model.on && model.standing !== "paused" ? state : "paused";
}

export interface AgentCheckInsViewProps {
  agent: AgentPayload;
  state: AgentAvatarState;
  model: CheckInsEditorModel;
  onAddToProject?: () => void;
}

export function AgentCheckInsView({ agent, state, model, onAddToProject }: AgentCheckInsViewProps) {
  const { document, settings, readOnly, on, paused, nowhere, never } = model;
  const { conflict, raw, block, problem } = document;
  const label = agent.label || agent.name;
  const active = on && !paused;
  const off = !on || readOnly || raw !== null;
  return (
    <div className="flex flex-col gap-6 py-2">
      <EditorHead
        agent={agent}
        state={pose(model, state)}
        subtitle={paused ? "Waiting for you" : scheduleSummary(settings, on)}
        title="Check-ins"
      >
        <div className="flex items-center gap-2.5 text-body text-fg">
          {active ? "On" : "Off"}
          <Switch
            aria-label="Check-ins"
            checked={active}
            disabled={nowhere || paused}
            onCheckedChange={model.turnOn}
          />
        </div>
      </EditorHead>
      {readOnly ? <ReadOnlyNotice agent={agent} /> : null}
      {conflict ? (
        <ConflictCard
          agentLabel={label}
          mine={<p>{raw === null ? scheduleSummary(settings, true) : "Your edits to the file"}</p>}
          onKeepMine={() => void document.save()}
          onUseTheirs={() => {
            document.chooseTheirs();
            toast.message(`Using ${label}'s version`);
          }}
          theirs={
            <p>
              {conflict.block
                ? "Changes the file in a way this page can't show"
                : scheduleSummary(conflict.form, true)}
            </p>
          }
          title={`${label} changed its schedule while you were editing`}
        />
      ) : null}
      <div className="grid grid-cols-[minmax(0,1fr)_23rem] items-start gap-5">
        <div className="flex min-w-0 flex-col gap-4">
          {block ? (
            <FileBlockNotice block={block} />
          ) : (
            <SettingsCard disabled={off} model={model} />
          )}
          <RawFile
            defaultOpen={block !== null || Boolean(problem)}
            error={problem}
            onChange={document.setRaw}
            readOnly={readOnly}
            value={document.source}
          />
        </div>
        <div className="sticky top-4 flex min-w-0 flex-col gap-4">
          {never || nowhere ? (
            <NotStartedCard
              agent={agent}
              nowhere={nowhere}
              onAddToProject={onAddToProject}
              onTurnOn={() => model.turnOn(true)}
            />
          ) : (
            <>
              <NextCard model={model} />
              <RecentCard activity={model.activity} />
            </>
          )}
        </div>
      </div>
      {readOnly ? null : (
        <UnsavedDock
          blocked={problem ? "Fix the file first" : undefined}
          count={document.changes}
          onDiscard={() => {
            document.discard();
            model.setLimit(false);
          }}
          onSave={() => void document.save()}
          saving={document.saving}
        />
      )}
    </div>
  );
}
