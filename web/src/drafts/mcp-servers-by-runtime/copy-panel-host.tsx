import { MCPAnchoredPanel } from "@/systems/settings/components/mcp-panels/mcp-anchored-panel";
import { MCPCopyPanel } from "@/systems/settings/components/mcp-panels/mcp-copy-panel";
import { ProviderLogo } from "@/systems/settings/components/provider-logo";
import { runtimeProviderName } from "@/systems/settings/lib/mcp-runtime-view-model";
import type { RuntimeMCPServer, RuntimeMCPSnapshot } from "@/systems/settings/types";

import { useCopyButtonAnchor } from "./hooks/use-copy-button-anchor";
import { copyFailure, type CopyDialogCase } from "./mcp-page-fixtures";

/** The production Add to CompozyOS panel beside the row it copies, for the `copy` states. */
export function CopyPanelHost({
  copying,
  copy,
  onClose,
}: {
  copying: { snapshot: RuntimeMCPSnapshot; server: RuntimeMCPServer } | null;
  copy: CopyDialogCase | undefined;
  onClose: () => void;
}) {
  const anchor = useCopyButtonAnchor(copying?.server);
  return (
    <MCPAnchoredPanel anchor={anchor} open={copying !== null} onClose={onClose}>
      {copying ? (
        <MCPCopyPanel
          canTurnOff={copying.snapshot.runtime_mcp.can_disable}
          failure={copyFailure(copy, copying.server)}
          mark={
            <ProviderLogo
              provider={runtimeProviderName(copying.server.runtime)}
              className="size-3.5"
            />
          }
          onClose={onClose}
          onSubmit={onClose}
          profile={null}
          runtimeName={copying.snapshot.display_name}
          server={copying.server}
          submitting={copy === "submitting"}
          workspaceId="ws-platform"
        />
      ) : null}
    </MCPAnchoredPanel>
  );
}
