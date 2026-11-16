import type { MCPServerManagement } from "../hooks/use-settings-mcp-page";
import { mcpDefinitionKey } from "../lib/mcp-management-target";
import { mcpApprovalReason, runtimeProviderName } from "../lib/mcp-runtime-view-model";
import { mcpAllowPanelLaunch } from "../lib/mcp-allow-launch";
import { summarizeMCPStatus } from "../lib/mcp-status-view-model";
import { MCPAllowPanel } from "./mcp-panels/mcp-allow-panel";
import { MCPAnchoredPanel } from "./mcp-panels/mcp-anchored-panel";
import { MCPCopyPanel } from "./mcp-panels/mcp-copy-panel";
import { MCPIssuePanel } from "./mcp-panels/mcp-issue-panel";
import { MCPSignInPopover } from "./mcp-panels/mcp-sign-in-popover";
import { MCPRuntimeModeSwitch } from "./mcp-runtime-mode-switch";
import { MCPOverrideEditor } from "./mcp-override-editor";
import { MCPSelectionStrip } from "./mcp-selection-strip";
import { MCPServerDeleteDialog } from "./mcp-server-delete-dialog";
import { MCPServerEditor } from "./mcp-server-editor";
import { isExtensionOwnedMCPServer } from "./mcp-server-labels";
import { MCPServersByRuntimeView } from "./mcp-servers-by-runtime-view";
import { ProviderLogo } from "./provider-logo";

interface MCPServersManagementProps {
  management: MCPServerManagement;
}

/**
 * Selection strip, then the servers by owner: the CompozyOS group and one read-only group per
 * runtime, with loading · error · empty · no match one at a time. Shared by Settings › MCP servers
 * and the Marketplace MCPs view.
 */
export function MCPServersManagementBody({ management }: MCPServersManagementProps) {
  const selected = management.selected;
  return (
    <>
      {selected ? (
        <MCPSelectionStrip
          onEdit={management.edit}
          onRemove={isExtensionOwnedMCPServer(selected) ? undefined : management.requestRemove}
          server={selected}
        />
      ) : null}
      <MCPServersManagementState management={management} />
    </>
  );
}

function MCPServersManagementState({ management }: MCPServersManagementProps) {
  const { collection, runtime } = management;
  const inventory = runtime.inventory;
  return (
    <MCPServersByRuntimeView
      compozy={{
        state: collection.isPending
          ? "pending"
          : collection.error && !collection.data
            ? "error"
            : "ready",
        errorMessage: collection.error?.message,
        onRetry: () => void collection.refetch(),
        servers: management.servers,
        visibleServers: management.filteredServers,
        onCreate: management.openCreate,
        table: {
          onAuthorize: management.authorize,
          onEdit: management.edit,
          onSelect: management.select,
          selectedServer: management.selectedKey,
          onAllow: runtime.openAllow,
          onCheck: management.serverCheck.open,
          allowingServer: runtime.allowingKey,
          approvalNotes: runtime.approvalNotes,
          highlightedServer: runtime.highlightedKey,
        },
      }}
      filter={runtime.runtimeFilter}
      onClearQuery={() => management.setQuery("")}
      onCopy={runtime.openCopy}
      onFilterChange={runtime.setRuntimeFilter}
      onShowFile={runtime.showFile}
      query={management.query}
      renderModeSwitch={snapshot => (
        <MCPRuntimeModeSwitch
          data-testid={`settings-mcp-runtime-switch-${snapshot.runtime}`}
          mode={snapshot.runtime_mcp.mode}
          provider={runtimeProviderName(snapshot.runtime)}
          runtimeName={snapshot.display_name}
        />
      )}
      runtimes={{
        state: inventory.isPending
          ? "pending"
          : inventory.error && !inventory.data
            ? "error"
            : "ready",
        errorMessage: inventory.error?.message,
        onRetry: () => void inventory.refetch(),
        snapshots: runtime.snapshots,
      }}
    />
  );
}

