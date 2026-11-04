import { Shield } from "@compozy/ui/icons";

import { Button, ListingRow, Pill, Time } from "@compozy/ui";

import { humanizeToolId } from "@/systems/session";

import { toolApprovalDecisionTone } from "../lib/decision-tone";
import type { ToolApprovalGrant } from "../types";
import { AgentIdentityChip } from "@/systems/agent/components/agent-identity-mark";

export interface ToolApprovalGrantRowProps {
  grant: ToolApprovalGrant;
  onRevoke: (grant: ToolApprovalGrant) => void;
}

/**
 * Reads the daemon's `read:<target>@<profile>` access scope: a remembered answer to a
 * cross-project or cross-profile read prompt, never a tool approval.
 */
function accessScopeLabel(accessScope: string): string {
  const separator = accessScope.lastIndexOf("@");
  const target = separator < 0 ? accessScope : accessScope.slice(0, separator);
  const anyProfile = separator >= 0 && accessScope.slice(separator + 1) === "*";
  const profile = separator < 0 ? "" : accessScope.slice(separator + 1);
  const suffix = anyProfile ? ", any profile" : profile ? `, profile ${profile}` : "";
  if (target === "read:user") return `Reads in User scope${suffix}`;
  if (target === "read:all-projects") return `Reads across All projects${suffix}`;
  const workspace = target.replace(/^read:workspace:/, "");
  return `Reads in project ${workspace}${suffix}`;
}

function approvalGrantScopeLabel(grant: ToolApprovalGrant): string {
  if (grant.access_scope) return accessScopeLabel(grant.access_scope);
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
      data-access-scope={grant.access_scope || undefined}
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
            form="plain"
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
              <AgentIdentityChip name={grant.agent_name} />
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
        {/* A worded control, like the terminal permission rows in the same list. */}
        <Button
          aria-label={`Revoke ${toolLabel} decision`}
          data-testid={`tool-approval-grant-revoke-${grant.id}`}
          onClick={() => onRevoke(grant)}
          size="sm"
          type="button"
          variant="quiet"
        >
          Revoke
        </Button>
      </ListingRow.Trail>
    </ListingRow>
  );
}
