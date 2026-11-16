import { FileText, Globe, ShieldCheck, Terminal } from "@compozy/ui/icons";

import { Button, PopoverPanel } from "@compozy/ui";

import type { MCPAllowPanelLaunch } from "../../lib/mcp-allow-launch";
import type { MCPApprovalReason } from "../../lib/mcp-runtime-view-model";
import { MCPCommandBlock, MCPPanelFact } from "./mcp-panel-parts";

export interface MCPAllowPanelProps {
  server: string;
  reason: MCPApprovalReason;
  launch: MCPAllowPanelLaunch | null;
  /** The project file that defines it, when known. */
  source?: string;
  /** Settings allows for the project at once; a chat picks it up on its next message. */
  appliesOn?: "now" | "next-message";
  allowing?: boolean;
  onAllow: () => void;
  onClose: () => void;
}

function fileName(path: string): string {
  return path.split(/[\\/]/).filter(Boolean).at(-1) ?? path;
}

function allowPanelSubtitle(reason: MCPApprovalReason, source: string | undefined): string {
  const file = source ? fileName(source) : undefined;
  if (reason === "changed") {
    return file
      ? `Changed in ${file} since you allowed it`
      : "Changed in a project file since you allowed it";
  }
  return file ? `Added by ${file} in this project` : "Added by a file in this project";
}

/**
 * Review a server a project file added (or changed) before allowing it: the one fact needed to
 * decide is what it runs on this computer.
 */
export function MCPAllowPanel({
  server,
  reason,
  launch,
  source,
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
      subtitle={allowPanelSubtitle(reason, source)}
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
      {source ? (
        <MCPPanelFact label="Defined in" icon={FileText}>
          <span
            data-testid="mcp-allow-panel-source"
            className="font-mono text-mono-id break-all text-muted-foreground"
          >
            {source}
          </span>
        </MCPPanelFact>
      ) : null}
    </PopoverPanel>
  );
}
