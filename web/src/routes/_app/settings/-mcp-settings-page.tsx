import { Plus } from "@compozy/ui/icons";

import {
  Button,
  ListingToolbar,
  NativeSelect,
  NativeSelectOption,
  HelpTip,
  PillGroup,
  SkeletonText,
} from "@compozy/ui";

import { useSettingsMCPPage } from "@/systems/settings/hooks/use-settings-mcp-page";
import {
  authorizeLabel,
  isExtensionOwnedMCPServer,
  MCPServersManagementBody,
  MCPServersManagementDialogs,
  SettingsPageFrame,
  useSettingsTopbar,
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
  useSettingsTopbar({
    actions: (
      <Button
        data-testid="settings-page-mcp-create"
        onClick={page.openCreate}
        size="sm"
        type="button"
        variant="secondary"
      >
        <Plus aria-hidden="true" />
        Add MCP server
      </Button>
    ),
  });

  const needingAuthorization = page.servers.filter(
    server => authorizeLabel(server) !== null
  ).length;
  const fromExtensions = page.servers.filter(isExtensionOwnedMCPServer).length;

  return (
    <SettingsPageFrame
      meta={[
        {
          key: "servers",
          content: page.collection.isPending ? (
            <SkeletonText className="w-16" />
          ) : (
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
        <MCPServersManagementBody management={page} />
      </div>

      <MCPServersManagementDialogs management={page} />
    </SettingsPageFrame>
  );
}

type MCPPageModel = ReturnType<typeof useSettingsMCPPage>;

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
