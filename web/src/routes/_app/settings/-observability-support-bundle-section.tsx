import { Download, ExternalLink } from "lucide-react";
import { useState } from "react";

import { Button, Checkbox, Pill, Spinner } from "@compozy/ui";
import {
  SettingsFieldRow,
  SettingsGroup,
  type SettingsObservabilitySection,
} from "@/systems/settings";
import { useSupportBundleDownload } from "@/systems/support";
import { formatBytes } from "./-observability-format";

type LogTailMeta = SettingsObservabilitySection["log_tail"];

function safeLogTailURL(value: string | undefined): string | null {
  if (!value || !URL.canParse(value, "http://localhost")) return null;
  const protocol = new URL(value, "http://localhost").protocol;
  return protocol === "http:" || protocol === "https:" ? value : null;
}

/** Diagnostics: two-step support-bundle consent + the live log stream row. */
export function ObservabilityDiagnosticsSection({ logTail }: { logTail: LogTailMeta }) {
  return (
    <SettingsGroup title="Diagnostics">
      <SupportBundleRow />
      <LogTailRow logTail={logTail} />
    </SettingsGroup>
  );
}

const BUNDLE_STATUS_LABEL: Record<string, string> = {
  completed: "Last bundle ready",
  failed: "Last bundle failed",
};

function SupportBundleRow() {
  const supportBundle = useSupportBundleDownload();
  const [consentOpen, setConsentOpen] = useState(false);
  const [approved, setApproved] = useState(false);
  const [consentError, setConsentError] = useState<string | null>(null);
  const operation = supportBundle.operation;
  const errorMessage =
    consentError ??
    (supportBundle.error instanceof Error ? supportBundle.error.message : undefined);

  const closeConsent = () => {
    setConsentOpen(false);
    setApproved(false);
    setConsentError(null);
  };

  const handleCreate = async () => {
    if (!approved) {
      setConsentError("Approval is required before creating a support bundle.");
      return;
    }
    setConsentError(null);
    try {
      await supportBundle.create({ includeStatus: true, yes: true });
      setConsentOpen(false);
      setApproved(false);
    } catch (error) {
      setConsentError(
        error instanceof Error ? error.message : "Support bundle creation failed. Try again."
      );
    }
  };

  return (
    <div className="flex flex-col" data-testid="settings-page-observability-support-bundle">
      <SettingsFieldRow
        label="Support bundle"
        help="A privacy-safe bundle to share with support"
        description={
          operation ? (
            <span
              className="text-form-hint text-subtle"
              data-testid="settings-page-observability-support-bundle-status"
              title={operation.status}
            >
              {BUNDLE_STATUS_LABEL[operation.status] ?? "Preparing bundle…"}
            </span>
          ) : undefined
        }
        control={
          consentOpen ? null : (
            <Button
              data-testid="settings-page-observability-support-bundle-button"
              disabled={supportBundle.isPending}
              onClick={() => setConsentOpen(true)}
              size="sm"
              type="button"
              variant="neutral"
            >
              Create bundle
            </Button>
          )
        }
      />
      {consentOpen ? (
        <div
          className="mx-4 mb-3 flex flex-col gap-3 rounded-lg bg-sunken px-4 py-3"
          data-testid="settings-page-observability-support-bundle-consent-panel"
        >
          <label className="flex items-start gap-3 text-small-body text-subtle">
            <Checkbox
              checked={approved}
              className="mt-0.5"
              data-testid="settings-page-observability-support-bundle-consent"
              onCheckedChange={checked => {
                setApproved(checked);
                if (checked) setConsentError(null);
              }}
            />
            <span>I approve creating a privacy-safe support bundle.</span>
          </label>
          <div className="flex items-center gap-2">
            <Button
              data-testid="settings-page-observability-support-bundle-create"
              disabled={supportBundle.isPending || !approved}
              onClick={() => void handleCreate()}
              size="sm"
              type="button"
            >
              {supportBundle.isPending ? (
                <Spinner className="size-3.5" />
              ) : (
                <Download className="size-3.5" />
              )}
              {supportBundle.isPending ? "Preparing" : "Create & download"}
            </Button>
            <Button
              data-testid="settings-page-observability-support-bundle-cancel"
              disabled={supportBundle.isPending}
              onClick={closeConsent}
              size="sm"
              type="button"
              variant="ghost"
            >
              Cancel
            </Button>
          </div>
          {operation?.size_bytes ? (
            <span
              className="text-form-hint text-muted"
              data-testid="settings-page-observability-support-size"
            >
              Size {formatBytes(operation.size_bytes)}
            </span>
          ) : null}
          {errorMessage ? (
            <p
              className="text-small-body text-danger"
              data-testid="settings-page-observability-support-bundle-error"
              role="alert"
            >
              {errorMessage}
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function LogTailRow({ logTail }: { logTail: LogTailMeta }) {
  const streamURL = safeLogTailURL(logTail.stream_url);
  return (
    <SettingsFieldRow
      data-testid="settings-page-observability-log-tail"
      label="Live logs"
      description={
        <Pill
          data-testid="settings-page-observability-log-tail-transport"
          size="xs"
          title={logTail.transport ? `Transport: ${logTail.transport}` : undefined}
          tone={logTail.available ? "success" : "neutral"}
        >
          {logTail.available ? "Available" : "Unavailable"}
        </Pill>
      }
      control={
        logTail.available && streamURL ? (
          <a
            className="inline-flex items-center gap-1.5 text-small-body text-fg transition-colors duration-base hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none"
            data-testid="settings-page-observability-log-tail-link"
            href={streamURL}
            rel="noreferrer"
            target="_blank"
          >
            <ExternalLink className="size-3" />
            Open stream
          </a>
        ) : null
      }
    />
  );
}
