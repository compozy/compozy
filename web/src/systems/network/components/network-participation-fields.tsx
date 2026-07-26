import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  Input,
  NativeSelect,
  NativeSelectOption,
  cn,
} from "@agh/ui";
import type { ComponentProps } from "react";

import type {
  NetworkParticipationDraft,
  NetworkParticipationMode,
  NetworkParticipationStrategy,
} from "../lib/network-participation";
import {
  isNetworkParticipationStrategy,
  networkParticipationValidationMessage,
} from "../lib/network-participation";

const STRATEGY_LABELS: Record<NetworkParticipationStrategy, string> = {
  named: "Named channel",
  run: "This task run",
  loop_run: "This Loop run",
};

export interface NetworkParticipationFieldsProps extends Omit<
  ComponentProps<"div">,
  "children" | "onChange"
> {
  value: NetworkParticipationDraft;
  onChange: (next: NetworkParticipationDraft) => void;
  disabled?: boolean;
  testIdPrefix?: string;
  allowedStrategies: readonly [NetworkParticipationStrategy, ...NetworkParticipationStrategy[]];
}

/** Shared create/edit participation controls (UT-060). Local is the default. */
export function NetworkParticipationFields({
  value,
  onChange,
  disabled = false,
  testIdPrefix = "network-participation",
  allowedStrategies,
  className,
  ...props
}: NetworkParticipationFieldsProps) {
  const currentStrategy =
    value.mode === "live" && allowedStrategies.includes(value.channelStrategy)
      ? value.channelStrategy
      : allowedStrategies[0];
  const validationMessage = networkParticipationValidationMessage(value, allowedStrategies);

  return (
    <div {...props} className={cn("flex flex-col gap-3", className)} data-testid={testIdPrefix}>
      <Field>
        <FieldLabel htmlFor={`${testIdPrefix}-mode`}>Network participation</FieldLabel>
        <FieldDescription>
          Local keeps the execution off the network. Live joins a channel explicitly — availability
          settings never enroll executions.
        </FieldDescription>
        <NativeSelect
          disabled={disabled}
          id={`${testIdPrefix}-mode`}
          data-testid={`${testIdPrefix}-mode`}
          value={value.mode}
          onChange={event => {
            const mode = event.target.value as NetworkParticipationMode;
            onChange(
              mode === "local"
                ? { mode: "local", channelId: "", channelStrategy: "" }
                : {
                    mode: "live",
                    channelId: currentStrategy === "named" ? value.channelId : "",
                    channelStrategy: currentStrategy,
                  }
            );
          }}
        >
          <NativeSelectOption value="local">Local</NativeSelectOption>
          <NativeSelectOption value="live">Live</NativeSelectOption>
        </NativeSelect>
      </Field>
      {value.mode === "live" ? (
        <>
          <Field>
            <FieldLabel htmlFor={`${testIdPrefix}-strategy`}>Channel strategy</FieldLabel>
            <NativeSelect
              disabled={disabled}
              id={`${testIdPrefix}-strategy`}
              data-testid={`${testIdPrefix}-strategy`}
              value={currentStrategy}
              onChange={event => {
                if (!isNetworkParticipationStrategy(event.target.value)) return;
                onChange({
                  mode: "live",
                  channelId: event.target.value === "named" ? value.channelId : "",
                  channelStrategy: event.target.value,
                });
              }}
            >
              {allowedStrategies.map(strategy => (
                <NativeSelectOption key={strategy} value={strategy}>
                  {STRATEGY_LABELS[strategy]}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </Field>
          {currentStrategy === "named" ? (
            <Field>
              <FieldLabel htmlFor={`${testIdPrefix}-channel`}>Channel</FieldLabel>
              <Input
                disabled={disabled}
                id={`${testIdPrefix}-channel`}
                data-testid={`${testIdPrefix}-channel`}
                placeholder="channel id"
                required
                aria-invalid={validationMessage ? true : undefined}
                value={value.channelId}
                onChange={event =>
                  onChange({ ...value, channelId: event.target.value, channelStrategy: "named" })
                }
              />
              {validationMessage ? <FieldError>{validationMessage}</FieldError> : null}
            </Field>
          ) : null}
        </>
      ) : null}
    </div>
  );
}
