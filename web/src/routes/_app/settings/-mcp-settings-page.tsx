import { AlertCircle, Cable, Plus, SearchX } from "lucide-react";

import {
  Button,
  Empty,
  ListingToolbar,
  NativeSelect,
  NativeSelectOption,
  HelpTip,
  PillGroup,
  SkeletonRows,
} from "@compozy/ui";

import { useSettingsMCPPage } from "@/systems/settings/hooks/use-settings-mcp-page";
import {
  authorizeLabel,
  isExtensionOwnedMCPServer,
  MCPAuthorizeDialog,
  MCPOverrideEditor,
  MCPSelectionStrip,
  MCPServerDeleteDialog,
  MCPServerEditor,
  MCPServersTable,
  SettingsPageFrame,
  useSettingsTopbar,
  type SettingsMCPServerEntry,
} from "@/systems/settings";

type MCPScopeLane = "user" | "workspace";

/**
 * Settings › MCP servers: the canonical management location for every MCP definition the daemon
 * resolves — hand-configured servers and the ones extensions provide, listed under their owner so
 * same-name definitions coexist visibly. The scope lane follows the shell's profile lens: Personal
 * (or the acting profile) versus one project; rows still act on the scope the daemon returned them
 * from, never on the lane selected here.
 */
export function MCPSettingsPage() {
  const page = useSettingsMCPPage();
  useSettingsTopbar("mcp", {
    actions: (
      <Button
        data-testid="settings-page-mcp-create"
        onClick={page.openCreate}
        size="sm"
        type="button"
      >
        <Plus aria-hidden="true" className="size-3" />
        Add MCP server
      </Button>
    ),
  });

  const needingAuthorization = page.servers.filter(
    server => authorizeLabel(server) !== null
  ).length;
  const fromExtensions = page.servers.filter(isExtensionOwnedMCPServer).length;
  const selected = page.selected;

  return (
    <SettingsPageFrame
      meta={[
        {
          key: "servers",
          content: (
            <span data-testid="settings-page-mcp-count">
              {page.servers.length} {page.servers.length === 1 ? "server" : "servers"}
            </span>
          ),
        },
        ...(fromExtensions > 0
          ? [
              {
                key: "extensions",
                content: (
                  <span data-testid="settings-page-mcp-extension-count">
                    {fromExtensions} from extensions
                  </span>
                ),
              },
            ]
          : []),
        ...(needingAuthorization > 0
          ? [
              {
                key: "authorization",
                content: (
                  <span data-testid="settings-page-mcp-needs-authorization">
                    {needingAuthorization} {needingAuthorization === 1 ? "needs" : "need"} sign-in
                  </span>
                ),
              },
            ]
          : []),
      ]}
      restart={page.page.restart}
      slug="mcp"
      width="wide"
    >
      <div className="flex flex-col gap-3">
        <ListingToolbar data-testid="settings-page-mcp-toolbar">
          <ListingToolbar.Leading>
            <MCPScopeSelector page={page} />
            <ListingToolbar.Search
              aria-label="Search MCP servers"
              data-testid="settings-page-mcp-search"
              kbd={null}
              onChange={page.setQuery}
              placeholder="Search servers"
              value={page.query}
            />
          </ListingToolbar.Leading>
          <ListingToolbar.Trailing>
            <HelpTip label="About MCP servers">
              Servers you add are edited in place. For servers an extension adds, your edits are
              saved on top of the extension.
            </HelpTip>
          </ListingToolbar.Trailing>
        </ListingToolbar>
        {selected ? (
          <MCPSelectionStrip
            onEdit={page.edit}
            onRemove={isExtensionOwnedMCPServer(selected) ? undefined : page.requestRemove}
            server={selected}
          />
        ) : null}
        <MCPServersBody
          onAuthorize={page.authorize}
          onEdit={page.edit}
          onSelect={page.select}
          page={page}
        />
      </div>

      <MCPSettingsDialogs page={page} />
    </SettingsPageFrame>
  );
}

type MCPPageModel = ReturnType<typeof useSettingsMCPPage>;

