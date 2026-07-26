import { AlertTriangle, Check } from "lucide-react";

import { cn, Pill } from "@agh/ui";

import type { JsonRow, MatchState } from "../../../lib/trigger-preview";
import { PreviewCard } from "./preview-card";

function MatchBadge({ state, label }: { state: MatchState; label: string }) {
  const Icon = state === "nomatch" ? AlertTriangle : Check;
  return (
    <Pill size="sm" tone={state === "nomatch" ? "warning" : "success"}>
      <Icon aria-hidden="true" />
      {label}
    </Pill>
  );
}

function indentOf(indent: number): string {
  return "  ".repeat(indent);
}

function JsonRowLine({ row }: { row: JsonRow }) {
  if (row.kind === "open") {
    return (
      <div>
        {indentOf(row.indent)}
        {row.label ? (
          <>
            <span className="text-info">{`"${row.label}"`}</span>
            <span className="text-muted">: </span>
          </>
        ) : null}
        {"{"}
      </div>
    );
  }

  if (row.kind === "close") {
    return (
      <div>
        {indentOf(row.indent)}
        {"}"}
        {row.comma ? "," : ""}
      </div>
    );
  }

  return (
    <div>
      {indentOf(row.indent)}
      <span
        className={cn(
          row.highlighted && "rounded-[3px] bg-accent-tint px-0.5 ring-1 ring-accent-dim ring-inset"
        )}
      >
        <span className="text-info">{`"${row.keyName}"`}</span>
        <span className="text-muted">: </span>
        <span className={row.highlighted ? "text-accent-strong" : "text-success"}>
          {`"${row.value}"`}
        </span>
      </span>
      {row.comma ? "," : ""}
    </div>
  );
}

interface SampleEventCardProps {
  json: JsonRow[];
  matchState: MatchState;
  matchLabel: string;
}

/** Sample envelope JSON with matched filter keys highlighted, plus a match badge. */
export function SampleEventCard({ json, matchState, matchLabel }: SampleEventCardProps) {
  return (
    <PreviewCard label="Sample event" right={<MatchBadge label={matchLabel} state={matchState} />}>
      <pre className="overflow-x-auto rounded border border-line-soft bg-rail p-3 font-mono text-form-hint leading-relaxed whitespace-pre text-fg">
        {json.map(row => (
          <JsonRowLine key={row.id} row={row} />
        ))}
      </pre>
    </PreviewCard>
  );
}
