import { TriangleAlert } from "lucide-react";

import { ActionResultBanner, MonoId } from "@agh/ui";

import type { RoleDiagnostic } from "../types";

interface RoleDiagnosticsNoticeProps {
  diagnostics: readonly RoleDiagnostic[];
  "data-testid"?: string;
}

/**
 * Inline warning notice for role-resolution diagnostics (e.g.
 * `role_agent_not_found`). Uses the semantic warning tone; the optional agent
 * renders as `MonoId` metadata, never a provenance chip.
 */
export function RoleDiagnosticsNotice({
  diagnostics,
  "data-testid": testId,
}: RoleDiagnosticsNoticeProps) {
  if (diagnostics.length === 0) {
    return null;
  }
  // Content-derived keys; identical diagnostics get an occurrence suffix so keys
  // stay unique without falling back to the array index.
  const seenKeys = new Map<string, number>();
  return (
    <div className="flex flex-col gap-2" data-testid={testId}>
      {diagnostics.map(diagnostic => {
        const base = `${diagnostic.code}:${diagnostic.message}`;
        const occurrence = seenKeys.get(base) ?? 0;
        seenKeys.set(base, occurrence + 1);
        return (
          <ActionResultBanner
            key={occurrence === 0 ? base : `${base}#${occurrence}`}
            tone="warning"
            icon={TriangleAlert}
            title="Warning"
            data-testid={testId ? `${testId}-${diagnostic.code}` : undefined}
            description={
              <span className="flex flex-wrap items-center gap-1.5">
                <span>{diagnostic.message}</span>
                {diagnostic.agent ? <MonoId value={diagnostic.agent} preserveCase /> : null}
              </span>
            }
          />
        );
      })}
    </div>
  );
}
