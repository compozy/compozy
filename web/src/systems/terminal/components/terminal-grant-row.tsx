import { Shield } from "lucide-react";

import { Button, ListingRow, MonoId, Time } from "@compozy/ui";

import { terminalGrantLabel } from "../lib/terminal-permission-copy";
import type { TerminalGrant } from "../lib/terminal-grant";

export interface TerminalGrantRowProps {
  grant: TerminalGrant;
  onRevoke: (grant: TerminalGrant) => void;
}

/**
 * A remembered terminal permission, in the list where every remembered decision
 * lives.
 *
 * The daemon stores a digest of the exact tool input — never the command line
 * or a terminal id — so the row says what the decision covers and shows the
 * digest as the only honest identity.
 */
export function TerminalGrantRow({ grant, onRevoke }: TerminalGrantRowProps) {
  const label = terminalGrantLabel(grant);
  return (
    <ListingRow
      data-grant-id={grant.id}
      data-testid={`terminal-grant-row-${grant.id}`}
      interactive={false}
    >
      <ListingRow.Icon>
        <Shield aria-hidden="true" className="size-4" />
      </ListingRow.Icon>
      <ListingRow.Main>
        <ListingRow.Name>
          <ListingRow.Title>{label}</ListingRow.Title>
        </ListingRow.Name>
        <ListingRow.Meta>
          <MonoId copy copyLabel="Copy input digest" size="sm" value={grant.inputDigest} />
          <ListingRow.MetaDot />
          <span>{grant.agentName}</span>
          <ListingRow.MetaDot />
          <span className="inline-flex items-center gap-1">
            remembered
            <Time className="tabular-nums" iso={grant.grantedAt} />
          </span>
        </ListingRow.Meta>
      </ListingRow.Main>
      <ListingRow.Trail>
        <Button
          aria-label={`Revoke ${label}`}
          data-testid={`terminal-grant-revoke-${grant.id}`}
          onClick={() => onRevoke(grant)}
          size="sm"
          type="button"
          variant="ghost"
        >
          Revoke
        </Button>
      </ListingRow.Trail>
    </ListingRow>
  );
}
