import {
  Field,
  FieldContent,
  FieldDescription,
  FieldGroup,
  FieldTitle,
  Input,
  NativeSelect,
  NativeSelectOption,
  Switch,
} from "@agh/ui";

import type {
  BridgeDeliveryDefaults,
  BridgeProgressConfig,
  BridgeProgressGrouping,
  BridgeProgressMode,
  BridgeRoutingPolicy,
} from "../types";

interface BridgeRoutingFieldsProps {
  onChange: (value: BridgeRoutingPolicy) => void;
  testIdPrefix: string;
  value: BridgeRoutingPolicy;
}

interface BridgeDeliveryFieldsProps {
  onChange: (value: BridgeDeliveryDefaults) => void;
  testIdPrefix: string;
  value: BridgeDeliveryDefaults;
}

type ProgressBooleanKey = "typing" | "reactions";

const DEFAULT_PROGRESS_OVERRIDE: BridgeProgressConfig = {
  grouping: "accumulate",
  tool_progress: "new",
};

function withoutProgress(value: BridgeDeliveryDefaults): BridgeDeliveryDefaults {
  const { progress: _progress, ...deliveryDefaults } = value;
  return deliveryDefaults;
}

function booleanSelectValue(value: boolean | undefined): string {
  if (value === undefined) return "";
  return value ? "true" : "false";
}

function withProgressBoolean(
  progress: BridgeProgressConfig,
  key: ProgressBooleanKey,
  value: string
): BridgeProgressConfig {
  const next = { ...progress };
  if (value === "") {
    delete next[key];
    return next;
  }

  next[key] = value === "true";
  return next;
}

export function BridgeRoutingFields({ onChange, testIdPrefix, value }: BridgeRoutingFieldsProps) {
  return (
    <FieldGroup className="gap-3">
      <Field orientation="horizontal">
        <Switch
          aria-label="Include peer in route identity"
          checked={value.include_peer}
          data-testid={`${testIdPrefix}-routing-include-peer`}
          onCheckedChange={checked => onChange({ ...value, include_peer: checked })}
        />
        <FieldContent>
          <FieldTitle>Include peer</FieldTitle>
          <FieldDescription>Differentiate direct targets by peer identifier.</FieldDescription>
        </FieldContent>
      </Field>
      <Field orientation="horizontal">
        <Switch
          aria-label="Include group in route identity"
          checked={value.include_group}
          data-testid={`${testIdPrefix}-routing-include-group`}
          onCheckedChange={checked => onChange({ ...value, include_group: checked })}
        />
        <FieldContent>
          <FieldTitle>Include group</FieldTitle>
          <FieldDescription>
            Keep routes isolated per group or channel when the platform supports it.
          </FieldDescription>
        </FieldContent>
      </Field>
      <Field orientation="horizontal">
        <Switch
          aria-label="Include thread in route identity"
          checked={value.include_thread}
          data-testid={`${testIdPrefix}-routing-include-thread`}
          onCheckedChange={checked => onChange({ ...value, include_thread: checked })}
        />
        <FieldContent>
          <FieldTitle>Include thread</FieldTitle>
          <FieldDescription>
            Use thread identity as an additional routing dimension.
          </FieldDescription>
        </FieldContent>
      </Field>
    </FieldGroup>
  );
}

