import { Shield, Trash2 } from "lucide-react";

import { Button, ListingRow, MonoId, Pill, Time } from "@agh/ui";

import { toolApprovalDecisionTone } from "../lib/decision-tone";
import type { ToolApprovalGrant } from "../types";

export interface ToolApprovalGrantRowProps {
  grant: ToolApprovalGrant;
  onRevoke: (grant: ToolApprovalGrant) => void;
}

function approvalGrantScopeLabel(grant: ToolApprovalGrant): string {
  if (grant.input_digest) {
    return grant.agent_name ? "exact input" : "exact input · any agent";
  }
  return grant.agent_name ? "agent-wide" : "tool-wide";
}

/**
 * One remembered decision. The row states whether daemon truth is exact, agent-wide,
 * input-wide, or tool-wide; the full grant id rides on `data-grant-id`.
 */
export function ToolApprovalGrantRow({ grant, onRevoke }: ToolApprovalGrantRowProps) {
  const scopeLabel = approvalGrantScopeLabel(grant);
  return (
    <ListingRow data-testid="tool-approval-grant-row" data-grant-id={grant.id} interactive={false}>
      <ListingRow.Icon>
        <Shield aria-hidden="true" className="size-4" />
      </ListingRow.Icon>
      <ListingRow.Main>
        <ListingRow.Name mono>
          <ListingRow.Title>{grant.tool_id}</ListingRow.Title>
          <Pill
            data-testid={`tool-approval-grant-decision-${grant.id}`}
            size="sm"
            tone={toolApprovalDecisionTone(grant.decision)}
          >
            <Pill.Dot />
            {grant.decision}
          </Pill>
        </ListingRow.Name>
        <ListingRow.Meta>
          <span data-testid={`tool-approval-grant-scope-${grant.id}`}>{scopeLabel}</span>
          <ListingRow.MetaDot />
          {grant.input_digest ? (
            <>
              <MonoId copy copyLabel="Copy input digest" size="sm" value={grant.input_digest} />
              <ListingRow.MetaDot />
            </>
          ) : null}
          {grant.agent_name ? (
            <>
              <span>
                agent <span className="font-mono text-faint">{grant.agent_name}</span>
              </span>
              <ListingRow.MetaDot />
            </>
          ) : null}
          <span className="inline-flex items-center gap-1">
            last used
            <Time
              className="tabular-nums"
              data-testid={`tool-approval-grant-last-used-${grant.id}`}
              iso={grant.last_used_at}
            />
          </span>
        </ListingRow.Meta>
      </ListingRow.Main>
      <ListingRow.Trail>
        <Button
          aria-label={`Revoke ${grant.tool_id} decision`}
          data-testid={`tool-approval-grant-revoke-${grant.id}`}
          onClick={() => onRevoke(grant)}
          size="icon-sm"
          type="button"
          variant="ghost"
        >
          <Trash2 aria-hidden="true" className="size-3" />
        </Button>
      </ListingRow.Trail>
    </ListingRow>
  );
}
