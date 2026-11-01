/** S7: cadence, hours, collectors and Test, in the shape of the Autopilot reference. */
import { FlaskConical, Play, Plus } from "lucide-react";
import { useState, type ReactNode } from "react";

import {
  Alert,
  AlertActions,
  AlertDescription,
  Button,
  NativeSelect,
  NativeSelectOption,
  Pill,
  PillGroup,
  Slider,
  Spinner,
  StatusDot,
  Switch,
  Textarea,
  cn,
} from "@compozy/ui";

import { AgentAvatar } from "../persistent-agents-shared/agent-avatar";
import { TRIAGE } from "../persistent-agents-shared/fixtures";
import {
  EditorCard,
  SegmentMeter,
  SettingRow,
  StopSlider,
} from "../persistent-agents-shared/rich-controls";

const CADENCE = ["5m", "15m", "30m", "1h", "2h", "4h"];
const COLLECTORS = [
  { id: "task", label: "Tasks" },
  { id: "session_health", label: "Session health" },
  { id: "sessions_idle", label: "Idle sessions" },
  { id: "automations_failed", label: "Failed automations" },
  { id: "monitors_fired", label: "Fired monitors" },
];

export type HeartbeatState =
  | "default"
  | "paused"
  | "fatigue"
  | "backoff"
  | "no-instances"
  | "floor"
  | "test-dry"
  | "test-running"
  | "advanced";

function Collector({ label, on: initial }: { label: string; on: boolean }) {
  const [on, setOn] = useState(initial);
  return (
    <Pill
      active={on}
      render={<button aria-pressed={on} onClick={() => setOn(!on)} type="button" />}
      size="sm"
    >
      {label}
    </Pill>
  );
}

function hourLabel(hour: number) {
  return `${String(hour).padStart(2, "0")}:00`;
}

function Hours() {
  const [range, setRange] = useState<readonly number[]>([9, 19]);
  return (
    <SettingRow
      title="Active hours"
      value={
        <span className="font-mono text-mono-id">
          {hourLabel(range[0]!)} to {hourLabel(range[1]!)} · America/Sao_Paulo
        </span>
      }
    >
      <Slider<readonly number[]>
        aria-label="Active hours"
        max={24}
        min={0}
        onValueChange={next => setRange(next as readonly number[])}
        step={1}
        value={range}
      />
    </SettingRow>
  );
}

function ChecksHeader({
  on,
  state,
  placeholder,
}: {
  on: boolean;
  state: HeartbeatState;
  placeholder?: boolean;
}) {
  return (
    <div className="flex items-center gap-3">
      <AgentAvatar
        appearance={TRIAGE.appearance}
        label="Triage Lead"
        provider="claude"
        quietIdle
        size={32}
        state={state === "paused" || state === "fatigue" ? "paused" : "idle"}
        surfaceClassName="ring-card"
      />
      <div className="flex min-w-0 flex-1 flex-col">
        <span className="text-item-title font-medium text-fg">Checks</span>
        <span className="text-small-body text-muted">
          {on ? "Every 30 min while CompozyOS is running" : "Off"}
        </span>
      </div>
      {placeholder ? null : (
        <NativeSelect aria-label="Where it runs" className="w-36" defaultValue="compozy">
          <NativeSelectOption value="compozy">compozy</NativeSelectOption>
          <NativeSelectOption value="advos">advos</NativeSelectOption>
        </NativeSelect>
      )}
      <Switch aria-label="Checks" defaultChecked={on} />
    </div>
  );
}

function Fact({ label, value, mono = false }: { label: string; value: ReactNode; mono?: boolean }) {
  return (
    <div className="flex items-baseline justify-between gap-3">
      <span className="text-small-body text-muted">{label}</span>
      <span className={cn("text-small-body text-fg", mono && "font-mono text-mono-id")}>
        {value}
      </span>
    </div>
  );
}

function Status({ state }: { state: HeartbeatState }) {
  const backoff = state === "backoff";
  const fatigue = state === "fatigue";
  const paused = state === "paused" || fatigue;
  return (
    <EditorCard className="gap-4">
      <div className="flex flex-col gap-2">
        <Fact label="Last check" value={fatigue ? "Failed 41m ago" : "Quiet, 12m ago"} />
        {paused ? null : <Fact label="Next check" value={backoff ? "in 1h 40m" : "in 18m"} />}
      </div>
      {fatigue ? (
        <div className="flex flex-col gap-2">
          <Fact label="Failed in a row" value="10 of 10" />
          <SegmentMeter label="10 of 10 failures" tone="danger" total={10} value={10} />
        </div>
      ) : (
        <div className="flex flex-col gap-2">
          <Fact label="Quiet in a row" value={backoff ? "7" : "2"} />
          <SegmentMeter label="Quiet runs" total={8} value={backoff ? 7 : 2} />
          {backoff ? (
            <span className="text-eyebrow text-subtle">
              Checking every 2h until something changes or you send a message.
            </span>
          ) : null}
        </div>
      )}
      <div className="flex flex-col gap-1.5">
        <span className="text-small-body font-medium text-fg">Recent reports</span>
        {[
          ["Unblock the token migration", "2h ago"],
          ["Review waiting on you", "Yesterday"],
        ].map(([title, when]) => (
          <div className="flex items-center gap-2 text-small-body text-muted" key={title}>
            <StatusDot size="sm" tone="faint" />
            <span className="min-w-0 flex-1 truncate">{title}</span>
            <span className="text-eyebrow text-subtle">{when}</span>
          </div>
        ))}
      </div>
    </EditorCard>
  );
}

