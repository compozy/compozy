import { useEffect, useState } from "react";

import { mcpPanelAnchor, type MCPPanelAnchor } from "@/systems/settings/lib/mcp-panel-anchor";
import type { RuntimeMCPServer } from "@/systems/settings/types";

/** The real Copy button of a runtime row, once the page has rendered it. */
export function useCopyButtonAnchor(server: RuntimeMCPServer | undefined): MCPPanelAnchor | null {
  const [anchor, setAnchor] = useState<MCPPanelAnchor | null>(null);
  const testId = server ? `settings-mcp-runtime-row-${server.runtime}-${server.name}-copy` : null;
  useEffect(() => {
    if (!testId) return undefined;
    let frame = 0;
    const find = () => {
      const found = document.querySelector<HTMLElement>(`[data-testid="${testId}"]`);
      if (found) setAnchor(mcpPanelAnchor(found));
      else frame = requestAnimationFrame(find);
    };
    find();
    return () => cancelAnimationFrame(frame);
  }, [testId]);
  return testId ? anchor : null;
}
