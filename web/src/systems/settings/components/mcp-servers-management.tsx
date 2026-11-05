import { AlertCircle, Cable, Plus, SearchX } from "@compozy/ui/icons";

import { Button, Empty, SkeletonRegion } from "@compozy/ui";

import type { MCPServerManagement } from "../hooks/use-settings-mcp-page";
import { MCPAuthorizeDialog } from "./mcp-authorize-dialog";
import { MCPOverrideEditor } from "./mcp-override-editor";
import { MCPSelectionStrip } from "./mcp-selection-strip";
import { MCPServerDeleteDialog } from "./mcp-server-delete-dialog";
import { MCPServerEditor } from "./mcp-server-editor";
import { isExtensionOwnedMCPServer } from "./mcp-server-labels";
import { MCPServersTable, MCPServersTableSkeleton } from "./mcp-servers-table";

interface MCPServersManagementProps {
  management: MCPServerManagement;
}

/**
 * Selection strip, then loading · error · empty · query-empty · matrix, one at a time, from the
 * collection query. Shared by Settings › MCP servers and the Marketplace MCPs view.
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
  const { collection } = management;
  if (collection.isPending) {
    return (
      <SkeletonRegion data-testid="settings-page-mcp-loading" label="Loading MCP servers">
        <MCPServersTableSkeleton />
      </SkeletonRegion>
    );
  }
  if (collection.error && !collection.data) {
    return (
      <Empty
        action={
          <Button onClick={() => void collection.refetch()} size="sm" type="button">
            Try again
          </Button>
        }
        cause={collection.error.message}
        data-testid="settings-page-mcp-error"
        description="Check that CompozyOS is running, then try again."
        framed
        icon={AlertCircle}
        title="Couldn't load MCP servers"
        titleAs="h3"
      />
    );
  }
  if (management.servers.length === 0) {
    return (
      <Empty
        action={
          <Button
            data-testid="settings-page-mcp-empty-create"
            onClick={management.openCreate}
            size="sm"
            type="button"
          >
            <Plus aria-hidden="true" />
            Add MCP server
          </Button>
        }
        data-testid="settings-page-mcp-empty"
        description="Servers you add here, and the ones your extensions add, show up in this list."
        icon={Cable}
        title="No MCP servers yet"
      />
    );
  }
  if (management.filteredServers.length === 0) {
    return (
      <Empty
        action={
          <Button
            onClick={() => management.setQuery("")}
            size="sm"
            type="button"
            variant="secondary"
          >
            Clear search
          </Button>
        }
        data-testid="settings-page-mcp-query-empty"
        description={`Nothing matches "${management.query}".`}
        icon={SearchX}
        title="No MCP servers match"
      />
    );
  }
  return (
    <MCPServersTable
      onAuthorize={management.authorize}
      onEdit={management.edit}
      onSelect={management.select}
      selectedServer={management.selectedKey}
      servers={management.filteredServers}
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
