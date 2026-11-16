/**
 * The shipped Settings › MCP servers page as the backdrop (the list stays as it is), plus a row
 * panel anchored to one of its real buttons. The panel replaces the dialog that button opens today.
 */
import { useEffect, useState, type ReactNode } from "react";

import { Popover, PopoverContent } from "@compozy/ui";
import { runtimeMCPInventoryFixture } from "@/systems/settings/mocks/runtime-mcp-fixtures";
import type { SettingsMCPServerEntry } from "@/systems/settings/types";

import { MCP_PANEL_CLASS } from "../mcp-redesign-shared/mcp-panel";
import { MCPServersByRuntimePage } from "../mcp-servers-by-runtime/mcp-page";
import { COMPOZY_SERVERS } from "../mcp-servers-by-runtime/mcp-page-fixtures";

/** The first project server waits for approval, as a project file declared it. */
export function withWaiting(servers: readonly SettingsMCPServerEntry[]): SettingsMCPServerEntry[] {
  let marked = false;
  return servers.map(server => {
    if (marked || server.scope !== "workspace") return server;
    marked = true;
    return {
      ...server,
      runtime_status: null,
      callable_in_sessions: false,
      approval: { required: true, state: "waiting", definition_digest: "sha256:3f9c" },
    };
  });
}

function useDomAnchor(selector: string | undefined): Element | null {
  const [element, setElement] = useState<Element | null>(null);
  useEffect(() => {
    if (!selector) return undefined;
    let frame = 0;
    const find = () => {
      const found = document.querySelector(selector);
      if (found) setElement(found);
      else frame = requestAnimationFrame(find);
    };
    find();
    return () => cancelAnimationFrame(frame);
  }, [selector]);
  return element;
}

function AnchoredPanel({ selector, children }: { selector: string; children: ReactNode }) {
  const anchor = useDomAnchor(selector);
  if (!anchor) return null;
  return (
    <Popover defaultOpen modal={false}>
      <PopoverContent side="bottom" align="end" sideOffset={6} anchor={anchor} className={MCP_PANEL_CLASS}>
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

export function SettingsBackdrop({ waiting = false, anchor, panel, overlay }: SettingsBackdropProps) {
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
