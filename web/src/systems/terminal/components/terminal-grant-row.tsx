import { Keyboard, Shield, Trash2 } from "lucide-react";

import { Button, ListingRow, MonoId, Time } from "@compozy/ui";

import type { TerminalGrant } from "../lib/terminal-grant";

export interface TerminalGrantRowProps {
  grant: TerminalGrant;
  onRevoke: (grant: TerminalGrant) => void;
}

/**
 * A remembered terminal permission, in the list where every remembered decision
 * lives.
 *
 * Only its reading differs from a tool grant: this is a promise about typing or
 * about running, not a tool id, and it says so. What it will not say is *which*
 * terminal or *which* command — the daemon remembers a digest of the exact tool
 * input, and no name can be recovered from it. The digest is shown instead, as
 * the only honest way to tell two decisions apart.
 */
export function TerminalGrantRow({ grant, onRevoke }: TerminalGrantRowProps) {
  const isTyping = grant.kind === "typing";
  // Typing is always scoped to one terminal generation, so a typing row always
  // carries its digest; only a remembered command can be tool-wide.
  const label = isTyping
    ? "Can type into one exact terminal"
    : grant.inputDigest
      ? "Always allowed: one exact command"
      : "Always allowed: any command in this project";
  return (
    <ListingRow
      data-grant-id={grant.id}
      data-testid={`terminal-grant-row-${grant.id}`}
      interactive={false}
    >
      <ListingRow.Icon>
        {isTyping ? (
          <Keyboard aria-hidden="true" className="size-4" />
        ) : (
          <Shield aria-hidden="true" className="size-4" />
        )}
      </ListingRow.Icon>
      <ListingRow.Main>
        <ListingRow.Name>
          <ListingRow.Title>{label}</ListingRow.Title>
        </ListingRow.Name>
        <ListingRow.Meta>
          {grant.inputDigest ? (
            <>
              <MonoId copy copyLabel="Copy input digest" size="sm" value={grant.inputDigest} />
              <ListingRow.MetaDot />
            </>
          ) : null}
          <span>{grant.agentName}</span>
          <ListingRow.MetaDot />
          <span className="inline-flex items-center gap-1">
            {isTyping ? "granted" : "remembered"}
            <Time className="tabular-nums" iso={grant.grantedAt} />
          </span>
        </ListingRow.Meta>
      </ListingRow.Main>
      <ListingRow.Trail>
        <Button
          aria-label={`Revoke ${label}`}
          data-testid={`terminal-grant-revoke-${grant.id}`}
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
