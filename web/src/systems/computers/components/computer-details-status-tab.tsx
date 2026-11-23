import { useState } from "react";
import { Copy, RefreshCw, TriangleAlert } from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  CodeBlock,
  Disclosure,
  MonoId,
  Progress,
  ProgressLabel,
  ProgressValue,
  PropertyRow,
  Spinner,
  toast,
} from "@compozy/ui";

import type { ComputerFixture } from "../mocks/remote-computers-fixtures";
import { buildHealthyDiagnosisSteps } from "../mocks/remote-computers-fixtures-lane-b";
import { ComputerLoadMeter } from "./computer-load-meter";
import { StepProgressList, type StepProgressItem } from "./step-progress-list";

const GB = 1024 * 1024 * 1024;

function formatGb(bytes: number): string {
  const gb = bytes / GB;
  return gb >= 100 ? `${Math.round(gb)}` : gb.toFixed(1);
}

function formatPlatform(platform: ComputerFixture["platform"]): string {
  switch (platform) {
    case "darwin_arm64":
      return "macOS (Apple Silicon)";
    case "darwin_x86_64":
      return "macOS (Intel)";
    case "linux_x86_64":
      return "Linux (x86_64)";
    case "linux_arm64":
      return "Linux (ARM64)";
    case "windows_x86_64":
      return "Windows (x86_64)";
  }
}

function buildDiagnosticsText(computer: ComputerFixture): string {
  const lines = [
    `Computer: ${computer.name} (${computer.id})`,
    `Status: ${computer.status}`,
    `Version: ${computer.version}`,
    `Platform: ${computer.platform}`,
    `Last connected: ${computer.lastConnectedLabel}`,
  ];
  if (computer.diagnosis) {
    lines.push(`Failing step: ${computer.diagnosis.failingStep}`);
    lines.push(`Error: ${computer.diagnosis.code} — ${computer.diagnosis.message}`);
    for (const step of computer.diagnosis.steps) {
      lines.push(`  [${step.status}] ${step.label}: ${step.message ?? "—"}`);
    }
  }
  return lines.join("\n");
}

export interface ComputerDetailsStatusTabProps {
  computer: ComputerFixture;
  onSelectTab?: (tab: "routes" | "permissions" | "providers" | "update") => void;
  onRetry?: () => void;
}

/**
 * Status tab of `ComputerDetailsPanel` (`_uiux.md` U7):
 * - Keeps only what isn't in the header or sibling tabs: alerts, capacity meters,
 *   flat `StepProgressList` connection check/diagnosis, and a collapsed `About` fold.
 */
