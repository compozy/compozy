import { Globe, ShieldCheck, Terminal } from "@compozy/ui/icons";

import { Button, PopoverPanel } from "@compozy/ui";

import type { MCPAllowPanelLaunch } from "../../lib/mcp-allow-launch";
import type { MCPApprovalReason } from "../../lib/mcp-runtime-view-model";
import { MCPCommandBlock, MCPPanelFact } from "./mcp-panel-parts";

export interface MCPAllowPanelProps {
  server: string;
  reason: MCPApprovalReason;
  launch: MCPAllowPanelLaunch | null;
  /** Settings allows for the project at once; a chat picks it up on its next message. */
  appliesOn?: "now" | "next-message";
  allowing?: boolean;
  onAllow: () => void;
  onClose: () => void;
}

/**
 * Review a server a project file added (or changed) before allowing it: the one fact needed to
 * decide is what it runs on this computer.
 */
export function MCPAllowPanel({
  server,
  reason,
  launch,
  appliesOn = "now",
  allowing = false,
  onAllow,
  onClose,
}: MCPAllowPanelProps) {
  return (
    <PopoverPanel
      data-testid="mcp-allow-panel"
      icon={ShieldCheck}
      tone="warning"
      title={`Allow ${server}?`}
      subtitle={
        reason === "changed"
          ? "Changed in a project file since you allowed it"
          : "Added by a file in this project"
      }
      onClose={onClose}
      footer={
        <>
          <Button type="button" size="sm" variant="ghost" className="ml-auto" onClick={onClose}>
            Not now
          </Button>
          <Button
            type="button"
            size="sm"
            disabled={allowing}
            onClick={onAllow}
            data-testid="mcp-allow-panel-allow"
          >
            {appliesOn === "now" ? "Allow" : "Allow from next message"}
          </Button>
        </>
      }
    >
      <p className="text-meta text-muted-foreground">
        Project files can add servers that run on your computer. Allow it only if you trust what it
        runs.
      </p>
      {launch ? (
        <MCPPanelFact
          label={launch.kind === "command" ? "It runs" : "It connects to"}
          icon={launch.kind === "command" ? Terminal : Globe}
        >
          <MCPCommandBlock testId="mcp-allow-panel-launch">{launch.value}</MCPCommandBlock>
        </MCPPanelFact>
      ) : null}
    </PopoverPanel>
  );
}
