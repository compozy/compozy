import { Info } from "lucide-react";

import { Alert, AlertDescription, Field, FieldDescription, FieldLabel, Input } from "@compozy/ui";

import type { AutomationFormModel } from "../../../hooks/use-automation-form";
import type { AutomationFormDraft } from "../../../lib/automation-form-draft";

const MONO = "font-mono text-form-label";

interface WebhookStartProps {
  draft: AutomationFormDraft;
  form: AutomationFormModel;
}

/** Link settings: always Global; the signing secret is write-only and never shown again. */
export function WebhookStart({ draft, form }: WebhookStartProps) {
  return (
    <div className="flex flex-col gap-3">
      <Alert data-testid="automation-webhook-global-note" role="note" variant="info">
        <Info aria-hidden="true" />
        <AlertDescription>
          Link automations are always <b className="font-medium text-fg">Global</b>. They
          aren&apos;t tied to one project.
        </AlertDescription>
      </Alert>
      <div className="grid grid-cols-2 gap-3">
        <Field>
          <FieldLabel htmlFor="automation-webhook-slug">Link name</FieldLabel>
          <Input
            className={MONO}
            data-testid="automation-webhook-slug"
            id="automation-webhook-slug"
            onChange={event => form.onEndpointSlug(event.target.value)}
            placeholder="ci-deploys"
            value={draft.endpoint_slug ?? ""}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor="automation-webhook-id">Webhook id</FieldLabel>
          <Input
            className={MONO}
            data-testid="automation-webhook-id"
            id="automation-webhook-id"
            onChange={event => form.onWebhookId(event.target.value)}
            placeholder="wbh_…"
            value={draft.webhook_id ?? ""}
          />
        </Field>
      </div>
      <Field>
        <FieldLabel htmlFor="automation-webhook-secret">
          Signing secret <span className="font-normal text-faint">write-only</span>
        </FieldLabel>
        <Input
          autoComplete="new-password"
          className={MONO}
          data-testid="automation-webhook-secret"
          id="automation-webhook-secret"
          onChange={event => form.onWebhookSecret(event.target.value)}
          placeholder="whsec_…"
          type="password"
          value={draft.webhook_secret_value ?? ""}
        />
        <FieldDescription>
          The other app signs each request with this. CompozyOS never shows it again.
        </FieldDescription>
      </Field>
    </div>
  );
}
