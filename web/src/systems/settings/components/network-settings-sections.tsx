import type { Dispatch, SetStateAction } from "react";
import { Link } from "@tanstack/react-router";

import { Switch } from "@agh/ui";

import type { SettingsNetworkSection } from "../types";
import { NetworkLiveSettingsSections } from "./network-live-settings-sections";
import { SettingLinkRow } from "./setting-row";
import { SettingsAdvancedFold } from "./settings-advanced-fold";
import { SettingsFieldRow } from "./settings-field-row";
import { SettingsGroup } from "./settings-group";
import { SettingsHeroBoard } from "./settings-hero-board";
import { SettingsLiveChip } from "./settings-live-chip";
import { SettingsNumberInput } from "./settings-number-input";
import { SettingsRuntimeUnavailable } from "./settings-runtime-unavailable";

type NetworkConfig = SettingsNetworkSection["config"];
type NetworkRuntime = SettingsNetworkSection["runtime"];

interface NetworkSettingsSectionsProps {
  runtime: NetworkRuntime;
  draft: NetworkConfig;
  setDraft: Dispatch<SetStateAction<NetworkConfig | null>>;
  validationErrors: Record<string, string | null>;
  setValidationError: (key: string) => (message: string | null) => void;
}

export function NetworkSettingsSections(props: NetworkSettingsSectionsProps) {
  return (
    <>
      <NetworkHero runtime={props.runtime} />
      <AvailabilitySection draft={props.draft} setDraft={props.setDraft} />
      <DeliverySafetySection {...props} />
      <SettingsAdvancedFold
        data-testid="settings-page-network-advanced"
        label="Advanced — live participation limits"
        padded
      >
        <NetworkLiveSettingsSections {...props} />
      </SettingsAdvancedFold>
    </>
  );
}

function NetworkHero({ runtime }: { runtime: NetworkRuntime }) {
  if (!runtime.available) {
    return (
      <SettingsRuntimeUnavailable
        slug="network"
        description="The Network listener did not return live status or traffic counters."
      />
    );
  }
  const enabled = runtime.enabled;
  const statusLabel = runtime.status ?? (enabled ? "ready" : "disabled");
  return (
    <SettingsHeroBoard
      data-testid="settings-page-network-hero"
      state={enabled ? "Mesh ready" : "Mesh disabled"}
      tone={enabled ? "success" : "neutral"}
      pulse={enabled}
      pill={statusLabel}
      stats={[
        {
          key: "participants",
          value: String(runtime.local_peers),
          label: "Live participants",
        },
        {
          key: "channels",
          value: String(runtime.channels),
          label: "Channels",
        },
        {
          key: "delivered",
          value: runtime.messages_delivered.toLocaleString(),
          label: "Messages delivered",
          sub: `${runtime.messages_received.toLocaleString()} received · ${runtime.messages_rejected.toLocaleString()} rejected`,
        },
      ]}
    />
  );
}

interface DraftSectionProps {
  draft: NetworkConfig;
  setDraft: Dispatch<SetStateAction<NetworkConfig | null>>;
}

function AvailabilitySection({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingsGroup title="Availability" description="applies live without enrollment">
      <SettingsFieldRow
        data-testid="settings-page-network-enabled"
        label="Embedded network"
        description={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Allow explicitly Live executions to join coordination conversations
            <SettingsLiveChip />
          </span>
        }
        control={
          <Switch
            aria-label="Embedded network"
            data-testid="settings-page-network-enabled-switch"
            checked={draft.enabled}
            onCheckedChange={enabled => setDraft(current => ({ ...(current ?? draft), enabled }))}
          />
        }
      />
    </SettingsGroup>
  );
}

function DeliverySafetySection({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: NetworkSettingsSectionsProps) {
  return (
    <SettingsGroup title="Delivery safety" description="replay protection">
      <SettingsFieldRow
        label="Accept replayed messages within"
        description="Messages older than this window are rejected"
        error={validationErrors.maxReplayAge ?? undefined}
        control={
          <div className="flex items-center gap-2">
            <SettingsNumberInput
              aria-label="Accept replayed messages within"
              className="w-32"
              data-testid="settings-page-network-max-replay-age"
              min={1}
              value={draft.max_replay_age}
              onValidityChange={setValidationError("maxReplayAge")}
              onValueChange={maxReplayAge =>
                setDraft(current => ({ ...(current ?? draft), max_replay_age: maxReplayAge }))
              }
            />
            <span className="text-form-label text-subtle">seconds</span>
          </div>
        }
      />
      <SettingLinkRow
        data-testid="settings-page-network-link-network"
        description="Channels, peers, and live coordination for this workspace."
        label="Open the network view"
        render={<Link to="/network" />}
      />
    </SettingsGroup>
  );
}