export function ComputerDetailsStatusTab({
  computer,
  onSelectTab,
  onRetry,
}: ComputerDetailsStatusTabProps) {
  const [retrying, setRetrying] = useState(false);

  const load = computer.load && !computer.load.stale ? computer.load : null;
  const cpuPct = load ? Math.max(0, Math.min(100, Math.round(load.cpuUtil * 100))) : null;
  const memUsedBytes = load ? Math.max(0, load.memTotalBytes - load.memAvailableBytes) : 0;
  const memPct =
    load && load.memTotalBytes > 0
      ? Math.max(0, Math.min(100, Math.round((memUsedBytes / load.memTotalBytes) * 100)))
      : null;

  const diagnosisSteps: StepProgressItem[] = computer.diagnosis
    ? computer.diagnosis.steps.map(step => ({
        id: step.step,
        title: step.label,
        status: step.status,
        detail: step.at,
        caption: step.message,
      }))
    : buildHealthyDiagnosisSteps(computer);

  const handleRetry = () => {
    setRetrying(true);
    onRetry?.();
    toast.message(`Retrying connection to ${computer.name}…`);
    setTimeout(() => setRetrying(false), 1200);
  };

  const handleCopyDiagnostics = async () => {
    const text = buildDiagnosticsText(computer);
    try {
      if (typeof navigator !== "undefined" && navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(text);
      }
      toast.success("Diagnostics copied to clipboard.");
    } catch {
      toast.success("Diagnostics ready.");
    }
  };

  return (
    <div className="flex flex-col gap-5" data-slot="computer-details-status-tab">
      {computer.attentionReason ? (
        <Alert variant="warning">
          <TriangleAlert aria-hidden="true" />
          <AlertTitle>{computer.attentionReason.summary}</AlertTitle>
          <AlertDescription className="flex flex-col gap-2.5">
            <p>{computer.attentionReason.detail}</p>
            {computer.attentionReason.command ? (
              <CodeBlock
                code={computer.attentionReason.command}
                copyable
                density="compact"
                language="bash"
                showPrompt
              />
            ) : null}
            {computer.attentionReason.kind === "provider_signed_out" && onSelectTab ? (
              <Button
                className="w-fit"
                onClick={() => onSelectTab("providers")}
                size="xs"
                type="button"
                variant="secondary"
              >
                Open Providers
              </Button>
            ) : null}
          </AlertDescription>
        </Alert>
      ) : null}

      {computer.status === "incompatible" && onSelectTab ? (
        <Alert variant="warning">
          <TriangleAlert aria-hidden="true" />
          <AlertTitle>{computer.name} needs an update</AlertTitle>
          <AlertDescription className="flex flex-col gap-2">
            <span>
              Runs CompozyOS {computer.version}. This app requires 0.8.0 or newer to connect.
            </span>
            <Button
              className="w-fit"
              onClick={() => onSelectTab("update")}
              size="xs"
              type="button"
              variant="secondary"
            >
              Open Update
            </Button>
          </AlertDescription>
        </Alert>
      ) : null}

      {computer.diagnosis ? (
        <section className="flex flex-col gap-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <h3 className="text-small-body font-semibold text-fg-strong">Connection diagnosis</h3>
            {computer.diagnosis.retryInSeconds ? (
              <span className="text-meta text-muted-foreground tabular-nums">
                Retrying in {computer.diagnosis.retryInSeconds}s
                {computer.diagnosis.attempt ? ` · attempt ${computer.diagnosis.attempt}` : ""}
              </span>
            ) : null}
          </div>

          <StepProgressList data-testid="computer-details-diagnosis-steps" steps={diagnosisSteps} />

          {computer.diagnosis.whatToTry && computer.diagnosis.whatToTry.length > 0 ? (
            <div className="flex flex-col gap-1.5 pt-1">
              <h4 className="eyebrow text-subtle">What to try</h4>
              <ol className="flex list-decimal flex-col gap-1.5 pl-4 text-meta text-muted-foreground">
                {computer.diagnosis.whatToTry.map(tip => (
                  <li key={tip}>{tip}</li>
                ))}
              </ol>
            </div>
          ) : null}

          <div className="flex flex-wrap items-center gap-2 pt-0.5">
            <Button
              disabled={retrying}
              onClick={handleRetry}
              size="sm"
              type="button"
              variant="secondary"
            >
              {retrying ? <Spinner className="size-3.5" /> : <RefreshCw aria-hidden="true" />}
              {retrying ? "Retrying…" : "Retry now"}
            </Button>
            <Button
              onClick={() => void handleCopyDiagnostics()}
              size="sm"
              type="button"
              variant="ghost"
            >
              <Copy aria-hidden="true" />
              Copy diagnostics
            </Button>
          </div>
        </section>
      ) : null}

      <section className="flex flex-col gap-3 border-t border-line-soft pt-4 first:border-t-0 first:pt-0">
        <div className="flex items-center justify-between gap-2">
          <h3 className="text-small-body font-semibold text-fg-strong">Capacity</h3>
          {load ? (
            <ComputerLoadMeter computerName={computer.name} load={load} />
          ) : (
            <span className="text-meta text-faint">Unsampled</span>
          )}
        </div>

        {load && cpuPct !== null && memPct !== null ? (
          <div className="flex flex-col gap-3">
            <Progress tone="brand" value={cpuPct}>
              <ProgressLabel className="text-meta">CPU</ProgressLabel>
              <ProgressValue className="text-meta">
                {() => `${cpuPct}% · ${load.cpuCount} cores`}
              </ProgressValue>
            </Progress>
            <Progress tone="brand" value={memPct}>
              <ProgressLabel className="text-meta">Memory</ProgressLabel>
              <ProgressValue className="text-meta">
                {() =>
                  `${formatGb(load.memAvailableBytes)} / ${formatGb(load.memTotalBytes)} GB free`
                }
              </ProgressValue>
            </Progress>
            <p className="text-badge text-faint tabular-nums">
              Sampled {load.sampledAgoLabel} · used by Load balancing when Auto is on
            </p>
          </div>
        ) : (
          <p className="text-meta text-muted-foreground">
            No live CPU or memory sample available. Last connected {computer.lastConnectedLabel}.
          </p>
        )}
      </section>

      {!computer.diagnosis ? (
        <section className="flex flex-col gap-3 border-t border-line-soft pt-4">
          <div className="flex items-center justify-between gap-2">
            <h3 className="text-small-body font-semibold text-fg-strong">Connection check</h3>
            <span className="text-meta text-success">All 4 checks passed</span>
          </div>
          <StepProgressList steps={diagnosisSteps} />
        </section>
      ) : null}

      <section className="border-t border-line-soft pt-3">
        <Disclosure label="About" variant="inline">
          <div className="flex flex-col divide-y divide-line-soft">
            <PropertyRow label="Version">CompozyOS {computer.version}</PropertyRow>
            <PropertyRow label="Platform">{formatPlatform(computer.platform)}</PropertyRow>
            <PropertyRow label="Added via">
              {computer.originKind === "ssh" ? "SSH" : "Account link"}
            </PropertyRow>
            <PropertyRow label="Computer ID">
              <MonoId copy preserveCase value={computer.id} />
            </PropertyRow>
          </div>
        </Disclosure>
      </section>
    </div>
  );
}
