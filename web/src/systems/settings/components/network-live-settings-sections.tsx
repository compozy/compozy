import type { Dispatch, SetStateAction } from "react";

import { Input } from "@agh/ui";

import type { SettingsNetworkSection } from "../types";
import { SettingsProvChip } from "./settings-advanced-fold";
import { SettingsFieldRow } from "./settings-field-row";
import { SettingsGroup } from "./settings-group";
import { SettingsNumberInput } from "./settings-number-input";

type NetworkConfig = SettingsNetworkSection["config"];
type LiveDefaults = NetworkConfig["live"]["defaults"];
type LiveLimits = NetworkConfig["live"]["limits"];

interface NetworkLiveSettingsProps {
  draft: NetworkConfig;
  setDraft: Dispatch<SetStateAction<NetworkConfig | null>>;
  validationErrors: Record<string, string | null>;
  setValidationError: (key: string) => (message: string | null) => void;
}

export function NetworkLiveSettingsSections(props: NetworkLiveSettingsProps) {
  return (
    <>
      <LiveDefaultsSection {...props} />
      <LiveLimitsSection {...props} />
    </>
  );
}

function rowDescription(text: string, configKey?: string) {
  if (!configKey) return text;
  return (
    <span className="inline-flex flex-wrap items-center gap-1.5">
      {text}
      <SettingsProvChip>{configKey}</SettingsProvChip>
    </span>
  );
}

