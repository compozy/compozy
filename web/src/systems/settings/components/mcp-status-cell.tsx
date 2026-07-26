import { Pill } from "@agh/ui";

import type { MCPStatusCell as MCPStatusCellModel } from "../lib/mcp-status-view-model";

/**
 * One composed status signal: a tone-tinted pill (with its leading dot) over an
 * optional mono detail line. Each cell traces to a single daemon field; the tone
 * is information, never decoration.
 */
export function MCPStatusCell({ cell, testId }: { cell: MCPStatusCellModel; testId?: string }) {
  return (
    <div className="min-w-0" data-testid={testId}>
      <Pill tone={cell.tone} data-testid={testId ? `${testId}-pill` : undefined}>
        <Pill.Dot />
        {cell.label}
      </Pill>
      {cell.code ? (
        <div
          className="mt-1 truncate font-mono text-mono-id text-subtle"
          title={cell.code}
          data-testid={testId ? `${testId}-code` : undefined}
        >
          {cell.code}
        </div>
      ) : null}
    </div>
  );
}
