import { Field, FieldLabel, Input } from "@agh/ui";

import type { EventFamily } from "../../lib/trigger-catalog";
import type { SubConfigValues } from "../../lib/trigger-sub-config";

interface EventSubConfigProps {
  family: EventFamily;
  values: SubConfigValues;
  onChange: (patch: Partial<SubConfigValues>) => void;
}

const MONO_INPUT = "font-mono text-form-label";
const BOX = "mt-1.5 ml-4 rounded-md border border-line bg-canvas-tint p-3.5";

/**
 * Inline per-event configuration rendered directly under the selected event
 * card. Hook and extension fields are encoded back into the runtime `event`
 * string; webhook fields map to dedicated draft fields.
 */
export function EventSubConfig({ family, values, onChange }: EventSubConfigProps) {
  if (family === "hook") {
    return (
      <div className={BOX}>
        <Field>
          <FieldLabel htmlFor="trigger-hook-name">
            Hook name
            <span className="font-normal text-faint">
              {" (fires on "}
              <code className="font-mono text-mono-id text-muted">hook.&lt;name&gt;.completed</code>
              {")"}
            </span>
          </FieldLabel>
          <Input
            className={MONO_INPUT}
            data-testid="trigger-hook-name-input"
            id="trigger-hook-name"
            onChange={event => onChange({ hookName: event.target.value })}
            placeholder="transform"
            value={values.hookName}
          />
        </Field>
      </div>
    );
  }

  if (family === "ext") {
    return (
      <div className={BOX}>
        <p className="mb-2.5 text-form-hint leading-snug text-subtle">
          {"Listens for "}
          <code className="font-mono text-mono-id text-muted">ext.&lt;ext&gt;.&lt;event&gt;</code>
          {" published by an installed extension."}
        </p>
        <div className="grid grid-cols-2 gap-3">
          <Field>
            <FieldLabel htmlFor="trigger-ext-ext">Extension</FieldLabel>
            <Input
              className={MONO_INPUT}
              data-testid="trigger-ext-ext-input"
              id="trigger-ext-ext"
              onChange={event => onChange({ extExt: event.target.value })}
              placeholder="release"
              value={values.extExt}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="trigger-ext-event">Event</FieldLabel>
            <Input
              className={MONO_INPUT}
              data-testid="trigger-ext-event-input"
              id="trigger-ext-event"
              onChange={event => onChange({ extEvent: event.target.value })}
              placeholder="deploy-started"
              value={values.extEvent}
            />
          </Field>
        </div>
      </div>
    );
  }

  if (family === "webhook") {
    return (
      <div className={BOX}>
        <p className="mb-2.5 text-form-hint leading-snug text-subtle">
          External callers POST to a signed endpoint. The URL and curl example are on the right.
        </p>
        <div className="mb-3 grid grid-cols-2 gap-3">
          <Field>
            <FieldLabel htmlFor="trigger-endpoint-slug">Endpoint slug</FieldLabel>
            <Input
              className={MONO_INPUT}
              data-testid="trigger-endpoint-slug-input"
              id="trigger-endpoint-slug"
              onChange={event => onChange({ endpointSlug: event.target.value })}
              placeholder="ci-deploys"
              value={values.endpointSlug}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor="trigger-webhook-id">Webhook id</FieldLabel>
            <Input
              className={MONO_INPUT}
              data-testid="trigger-webhook-id-input"
              id="trigger-webhook-id"
              onChange={event => onChange({ webhookId: event.target.value })}
              placeholder="wbh_…"
              value={values.webhookId}
            />
          </Field>
        </div>
        <Field>
          <FieldLabel htmlFor="trigger-webhook-secret-value">
            Signing secret
            <span className="font-normal text-faint">
              {" "}
              (write-only, used for HMAC verification)
            </span>
          </FieldLabel>
          <Input
            className={MONO_INPUT}
            data-testid="trigger-webhook-secret-value-input"
            id="trigger-webhook-secret-value"
            onChange={event => onChange({ webhookSecret: event.target.value })}
            placeholder="whsec_…"
            type="password"
            value={values.webhookSecret}
          />
        </Field>
      </div>
    );
  }

  return null;
}