/** The Allow, Not working and Add to CompozyOS panels, each beside the row button that opened it. */
function MCPServerRowPanels({ management }: MCPServersManagementProps) {
  const runtime = management.runtime;
  const serverCheck = management.serverCheck;
  const inspecting = serverCheck.inspecting;
  const reviewing = runtime.reviewing;
  const copying = runtime.copying;
  return (
    <>
      <MCPAnchoredPanel
        anchor={reviewing?.anchor ?? null}
        open={reviewing !== null}
        onClose={runtime.closeAllow}
      >
        {reviewing ? (
          <MCPAllowPanel
            server={reviewing.entry.name}
            reason={
              runtime.approvalNotes[mcpDefinitionKey(reviewing.entry)]?.reason ??
              mcpApprovalReason(reviewing.entry)
            }
            launch={mcpAllowPanelLaunch(reviewing.entry)}
            source={reviewing.entry.definition_source}
            allowing={runtime.allowingKey !== undefined}
            onAllow={() => {
              runtime.closeAllow();
              runtime.allow(reviewing.entry);
            }}
            onClose={runtime.closeAllow}
          />
        ) : null}
      </MCPAnchoredPanel>
      <MCPAnchoredPanel
        anchor={inspecting?.anchor ?? null}
        open={inspecting !== null}
        onClose={serverCheck.close}
      >
        {inspecting ? (
          <MCPIssuePanel
            server={inspecting.entry.name}
            local={inspecting.entry.transport === "stdio"}
            summary={summarizeMCPStatus(inspecting.entry).label}
            reason={
              inspecting.entry.runtime_status?.diagnostic?.trim() ||
              inspecting.entry.runtime_status?.reason?.trim() ||
              null
            }
            checking={serverCheck.checking}
            stillFailing={serverCheck.stillFailing}
            onCheck={serverCheck.check}
            onClose={serverCheck.close}
          />
        ) : null}
      </MCPAnchoredPanel>
      <MCPAnchoredPanel
        anchor={copying?.anchor ?? null}
        open={copying !== null}
        onClose={runtime.closeCopy}
      >
        {copying ? (
          <MCPCopyPanel
            key={JSON.stringify([copying.server.runtime, copying.server.name])}
            canTurnOff={copying.snapshot.runtime_mcp.can_disable}
            failure={runtime.copyFailure}
            mark={
              <ProviderLogo
                provider={runtimeProviderName(copying.server.runtime)}
                className="size-3.5"
              />
            }
            onClose={runtime.closeCopy}
            onSubmit={runtime.submitCopy}
            profile={management.profile === "default" ? null : management.profile}
            runtimeName={copying.snapshot.display_name}
            server={copying.server}
            submitting={runtime.copySubmitting}
            workspaceId={management.workspaceId}
          />
        ) : null}
      </MCPAnchoredPanel>
    </>
  );
}

/** Editor, extension override editor, sign-in, row panels, and remove confirmation. */
export function MCPServersManagementDialogs({ management }: MCPServersManagementProps) {
  const editorProps = management.editorProps;
  return (
    <>
      {editorProps ? (
        <MCPServerEditor
          {...editorProps}
          onRemove={
            editorProps.entry && !isExtensionOwnedMCPServer(editorProps.entry)
              ? () => {
                  const entry = editorProps.entry;
                  editorProps.onClose();
                  if (entry) management.requestRemove(entry);
                }
              : undefined
          }
        />
      ) : null}
      {management.overrideProps ? <MCPOverrideEditor {...management.overrideProps} /> : null}
      <MCPSignInPopover
        authorize={management.authorization.authorize}
        server={management.authorization.server}
        appliesOn="now"
      />
      <MCPServerRowPanels management={management} />
      <MCPServerDeleteDialog
        error={management.removal.error?.message ?? null}
        isDeleting={management.removal.isPending}
        onClose={management.closeRemove}
        onConfirm={management.remove}
        target={management.removing}
      />
    </>
  );
}