/** Loading · error · empty · query-empty · matrix, one at a time, from the collection query. */
function MCPServersBody({
  page,
  onSelect,
  onEdit,
  onAuthorize,
}: {
  page: MCPPageModel;
  onSelect: (entry: SettingsMCPServerEntry) => void;
  onEdit: (entry: SettingsMCPServerEntry) => void;
  onAuthorize: (entry: SettingsMCPServerEntry) => void;
}) {
  const { collection } = page;
  if (collection.isPending) {
    return (
      <div
        aria-busy="true"
        className="rounded-lg border border-line bg-canvas-soft p-3.5"
        data-testid="settings-page-mcp-loading"
        role="status"
      >
        <SkeletonRows count={3} rowClassName="border-t border-line-soft py-3 first:border-t-0" />
        <span className="sr-only">Loading MCP servers</span>
      </div>
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
  if (page.servers.length === 0) {
    return (
      <Empty
        action={
          <Button
            data-testid="settings-page-mcp-empty-create"
            onClick={page.openCreate}
            size="sm"
            type="button"
            variant="neutral"
          >
            <Plus aria-hidden="true" className="size-3" />
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
  if (page.filteredServers.length === 0) {
    return (
      <Empty
        action={
          <Button onClick={() => page.setQuery("")} size="sm" type="button" variant="outline">
            Clear search
          </Button>
        }
        data-testid="settings-page-mcp-query-empty"
        description={`Nothing matches "${page.query}".`}
        icon={SearchX}
        title="No MCP servers match"
      />
    );
  }
  return (
    <MCPServersTable
      onAuthorize={onAuthorize}
      onEdit={onEdit}
      onSelect={onSelect}
      selectedServer={page.selectedKey}
      servers={page.filteredServers}
    />
  );
}

function MCPScopeSelector({ page }: { page: MCPPageModel }) {
  const lane: MCPScopeLane = page.workspaceId ? "workspace" : "user";
  const personalLabel = page.profile === "default" ? "Personal" : `Profile · ${page.profile}`;
  const firstWorkspace = page.workspaces[0];
  return (
    <div className="flex flex-wrap items-center gap-2.5" data-testid="settings-page-mcp-scope">
      <PillGroup<MCPScopeLane>
        aria-label="Show servers for"
        items={[
          { value: "user", label: personalLabel, testId: "settings-page-mcp-scope-user" },
          {
            value: "workspace",
            label: "Project",
            disabled: page.workspaces.length === 0,
            testId: "settings-page-mcp-scope-workspace",
          },
        ]}
        onChange={next =>
          page.selectWorkspace(next === "user" ? null : (firstWorkspace?.id ?? null))
        }
        size="sm"
        value={lane}
      />
      {lane === "workspace" ? (
        <NativeSelect
          aria-label="Project"
          className="w-56"
          data-testid="settings-page-mcp-workspace"
          onChange={event => page.selectWorkspace(event.target.value || null)}
          value={page.workspaceId ?? ""}
        >
          {page.workspaces.map(workspace => (
            <NativeSelectOption key={workspace.id} value={workspace.id}>
              {workspace.name}
            </NativeSelectOption>
          ))}
        </NativeSelect>
      ) : null}
    </div>
  );
}

function MCPSettingsDialogs({ page }: { page: MCPPageModel }) {
  return (
    <>
      {page.editorProps ? (
        <MCPServerEditor
          {...page.editorProps}
          onRemove={
            page.editorProps.entry && !isExtensionOwnedMCPServer(page.editorProps.entry)
              ? () => {
                  const entry = page.editorProps?.entry;
                  page.editorProps?.onClose();
                  if (entry) page.requestRemove(entry);
                }
              : undefined
          }
        />
      ) : null}
      {page.overrideProps ? <MCPOverrideEditor {...page.overrideProps} /> : null}
      <MCPAuthorizeDialog
        authorize={page.authorization.authorize}
        scope={page.authorization.scope}
        server={page.authorization.server}
      />
      <MCPServerDeleteDialog
        error={page.removal.error?.message ?? null}
        isDeleting={page.removal.isPending}
        onClose={page.closeRemove}
        onConfirm={page.remove}
        target={page.removing}
      />
    </>
  );
}
