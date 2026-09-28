import { Shield, Trash2 } from "lucide-react";

import { Button, ListingRow, Pill, Time } from "@compozy/ui";

import { humanizeToolId } from "@/systems/session";

import { toolApprovalDecisionTone } from "../lib/decision-tone";
import type { ToolApprovalGrant } from "../types";

export interface ToolApprovalGrantRowProps {
  grant: ToolApprovalGrant;
  onRevoke: (grant: ToolApprovalGrant) => void;
}

function approvalGrantScopeLabel(grant: ToolApprovalGrant): string {
  if (grant.input_digest) {
    return grant.agent_name ? "Only this exact request" : "Only this exact request, any agent";
  }
  return grant.agent_name ? "Every request from this agent" : "Every request, any agent";
}

const DECISION_LABEL: Record<string, string> = { allow: "Allowed", reject: "Blocked" };

/**
 * One remembered decision. The row states whether daemon truth is exact, agent-wide,
 * input-wide, or tool-wide in plain words; the grant id, raw tool id, and input
 * digest ride on data attributes and the title tooltip.
 */
export function ToolApprovalGrantRow({ grant, onRevoke }: ToolApprovalGrantRowProps) {
  const scopeLabel = approvalGrantScopeLabel(grant);
  const toolLabel = humanizeToolId(grant.tool_id);
  return (
    <ListingRow
      data-testid="tool-approval-grant-row"
      data-grant-id={grant.id}
      data-input-digest={grant.input_digest || undefined}
      data-tool-id={grant.tool_id}
      interactive={false}
    >
      <ListingRow.Icon>
        <Shield aria-hidden="true" className="size-4" />
      </ListingRow.Icon>
      <ListingRow.Main>
        <ListingRow.Name>
          <ListingRow.Title title={grant.tool_id}>{toolLabel}</ListingRow.Title>
          <Pill
            data-testid={`tool-approval-grant-decision-${grant.id}`}
            size="sm"
            tone={toolApprovalDecisionTone(grant.decision)}
          >
            <Pill.Dot />
            {DECISION_LABEL[grant.decision] ?? grant.decision}
          </Pill>
        </ListingRow.Name>
        <ListingRow.Meta>
          <span data-testid={`tool-approval-grant-scope-${grant.id}`}>{scopeLabel}</span>
          <ListingRow.MetaDot />
          {grant.agent_name ? (
            <>
              <span>{grant.agent_name}</span>
              <ListingRow.MetaDot />
            </>
          ) : null}
          <span className="inline-flex items-center gap-1">
            Last used
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
          aria-label={`Revoke ${toolLabel} decision`}
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
