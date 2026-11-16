/**
 * The shipped Settings › MCP servers page as the backdrop (the list stays as it is), plus a row
 * panel anchored to one of its real buttons. The panel replaces the dialog that button opens today.
 */
import type { ReactNode } from "react";

import { Popover, PopoverContent } from "@compozy/ui";
import { runtimeMCPInventoryFixture } from "@/systems/settings/mocks/runtime-mcp-fixtures";

import { MCP_PANEL_CLASS } from "../mcp-redesign-shared/mcp-panel-class";
import { MCPServersByRuntimePage } from "../mcp-servers-by-runtime/mcp-page";
import { COMPOZY_SERVERS } from "../mcp-servers-by-runtime/mcp-page-fixtures";
import { useDomAnchor } from "./hooks/use-dom-anchor";
import { withWaiting } from "./settings-backdrop-fixtures";

function AnchoredPanel({ selector, children }: { selector: string; children: ReactNode }) {
  const anchor = useDomAnchor(selector);
  if (!anchor) return null;
  return (
    <Popover defaultOpen modal={false}>
      <PopoverContent
        side="bottom"
        align="end"
        sideOffset={6}
        anchor={anchor}
        className={MCP_PANEL_CLASS}
      >
        {children}
      </PopoverContent>
    </Popover>
  );
}

export interface SettingsBackdropProps {
  waiting?: boolean;
  /** CSS selector of the real button the panel opens from. */
  anchor?: string;
  panel?: ReactNode;
  /** A dialog or confirm pinned over the page. */
  overlay?: ReactNode;
}

export function SettingsBackdrop({
  waiting = false,
  anchor,
  panel,
  overlay,
}: SettingsBackdropProps) {
  return (
    <>
      <MCPServersByRuntimePage
        compozy={waiting ? withWaiting(COMPOZY_SERVERS) : COMPOZY_SERVERS}
        runtimes={runtimeMCPInventoryFixture().runtimes}
      />
      {anchor && panel ? <AnchoredPanel selector={anchor}>{panel}</AnchoredPanel> : null}
      {overlay}
    </>
  );
}