function LiveDefaultsSection({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: NetworkLiveSettingsProps) {
  const update = (patch: Partial<LiveDefaults>) => {
    setDraft(current => ({
      ...(current ?? draft),
      live: {
        ...(current ?? draft).live,
        defaults: { ...(current ?? draft).live.defaults, ...patch },
      },
    }));
  };

  return (
    <SettingsGroup
      title="Per-participation defaults"
      description="Omitted per-execution bounds resolve to these finite defaults. Local executions ignore every value in this section."
    >
      <NumberRow
        label="Max wakes"
        description={rowDescription("Wakes one Live participation may consume")}
        testId="settings-page-network-live-default-max-wakes"
        value={draft.live.defaults.max_wakes}
        errorMessage={validationErrors.defaultMaxWakes ?? undefined}
        onValidityChange={setValidationError("defaultMaxWakes")}
        onChange={value => update({ max_wakes: value })}
      />
      <NumberRow
        label="Max wake depth"
        description={rowDescription("How deep wake chains may cascade")}
        testId="settings-page-network-live-default-max-depth"
        value={draft.live.defaults.max_wake_depth}
        errorMessage={validationErrors.defaultMaxDepth ?? undefined}
        onValidityChange={setValidationError("defaultMaxDepth")}
        onChange={value => update({ max_wake_depth: value })}
      />
      <NumberRow
        label="Input token budget"
        description={rowDescription(
          "Prompt tokens per participation",
          "live.defaults.max_input_tokens"
        )}
        testId="settings-page-network-live-default-input-tokens"
        value={draft.live.defaults.max_input_tokens}
        errorMessage={validationErrors.defaultInputTokens ?? undefined}
        onValidityChange={setValidationError("defaultInputTokens")}
        onChange={value => update({ max_input_tokens: value })}
      />
      <NumberRow
        label="Output token budget"
        description={rowDescription("Completion tokens per participation")}
        testId="settings-page-network-live-default-output-tokens"
        value={draft.live.defaults.max_output_tokens}
        errorMessage={validationErrors.defaultOutputTokens ?? undefined}
        onValidityChange={setValidationError("defaultOutputTokens")}
        onChange={value => update({ max_output_tokens: value })}
      />
      <TextRow
        label="Wake timeout"
        description={rowDescription("Wall time one wake may run")}
        testId="settings-page-network-live-default-wake-time"
        value={draft.live.defaults.max_wake_wall_time}
        onChange={value => update({ max_wake_wall_time: value })}
      />
      <TextRow
        label="Total timeout"
        description={rowDescription("Wall time across the whole participation")}
        testId="settings-page-network-live-default-total-time"
        value={draft.live.defaults.max_total_wall_time}
        onChange={value => update({ max_total_wall_time: value })}
      />
      <TextRow
        label="Coalesce window"
        description={rowDescription("Burst messages merge into one wake inside this window")}
        testId="settings-page-network-live-default-coalesce"
        value={draft.live.defaults.coalesce_window}
        onChange={value => update({ coalesce_window: value })}
      />
    </SettingsGroup>
  );
}

function LiveLimitsSection({
  draft,
  setDraft,
  validationErrors,
  setValidationError,
}: NetworkLiveSettingsProps) {
  const update = (patch: Partial<LiveLimits>) => {
    setDraft(current => ({
      ...(current ?? draft),
      live: {
        ...(current ?? draft).live,
        limits: { ...(current ?? draft).live.limits, ...patch },
      },
    }));
  };

  return (
    <SettingsGroup
      title="Hard ceilings"
      description="Execution requests above these ceilings are rejected before durable run state is created."
    >
      <NumberRow
        label="Max wakes"
        description={rowDescription("Inclusive ceiling", "live.limits.max_wakes")}
        testId="settings-page-network-live-limit-max-wakes"
        value={draft.live.limits.max_wakes}
        errorMessage={validationErrors.limitMaxWakes ?? undefined}
        onValidityChange={setValidationError("limitMaxWakes")}
        onChange={value => update({ max_wakes: value })}
      />
      <NumberRow
        label="Max wake depth"
        description={rowDescription("Inclusive ceiling", "live.limits.max_wake_depth")}
        testId="settings-page-network-live-limit-max-depth"
        value={draft.live.limits.max_wake_depth}
        errorMessage={validationErrors.limitMaxDepth ?? undefined}
        onValidityChange={setValidationError("limitMaxDepth")}
        onChange={value => update({ max_wake_depth: value })}
      />
      <NumberRow
        label="Input token budget"
        description={rowDescription("Inclusive ceiling", "live.limits.max_input_tokens")}
        testId="settings-page-network-live-limit-input-tokens"
        value={draft.live.limits.max_input_tokens}
        errorMessage={validationErrors.limitInputTokens ?? undefined}
        onValidityChange={setValidationError("limitInputTokens")}
        onChange={value => update({ max_input_tokens: value })}
      />
      <NumberRow
        label="Output token budget"
        description={rowDescription("Inclusive ceiling", "live.limits.max_output_tokens")}
        testId="settings-page-network-live-limit-output-tokens"
        value={draft.live.limits.max_output_tokens}
        errorMessage={validationErrors.limitOutputTokens ?? undefined}
        onValidityChange={setValidationError("limitOutputTokens")}
        onChange={value => update({ max_output_tokens: value })}
      />
      <TextRow
        label="Wake timeout"
        description={rowDescription("Inclusive ceiling", "live.limits.max_wake_wall_time")}
        testId="settings-page-network-live-limit-wake-time"
        value={draft.live.limits.max_wake_wall_time}
        onChange={value => update({ max_wake_wall_time: value })}
      />
      <TextRow
        label="Total timeout"
        description={rowDescription("Inclusive ceiling", "live.limits.max_total_wall_time")}
        testId="settings-page-network-live-limit-total-time"
        value={draft.live.limits.max_total_wall_time}
        onChange={value => update({ max_total_wall_time: value })}
      />
      <TextRow
        label="Minimum coalesce window"
        description={rowDescription(
          "Floor for per-execution coalescing",
          "live.limits.min_coalesce_window"
        )}
        testId="settings-page-network-live-limit-min-coalesce"
        value={draft.live.limits.min_coalesce_window}
        onChange={value => update({ min_coalesce_window: value })}
      />
      <TextRow
        label="Maximum coalesce window"
        description={rowDescription(
          "Ceiling for per-execution coalescing",
          "live.limits.max_coalesce_window"
        )}
        testId="settings-page-network-live-limit-max-coalesce"
        value={draft.live.limits.max_coalesce_window}
        onChange={value => update({ max_coalesce_window: value })}
      />
    </SettingsGroup>
  );
}

interface NumberRowProps {
  label: string;
  description: React.ReactNode;
  testId: string;
  value: number;
  errorMessage?: string;
  onValidityChange: (message: string | null) => void;
  onChange: (value: number) => void;
}

function NumberRow(props: NumberRowProps) {
  return (
    <SettingsFieldRow
      label={props.label}
      description={props.description}
      error={props.errorMessage}
      control={
        <SettingsNumberInput
          aria-label={props.label}
          className="w-32"
          data-testid={props.testId}
          min={1}
          value={props.value}
          onValidityChange={props.onValidityChange}
          onValueChange={props.onChange}
        />
      }
    />
  );
}

interface TextRowProps {
  label: string;
  description: React.ReactNode;
  testId: string;
  value: string;
  onChange: (value: string) => void;
}

function TextRow(props: TextRowProps) {
  return (
    <SettingsFieldRow
      label={props.label}
      description={props.description}
      control={
        <Input
          aria-label={props.label}
          className="w-32 font-mono"
          data-testid={props.testId}
          value={props.value}
          onChange={event => props.onChange(event.target.value.trim())}
        />
      }
    />
  );
}
