import { Alert, AlertActions, AlertDescription, AlertTitle, Button } from "@agh/ui";
import { CircleAlert, CircleCheck } from "lucide-react";

import type { SettingsMCPServerEntry } from "../types";

import { mcpProvenanceLine } from "./mcp-server-labels";

export interface MCPSelectionStripProps {
  selectedName: string;
  server: SettingsMCPServerEntry | null;
  scope: "workspace" | "global";
  onClear: () => void;
}

/**
 * Provenance + selection surface shown when a row is preselected (Marketplace
 * `Manage ->` or a name click). Catalog-installed servers link back to the
 * marketplace detail; hand-configured servers invent nothing.
 */
export function MCPSelectionStrip({
  selectedName,
  server,
  scope,
  onClear,
}: MCPSelectionStripProps) {
  if (!selectedName) return null;

  if (!server) {
    return (
      <Alert variant="warning" data-testid="settings-page-mcp-selection-missing">
        <CircleAlert />
        <AlertTitle>Server not found in this scope</AlertTitle>
        <AlertDescription>
          No server named {selectedName} was returned for {scope}.
        </AlertDescription>
        <AlertActions>
          <Button variant="ghost" size="sm" onClick={onClear}>
            Clear
          </Button>
        </AlertActions>
      </Alert>
    );
  }

  const provenance = mcpProvenanceLine(server.catalog_entry, server.catalog_version);
  const catalogEntry = server.catalog_entry?.trim();

  return (
    <Alert variant="info" data-testid="settings-page-mcp-selection">
      <CircleCheck />
      <AlertTitle>{server.name} selected</AlertTitle>
      <AlertDescription>
        {provenance ?? "Hand-configured server"}
        {catalogEntry ? (
          <>
            {" · "}
            <a
              className="text-fg underline underline-offset-[3px]"
              href={`/marketplace/mcp/${encodeURIComponent(catalogEntry)}`}
              data-testid="settings-page-mcp-selection-marketplace-link"
            >
              View in marketplace
            </a>
          </>
        ) : null}
      </AlertDescription>
      <AlertActions>
        <Button
          variant="ghost"
          size="sm"
          onClick={onClear}
          data-testid="settings-page-mcp-selection-clear"
        >
          Clear
        </Button>
      </AlertActions>
    </Alert>
  );
}