export function BridgeDeliveryFields({ onChange, testIdPrefix, value }: BridgeDeliveryFieldsProps) {
  const progress = value.progress;

  const updateProgress = (next: BridgeProgressConfig) => {
    onChange({ ...value, progress: next });
  };

  return (
    <div className="flex flex-col gap-4">
      <FieldGroup className="grid gap-4 lg:grid-cols-2">
        <Field>
          <FieldContent>
            <FieldTitle>Mode</FieldTitle>
          </FieldContent>
          <NativeSelect
            aria-label="Delivery mode"
            data-testid={`${testIdPrefix}-delivery-mode-select`}
            onChange={event =>
              onChange({
                ...value,
                mode:
                  event.target.value === ""
                    ? undefined
                    : (event.target.value as NonNullable<BridgeDeliveryDefaults["mode"]>),
              })
            }
            value={value.mode ?? ""}
          >
            <NativeSelectOption value="">Use runtime default</NativeSelectOption>
            <NativeSelectOption value="reply">Reply</NativeSelectOption>
            <NativeSelectOption value="direct-send">Direct send</NativeSelectOption>
          </NativeSelect>
        </Field>
        <Field>
          <FieldContent>
            <FieldTitle>Peer ID</FieldTitle>
          </FieldContent>
          <Input
            aria-label="Default peer ID"
            data-testid={`${testIdPrefix}-delivery-peer-input`}
            onChange={event => onChange({ ...value, peer_id: event.target.value })}
            placeholder="peer_123"
            value={value.peer_id ?? ""}
          />
        </Field>
        <Field>
          <FieldContent>
            <FieldTitle>Thread ID</FieldTitle>
          </FieldContent>
          <Input
            aria-label="Default thread ID"
            data-testid={`${testIdPrefix}-delivery-thread-input`}
            onChange={event => onChange({ ...value, thread_id: event.target.value })}
            placeholder="thread_456"
            value={value.thread_id ?? ""}
          />
        </Field>
        <Field>
          <FieldContent>
            <FieldTitle>Group ID</FieldTitle>
          </FieldContent>
          <Input
            aria-label="Default group ID"
            data-testid={`${testIdPrefix}-delivery-group-input`}
            onChange={event => onChange({ ...value, group_id: event.target.value })}
            placeholder="group_789"
            value={value.group_id ?? ""}
          />
        </Field>
      </FieldGroup>

      <div className="border-t border-line pt-4">
        <div className="mb-3">
          <FieldTitle>Delivery progress</FieldTitle>
          <FieldDescription>
            Keep provider defaults, or define how tool progress appears in external conversations.
          </FieldDescription>
        </div>
        <FieldGroup className="grid gap-4 lg:grid-cols-2">
          <Field>
            <FieldContent>
              <FieldTitle>Tool progress</FieldTitle>
              <FieldDescription>
                Provider default omits the entire progress override.
              </FieldDescription>
            </FieldContent>
            <NativeSelect
              aria-label="Tool progress mode"
              data-testid={`${testIdPrefix}-delivery-progress-mode-select`}
              onChange={event => {
                if (event.target.value === "") {
                  onChange(withoutProgress(value));
                  return;
                }
                updateProgress({
                  ...(progress ?? DEFAULT_PROGRESS_OVERRIDE),
                  tool_progress: event.target.value as BridgeProgressMode,
                });
              }}
              value={progress?.tool_progress ?? ""}
            >
              <NativeSelectOption value="">Use provider default</NativeSelectOption>
              <NativeSelectOption value="off">Off</NativeSelectOption>
              <NativeSelectOption value="new">New tools</NativeSelectOption>
              <NativeSelectOption value="all">All lifecycle updates</NativeSelectOption>
              <NativeSelectOption value="verbose">Verbose</NativeSelectOption>
            </NativeSelect>
          </Field>
          <Field>
            <FieldContent>
              <FieldTitle>Grouping</FieldTitle>
              <FieldDescription>
                Accumulate updates or post each update separately.
              </FieldDescription>
            </FieldContent>
            <NativeSelect
              aria-label="Progress grouping"
              data-testid={`${testIdPrefix}-delivery-progress-grouping-select`}
              disabled={!progress}
              onChange={event =>
                progress &&
                updateProgress({
                  ...progress,
                  grouping: event.target.value as BridgeProgressGrouping,
                })
              }
              value={progress?.grouping ?? ""}
            >
              <NativeSelectOption value="accumulate">Accumulate</NativeSelectOption>
              <NativeSelectOption value="separate">Separate</NativeSelectOption>
            </NativeSelect>
          </Field>
          <Field>
            <FieldContent>
              <FieldTitle>Typing indicator</FieldTitle>
            </FieldContent>
            <NativeSelect
              aria-label="Typing indicator"
              data-testid={`${testIdPrefix}-delivery-progress-typing-select`}
              disabled={!progress}
              onChange={event =>
                progress &&
                updateProgress(withProgressBoolean(progress, "typing", event.target.value))
              }
              value={booleanSelectValue(progress?.typing)}
            >
              <NativeSelectOption value="">Default</NativeSelectOption>
              <NativeSelectOption value="true">Enabled</NativeSelectOption>
              <NativeSelectOption value="false">Disabled</NativeSelectOption>
            </NativeSelect>
          </Field>
          <Field>
            <FieldContent>
              <FieldTitle>Progress reactions</FieldTitle>
            </FieldContent>
            <NativeSelect
              aria-label="Progress reactions"
              data-testid={`${testIdPrefix}-delivery-progress-reactions-select`}
              disabled={!progress}
              onChange={event =>
                progress &&
                updateProgress(withProgressBoolean(progress, "reactions", event.target.value))
              }
              value={booleanSelectValue(progress?.reactions)}
            >
              <NativeSelectOption value="">Default</NativeSelectOption>
              <NativeSelectOption value="true">Enabled</NativeSelectOption>
              <NativeSelectOption value="false">Disabled</NativeSelectOption>
            </NativeSelect>
          </Field>
        </FieldGroup>
      </div>
    </div>
  );
}
