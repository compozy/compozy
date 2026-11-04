/**
 * Agents › Merge Train › Heartbeat inside the app shell, laid out like `AgentCheckInsView` from
 * its production parts. The page's model reads the daemon, so the settings and the side card are
 * fixed here; only the Sources block changes between the variations.
 */
import { Clock3, Coffee, ListChecks, MessagesSquare, Play, Workflow } from "@compozy/ui/icons";
import { useState } from "react";

import { Button, Eyebrow, Switch } from "@compozy/ui";

import {
  DayRangeSlider,
  DayStrip,
  DayStripKey,
  EditorCard,
  SettingRow,
  StopSlider,
  ToggleRow,
} from "@/systems/agent/components/agent-editors/rich-controls";
import { EditorHead } from "@/systems/agent/components/agent-editors/editor-head";
import { HOW_OFTEN, LUNCH } from "@/systems/agent/lib/agent-check-in-form";
import type { DayCell } from "@/systems/agent/lib/agent-check-in-activity";

import { AgentAppPage } from "../always-on-agents-shared/agent-page";
import { MERGE_TRAIN, MERGE_TRAIN_PAYLOAD } from "./fleet-fixtures";
import { ScriptSourcesBlock, type SourcesCase, type SourcesVariant } from "./sources-card";

const LOOKS_AT = [
  { id: "task", label: "Tasks that got stuck", icon: ListChecks, on: false },
  { id: "session_health", label: "Sessions having trouble", icon: MessagesSquare, on: false },
  { id: "sessions_idle", label: "Sessions that went quiet", icon: Clock3, on: false },
  { id: "automations_failed", label: "Automations that failed", icon: Workflow, on: false },
] as const;

/** Every 30 minutes from 9:00 to 18:00, lunch skipped: 16 heartbeats, none done yet. */
const SLOTS = 16;

function SettingsCard({
  variant,
  which,
  disabled,
}: {
  variant: SourcesVariant;
  which: SourcesCase;
  disabled: boolean;
}) {
  const [stop, setStop] = useState(2);
  const [hours, setHours] = useState<readonly [number, number]>([9, 18]);
  const [lunch, setLunch] = useState(true);
  return (
    <EditorCard className="gap-7">
      <SettingRow title="How often" value={`Every ${HOW_OFTEN[stop]!.label}`}>
        <StopSlider
          disabled={disabled}
          floor={0}
          label="How often"
          onChange={setStop}
          stops={HOW_OFTEN.map(item => item.label)}
          value={stop}
        />
      </SettingRow>
      <SettingRow title="When" value="9:00 to 18:00">
        <DayRangeSlider
          disabled={disabled}
          gap={lunch ? LUNCH : undefined}
          label="When"
          now={10}
          onChange={setHours}
          value={hours}
        />
        <ToggleRow
          checked={lunch}
          disabled={disabled}
          icon={Coffee}
          label="Skip lunch, 12:00 to 13:00"
          onChange={setLunch}
        />
      </SettingRow>
      <SettingRow title="What it looks at">
        <div className="flex flex-col divide-y divide-line-soft">
          {LOOKS_AT.map(item => (
            <ToggleRow
              checked={item.on}
              disabled={disabled}
              icon={item.icon}
              key={item.id}
              label={item.label}
              onChange={() => undefined}
            />
          ))}
        </div>
        <div className="flex flex-col gap-3 pt-3">
          <ScriptSourcesBlock variant={variant} which={which} />
        </div>
      </SettingRow>
    </EditorCard>
  );
}

/** `NextCard` before the first heartbeat, or `StoppedStatement` on a paused agent. */
function SideCard({ paused }: { paused: boolean }) {
  const cells: DayCell[] = Array.from({ length: SLOTS }, (_, index) =>
    !paused && index === 0 ? "next" : "later"
  );
  return (
    <div className="sticky top-4 flex min-w-0 flex-col gap-4">
      <EditorCard className="gap-5">
        <div className="-my-1 flex items-center justify-between gap-2">
          <Eyebrow>{paused ? "Heartbeat" : "Next heartbeat"}</Eyebrow>
          <span className="text-small-body text-muted-foreground">compozy</span>
        </div>
        {paused ? (
          <div className="flex flex-col gap-3">
            <div className="flex flex-col gap-0.5">
              <span className="text-display-2xl font-medium text-fg">Paused</span>
              <span className="text-small-body text-muted-foreground">
                You paused it. It waits until you start it again.
              </span>
            </div>
            <Button className="self-start" size="sm" type="button">
              Resume
            </Button>
          </div>
        ) : (
          <div className="flex flex-col gap-0.5">
            <span className="text-display-2xl font-medium text-fg tabular-nums">
              Today at 10:30
            </span>
            <span className="text-small-body text-muted-foreground">In 22 minutes</span>
          </div>
        )}
        <div className="flex flex-col gap-2.5">
          <div className="text-small-body text-muted-foreground tabular-nums">
            <span className="font-medium text-fg">0</span> of {SLOTS} today
          </div>
          <DayStrip cells={cells} label={`0 of ${SLOTS} heartbeats done today`} />
          <div className="flex flex-wrap gap-x-3 gap-y-1">
            <DayStripKey cell="quiet">All quiet</DayStripKey>
            <DayStripKey cell="told">Told you</DayStripKey>
            {paused ? null : <DayStripKey cell="next">Next</DayStripKey>}
          </div>
        </div>
        <Button type="button" variant={paused ? "secondary" : "default"}>
          <Play aria-hidden="true" />
          Try it now
        </Button>
      </EditorCard>
      <EditorCard className="gap-4">
        <h2 className="text-item-title font-medium text-fg">Today so far</h2>
        <p className="text-small-body text-muted-foreground">
          Heartbeats will show up here once they start.
        </p>
      </EditorCard>
    </div>
  );
}

export function HeartbeatInShell({
  variant,
  which,
}: {
  variant: SourcesVariant;
  which: SourcesCase;
}) {
  const paused = which === "paused";
  return (
    <AgentAppPage
      agent={MERGE_TRAIN}
      leaf="Heartbeat"
      shell={{ notifications: 3 }}
      width="max-w-5xl"
    >
      <div className="flex flex-col gap-6 py-2">
        <EditorHead
          agent={MERGE_TRAIN_PAYLOAD}
          state={paused ? "paused" : "idle"}
          subtitle={paused ? "Waiting for you" : "Every 30 min, 9:00 to 18:00, skips lunch"}
          title="Heartbeat"
        >
          <div className="flex items-center gap-2.5 text-body text-fg">
            {paused ? "Off" : "On"}
            <Switch aria-label="Heartbeat" checked={!paused} disabled={paused} />
          </div>
        </EditorHead>
        <div className="grid grid-cols-[minmax(0,1fr)_23rem] items-start gap-5">
          <SettingsCard disabled={paused} variant={variant} which={which} />
          <SideCard paused={paused} />
        </div>
      </div>
    </AgentAppPage>
  );
}
