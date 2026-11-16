import type { Meta, StoryObj } from "@storybook/react-vite";
import { delay, HttpResponse } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import {
  StorybookRouteCanvas,
  StorybookWorkspaceSetup,
  appRouteParameters,
} from "@/storybook/route-story-meta";
import { MCPServerEditor } from "@/systems/settings/components/mcp-server-editor";
import { emptyDraft } from "@/systems/settings/lib/mcp-editor-model";
import {
  mcpManagementCollectionFixture,
  mcpOwnerCollectionFixture,
} from "@/systems/settings/mocks";

const meta: Meta<typeof StorybookRouteCanvas> = {
  title: "systems/settings/routes/McpServers",
  component: StorybookRouteCanvas,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Settings › MCP servers: the canonical management page. Manual and extension-provided definitions coexist under their owner with the composed status matrix, the sign-in panel beside the row, the manual editor and the extension override editor.",
      },
    },
  },
  // Sign in opens a browser tab by itself; the canvas keeps it from leaving the story.
  beforeEach: () => {
    const open = window.open;
    window.open = () => null;
    return () => {
      window.open = open;
    };
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const ROUTE = "/settings/mcp";

// The reference matrix (nine manual servers) plus the owner-aware rows from task_04.
const ownerMsw = storybookMswParameters({
  settings: [
    compozyApiMock.get("/api/settings/mcp-servers", () =>
      HttpResponse.json(mcpOwnerCollectionFixture)
    ),
    compozyApiMock.get("/api/vault/secrets", () =>
      HttpResponse.json({
        secrets: [
          {
            ref: "vault:mcp/ws/ws-platform/github-local/env/github_personal_access_token",
            namespace: "mcp",
            present: true,
            created_at: "2026-07-01T00:00:00Z",
            updated_at: "2026-07-01T00:00:00Z",
          },
        ],
      })
    ),
  ],
});

function ownerParams(path = ROUTE) {
  return { ...appRouteParameters(path), ...ownerMsw };
}

const EXTENSION_GITHUB_ROW = "settings-page-mcp-servers-row-github--github";
const MANUAL_GITHUB_ROW = "settings-page-mcp-servers-row-github";

async function clickExtensionGithubAuthorize(canvasElement: HTMLElement) {
  const canvas = within(canvasElement);
  const page = within(canvasElement.ownerDocument.body);
  await userEvent.click(await canvas.findByTestId(`${EXTENSION_GITHUB_ROW}-authorize`));
  return page;
}

async function openAuthorizeWaiting(canvasElement: HTMLElement) {
  const page = await clickExtensionGithubAuthorize(canvasElement);
  await page.findByTestId("mcp-sign-in-copy-link");
  return page;
}

async function finishByPaste(page: ReturnType<typeof within>, address: string) {
  await userEvent.click(page.getByTestId("mcp-sign-in-paste-toggle"));
  const input = await page.findByTestId("mcp-sign-in-paste-input");
  await waitFor(() => expect(input).toBeEnabled());
  await userEvent.type(input, address);
  await userEvent.click(page.getByTestId("mcp-sign-in-paste-submit"));
}

/** matrix-desktop / matrix-mobile: manual `github` and `extension:github` (runs as github.github) side by side. */
export const Matrix: Story = {
  args: {},
  parameters: ownerParams(),
  render: () => <StorybookWorkspaceSetup />,
};

/** Selecting a name picks that exact definition; the strip offers Delete for manual rows only. */
export const SelectedManualRow: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: ownerParams(),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId(`${MANUAL_GITHUB_ROW}-name`));
    await canvas.findByTestId("settings-page-mcp-selection-delete");
  },
};

/** An extension-provided row selected: no Delete — the server leaves with its extension. */
export const SelectedExtensionRow: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: ownerParams(),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId(`${EXTENSION_GITHUB_ROW}-name`));
    await canvas.findByTestId("settings-page-mcp-selection");
    await expect(canvas.queryByTestId("settings-page-mcp-selection-delete")).toBeNull();
  },
};

/** authorize-waiting-desktop / authorize-mobile: the extension's own token, not the manual one, in the panel beside the row. */
export const AuthorizeWaiting: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: ownerParams(),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const page = await openAuthorizeWaiting(canvasElement);
    await expect(page.getByTestId("mcp-sign-in-panel")).toHaveAttribute("data-phase", "waiting");
  },
};

/** authorize-manual-desktop */
export const AuthorizeManual: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: ownerParams(),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const page = await openAuthorizeWaiting(canvasElement);
    await userEvent.click(page.getByTestId("mcp-sign-in-paste-toggle"));
    const input = await page.findByTestId("mcp-sign-in-paste-input");
    await waitFor(() => expect(input).toBeEnabled());
  },
};

