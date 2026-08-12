import { ShieldCheck } from "lucide-react";

import { Button, Empty, MonoId, Spinner } from "@compozy/ui";

import { auditSeverityCopy } from "../lib/gateway-copy";
import type { GatewayAuditReport } from "../types";
import { GatewayStatusChip } from "./gateway-status-chip";
import { SettingsGroup } from "@/systems/settings";

export interface GatewayAuditPanelProps {
  hasRun: boolean;
  isRunning: boolean;
  error?: string | null;
  report: GatewayAuditReport | null;
  onRun: () => void;
}

/**
 * The self-audit result. Findings arrive already ranked by the daemon
 * (critical → error → warning → info, then by stable id), so they are rendered
 * in the order received rather than re-sorted here. "No findings" is an
 * explicit result, never an empty list left to interpretation.
 */
export function GatewayAuditPanel({
  hasRun,
  isRunning,
  error,
  report,
  onRun,
}: GatewayAuditPanelProps) {
  return (
    <SettingsGroup
      action={
        <Button
          data-testid="gateway-audit-run"
          disabled={isRunning}
          onClick={onRun}
          size="sm"
          type="button"
          variant="neutral"
        >
          {hasRun ? "Run again" : "Run audit"}
        </Button>
      }
      data-testid="gateway-audit-panel"
      title="Self-audit"
    >
      {isRunning ? (
        <div className="flex items-center justify-center py-8" role="status">
          <Spinner aria-label="Running the gateway audit" className="size-5 text-subtle" />
        </div>
      ) : null}

      {!isRunning && error ? (
        <p className="px-4 py-4 text-form-label text-danger" role="alert">
          {error}
        </p>
      ) : null}

      {!isRunning && !error && !hasRun ? (
        <Empty
          className="py-8"
          description="Run the audit to see what is reachable right now and what to do about it."
          title="Audit has not run yet"
        />
      ) : null}

      {!isRunning && !error && hasRun && report?.no_findings ? (
        <Empty
          className="py-8"
          data-testid="gateway-audit-clear"
          description={
            report.local_only
              ? "Nothing is reachable beyond this machine."
              : "Every reachable surface matches what you asked for."
          }
          icon={ShieldCheck}
          title="No findings"
        />
      ) : null}

      {!isRunning && !error && hasRun && report && !report.no_findings
        ? report.findings.map(finding => {
            const severity = auditSeverityCopy(finding.severity);
            return (
              <article
                className="flex flex-col gap-2 border-t border-line-soft px-4 py-3 first:border-t-0"
                data-testid={`gateway-audit-finding-${finding.id}`}
                key={finding.id}
              >
                <div className="flex flex-wrap items-center gap-2">
                  <GatewayStatusChip label={severity.label} tone={severity.tone} />
                  <MonoId preserveCase value={finding.id} />
                </div>
                <p className="text-form-input text-fg">{finding.summary}</p>
                <p
                  className="text-form-label text-muted"
                  data-testid={`gateway-audit-remediation-${finding.id}`}
                >
                  {finding.remediation}
                </p>
              </article>
            );
          })
        : null}
    </SettingsGroup>
  );
}
