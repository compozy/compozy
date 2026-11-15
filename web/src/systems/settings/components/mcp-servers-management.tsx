import type { MCPServerManagement } from "../hooks/use-settings-mcp-page";
import { runtimeProviderName } from "../lib/mcp-runtime-view-model";
import { MCPAuthorizeDialog } from "./mcp-authorize-dialog";
import { MCPCopyToCompozyDialog } from "./mcp-copy-to-compozy-dialog";
import { MCPRuntimeModeSwitch } from "./mcp-runtime-mode-switch";
import { MCPOverrideEditor } from "./mcp-override-editor";
import { MCPSelectionStrip } from "./mcp-selection-strip";
import { MCPServerDeleteDialog } from "./mcp-server-delete-dialog";
import { MCPServerEditor } from "./mcp-server-editor";
import { isExtensionOwnedMCPServer } from "./mcp-server-labels";
import { MCPServersByRuntimeView } from "./mcp-servers-by-runtime-view";

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
          onAllow: runtime.allow,
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

/** Editor, extension override editor, sign-in, and remove confirmation for the managed list. */
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
      <MCPAuthorizeDialog
        authorize={management.authorization.authorize}
        scope={management.authorization.scope}
        server={management.authorization.server}
      />
      {management.runtime.copying ? (
        <MCPCopyToCompozyDialog
          canTurnOff={management.runtime.copying.snapshot.runtime_mcp.can_disable}
          failure={management.runtime.copyFailure}
          key={JSON.stringify([
            management.runtime.copying.server.runtime,
            management.runtime.copying.server.name,
          ])}
          onClose={management.runtime.closeCopy}
          onSubmit={management.runtime.submitCopy}
          profile={management.profile === "default" ? null : management.profile}
          runtimeName={management.runtime.copying.snapshot.display_name}
          server={management.runtime.copying.server}
          submitting={management.runtime.copySubmitting}
          workspaceId={management.workspaceId}
        />
      ) : null}
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