/** auth-failure-desktop */
export const AuthFailure: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: {
    ...appRouteParameters(ROUTE),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/mcp-servers", () =>
          HttpResponse.json(mcpOwnerCollectionFixture)
        ),
        compozyApiMock.get("/api/vault/secrets", () => HttpResponse.json({ secrets: [] })),
        // Exchange returns without a confirmed token -> the UI stays failed.
        compozyApiMock.post("/api/settings/mcp-servers/{name}/auth/exchange", () =>
          HttpResponse.json({
            server_name: "github",
            owner: "extension:github",
            scope: "user",
            status: "needs_login",
            token_present: false,
            refreshable: true,
          })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const page = await openAuthorizeWaiting(canvasElement);
    await finishByPaste(page, "http://127.0.0.1:2123/api/mcp/oauth/callback?code=rejected&state=x");
    await page.findByText("Sign-in didn't finish");
  },
};

/** auth-begin-failure-desktop */
export const AuthBeginFailure: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: {
    ...appRouteParameters(ROUTE),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/mcp-servers", () =>
          HttpResponse.json(mcpOwnerCollectionFixture)
        ),
        compozyApiMock.get("/api/vault/secrets", () => HttpResponse.json({ secrets: [] })),
        compozyApiMock.post("/api/settings/mcp-servers/{name}/auth/begin", () =>
          HttpResponse.json({ error: "OAuth provider unavailable" }, { status: 503 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const page = await clickExtensionGithubAuthorize(canvasElement);
    await page.findByText("Sign-in didn't finish");
    await page.findByTestId("mcp-sign-in-retry");
  },
};

/** authenticated-token-desktop */
export const Authenticated: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: ownerParams(),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const page = await openAuthorizeWaiting(canvasElement);
    await finishByPaste(page, "http://127.0.0.1:2123/api/mcp/oauth/callback?code=valid&state=x");
    await page.findByText("Connected to github");
  },
};

/** editor-stdio-desktop: Add MCP server opens the manual editor. */
export const EditorStdio: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: ownerParams(),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const page = within(canvasElement.ownerDocument.body);
    await userEvent.click(await canvas.findByTestId("settings-page-mcp-create"));
    await page.findByTestId("settings-mcp-editor-stdio");
  },
};

/** editor-stdio-incomplete-secret-desktop */
export const EditorStdioIncompleteSecret: Story = {
  args: {},
  render: () => (
    <div className="min-h-screen bg-canvas">
      <MCPServerEditor
        open
        mode="edit"
        draft={{
          ...emptyDraft("stdio"),
          name: "github-local",
          command: "npx",
          args: ["-y", "@modelcontextprotocol/server-github"],
          secretEnv: [
            {
              key: "GITHUB_PERSONAL_ACCESS_TOKEN",
              binding: { mode: "typed", existing: false, typedValue: "", vaultRef: "" },
            },
          ],
        }}
        scope="workspace"
        errors={{ secretEnv: { 0: "Enter a value or select a Vault reference" } }}
        isValid={false}
        isSaving={false}
        saveError={null}
        vaultInventory={{ status: "ready", refs: [] }}
        target="config"
        availableTargets={["config"]}
        entry={null}
        onChange={() => undefined}
        onTargetChange={() => undefined}
        onClose={() => undefined}
        onSave={() => undefined}
      />
    </div>
  ),
};

/** editor-http-desktop / remote-editor-mobile: a manual remote definition edited in place. */
export const EditorHttp: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: ownerParams(),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const page = within(canvasElement.ownerDocument.body);
    await userEvent.click(await canvas.findByTestId("settings-page-mcp-servers-row-linear-edit"));
    await page.findByTestId("settings-mcp-editor-remote");
  },
};

/**
 * override-editor-desktop: Edit on an extension-provided row opens the override editor — package
 * fields read-only, headers and endpoint editable, Reset override offered because one is stored.
 */
export const OverrideEditor: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: ownerParams(),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const page = within(canvasElement.ownerDocument.body);
    await userEvent.click(await canvas.findByTestId(`${EXTENSION_GITHUB_ROW}-edit`));
    await page.findByTestId("settings-mcp-override-editor");
    await page.findByTestId("settings-mcp-override-editor-reset");
    await expect(page.getByTestId("settings-mcp-override-editor-url")).toHaveValue("");
  },
};

/** loading-desktop */
export const Loading: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(ROUTE),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/mcp-servers", async () => {
          await delay("infinite");
          return HttpResponse.json(mcpManagementCollectionFixture);
        }),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** empty-desktop */
export const Empty: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(ROUTE),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/mcp-servers", () =>
          HttpResponse.json({ ...mcpManagementCollectionFixture, mcp_servers: [], scope: "user" })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};

/** query-empty-desktop */
export const QueryEmpty: Story = {
  args: {},
  tags: ["play-fn"],
  parameters: ownerParams(),
  render: () => <StorybookWorkspaceSetup />,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.type(await canvas.findByTestId("settings-page-mcp-search"), "tailscale");
    await canvas.findByTestId("settings-page-mcp-query-empty");
  },
};

/** error-desktop */
export const Error: Story = {
  args: {},
  parameters: {
    ...appRouteParameters(ROUTE),
    ...storybookMswParameters({
      settings: [
        compozyApiMock.get("/api/settings/mcp-servers", () =>
          HttpResponse.json({ error: "Failed to load MCP servers" }, { status: 500 })
        ),
      ],
    }),
  },
  render: () => <StorybookWorkspaceSetup />,
};