function TestPanel({ state }: { state: HeartbeatState }) {
  return (
    <EditorCard className="gap-3">
      <div className="flex items-center gap-2">
        <FlaskConical aria-hidden="true" className="size-4 text-muted" />
        <span className="min-w-0 flex-1 text-small-body font-medium text-fg">Test</span>
        <Button disabled={state === "no-instances"} size="sm" type="button" variant="secondary">
          Dry run
        </Button>
        <Button
          disabled={state === "no-instances" || state === "paused" || state === "fatigue"}
          size="sm"
          type="button"
          variant="quiet"
        >
          <Play aria-hidden="true" />
          Run a check
        </Button>
      </div>
      {state === "test-dry" ? (
        <div className="flex flex-col gap-1.5 rounded-md bg-sunken p-3">
          <span className="text-eyebrow text-subtle">Would run · 2 items</span>
          <p className="text-small-body text-fg-2">Migrate tokens is blocked</p>
          <p className="text-small-body text-fg-2">Nightly triage failed twice</p>
        </div>
      ) : null}
      {state === "test-running" ? (
        <div className="flex items-center gap-2 rounded-md bg-sunken p-3 text-small-body text-muted">
          <Spinner className="size-3.5" />
          Reading 2 items…
        </div>
      ) : null}
    </EditorCard>
  );
}

function Form({ state }: { state: HeartbeatState }) {
  const [cadence, setCadence] = useState(state === "floor" ? 1 : 2);
  return (
    <EditorCard>
      <SettingRow title="Cadence" value={`Every ${CADENCE[cadence]}`}>
        <StopSlider
          floor={1}
          floorLabel={
            state === "floor" ? "15m is the shortest cadence set for this profile." : undefined
          }
          label="Cadence"
          onChange={setCadence}
          stops={CADENCE}
          value={cadence}
        />
      </SettingRow>
      <Hours />
      <SettingRow title="Quiet windows">
        <div className="flex flex-wrap items-center gap-1.5">
          <Pill mono size="sm">
            12:00 to 13:00
          </Pill>
          <Button aria-label="Add quiet window" size="icon-xs" type="button" variant="quiet">
            <Plus aria-hidden="true" />
          </Button>
        </div>
      </SettingRow>
      <SettingRow title="Looks at">
        <div className="flex flex-wrap gap-1.5">
          {COLLECTORS.map((item, index) => (
            <Collector key={item.id} label={item.label} on={index < 3} />
          ))}
        </div>
      </SettingRow>
      <SettingRow title="Standing instructions">
        <Textarea defaultValue="Flag blocked tasks and PRs waiting on me." rows={2} />
      </SettingRow>
    </EditorCard>
  );
}

const RAW = `---
version: 1
enabled: true
preferences:
  min_interval: 30m
  active_hours:
    - timezone: America/Sao_Paulo
      start: "09:00"
      end: "19:00"
context:
  include: [task, session_health, sessions_idle]
---
Flag blocked tasks and PRs waiting on me.`;

export function HeartbeatEditor({ state }: { state: HeartbeatState }) {
  const [mode, setMode] = useState(state === "advanced" ? "advanced" : "form");
  const none = state === "no-instances";
  const paused = state === "paused" || state === "fatigue";
  return (
    <div className="grid grid-cols-[minmax(0,1fr)_20rem] items-start gap-4 py-2">
      <div className="flex min-w-0 flex-col gap-4">
        {none ? (
          <Alert variant="info">
            <AlertDescription>Checks need a place to run.</AlertDescription>
            <AlertActions>
              <Button size="xs" type="button" variant="secondary">
                Add to a workspace
              </Button>
            </AlertActions>
          </Alert>
        ) : null}
        {state === "paused" ? (
          <Alert variant="warning">
            <AlertDescription>Paused by you. Checks wait until you resume.</AlertDescription>
            <AlertActions>
              <Button size="xs" type="button" variant="secondary">
                Resume
              </Button>
            </AlertActions>
          </Alert>
        ) : null}
        {state === "fatigue" ? (
          <Alert variant="danger">
            <AlertDescription>Paused after 10 failed checks in a row.</AlertDescription>
            <AlertActions>
              <Button size="xs" type="button" variant="secondary">
                Resume
              </Button>
            </AlertActions>
          </Alert>
        ) : null}
        <EditorCard>
          <ChecksHeader on={!none && !paused} placeholder={none} state={state} />
        </EditorCard>
        <PillGroup
          items={[
            { value: "form", label: "Form" },
            { value: "advanced", label: "Advanced" },
          ]}
          onChange={setMode}
          value={mode}
        />
        {mode === "form" ? (
          <Form state={state} />
        ) : (
          <Textarea aria-label="HEARTBEAT.md" className="min-h-96 font-mono" defaultValue={RAW} />
        )}
      </div>
      <div className="flex min-w-0 flex-col gap-4">
        {none ? null : <Status state={state} />}
        <TestPanel state={state} />
      </div>
    </div>
  );
}
