import { Download, ExternalLink } from "lucide-react";
import { useState } from "react";

import { Button, Eyebrow, Pill, Spinner } from "@agh/ui";
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

/** Diagnostics: two-step support-bundle consent + the live daemon log stream row. */
export function ObservabilityDiagnosticsSection({ logTail }: { logTail: LogTailMeta }) {
  return (
    <SettingsGroup title="Diagnostics" description="support bundle and live logs">
      <SupportBundleRow />
      <LogTailRow logTail={logTail} />
    </SettingsGroup>
  );
}

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
        description={
          <span className="inline-flex flex-wrap items-center gap-1.5">
            Redacted daemon archive for sharing with maintainers
            <Eyebrow
              className="text-faint"
              data-testid="settings-page-observability-support-bundle-status"
            >
              status: {operation?.status ?? "idle"}
            </Eyebrow>
          </span>
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
          className="mx-4 mb-3 flex flex-col gap-3 rounded-md border border-line bg-canvas px-4 py-3"
          data-testid="settings-page-observability-support-bundle-consent-panel"
        >
          <label className="flex items-start gap-3 text-sm text-subtle">
            <input
              checked={approved}
              className="mt-0.5 size-4 rounded border border-line bg-canvas-soft accent-[var(--accent)] focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent"
              data-testid="settings-page-observability-support-bundle-consent"
              onChange={event => {
                setApproved(event.currentTarget.checked);
                if (event.currentTarget.checked) setConsentError(null);
              }}
              type="checkbox"
            />
            <span>I approve creating a redacted diagnostics archive.</span>
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
            <Eyebrow className="text-muted" data-testid="settings-page-observability-support-size">
              size: {formatBytes(operation.size_bytes)}
            </Eyebrow>
          ) : null}
          {errorMessage ? (
            <p
              className="text-sm text-danger"
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
      label="Live daemon logs"
      description={
        <span className="inline-flex flex-wrap items-center gap-1.5">
          <Pill size="xs" tone={logTail.available ? "success" : "neutral"}>
            {logTail.available ? "Available" : "Unavailable"}
          </Pill>
          <span data-testid="settings-page-observability-log-tail-transport">
            transport: {logTail.transport ?? "none"}
          </span>
        </span>
      }
      control={
        logTail.available && streamURL ? (
          <a
            className="inline-flex items-center gap-1.5 text-sm text-fg transition-colors duration-base hover:text-fg-strong focus-visible:shadow-focus-ring focus-visible:outline-none"
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
